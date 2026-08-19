"""Real local and remote provider adapters for the provider-neutral ports.

The adapters deliberately depend on the standard library for HTTP.  Optional
heavy runtimes are imported only when their adapter is selected, so the fake
conformance path remains deterministic and lightweight.  Credentials are
resolved from an allow-listed environment variable name and never from a job
command or provider response.
"""

from __future__ import annotations

import base64
import hashlib
import io
import json
import math
import os
import re
import tempfile
import time
import wave
from dataclasses import asdict, is_dataclass
from pathlib import Path
from typing import Any, Mapping
from urllib import error as urlerror
from urllib import request as urlrequest

from .contracts import (
    ASRRequest,
    EmbeddingRequest,
    EmbeddingResponse,
    EmbeddingVector,
    LLMRequest,
    LLMResponse,
    ProviderCallContext,
    ProviderDescriptor,
    ProviderError,
    ProviderKind,
    ProviderResultMeta,
    ProducedBlob,
    ProviderTranscript,
    ProviderTranscriptSegment,
    ProviderTranscriptWord,
    TTSRequest,
    TTSResponse,
    VLMDescription,
    VLMRequest,
    VLMResponse,
)


def _safe_id(value: str) -> str:
    return hashlib.sha256(value.encode("utf-8", "replace")).hexdigest()[:16]


def _jsonable(value: Any) -> Any:
    if is_dataclass(value):
        return _jsonable(asdict(value))
    if isinstance(value, bytes):
        return {"sha256": hashlib.sha256(value).hexdigest(), "size_bytes": len(value)}
    if isinstance(value, Mapping):
        return {str(key): _jsonable(item) for key, item in value.items()}
    if isinstance(value, (list, tuple)):
        return [_jsonable(item) for item in value]
    return value


def _context_check(context: ProviderCallContext) -> None:
    context.validate()
    if context.cancelled():
        raise ProviderError("cancelled", "real-provider", safe_message="Provider call was cancelled.")


def _model(spec: Mapping[str, Any], default: str) -> str:
    value = str(spec.get("model", default)).strip()
    if not value or any(char in value for char in ("\r", "\n")):
        raise ValueError("provider model is invalid")
    return value


def _endpoint(spec: Mapping[str, Any], default: str) -> str:
    value = str(spec.get("endpoint", default)).strip().rstrip("/")
    if not value.startswith(("http://", "https://")) or any(char in value for char in ("\r", "\n")):
        raise ValueError("provider endpoint is invalid")
    return value


def _url(endpoint: str, suffix: str) -> str:
    return endpoint if endpoint.endswith(suffix) else endpoint + "/" + suffix.lstrip("/")


def _credential(spec: Mapping[str, Any], *, required: bool) -> str:
    key_ref = str(spec.get("key_ref", "")).strip()
    if not key_ref:
        key_ref = "env:OPENAI_API_KEY"
    if not key_ref.startswith("env:"):
        raise ProviderError("auth_failed", "real-provider", safe_message="The configured provider credential reference is not allowed.")
    env_name = key_ref[4:]
    if not re.fullmatch(r"[A-Z][A-Z0-9_]{2,127}", env_name):
        raise ProviderError("auth_failed", "real-provider", safe_message="The configured provider credential reference is invalid.")
    value = os.environ.get(env_name, "")
    if required and not value:
        raise ProviderError("auth_failed", "real-provider", safe_message="The configured provider credential is unavailable.")
    return value


def _error_code(status: int) -> str:
    if status in {401, 403}:
        return "auth_failed"
    if status in {408, 409, 425, 429}:
        return "rate_limited" if status == 429 else "timeout"
    if 500 <= status <= 599:
        return "unavailable"
    if 400 <= status <= 499:
        return "invalid_request"
    return "internal"


