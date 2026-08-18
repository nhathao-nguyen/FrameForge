from __future__ import annotations

import pytest

from nh_media.providers import CircuitBreakerPolicy, ProviderResolver, RetryPolicy
from nh_media.providers.contracts import (
    FakeProvider,
    LLMRequest,
    LLMResponse,
    ProviderCallContext,
    ProviderError,
    ProviderKind,
    ProviderRegistry,
)


def context(timeout_sec: float = 30.0) -> ProviderCallContext:
    return ProviderCallContext("request_retry", "correlation_retry", "project_retry", "job_retry", "step_retry", "config_retry", 1, timeout_sec=timeout_sec)


class ScriptedProvider(FakeProvider):
    def __init__(self, adapter_key: str, failures: list[ProviderError]) -> None:
        super().__init__(ProviderKind.LLM, adapter_key)
        object.__setattr__(self.descriptor, "endpoint_key", adapter_key)
        self.failures = failures
        self.calls = 0

    def complete(self, call_context: ProviderCallContext, request: LLMRequest) -> LLMResponse:
        self.calls += 1
        if self.failures:
            raise self.failures.pop(0)
        return super().complete(call_context, request)


def test_transient_retry_uses_exponential_jitter_and_retry_after_before_fallback() -> None:
    first = ScriptedProvider(
        "retry-primary",
        [
            ProviderError("unavailable", "retry-primary"),
            ProviderError("rate_limited", "retry-primary", retry_after_sec=0.7),
        ],
    )
    fallback = ScriptedProvider("retry-fallback", [])
    delays: list[float] = []
    resolver = ProviderResolver(
        ProviderRegistry((first, fallback)),
        RetryPolicy(max_attempts=3, base_delay_sec=0.5, max_delay_sec=2.0, jitter_ratio=0.2),
        sleep=delays.append,
        random_value=lambda: 1.0,
    )
    response = resolver.resolve(ProviderKind.LLM, ("retry-primary", "retry-fallback"), LLMRequest("", ()), context())
    assert response.meta.adapter_key == "retry-primary"
    assert first.calls == 3 and fallback.calls == 0
    assert delays == pytest.approx([0.6, 1.2])


def test_permanent_error_is_not_retried_or_fallback_masked() -> None:
    provider = ScriptedProvider("auth-primary", [ProviderError("auth_failed", "auth-primary")])
    fallback = ScriptedProvider("auth-fallback", [])
    resolver = ProviderResolver(ProviderRegistry((provider, fallback)), sleep=lambda _delay: None)
    with pytest.raises(ProviderError) as raised:
        resolver.resolve(ProviderKind.LLM, ("auth-primary", "auth-fallback"), LLMRequest("", ()), context())
    assert raised.value.code == "auth_failed"
    assert provider.calls == 1 and fallback.calls == 0


def test_circuit_is_keyed_by_configuration_endpoint_and_capability_and_recovers_half_open() -> None:
    now = [0.0]
    primary = ScriptedProvider("circuit-primary", [ProviderError("timeout", "circuit-primary")])
    fallback = ScriptedProvider("circuit-fallback", [])
    resolver = ProviderResolver(
        ProviderRegistry((primary, fallback)),
        RetryPolicy(max_attempts=1),
        CircuitBreakerPolicy(failure_threshold=1, recovery_timeout_sec=5),
        sleep=lambda _delay: None,
        clock=lambda: now[0],
    )
    response = resolver.resolve(ProviderKind.LLM, ("circuit-primary", "circuit-fallback"), LLMRequest("", ()), context())
    assert response.meta.adapter_key == "circuit-fallback"
    assert resolver.circuit_state("config_retry", "circuit-primary", ProviderKind.LLM) == "open"
    resolver.resolve(ProviderKind.LLM, ("circuit-primary", "circuit-fallback"), LLMRequest("", ()), context())
    assert primary.calls == 1
    now[0] = 6.0
    response = resolver.resolve(ProviderKind.LLM, ("circuit-primary",), LLMRequest("", ()), context())
    assert response.meta.adapter_key == "circuit-primary"
    assert resolver.circuit_state("config_retry", "circuit-primary", ProviderKind.LLM) == "closed"
