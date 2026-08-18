"""Typed, provider-neutral Gate G contracts and deterministic adapters.

The product/domain layer talks to one of the five capability ports below.  The
legacy ``call`` compatibility method on :class:`FakeProvider` is intentionally
limited to old fixtures; production node code uses the typed methods.
"""

from __future__ import annotations

import hashlib
import io
import math
import struct
import time
import wave
from dataclasses import asdict, dataclass, field, is_dataclass
from enum import StrEnum
from typing import Any, Callable, Iterable, Mapping, Protocol, Sequence


JsonObject = dict[str, Any]


def canonical_json(value: Any) -> str:
    import json

    def make_jsonable(item: Any) -> Any:
        if is_dataclass(item):
            return make_jsonable(asdict(item))  # type: ignore[arg-type]
        if isinstance(item, StrEnum):
            return item.value
        if isinstance(item, Mapping):
            return {str(key): make_jsonable(val) for key, val in item.items()}
        if isinstance(item, (tuple, list)):
            return [make_jsonable(val) for val in item]
        if isinstance(item, bytes):
            return hashlib.sha256(item).hexdigest()
        return item

    return json.dumps(make_jsonable(value), ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def sha256_json(value: Any) -> str:
    return hashlib.sha256(canonical_json(value).encode("utf-8")).hexdigest()


class ProviderKind(StrEnum):
    LLM = "llm"
    VLM = "vlm"
    TTS = "tts"
    ASR = "asr"
    EMBEDDING = "embedding"


class ProviderErrorCode(StrEnum):
    TIMEOUT = "timeout"
    RATE_LIMITED = "rate_limited"
    UNAVAILABLE = "unavailable"
    AUTH_FAILED = "auth_failed"
    INVALID_REQUEST = "invalid_request"
    INVALID_RESPONSE = "invalid_response"
    CONTENT_REJECTED = "content_rejected"
    UNSUPPORTED_CAPABILITY = "unsupported_capability"
    CANCELLED = "cancelled"
    INTERNAL = "internal"


class ProviderErrorCategory(StrEnum):
    TRANSIENT = "transient"
    PERMANENT = "permanent"
    POLICY = "policy"
    CANCELLED = "cancelled"


_CODE_ALIASES = {"rate_limit": "rate_limited", "provider_unavailable": "unavailable", "provider_not_allowed": "unsupported_capability"}


class ProviderError(Exception):
    """Safe, normalized provider failure; SDK exceptions never cross the port."""

    def __init__(
        self,
        code: str,
        provider: str,
        retryable: bool | None = None,
        safe_message: str = "Provider request failed.",
        category: str | None = None,
        provider_request_id: str | None = None,
        retry_after_sec: float | None = None,
        *,
        cause_category: str | None = None,
    ) -> None:
        normalized_code = _CODE_ALIASES.get(code, code)
        if normalized_code not in {item.value for item in ProviderErrorCode} and normalized_code != "fallback_exhausted":
            normalized_code = ProviderErrorCode.INTERNAL.value
        if cause_category is not None and category is None:
            category = cause_category
        if category is None:
            category = self._default_category(normalized_code)
        if retryable is None:
            retryable = normalized_code in {ProviderErrorCode.TIMEOUT.value, ProviderErrorCode.RATE_LIMITED.value, ProviderErrorCode.UNAVAILABLE.value}
        if normalized_code in {ProviderErrorCode.AUTH_FAILED.value, ProviderErrorCode.INVALID_REQUEST.value, ProviderErrorCode.INVALID_RESPONSE.value, ProviderErrorCode.CONTENT_REJECTED.value, ProviderErrorCode.UNSUPPORTED_CAPABILITY.value, ProviderErrorCode.CANCELLED.value}:
            retryable = False
        self.code = normalized_code
        self.provider = provider
        self.retryable = bool(retryable)
        self.safe_message = safe_message[:512]
        self.category = category
        self.cause_category = category  # compatibility with the original fixture contract
        self.provider_request_id = provider_request_id
        self.retry_after_sec = retry_after_sec
        super().__init__(self.safe_message)

    @staticmethod
    def _default_category(code: str) -> str:
        if code == ProviderErrorCode.CANCELLED.value:
            return ProviderErrorCategory.CANCELLED.value
        if code in {ProviderErrorCode.AUTH_FAILED.value, ProviderErrorCode.INVALID_REQUEST.value, ProviderErrorCode.INVALID_RESPONSE.value, ProviderErrorCode.CONTENT_REJECTED.value, ProviderErrorCode.UNSUPPORTED_CAPABILITY.value}:
            return ProviderErrorCategory.POLICY.value if code in {ProviderErrorCode.CONTENT_REJECTED.value, ProviderErrorCode.UNSUPPORTED_CAPABILITY.value} else ProviderErrorCategory.PERMANENT.value
        return ProviderErrorCategory.TRANSIENT.value

    def as_dict(self) -> JsonObject:
        return {
            "code": self.code,
            "provider": self.provider,
            "category": self.category,
            "retryable": self.retryable,
            "safe_message": self.safe_message,
            "provider_request_id": self.provider_request_id,
            "retry_after_sec": self.retry_after_sec,
        }


@dataclass(frozen=True)
class ProviderCallContext:
    request_id: str
    correlation_id: str
    project_ref: str
    job_id: str
    job_step_id: str
    provider_configuration_id: str
    configuration_revision: int
    credential_ref: str | None = None
    timeout_sec: float = 30.0
    locale: str = "en"
    idempotency_key: str = ""
    privacy_policy: str = "local_only"
    cost_budget: float | None = None
    cancelled: Callable[[], bool] = lambda: False

    def validate(self) -> None:
        if not all((self.request_id, self.correlation_id, self.project_ref, self.job_id, self.job_step_id, self.provider_configuration_id)):
            raise ValueError("provider call identity is required")
        if self.configuration_revision < 1 or self.timeout_sec <= 0:
            raise ValueError("provider call bounds are invalid")
        if self.credential_ref and ("=" in self.credential_ref or "://" in self.credential_ref or any(ch.isspace() for ch in self.credential_ref)):
            raise ValueError("credential_ref must be an opaque reference")
        if self.cost_budget is not None and (not math.isfinite(self.cost_budget) or self.cost_budget < 0):
            raise ValueError("cost budget is invalid")

    def safe_snapshot(self) -> JsonObject:
        return {
            "request_id": self.request_id,
            "correlation_id": self.correlation_id,
            "project_ref": self.project_ref,
            "job_id": self.job_id,
            "job_step_id": self.job_step_id,
            "provider_configuration_id": self.provider_configuration_id,
            "configuration_revision": self.configuration_revision,
            "timeout_sec": self.timeout_sec,
            "locale": self.locale,
            "idempotency_key": self.idempotency_key,
            "privacy_policy": self.privacy_policy,
            "cost_budget": self.cost_budget,
        }


@dataclass(frozen=True)
class ProviderDescriptor:
    adapter_key: str
    kind: ProviderKind
    adapter_version: str
    capabilities: tuple[str, ...]
    models: tuple[str, ...]
    deployment: str

    def validate(self) -> None:
        if not self.adapter_key or not self.adapter_version or not self.models or self.deployment not in {"local", "remote"}:
            raise ValueError("provider descriptor is incomplete")
        if any(not item or any(ch.isspace() for ch in item) for item in (self.adapter_key, self.adapter_version, *self.models)):
            raise ValueError("provider descriptor contains an invalid identifier")


@dataclass(frozen=True)
class ProviderResultMeta:
    adapter_key: str
    adapter_version: str
    kind: ProviderKind
    configuration_id: str
    configuration_revision: int
    model: str
    provider_request_id: str
    latency_ms: int
    usage: Mapping[str, int] = field(default_factory=dict)
    estimated_cost: float | None = None
    warnings: tuple[str, ...] = ()

    def as_dict(self) -> JsonObject:
        return {
            "adapter_key": self.adapter_key,
            "adapter_version": self.adapter_version,
            "kind": self.kind.value,
            "provider_configuration_id": self.configuration_id,
            "provider_configuration_revision": self.configuration_revision,
            "model": self.model,
            "provider_request_id": self.provider_request_id,
            "latency_ms": self.latency_ms,
            "usage": dict(self.usage),
            "estimated_cost": self.estimated_cost,
            "warnings": list(self.warnings),
        }


@dataclass(frozen=True)
class LLMMessage:
    role: str
    content: str
    trusted: bool = False


@dataclass(frozen=True)
class LLMRequest:
    system_instructions: str
    messages: tuple[LLMMessage, ...]
    response_schema: JsonObject | None = None
    temperature: float | None = None
    max_output_tokens: int | None = None
    language: str | None = None
    safety_policy: str = "default"
    metadata: Mapping[str, str] = field(default_factory=dict)


@dataclass(frozen=True)
class LLMResponse:
    text: str
    structured_output: JsonObject | None
    finish_reason: str
    usage: Mapping[str, int]
    meta: ProviderResultMeta


@dataclass(frozen=True)
class VLMInput:
    input_id: str
    artifact_ref: str
    media_type: str
    scene_id: str
    source_revision: str
    timestamp_sec: float | None = None


@dataclass(frozen=True)
class VLMRequest:
    inputs: tuple[VLMInput, ...]
    prompt: str
    model: str | None = None
    response_schema: JsonObject | None = None
    language: str | None = None
    sampling_policy: Mapping[str, Any] = field(default_factory=dict)


@dataclass(frozen=True)
class VLMDescription:
    input_id: str
    text: str
    entities: tuple[str, ...]
    actions: tuple[str, ...] = ()
    location: str | None = None
    emotion: str | None = None
    confidence: float | None = None


@dataclass(frozen=True)
class VLMResponse:
    descriptions: tuple[VLMDescription, ...]
    structured_output: JsonObject | None
    meta: ProviderResultMeta


@dataclass(frozen=True)
class VoiceDescriptor:
    provider_voice_id: str
    language: str
    name: str
    capabilities: tuple[str, ...] = ()


@dataclass(frozen=True)
class TTSRequest:
    text: str
    language: str
    provider_voice_id: str
    voice_snapshot: Mapping[str, Any]
    model: str | None = None
    speaking_rate: float = 1.0
    pitch: float | None = None
    style_prompt: str = ""
    audio_format: str = "wav"
    sample_rate: int | None = None
    channels: int | None = None


@dataclass(frozen=True)
class ProducedBlob:
    kind: str
    role: str
    data: bytes
    mime_type: str
    provenance: JsonObject

    @property
    def sha256(self) -> str:
        return hashlib.sha256(self.data).hexdigest()

    @property
    def size_bytes(self) -> int:
        return len(self.data)


@dataclass(frozen=True)
class TTSResponse:
    audio_blob: ProducedBlob
    duration_sec: float | None
    timing_data: JsonObject | None
    meta: ProviderResultMeta


@dataclass(frozen=True)
class ASRRequest:
    audio_artifact_ref: str
    language: str | None = None
    model: str | None = None
    word_timestamps: bool = True
    diarization: bool = False
    vocabulary_hints: tuple[str, ...] = ()
    # Only deterministic tests may provide a fixture transcript.  Production
    # nodes leave these fields empty and the adapter reads the audio Artifact.
    fixture_text: str = ""
    fixture_duration_sec: float | None = None


@dataclass(frozen=True)
class ProviderTranscriptWord:
    text: str
    start_sec: float
    end_sec: float
    confidence: float = 1.0


@dataclass(frozen=True)
class ProviderTranscriptSegment:
    segment_id: str
    text: str
    start_sec: float
    end_sec: float
    confidence: float
    speaker: str | None = None
    words: tuple[ProviderTranscriptWord, ...] = ()


@dataclass(frozen=True)
class ProviderTranscript:
    language: str
    model: str
    duration_sec: float
    segments: tuple[ProviderTranscriptSegment, ...]
    meta: ProviderResultMeta


@dataclass(frozen=True)
class EmbeddingItem:
    item_id: str
    modality: str
    text: str | None = None
    artifact_ref: str | None = None


@dataclass(frozen=True)
class EmbeddingRequest:
    model: str
    modality: str
    items: tuple[EmbeddingItem, ...]
    normalize: bool = True


@dataclass(frozen=True)
class EmbeddingVector:
    item_id: str
    values: tuple[float, ...]


@dataclass(frozen=True)
class EmbeddingResponse:
    dimension: int
    metric: str
    model: str
    normalized: bool
    vectors: tuple[EmbeddingVector, ...]
    item_index: tuple[str, ...]
    vector_artifact: ProducedBlob | None
    meta: ProviderResultMeta


class LLMProvider(Protocol):
    descriptor: ProviderDescriptor

    def complete(self, context: ProviderCallContext, request: LLMRequest) -> LLMResponse: ...


class VLMProvider(Protocol):
    descriptor: ProviderDescriptor

    def analyze(self, context: ProviderCallContext, request: VLMRequest) -> VLMResponse: ...


class TTSProvider(Protocol):
    descriptor: ProviderDescriptor

    def synthesize(self, context: ProviderCallContext, request: TTSRequest) -> TTSResponse: ...


class ASRProvider(Protocol):
    descriptor: ProviderDescriptor

    def transcribe(self, context: ProviderCallContext, request: ASRRequest) -> ProviderTranscript: ...


class EmbeddingProvider(Protocol):
    descriptor: ProviderDescriptor

    def embed(self, context: ProviderCallContext, request: EmbeddingRequest) -> EmbeddingResponse: ...


def _wav_for_text(text: str, duration_sec: float, sample_rate: int = 16_000) -> bytes:
    frames = max(1, int(round(duration_sec * sample_rate)))
    frequency = 220 + int(hashlib.sha256(text.encode("utf-8")).hexdigest()[:4], 16) % 160
    pcm = bytearray()
    for index in range(frames):
        value = int(1800 * math.sin(2 * math.pi * frequency * index / sample_rate))
        pcm.extend(struct.pack("<h", value))
    output = io.BytesIO()
    with wave.open(output, "wb") as wav_file:
        wav_file.setnchannels(1)
        wav_file.setsampwidth(2)
        wav_file.setframerate(sample_rate)
        wav_file.writeframes(bytes(pcm))
    return output.getvalue()


def _vector(seed: str, dimension: int, normalize: bool) -> tuple[float, ...]:
    values = tuple((int.from_bytes(hashlib.sha256(f"{seed}:{index}".encode()).digest()[:4], "big") / 2**32) * 2 - 1 for index in range(dimension))
    if not normalize:
        return values
    length = math.sqrt(sum(item * item for item in values)) or 1.0
    return tuple(item / length for item in values)


class FakeProvider:
    """Deterministic adapter used by conformance and local acceptance only."""

    def __init__(self, kind: ProviderKind, adapter_key: str | None = None, model: str = "fake-v1", fail_code: str | None = None, delay_sec: float = 0.0) -> None:
        self.descriptor = ProviderDescriptor(adapter_key or f"fake-{kind.value}", kind, "2.0.0", ("deterministic", "cancel", "timeout", "structured_output"), (model,), "local")
        self.model = model
        self.fail_code = fail_code
        self.delay_sec = max(0.0, delay_sec)

    def _begin(self, context: ProviderCallContext) -> float:
        context.validate()
        if context.cancelled():
            raise ProviderError("cancelled", self.descriptor.adapter_key, safe_message="Provider call was cancelled.")
        if self.delay_sec > context.timeout_sec:
            raise ProviderError("timeout", self.descriptor.adapter_key, safe_message="Provider call timed out.")
        if self.fail_code:
            raise ProviderError(self.fail_code, self.descriptor.adapter_key, safe_message="Provider call failed under the configured test policy.")
        if self.delay_sec:
            time.sleep(self.delay_sec)
        if context.cancelled():
            raise ProviderError("cancelled", self.descriptor.adapter_key, safe_message="Provider call was cancelled.")
        return time.monotonic()

    def _meta(self, context: ProviderCallContext, started: float, request: Any) -> ProviderResultMeta:
        return ProviderResultMeta(self.descriptor.adapter_key, self.descriptor.adapter_version, self.descriptor.kind, context.provider_configuration_id, context.configuration_revision, self.model, f"{self.descriptor.adapter_key}-{sha256_json(request)[:16]}", int((time.monotonic() - started) * 1000), {"input_items": len(getattr(request, "items", ()))})

    def complete(self, context: ProviderCallContext, request: LLMRequest) -> LLMResponse:
        if self.descriptor.kind is not ProviderKind.LLM:
            raise ProviderError("unsupported_capability", self.descriptor.adapter_key, safe_message="Adapter does not implement LLM.")
        started = self._begin(context)
        untrusted = next((item.content for item in request.messages if not item.trusted), "source")
        try:
            import json

            decoded = json.loads(untrusted)
        except (TypeError, ValueError):
            decoded = {}
        if '"target_language"' in untrusted and '"text"' in untrusted:
            text = "Deterministic translation."
        elif isinstance(decoded, dict) and decoded.get("query"):
            text = "Deterministic research."
        elif isinstance(decoded, dict) and decoded.get("summary"):
            text = str(decoded["summary"])[:120]
        else:
            text = f"Deterministic research for {untrusted[:120]}."
        return LLMResponse(text, {"text": text} if request.response_schema else None, "stop", {"input_tokens": len(untrusted.split()), "output_tokens": len(text.split())}, self._meta(context, started, request))

    def analyze(self, context: ProviderCallContext, request: VLMRequest) -> VLMResponse:
        if self.descriptor.kind is not ProviderKind.VLM:
            raise ProviderError("unsupported_capability", self.descriptor.adapter_key, safe_message="Adapter does not implement VLM.")
        started = self._begin(context)
        descriptions = tuple(VLMDescription(item.input_id, f"Deterministic description of {item.scene_id}.", (), (), None, None, 0.5) for item in request.inputs)
        return VLMResponse(descriptions, None, self._meta(context, started, request))

    def synthesize(self, context: ProviderCallContext, request: TTSRequest) -> TTSResponse:
        if self.descriptor.kind is not ProviderKind.TTS:
            raise ProviderError("unsupported_capability", self.descriptor.adapter_key, safe_message="Adapter does not implement TTS.")
        started = self._begin(context)
        # A deliberately slow, audible fixture keeps subtitle QA meaningful;
        # real adapters return their measured duration instead.
        duration = max(0.1, len(request.text.split()) * 1.5 / max(0.25, request.speaking_rate))
        sample_rate = request.sample_rate or 16_000
        blob = ProducedBlob("audio", "narration", _wav_for_text(request.text, duration, sample_rate), "audio/wav", {"provider_adapter": self.descriptor.adapter_key, "model": self.model})
        return TTSResponse(blob, duration, None, self._meta(context, started, request))

    def transcribe(self, context: ProviderCallContext, request: ASRRequest) -> ProviderTranscript:
        if self.descriptor.kind is not ProviderKind.ASR:
            raise ProviderError("unsupported_capability", self.descriptor.adapter_key, safe_message="Adapter does not implement ASR.")
        started = self._begin(context)
        segments: tuple[ProviderTranscriptSegment, ...]
        if request.fixture_text:
            duration = request.fixture_duration_sec or max(0.1, len(request.fixture_text.split()) * 0.35)
            words = request.fixture_text.split()
            step = duration / max(1, len(words))
            word_items = tuple(ProviderTranscriptWord(word, index * step, (index + 1) * step) for index, word in enumerate(words))
            segments = (ProviderTranscriptSegment("transcript_segment_001", request.fixture_text.strip(), 0.0, duration, 1.0, None, word_items),)
        else:
            duration = 0.0
            segments = tuple()
        return ProviderTranscript(request.language or "en", self.model, duration, segments, self._meta(context, started, request))

    def embed(self, context: ProviderCallContext, request: EmbeddingRequest) -> EmbeddingResponse:
        if self.descriptor.kind is not ProviderKind.EMBEDDING:
            raise ProviderError("unsupported_capability", self.descriptor.adapter_key, safe_message="Adapter does not implement embeddings.")
        started = self._begin(context)
        dimension = 16
        vectors = tuple(EmbeddingVector(item.item_id, _vector(f"{self.model}:{request.modality}:{item.text or item.artifact_ref or item.item_id}", dimension, request.normalize)) for item in request.items)
        return EmbeddingResponse(dimension, "cosine", self.model, request.normalize, vectors, tuple(item.item_id for item in request.items), None, self._meta(context, started, request))

    def call(self, request: JsonObject, context: ProviderCallContext) -> JsonObject:
        """Compatibility shim for pre-T500 fixtures; nodes must not use it."""
        kind = self.descriptor.kind
        response: Any
        if kind is ProviderKind.LLM:
            response = self.complete(context, LLMRequest("", (LLMMessage("user", str(request.get("untrusted_input", {}))),)))
            return {"text": response.text, "structured": response.structured_output or {}, "meta": response.meta}
        if kind is ProviderKind.VLM:
            response = self.analyze(context, VLMRequest((VLMInput(str(request.get("scene_id", "scene")), "keyframe_ref", "image/jpeg", str(request.get("scene_id", "scene")), "source_revision"),), "describe"))
            item = response.descriptions[0]
            return {"text": item.text, "entities": list(item.entities), "confidence": item.confidence, "meta": response.meta}
        if kind is ProviderKind.TTS:
            response = self.synthesize(context, TTSRequest(str(request.get("text", "")), str(request.get("language", "en")), str(request.get("voice", "voice")), {}))
            return {"audio_blob": response.audio_blob, "duration_sec": response.duration_sec, "meta": response.meta}
        if kind is ProviderKind.ASR:
            response = self.transcribe(context, ASRRequest(str(request.get("audio_artifact_ref", "audio_artifact")), str(request.get("language", "en")), str(request.get("model", self.model))))
            return {"segments": list(response.segments), "language": response.language, "meta": response.meta}
        response = self.embed(context, EmbeddingRequest(str(request.get("model", self.model)), "text", tuple(EmbeddingItem(str(item.get("id", "")), str(item.get("modality", "text")), str(item.get("text", ""))) for item in request.get("items", []) if isinstance(item, dict))))
        return {"vectors": response.vectors, "dimension": response.dimension, "model": response.model, "meta": response.meta}


class ProviderRegistry:
    """Explicit allowlist; no entry-point or arbitrary plugin loading."""

    def __init__(self, providers: Iterable[Any] = ()) -> None:
        self._providers: dict[tuple[ProviderKind, str], Any] = {}
        for provider in providers:
            self.register(provider)

    def register(self, provider: Any) -> None:
        provider.descriptor.validate()
        key = (provider.descriptor.kind, provider.descriptor.adapter_key)
        if key in self._providers:
            raise ValueError(f"provider already registered: {key[0].value}/{key[1]}")
        self._providers[key] = provider

    def resolve(self, kind: ProviderKind, adapter_key: str) -> Any:
        provider = self._providers.get((kind, adapter_key))
        if provider is None:
            raise ProviderError("unsupported_capability", adapter_key, retryable=False, safe_message="The requested provider is not allowlisted.", category="policy")
        return provider

    def descriptors(self) -> tuple[ProviderDescriptor, ...]:
        return tuple(sorted((provider.descriptor for provider in self._providers.values()), key=lambda item: (item.kind.value, item.adapter_key)))


def resolve_with_fallback(registry: ProviderRegistry, kind: ProviderKind, adapter_keys: Sequence[str], request: Any, context: ProviderCallContext) -> Any:
    if not adapter_keys:
        raise ValueError("at least one provider is required")
    errors: list[ProviderError] = []
    for adapter_key in adapter_keys:
        try:
            provider = registry.resolve(kind, adapter_key)
            if kind is ProviderKind.LLM:
                return provider.complete(context, request if isinstance(request, LLMRequest) else LLMRequest("", (LLMMessage("user", str(request)),)))
            if kind is ProviderKind.VLM:
                return provider.analyze(context, request)
            if kind is ProviderKind.TTS:
                return provider.synthesize(context, request)
            if kind is ProviderKind.ASR:
                return provider.transcribe(context, request)
            return provider.embed(context, request)
        except ProviderError as error:
            errors.append(error)
            if not error.retryable:
                raise
    last = errors[-1]
    raise ProviderError("internal", last.provider, retryable=False, safe_message="All approved provider fallbacks failed.", category="permanent", provider_request_id=last.provider_request_id)


def provider_conformance(registry: ProviderRegistry) -> dict[str, str]:
    """Run the shared typed success/provenance conformance surface for all kinds."""
    context = ProviderCallContext("request_001", "corr_001", "project_001", "job_001", "step_001", "config_001", 1)
    result: dict[str, str] = {}
    requests: dict[ProviderKind, Any] = {
        ProviderKind.LLM: LLMRequest("Return structured output.", (LLMMessage("user", "fixture", False),), {"type": "object"}),
        ProviderKind.VLM: VLMRequest((VLMInput("input_1", "artifact_keyframe_1", "image/jpeg", "scene_1", "source_revision_1"),), "Describe the scene."),
        ProviderKind.TTS: TTSRequest("fixture speech", "en", "voice_1", {"revision": 1}),
        ProviderKind.ASR: ASRRequest("artifact_audio_1", "en", "fake-asr-v1"),
        ProviderKind.EMBEDDING: EmbeddingRequest("fake-embedding-v1", "text", (EmbeddingItem("item_1", "text", "fixture"),)),
    }
    for kind, request in requests.items():
        descriptors = [item for item in registry.descriptors() if item.kind is kind]
        if not descriptors:
            raise AssertionError(f"missing provider kind: {kind.value}")
        provider = registry.resolve(kind, descriptors[0].adapter_key)
        response: Any
        meta: ProviderResultMeta
        if kind is ProviderKind.LLM:
            response = provider.complete(context, request)
            meta = response.meta
        elif kind is ProviderKind.VLM:
            response = provider.analyze(context, request)
            meta = response.meta
        elif kind is ProviderKind.TTS:
            response = provider.synthesize(context, request)
            meta = response.meta
            if not response.audio_blob.data:
                raise AssertionError("TTS provider returned an empty ProducedBlob")
        elif kind is ProviderKind.ASR:
            response = provider.transcribe(context, request)
            meta = response.meta
        else:
            response = provider.embed(context, request)
            meta = response.meta
            if not response.vectors:
                raise AssertionError("embedding provider returned no vectors")
        if meta.configuration_id != context.provider_configuration_id or meta.adapter_key != descriptors[0].adapter_key:
            raise AssertionError("provider provenance is incomplete")
        result[kind.value] = meta.provider_request_id
    return result


__all__ = [
    "ASRProvider", "ASRRequest", "EmbeddingItem", "EmbeddingProvider", "EmbeddingRequest", "EmbeddingResponse", "EmbeddingVector",
    "FakeProvider", "LLMMessage", "LLMProvider", "LLMRequest", "LLMResponse", "ProducedBlob", "ProviderCallContext", "ProviderDescriptor",
    "ProviderError", "ProviderErrorCategory", "ProviderErrorCode", "ProviderKind", "ProviderRegistry", "ProviderResultMeta", "ProviderTranscript",
    "ProviderTranscriptSegment", "ProviderTranscriptWord", "TTSProvider", "TTSRequest", "TTSResponse", "VLMDescription", "VLMInput", "VLMProvider",
    "VLMRequest", "VLMResponse", "VoiceDescriptor", "canonical_json", "provider_conformance", "resolve_with_fallback", "sha256_json",
]