def _request_json(
    context: ProviderCallContext,
    provider: str,
    url: str,
    body: Mapping[str, Any],
    headers: Mapping[str, str] | None = None,
) -> tuple[Any, Mapping[str, str], int]:
    _context_check(context)
    encoded = json.dumps(_jsonable(body), ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    request = urlrequest.Request(
        url,
        data=encoded,
        method="POST",
        headers={"Content-Type": "application/json", **dict(headers or {})},
    )
    started = time.monotonic()
    try:
        with urlrequest.urlopen(request, timeout=context.timeout_sec) as response:  # nosec B310 - endpoint is policy configured
            raw = response.read()
            status = int(response.status)
            response_headers = dict(response.headers.items())
    except urlerror.HTTPError as exc:
        raise ProviderError(_error_code(int(exc.code)), provider, safe_message="Provider HTTP request failed.") from exc
    except (urlerror.URLError, TimeoutError, OSError) as exc:
        if context.cancelled():
            raise ProviderError("cancelled", provider, safe_message="Provider call was cancelled.") from exc
        raise ProviderError("timeout", provider, safe_message="Provider request timed out or was unreachable.") from exc
    if time.monotonic() - started > context.timeout_sec:
        raise ProviderError("timeout", provider, safe_message="Provider request timed out.")
    try:
        return json.loads(raw.decode("utf-8")), response_headers, status
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ProviderError("invalid_response", provider, safe_message="Provider returned malformed JSON.") from exc


def _request_bytes(
    context: ProviderCallContext,
    provider: str,
    url: str,
    body: bytes,
    headers: Mapping[str, str],
) -> tuple[bytes, Mapping[str, str], int]:
    _context_check(context)
    request = urlrequest.Request(url, data=body, method="POST", headers=dict(headers))
    try:
        with urlrequest.urlopen(request, timeout=context.timeout_sec) as response:  # nosec B310 - endpoint is policy configured
            return response.read(), dict(response.headers.items()), int(response.status)
    except urlerror.HTTPError as exc:
        raise ProviderError(_error_code(int(exc.code)), provider, safe_message="Provider HTTP request failed.") from exc
    except (urlerror.URLError, TimeoutError, OSError) as exc:
        raise ProviderError("timeout", provider, safe_message="Provider request timed out or was unreachable.") from exc


def _meta(
    descriptor: ProviderDescriptor,
    context: ProviderCallContext,
    model: str,
    started: float,
    request: Any,
    headers: Mapping[str, str] | None = None,
    usage: Mapping[str, int] | None = None,
) -> ProviderResultMeta:
    provider_request_id = str((headers or {}).get("x-request-id", ""))[:128]
    if not provider_request_id:
        provider_request_id = f"{descriptor.adapter_key}-{_safe_id(json.dumps(_jsonable(request), sort_keys=True))}"
    return ProviderResultMeta(
        descriptor.adapter_key,
        descriptor.adapter_version,
        descriptor.kind,
        context.provider_configuration_id,
        context.configuration_revision,
        model,
        provider_request_id,
        int((time.monotonic() - started) * 1000),
        {str(key): int(value) for key, value in dict(usage or {}).items() if isinstance(value, (int, float))},
    )


def _object(value: Any, provider: str) -> Mapping[str, Any]:
    if not isinstance(value, Mapping):
        raise ProviderError("invalid_response", provider, safe_message="Provider returned an unexpected object.")
    return value


def _content(value: Any) -> str:
    if isinstance(value, str):
        return value
    if isinstance(value, list):
        return "".join(str(item.get("text", "")) if isinstance(item, Mapping) else str(item) for item in value)
    return str(value or "")


def _structured(text: str) -> dict[str, Any] | None:
    candidate = text.strip()
    if candidate.startswith("```"):
        candidate = re.sub(r"^```(?:json)?\s*|\s*```$", "", candidate, flags=re.IGNORECASE | re.DOTALL).strip()
    try:
        value = json.loads(candidate)
    except (TypeError, ValueError):
        return None
    return dict(value) if isinstance(value, Mapping) else None


class _Configured:
    def __init__(self, descriptor: ProviderDescriptor, spec: Mapping[str, Any]) -> None:
        descriptor.validate()
        self.descriptor = descriptor
        self.spec = dict(spec)
        self.model = descriptor.models[0]


class OpenAIChatProvider(_Configured):
    def __init__(self, adapter_key: str, spec: Mapping[str, Any], *, deployment: str = "remote") -> None:
        model = _model(spec, "gpt-4o-mini")
        super().__init__(ProviderDescriptor(adapter_key, ProviderKind.LLM, "real-openai-compatible-v1", ("chat", "structured_output"), (model,), deployment, "openai_chat"), spec)

    def complete(self, context: ProviderCallContext, request: LLMRequest) -> LLMResponse:
        started = time.monotonic()
        messages = []
        if request.system_instructions:
            messages.append({"role": "system", "content": request.system_instructions})
        messages.extend({"role": item.role, "content": item.content} for item in request.messages)
        body: dict[str, Any] = {"model": self.model, "messages": messages, "stream": False}
        if request.temperature is not None:
            body["temperature"] = request.temperature
        if request.max_output_tokens is not None:
            body["max_tokens"] = request.max_output_tokens
        if request.response_schema:
            body["response_format"] = {"type": "json_object"}
        headers = {"Authorization": f"Bearer {_credential(self.spec, required=True)}"}
        value, response_headers, _status = _request_json(context, self.descriptor.adapter_key, _url(_endpoint(self.spec, "https://api.openai.com/v1"), "chat/completions"), body, headers)
        payload = _object(value, self.descriptor.adapter_key)
        choices = payload.get("choices")
        if not isinstance(choices, list) or not choices or not isinstance(choices[0], Mapping):
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Provider returned no chat choice.")
        message = choices[0].get("message", {})
        text = _content(message.get("content", "")) if isinstance(message, Mapping) else ""
        if not text.strip():
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Provider returned empty chat content.")
        usage = payload.get("usage") if isinstance(payload.get("usage"), Mapping) else {}
        return LLMResponse(text, _structured(text) if request.response_schema else None, str(choices[0].get("finish_reason", "stop")), usage, _meta(self.descriptor, context, self.model, started, body, response_headers))


class OllamaLLMProvider(OpenAIChatProvider):
    def __init__(self, adapter_key: str, spec: Mapping[str, Any]) -> None:
        super().__init__(adapter_key, spec, deployment="local")
        self.spec.setdefault("endpoint", "http://127.0.0.1:11434/v1")

    def complete(self, context: ProviderCallContext, request: LLMRequest) -> LLMResponse:
        # Ollama's OpenAI-compatible endpoint intentionally shares the same
        # typed request and response path as the remote adapter.
        started = time.monotonic()
        messages = []
        if request.system_instructions:
            messages.append({"role": "system", "content": request.system_instructions})
        messages.extend({"role": item.role, "content": item.content} for item in request.messages)
        body: dict[str, Any] = {"model": self.model, "messages": messages, "stream": False}
        if request.temperature is not None:
            body["temperature"] = request.temperature
        if request.max_output_tokens is not None:
            body["max_tokens"] = request.max_output_tokens
        if request.response_schema:
            body["response_format"] = {"type": "json_object"}
        value, response_headers, _status = _request_json(context, self.descriptor.adapter_key, _url(_endpoint(self.spec, "http://127.0.0.1:11434/v1"), "chat/completions"), body)
        payload = _object(value, self.descriptor.adapter_key)
        choices = payload.get("choices")
        if not isinstance(choices, list) or not choices or not isinstance(choices[0], Mapping):
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Local model returned no chat choice.")
        message = choices[0].get("message", {})
        text = _content(message.get("content", "")) if isinstance(message, Mapping) else ""
        if not text.strip():
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Local model returned empty chat content.")
        usage = payload.get("usage") if isinstance(payload.get("usage"), Mapping) else {}
        return LLMResponse(text, _structured(text) if request.response_schema else None, str(choices[0].get("finish_reason", "stop")), usage, _meta(self.descriptor, context, self.model, started, body, response_headers))


class OpenAIVLMProvider(_Configured):
    def __init__(self, adapter_key: str, spec: Mapping[str, Any], *, deployment: str = "remote") -> None:
        model = _model(spec, "gpt-4o-mini")
        super().__init__(ProviderDescriptor(adapter_key, ProviderKind.VLM, "real-openai-compatible-v1", ("vision", "image_analysis"), (model,), deployment, "openai_chat"), spec)

    def analyze(self, context: ProviderCallContext, request: VLMRequest) -> VLMResponse:
        started = time.monotonic()
        content: list[dict[str, Any]] = [{"type": "text", "text": request.prompt}]
        for item in request.inputs:
            if not item.image_bytes:
                raise ProviderError("invalid_request", self.descriptor.adapter_key, safe_message="VLM input image bytes are missing.")
            encoded = base64.b64encode(item.image_bytes).decode("ascii")
            content.append({"type": "image_url", "image_url": {"url": f"data:{item.media_type};base64,{encoded}"}})
        body = {"model": self.model, "messages": [{"role": "user", "content": content}], "max_tokens": 256, "stream": False}
        value, response_headers, _status = _request_json(context, self.descriptor.adapter_key, _url(_endpoint(self.spec, "https://api.openai.com/v1"), "chat/completions"), body, {"Authorization": f"Bearer {_credential(self.spec, required=True)}"})
        payload = _object(value, self.descriptor.adapter_key)
        choices = payload.get("choices")
        if not isinstance(choices, list) or not choices or not isinstance(choices[0], Mapping):
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Provider returned no visual analysis.")
        message = choices[0].get("message", {})
        text = _content(message.get("content", "")) if isinstance(message, Mapping) else ""
        if not text.strip():
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Provider returned empty visual analysis.")
        descriptions = tuple(VLMDescription(item.input_id, text, (), confidence=0.75) for item in request.inputs)
        return VLMResponse(descriptions, _structured(text), _meta(self.descriptor, context, self.model, started, body, response_headers))


class OllamaVLMProvider(OpenAIVLMProvider):
    def __init__(self, adapter_key: str, spec: Mapping[str, Any]) -> None:
        model = _model(spec, "moondream")
        _Configured.__init__(self, ProviderDescriptor(adapter_key, ProviderKind.VLM, "real-ollama-v1", ("vision", "image_analysis"), (model,), "local", "ollama_chat"), spec)
        self.spec.setdefault("endpoint", "http://127.0.0.1:11434")

    def analyze(self, context: ProviderCallContext, request: VLMRequest) -> VLMResponse:
        started = time.monotonic()
        if not request.inputs or any(not item.image_bytes for item in request.inputs):
            raise ProviderError("invalid_request", self.descriptor.adapter_key, safe_message="Local VLM input image bytes are missing.")
        first = request.inputs[0]
        body = {"model": self.model, "messages": [{"role": "user", "content": request.prompt, "images": [base64.b64encode(first.image_bytes or b"").decode("ascii")]}], "stream": False}
        value, response_headers, _status = _request_json(context, self.descriptor.adapter_key, _url(_endpoint(self.spec, "http://127.0.0.1:11434"), "api/chat"), body)
        payload = _object(value, self.descriptor.adapter_key)
        message = payload.get("message", {})
        text = _content(message.get("content", "")) if isinstance(message, Mapping) else _content(payload.get("response", ""))
        if not text.strip():
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Local VLM returned empty analysis.")
        descriptions = tuple(VLMDescription(item.input_id, text, (), confidence=0.75) for item in request.inputs)
        return VLMResponse(descriptions, _structured(text), _meta(self.descriptor, context, self.model, started, body, response_headers))


class SapiTTSProvider(_Configured):
    def __init__(self, adapter_key: str, spec: Mapping[str, Any]) -> None:
        super().__init__(ProviderDescriptor(adapter_key, ProviderKind.TTS, "real-windows-sapi-v1", ("speech_synthesis", "wav"), ("windows-sapi",), "local", "windows_sapi"), spec)
        self.model = "windows-sapi"

    def synthesize(self, context: ProviderCallContext, request: TTSRequest) -> TTSResponse:
        _context_check(context)
        started = time.monotonic()
        try:
            import pyttsx3  # type: ignore[import-not-found]
        except ImportError as exc:
            raise ProviderError("unavailable", self.descriptor.adapter_key, safe_message="Windows speech runtime is not installed.") from exc
        with tempfile.TemporaryDirectory(prefix="nh-media-tts-") as root:
            output = Path(root) / "narration.wav"
            try:
                engine = pyttsx3.init()
                requested_voice = request.provider_voice_id
                if requested_voice and requested_voice != "voice-local":
                    for voice in engine.getProperty("voices") or []:
                        if requested_voice in {str(getattr(voice, "id", "")), str(getattr(voice, "name", ""))}:
                            engine.setProperty("voice", voice.id)
                            break
                engine.setProperty("rate", max(80, min(360, int(180 * request.speaking_rate))))
                engine.save_to_file(request.text, str(output))
                engine.runAndWait()
                engine.stop()
                data = output.read_bytes()
            except (OSError, RuntimeError, ValueError) as exc:
                raise ProviderError("unavailable", self.descriptor.adapter_key, safe_message="Windows speech synthesis failed.") from exc
        try:
            with wave.open(io.BytesIO(data), "rb") as wav_file:
                duration = wav_file.getnframes() / max(1, wav_file.getframerate())
                channels = wav_file.getnchannels()
        except (wave.Error, EOFError, ValueError) as exc:
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Windows speech runtime returned invalid audio.") from exc
        if duration <= 0 or channels < 1:
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Windows speech runtime returned empty audio.")
        blob = ProducedBlob("audio", "narration", data, "audio/wav", {"provider_adapter": self.descriptor.adapter_key, "model": self.model})
        return TTSResponse(blob, duration, None, _meta(self.descriptor, context, self.model, started, request))


class OpenAITTSProvider(_Configured):
    def __init__(self, adapter_key: str, spec: Mapping[str, Any], *, deployment: str = "remote") -> None:
        model = _model(spec, "gpt-4o-mini-tts")
        super().__init__(ProviderDescriptor(adapter_key, ProviderKind.TTS, "real-openai-compatible-v1", ("speech_synthesis", "wav"), (model,), deployment, "openai_audio"), spec)

    def synthesize(self, context: ProviderCallContext, request: TTSRequest) -> TTSResponse:
        body = json.dumps({"model": self.model, "input": request.text, "voice": request.provider_voice_id or "alloy", "response_format": "wav"}, separators=(",", ":")).encode()
        headers = {"Authorization": f"Bearer {_credential(self.spec, required=True)}", "Content-Type": "application/json"}
        started = time.monotonic()
        data, response_headers, _status = _request_bytes(context, self.descriptor.adapter_key, _url(_endpoint(self.spec, "https://api.openai.com/v1"), "audio/speech"), body, headers)
        try:
            with wave.open(io.BytesIO(data), "rb") as wav_file:
                duration = wav_file.getnframes() / max(1, wav_file.getframerate())
        except (wave.Error, EOFError, ValueError) as exc:
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Remote TTS returned invalid audio.") from exc
        return TTSResponse(ProducedBlob("audio", "narration", data, "audio/wav", {"provider_adapter": self.descriptor.adapter_key, "model": self.model}), duration, None, _meta(self.descriptor, context, self.model, started, request, response_headers))


class FasterWhisperASRProvider(_Configured):
    def __init__(self, adapter_key: str, spec: Mapping[str, Any]) -> None:
        model = _model(spec, "tiny.en")
        super().__init__(ProviderDescriptor(adapter_key, ProviderKind.ASR, "real-faster-whisper-v1", ("transcription", "word_timestamps"), (model,), "local", "faster_whisper"), spec)

    def transcribe(self, context: ProviderCallContext, request: ASRRequest) -> ProviderTranscript:
        _context_check(context)
        if not request.audio_bytes:
            raise ProviderError("invalid_request", self.descriptor.adapter_key, safe_message="ASR audio bytes are missing.")
        try:
            from faster_whisper import WhisperModel  # type: ignore[import-not-found]
        except ImportError as exc:
            raise ProviderError("unavailable", self.descriptor.adapter_key, safe_message="The local ASR runtime is not installed.") from exc
        device = str(self.spec.get("device", "cpu"))
        compute_type = str(self.spec.get("compute_type", "int8"))
        model = WhisperModel(self.model, device=device, compute_type=compute_type, download_root=os.environ.get("NH_MEDIA_MODEL_CACHE_DIR") or None)
        with tempfile.NamedTemporaryFile(prefix="nh-media-asr-", suffix=".wav", delete=False) as handle:
            handle.write(request.audio_bytes)
            audio_file = handle.name
        try:
            segments_iter, info = model.transcribe(audio_file, language=request.language or None, word_timestamps=request.word_timestamps, vad_filter=True)
            segments: list[ProviderTranscriptSegment] = []
            for index, segment in enumerate(segments_iter, 1):
                if context.cancelled():
                    raise ProviderError("cancelled", self.descriptor.adapter_key, safe_message="Provider call was cancelled.")
                confidence = max(0.0, min(1.0, math.exp(float(getattr(segment, "avg_logprob", -0.5)))))
                words = tuple(ProviderTranscriptWord(str(word.word).strip(), float(word.start), float(word.end), confidence) for word in (getattr(segment, "words", None) or ()) if word.end is not None and word.start is not None and word.end > word.start)
                start, end = float(segment.start), float(segment.end)
                if end > start and str(segment.text).strip():
                    segments.append(ProviderTranscriptSegment(f"segment_{index:04d}", str(segment.text).strip(), start, end, confidence, None, words))
            if not segments:
                raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Local ASR returned no transcript segments.")
            language = str(getattr(info, "language", request.language or "en"))
            duration = float(getattr(info, "duration", segments[-1].end_sec) or segments[-1].end_sec)
            return ProviderTranscript(language, self.model, duration, tuple(segments), _meta(self.descriptor, context, self.model, time.monotonic(), {"audio": request.audio_bytes}))
        except ProviderError:
            raise
        except (OSError, RuntimeError, ValueError) as exc:
            raise ProviderError("internal", self.descriptor.adapter_key, safe_message="Local ASR failed safely.") from exc
        finally:
            try:
                Path(audio_file).unlink(missing_ok=True)
            except OSError:
                pass


class OpenAIASRProvider(_Configured):
    def __init__(self, adapter_key: str, spec: Mapping[str, Any], *, deployment: str = "remote") -> None:
        model = _model(spec, "whisper-1")
        super().__init__(ProviderDescriptor(adapter_key, ProviderKind.ASR, "real-openai-compatible-v1", ("transcription", "word_timestamps"), (model,), deployment, "openai_audio"), spec)

    def transcribe(self, context: ProviderCallContext, request: ASRRequest) -> ProviderTranscript:
        if not request.audio_bytes:
            raise ProviderError("invalid_request", self.descriptor.adapter_key, safe_message="ASR audio bytes are missing.")
        boundary = "----nhmedia-form-" + _safe_id(request.audio_artifact_ref)
        fields = [("model", self.model), ("response_format", "verbose_json"), ("timestamp_granularities[]", "word" if request.word_timestamps else "segment")]
        chunks: list[bytes] = []
        for key, value in fields:
            chunks.extend([f"--{boundary}\r\nContent-Disposition: form-data; name=\"{key}\"\r\n\r\n{value}\r\n".encode()])
        chunks.append(f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; filename=\"audio.wav\"\r\nContent-Type: audio/wav\r\n\r\n".encode() + request.audio_bytes + b"\r\n")
        chunks.append(f"--{boundary}--\r\n".encode())
        started = time.monotonic()
        data, headers, _status = _request_bytes(context, self.descriptor.adapter_key, _url(_endpoint(self.spec, "https://api.openai.com/v1"), "audio/transcriptions"), b"".join(chunks), {"Authorization": f"Bearer {_credential(self.spec, required=True)}", "Content-Type": f"multipart/form-data; boundary={boundary}"})
        try:
            payload = _object(json.loads(data.decode("utf-8")), self.descriptor.adapter_key)
        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Remote ASR returned malformed JSON.") from exc
        raw_segments = payload.get("segments")
        if not isinstance(raw_segments, list):
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Remote ASR returned no segments.")
        segments = tuple(ProviderTranscriptSegment(f"segment_{index:04d}", str(item.get("text", "")).strip(), float(item.get("start", 0)), float(item.get("end", 0)), 1.0, None, tuple(ProviderTranscriptWord(str(word.get("word", "")).strip(), float(word.get("start", 0)), float(word.get("end", 0)), 1.0) for word in item.get("words", []) if isinstance(word, Mapping))) for index, item in enumerate(raw_segments, 1) if isinstance(item, Mapping) and float(item.get("end", 0)) > float(item.get("start", 0)) and str(item.get("text", "")).strip())
        if not segments:
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Remote ASR returned empty segments.")
        return ProviderTranscript(str(payload.get("language", request.language or "en")), self.model, float(payload.get("duration", segments[-1].end_sec)), segments, _meta(self.descriptor, context, self.model, started, {"audio": request.audio_bytes}, headers))


class OllamaEmbeddingProvider(_Configured):
    def __init__(self, adapter_key: str, spec: Mapping[str, Any]) -> None:
        model = _model(spec, "nomic-embed-text")
        super().__init__(ProviderDescriptor(adapter_key, ProviderKind.EMBEDDING, "real-ollama-v1", ("text_embedding", "normalized_vectors"), (model,), "local", "ollama_embed"), spec)
        self.spec.setdefault("endpoint", "http://127.0.0.1:11434")

    def embed(self, context: ProviderCallContext, request: EmbeddingRequest) -> EmbeddingResponse:
        started = time.monotonic()
        inputs = [item.text or item.artifact_ref or item.item_id for item in request.items]
        value, headers, _status = _request_json(context, self.descriptor.adapter_key, _url(_endpoint(self.spec, "http://127.0.0.1:11434"), "api/embed"), {"model": self.model, "input": inputs})
        payload = _object(value, self.descriptor.adapter_key)
        raw_vectors = payload.get("embeddings")
        if not isinstance(raw_vectors, list) or len(raw_vectors) != len(request.items):
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Local embedding model returned an incomplete batch.")
        vectors = tuple(EmbeddingVector(item.item_id, tuple(float(value) for value in raw)) for item, raw in zip(request.items, raw_vectors) if isinstance(raw, list) and raw)
        if len(vectors) != len(request.items) or len({len(item.values) for item in vectors}) != 1:
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Local embedding model returned invalid vectors.")
        dimension = len(vectors[0].values)
        if request.normalize:
            normalized: list[EmbeddingVector] = []
            for vector in vectors:
                norm = math.sqrt(sum(value * value for value in vector.values)) or 1.0
                normalized.append(EmbeddingVector(vector.item_id, tuple(value / norm for value in vector.values)))
            vectors = tuple(normalized)
        return EmbeddingResponse(dimension, "cosine", self.model, request.normalize, vectors, tuple(item.item_id for item in request.items), None, _meta(self.descriptor, context, self.model, started, request, headers))


class OpenAIEmbeddingProvider(_Configured):
    def __init__(self, adapter_key: str, spec: Mapping[str, Any], *, deployment: str = "remote") -> None:
        model = _model(spec, "text-embedding-3-small")
        super().__init__(ProviderDescriptor(adapter_key, ProviderKind.EMBEDDING, "real-openai-compatible-v1", ("text_embedding", "normalized_vectors"), (model,), deployment, "openai_embed"), spec)

    def embed(self, context: ProviderCallContext, request: EmbeddingRequest) -> EmbeddingResponse:
        started = time.monotonic()
        inputs = [item.text or item.artifact_ref or item.item_id for item in request.items]
        body = {"model": self.model, "input": inputs, "encoding_format": "float"}
        value, headers, _status = _request_json(context, self.descriptor.adapter_key, _url(_endpoint(self.spec, "https://api.openai.com/v1"), "embeddings"), body, {"Authorization": f"Bearer {_credential(self.spec, required=True)}"})
        payload = _object(value, self.descriptor.adapter_key)
        raw_vectors = payload.get("data")
        if not isinstance(raw_vectors, list) or len(raw_vectors) != len(request.items):
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Remote embedding model returned an incomplete batch.")
        ordered = sorted((item for item in raw_vectors if isinstance(item, Mapping)), key=lambda item: int(item.get("index", 0)))
        vectors = tuple(EmbeddingVector(request.items[index].item_id, tuple(float(value) for value in item.get("embedding", []))) for index, item in enumerate(ordered))
        if len(vectors) != len(request.items) or not vectors or len({len(item.values) for item in vectors}) != 1:
            raise ProviderError("invalid_response", self.descriptor.adapter_key, safe_message="Remote embedding model returned invalid vectors.")
        if request.normalize:
            vectors = tuple(EmbeddingVector(vector.item_id, tuple(value / (math.sqrt(sum(item * item for item in vector.values)) or 1.0) for value in vector.values)) for vector in vectors)
        return EmbeddingResponse(len(vectors[0].values), "cosine", self.model, request.normalize, vectors, tuple(item.item_id for item in request.items), None, _meta(self.descriptor, context, self.model, started, body, headers))


def build_real_providers(policy: Mapping[str, Any]) -> tuple[Any, ...]:
    """Build only explicitly named adapters from a validated provider policy."""
    raw_keys = policy.get("adapter_keys", {})
    raw_specs = policy.get("providers", {})
    if not isinstance(raw_keys, Mapping) or not isinstance(raw_specs, Mapping):
        raise ValueError("real provider policy must contain adapter_keys and providers")
    result: list[Any] = []
    for kind in ProviderKind:
        keys = raw_keys.get(kind.value, ())
        if not isinstance(keys, (list, tuple)):
            raise ValueError("real provider adapter bindings are invalid")
        for adapter_key_value in keys:
            adapter_key = str(adapter_key_value)
            if adapter_key.startswith("fake-"):
                continue
            spec_value = raw_specs.get(adapter_key, {})
            if not isinstance(spec_value, Mapping):
                raise ValueError("real provider specification is invalid")
            deployment = str(spec_value.get("deployment", policy.get("mode", "local")))
            if deployment not in {"local", "remote"}:
                raise ValueError("provider deployment is invalid")
            if kind is ProviderKind.LLM:
                result.append(OllamaLLMProvider(adapter_key, spec_value) if adapter_key.startswith("ollama-") else OpenAIChatProvider(adapter_key, spec_value, deployment=deployment))
            elif kind is ProviderKind.VLM:
                result.append(OllamaVLMProvider(adapter_key, spec_value) if adapter_key.startswith("ollama-") else OpenAIVLMProvider(adapter_key, spec_value, deployment=deployment))
            elif kind is ProviderKind.TTS:
                result.append(SapiTTSProvider(adapter_key, spec_value) if adapter_key == "sapi-tts" else OpenAITTSProvider(adapter_key, spec_value, deployment=deployment))
            elif kind is ProviderKind.ASR:
                result.append(FasterWhisperASRProvider(adapter_key, spec_value) if adapter_key == "faster-whisper-asr" else OpenAIASRProvider(adapter_key, spec_value, deployment=deployment))
            elif kind is ProviderKind.EMBEDDING:
                result.append(OllamaEmbeddingProvider(adapter_key, spec_value) if adapter_key.startswith("ollama-") else OpenAIEmbeddingProvider(adapter_key, spec_value, deployment=deployment))
    return tuple(result)


__all__ = [
    "FasterWhisperASRProvider",
    "OllamaEmbeddingProvider",
    "OllamaLLMProvider",
    "OllamaVLMProvider",
    "OpenAIASRProvider",
    "OpenAIChatProvider",
    "OpenAIEmbeddingProvider",
    "OpenAITTSProvider",
    "OpenAIVLMProvider",
    "SapiTTSProvider",
    "build_real_providers",
]
