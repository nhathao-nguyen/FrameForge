"""Bounded provider retry, fallback and endpoint circuit breaking."""

from __future__ import annotations

import random
import threading
import time
from dataclasses import dataclass
from typing import Any, Callable, Sequence

from .contracts import (
    ASRRequest,
    EmbeddingRequest,
    LLMMessage,
    LLMRequest,
    ProviderCallContext,
    ProviderError,
    ProviderKind,
    ProviderRegistry,
    TTSRequest,
    VLMRequest,
)


@dataclass(frozen=True)
class RetryPolicy:
    max_attempts: int = 3
    base_delay_sec: float = 0.25
    max_delay_sec: float = 4.0
    jitter_ratio: float = 0.2

    def validate(self) -> None:
        if self.max_attempts < 1 or self.max_attempts > 10:
            raise ValueError("provider retry attempts are out of range")
        if self.base_delay_sec < 0 or self.max_delay_sec < self.base_delay_sec or self.max_delay_sec > 60:
            raise ValueError("provider retry delay is out of range")
        if self.jitter_ratio < 0 or self.jitter_ratio > 1:
            raise ValueError("provider retry jitter is out of range")


@dataclass(frozen=True)
class CircuitBreakerPolicy:
    failure_threshold: int = 3
    recovery_timeout_sec: float = 30.0

    def validate(self) -> None:
        if self.failure_threshold < 1 or self.failure_threshold > 100 or self.recovery_timeout_sec <= 0 or self.recovery_timeout_sec > 3600:
            raise ValueError("provider circuit-breaker policy is out of range")


@dataclass
class _CircuitState:
    failures: int = 0
    open_until: float = 0.0
    half_open_in_flight: bool = False


