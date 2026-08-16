# 11 — Provider Architecture

## 1. Boundary

Provider là engine port cho LLM, VLM, TTS, ASR và Embedding. PipelineNode phụ thuộc capability và typed request/response; không import SDK hoặc rẽ nhánh `if provider == ...`. Provider-specific auth, endpoint, payload, streaming và error translation chỉ nằm trong adapter.

Product backend sở hữu ProviderConfiguration, authorization và credential reference. Engine nhận resolved provider binding snapshot. Worker chỉ nhận short-lived/scoped credential cần cho node; render/probe worker không nhận LLM/TTS secret nếu không dùng.

```text
PipelineNode
  → ProviderResolver(kind, binding snapshot)
      → provider-neutral port
          → OpenAI-compatible / Ollama / local model / vendor adapter
```

## 2. Common contracts

```text
ProviderDescriptor
  adapter_key: string
  kind: llm | vlm | tts | asr | embedding
  adapter_version: string
  capabilities: set[string]
  models: list[ModelDescriptor]
  deployment: local | remote

ProviderCallContext
  request_id, correlation_id, project_ref, job_id, job_step_id
  provider_configuration_id + revision
  credential_ref (resolved outside serialized domain payload)
  timeout_sec, cancellation_token, locale
  idempotency_key, privacy_policy, cost_budget

ProviderResultMeta
  adapter_key, adapter_version, provider_configuration_id
  model, provider_request_id, latency_ms
  usage, estimated_cost, warnings
```

`project_ref` chỉ để policy/audit, không chứa User hay membership model. `credential_ref` không được xuất hiện trong Job snapshot, checkpoint, event, API response hoặc log; snapshot chỉ giữ stable configuration ID/revision và non-secret adapter/model policy.

## 3. LLM port

```text
LLMProvider.complete(context, LLMRequest) -> LLMResponse

LLMRequest
  model?, system_instructions, messages
  response_schema?, temperature?, max_output_tokens?
  language?, safety_policy, metadata

LLMResponse
  text, structured_output?, finish_reason
  usage {input_tokens, output_tokens}
  meta: ProviderResultMeta
```

Rules:

- `structured_output` phải validate `response_schema`; parse failure là `invalid_response`, không tự coi raw text là domain object.
- Media transcript/user prompt là untrusted data, phải phân tách với system instructions.
- Streaming là optional capability `streaming`; domain result chỉ commit sau stream hoàn tất và validate.
- Pipeline không phụ thuộc OpenAI message/response classes.

## 4. VLM port

```text
VLMProvider.analyze(context, VLMRequest) -> VLMResponse

VLMRequest
  model?
  inputs: [{artifact_ref, media_type, timestamp_sec?, scene_id?}]
  prompt, response_schema?, language?, sampling_policy

VLMResponse
  descriptions: [{input_id, text, entities, actions, location?, emotion?, confidence?}]
  structured_output?, meta
```

Worker resolves Artifact refs; adapter không nhận globally meaningful local path. Sampling/keyframe extraction là media adapter/node responsibility trừ khi provider capability khai báo nhận video trực tiếp. Output phải trace về exact Scene/source Artifact revision.

## 5. TTS port

```text
TTSProvider.list_voices(context, VoiceQuery) -> list[VoiceDescriptor]
TTSProvider.synthesize(context, TTSRequest) -> TTSResponse

TTSRequest
  text, language, voice {provider_voice_id, snapshot}
  model?, speaking_rate?, pitch?, style_prompt?
  audio_format, sample_rate?, channels?, idempotency_key

TTSResponse
  audio_blob: ProducedBlob
  duration_sec?, timing_data?, meta
```

Provider returns produced bytes/stream to Artifact commit layer, not a public filesystem identity. Engine probes duration, commits Artifact, creates Narration and optional timing Artifact. Cache key includes contract version, adapter/version, model, voice snapshot, normalized text, style/rate/pitch/audio settings.

Upstream Edge-TTS adapter chỉ hợp lệ local/test/personal theo security policy; production commercial cần provider được owner/legal approve.