class ProviderResolver:
    """Resolve allowlisted typed providers inside one bounded node budget."""

    def __init__(
        self,
        registry: ProviderRegistry,
        retry_policy: RetryPolicy = RetryPolicy(),
        circuit_policy: CircuitBreakerPolicy = CircuitBreakerPolicy(),
        *,
        sleep: Callable[[float], None] = time.sleep,
        clock: Callable[[], float] = time.monotonic,
        random_value: Callable[[], float] = random.random,
    ) -> None:
        retry_policy.validate()
        circuit_policy.validate()
        self.registry = registry
        self.retry_policy = retry_policy
        self.circuit_policy = circuit_policy
        self._sleep = sleep
        self._clock = clock
        self._random = random_value
        self._circuits: dict[tuple[str, str, str], _CircuitState] = {}
        self._lock = threading.Lock()

    def resolve(self, kind: ProviderKind, adapter_keys: Sequence[str], request: Any, context: ProviderCallContext) -> Any:
        if not adapter_keys:
            raise ValueError("at least one provider is required")
        context.validate()
        started = self._clock()
        failures: list[ProviderError] = []
        for adapter_key in adapter_keys:
            provider = self.registry.resolve(kind, adapter_key)
            descriptor = provider.descriptor
            endpoint = getattr(descriptor, "endpoint_key", "") or f"{descriptor.adapter_key}:{descriptor.deployment}"
            circuit_key = (context.provider_configuration_id, endpoint, kind.value)
            if not self._acquire_circuit(circuit_key):
                failures.append(ProviderError("unavailable", adapter_key, safe_message="Provider circuit is open."))
                continue
            succeeded = False
            try:
                for attempt in range(1, self.retry_policy.max_attempts + 1):
                    if context.cancelled():
                        raise ProviderError("cancelled", adapter_key, safe_message="Provider call was cancelled.")
                    try:
                        result = self._invoke(provider, kind, request, context)
                        self._record_success(circuit_key)
                        succeeded = True
                        return result
                    except ProviderError as error:
                        failures.append(error)
                        self._record_failure(circuit_key, error)
                        if not error.retryable:
                            raise
                        if attempt >= self.retry_policy.max_attempts:
                            break
                        delay = self._retry_delay(attempt, error)
                        remaining = context.timeout_sec - (self._clock() - started)
                        if remaining <= 0 or delay >= remaining:
                            failures.append(ProviderError("timeout", adapter_key, safe_message="Provider retry budget was exhausted."))
                            break
                        self._sleep(delay)
            finally:
                if not succeeded:
                    self._release_half_open(circuit_key)
        last = failures[-1] if failures else ProviderError("unavailable", "provider", safe_message="No provider was available.")
        raise ProviderError(
            "fallback_exhausted",
            last.provider,
            retryable=False,
            safe_message="All approved provider fallbacks exhausted their bounded retry policy.",
            category=last.category,
            provider_request_id=last.provider_request_id,
        )

    def circuit_state(self, configuration_id: str, endpoint: str, kind: ProviderKind) -> str:
        key = (configuration_id, endpoint, kind.value)
        with self._lock:
            state = self._circuits.get(key)
            if state is None or state.failures == 0:
                return "closed"
            if state.open_until > self._clock():
                return "open"
            return "half_open"

    def _retry_delay(self, attempt: int, error: ProviderError) -> float:
        base = min(self.retry_policy.max_delay_sec, self.retry_policy.base_delay_sec * (2 ** (attempt - 1)))
        jitter = 1 + self.retry_policy.jitter_ratio * (2 * max(0.0, min(1.0, self._random())) - 1)
        delay = max(0.0, base * jitter)
        if error.retry_after_sec is not None:
            delay = max(delay, min(max(0.0, error.retry_after_sec), self.retry_policy.max_delay_sec))
        return float(min(delay, self.retry_policy.max_delay_sec))

    def _acquire_circuit(self, key: tuple[str, str, str]) -> bool:
        now = self._clock()
        with self._lock:
            state = self._circuits.setdefault(key, _CircuitState())
            if state.open_until > now:
                return False
            if state.failures >= self.circuit_policy.failure_threshold:
                if state.half_open_in_flight:
                    return False
                state.half_open_in_flight = True
            return True

    def _record_success(self, key: tuple[str, str, str]) -> None:
        with self._lock:
            self._circuits[key] = _CircuitState()

    def _record_failure(self, key: tuple[str, str, str], error: ProviderError) -> None:
        if not error.retryable:
            return
        with self._lock:
            state = self._circuits.setdefault(key, _CircuitState())
            state.failures += 1
            state.half_open_in_flight = False
            if state.failures >= self.circuit_policy.failure_threshold:
                state.open_until = self._clock() + self.circuit_policy.recovery_timeout_sec

    def _release_half_open(self, key: tuple[str, str, str]) -> None:
        with self._lock:
            state = self._circuits.get(key)
            if state is not None:
                state.half_open_in_flight = False

    @staticmethod
    def _invoke(provider: Any, kind: ProviderKind, request: Any, context: ProviderCallContext) -> Any:
        if kind is ProviderKind.LLM:
            typed = request if isinstance(request, LLMRequest) else LLMRequest("", (LLMMessage("user", str(request)),))
            return provider.complete(context, typed)
        if kind is ProviderKind.VLM:
            if not isinstance(request, VLMRequest):
                raise ProviderError("invalid_request", provider.descriptor.adapter_key, safe_message="VLM request is invalid.")
            return provider.analyze(context, request)
        if kind is ProviderKind.TTS:
            if not isinstance(request, TTSRequest):
                raise ProviderError("invalid_request", provider.descriptor.adapter_key, safe_message="TTS request is invalid.")
            return provider.synthesize(context, request)
        if kind is ProviderKind.ASR:
            if not isinstance(request, ASRRequest):
                raise ProviderError("invalid_request", provider.descriptor.adapter_key, safe_message="ASR request is invalid.")
            return provider.transcribe(context, request)
        if not isinstance(request, EmbeddingRequest):
            raise ProviderError("invalid_request", provider.descriptor.adapter_key, safe_message="Embedding request is invalid.")
        return provider.embed(context, request)


def resolve_with_fallback(
    registry: ProviderRegistry,
    kind: ProviderKind,
    adapter_keys: Sequence[str],
    request: Any,
    context: ProviderCallContext,
) -> Any:
    resolver = getattr(registry, "_bounded_resolver", None)
    if not isinstance(resolver, ProviderResolver):
        resolver = ProviderResolver(registry)
        setattr(registry, "_bounded_resolver", resolver)
    return resolver.resolve(kind, adapter_keys, request, context)


__all__ = ["CircuitBreakerPolicy", "ProviderResolver", "RetryPolicy", "resolve_with_fallback"]