## 6. ASR port

```text
ASRProvider.transcribe(context, ASRRequest) -> Transcript

ASRRequest
  audio_artifact_ref, language?, model?
  word_timestamps: boolean
  diarization: boolean
  vocabulary_hints?

Transcript
  language, duration_sec
  segments [{id, text, start_sec, end_sec, confidence?, speaker?, words[]}]
  meta
```

WhisperX, faster-whisper và FunASR adapters phải trả cùng segment semantics. Fallback order thuộc PipelineNode policy snapshot, không hard-code ở Product API hay provider registry. Backend không hỗ trợ requested capability trả `unsupported_capability`, không silently giảm precision nếu node contract không cho phép.

## 7. Embedding port

```text
EmbeddingProvider.embed(context, EmbeddingRequest) -> EmbeddingResponse

EmbeddingRequest
  model
  modality: text | image | multimodal
  items: [{item_id, text? | artifact_ref?}]
  normalize: boolean

EmbeddingResponse
  dimension, metric, model
  vectors? | vector_artifact
  item_index, meta
```

Vector nhỏ có thể trả in-memory trong một node attempt; persistent/batch vector phải commit thành Artifact hoặc vector-store port. Model/version/dimension/normalization là một phần input fingerprint. Không so sánh vectors khác model space.

## 8. Provider errors, retry and fallback

Canonical `ProviderError`:

```text
code: timeout | rate_limited | unavailable | auth_failed |
      invalid_request | invalid_response | content_rejected |
      unsupported_capability | cancelled | internal
category: transient | permanent | policy | cancelled
retryable: boolean
safe_message
adapter_key, provider_request_id?
retry_after_sec?
```

- Retry timeout/rate-limit/unavailable chỉ trong node budget, exponential backoff + jitter và `Retry-After` cap.
- Không retry auth, invalid schema/request, content policy hoặc user cancellation.
- Circuit breaker key tối thiểu gồm configuration + endpoint + capability; breaker open tạo transient failure nếu fallback còn, nếu không theo node failure mode.
- Fallback chain được khai báo trong PipelineNode config, ví dụ `[vlm_primary, vlm_local]`, với điều kiện lỗi/cost/privacy cụ thể.
- Fallback tạo `provider_runs` riêng và event progress safe; output snapshot ghi adapter/model thực tế.
- Partial batch phải khai báo: all-or-nothing hoặc per-item result. Không tự trộn outputs nhiều provider mà thiếu provenance.

## 9. Provider resolver and adapters

`ProviderResolver` nhận `kind`, ProviderConfiguration ID/revision và requested capabilities; trả adapter instance nếu:

1. configuration `active` và caller được authorize;
2. adapter kind khớp;
3. model/capability hợp lệ;
4. credential resolve thành công trong worker scope;
5. endpoint/egress policy cho phép.

Registry không auto-load arbitrary Python entry point trong production. Built-in adapters đăng ký tĩnh hoặc qua allowlist package/version/hash. Adapter lifecycle phải close HTTP client/GPU/session sau use theo scope.

## 10. Configuration API scope

Product scope hỗ trợ:

- list provider catalog/capabilities không secret;
- create/update/validate/disable ProviderConfiguration cho authorized admin;
- rotate credential bằng secret-manager operation, DB chỉ đổi reference/revision;
- select provider binding ở Workspace/Project/Job policy;
- audit mọi validation/rotation/use.

Credential ownership cụ thể còn ở OQ-05; task phụ thuộc không được mặc định plaintext `.env` thành multi-user product policy.

## 11. Contract tests

Mỗi adapter phải chạy cùng provider conformance suite:

- descriptor/capability validation;
- request mapping và structured response validation;
- timeout/cancel/resource cleanup;
- error taxonomy + retryability;
- secret/log redaction;
- deterministic fake adapter;
- provenance/usage snapshot;
- unsupported capability;
- fallback integration ở node layer, không trong business API.

Upstream registry/TTS/Vision/ASR disposition được ghi ở `UPSTREAM-MODULE-AUDIT.md`.
