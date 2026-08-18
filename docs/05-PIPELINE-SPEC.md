# 05 — Pipeline Specification

## 1. Runtime model

Pipeline là DAG versioned, không phải một hàm `step1(); step2(); ...`. Mỗi node có input/output contract, dependency, capability và policy. Runtime vẫn có thể thực thi graph tuyến tính ở `movie_recap` baseline, nhưng graph representation phải cho phép branching, join, partial run và future workflow.

```text
Pipeline definition
  → validate DAG + schemas
  → create Job + PipelineRun + JobSteps
  → scheduler selects ready nodes
  → worker claims node lease
  → node reads IDs/artifacts from JobContext
  → node commits outputs atomically
  → checkpoint + progress event
  → unlock dependants / human gate / terminal Job
```

## 2. Pipeline definition

Một Pipeline version có:

```text
workflow_key: movie_recap
version: integer
nodes: [{key, type, depends_on, input_schema, output_schema,
         execution_class, soft, human_gate, resource_requirements,
         timeout, retry_policy}]
policies: {strict, checkpoint, artifact_retention, progress_weights}
provenance: {nh_media_release, contract_version, optional_reference_observation}
```

Validation trước khi activate:

- node key duy nhất, type đã registry;
- graph acyclic, dependency cùng pipeline;
- có entry node và terminal node;
- `start_from`/`stop_after` là node key hợp lệ;
- schema input/output tương thích trên mỗi edge;
- hard/soft consequence được khai báo;
- execution class có worker capability tương ứng;
- pipeline content hash được lưu immutable.

## 3. `JobContext`

Context engine-neutral tối thiểu:

```text
job_id, pipeline_run_id, project_id, workspace_id
pipeline_id, pipeline_version
input_refs: asset/script/timeline IDs + revisions
config_snapshot: validated JSON
node_outputs: map[node_key, ArtifactRef/DomainRef]
metadata: diagnostics only
cancel_token, checkpoint_policy
provider_snapshot
```

Rules:

1. Context truyền reference/typed domain data, không truyền absolute path giữa service.
2. Worker resolves `ArtifactRef` thành local handle tạm trong executor sandbox khi media/model tool
   requires a file.
3. Node không tự ghi DB product ngoài `NodeOutput`/repository port.
4. Context snapshot serializable sau node; service handles/logger không serialize.
5. Mọi output phải khai báo semantic type và checksum.

## 4. PipelineNode definition and execution contract

Đây là contract logic, không phải application implementation:

```text
PipelineNodeDefinition
  key, type, display_name
  input_contract, output_contract
  required_artifact_roles, produced_artifact_roles
  dependencies [{node_key, required, condition}]
  execution_class, resource_requirements
  timeout_policy {wall_time_sec, idle_time_sec?, kill_grace_sec}
  retry_policy {max_attempts, backoff, jitter, retryable_categories}
  progress_policy {unit, total_source, emit_interval}
  checkpoint_policy {mode, resume_compatibility, invalidation_inputs}
  idempotency_policy {fingerprint_fields, reuse, side_effects}
  failure_mode: hard | soft
  review_policy

PipelineNodeExecutor
  validate(context, resolved_inputs) -> ValidationResult
  execute(context, resolved_inputs, reporter, cancellation) -> NodeResult
  resume(context, checkpoint, ...) -> NodeResult       # khi capability hỗ trợ
  compensate(staged_result) -> CleanupResult           # optional
```

`NodeResult.outcome` chỉ dùng `completed|skipped|paused|waiting_for_review|failed|cancelled`; runtime map nguyên nghĩa vào JobStep canonical state. Result gồm declared `outputs`, warnings, metrics, checkpoint payload, review request và provider usage. Node không trả raw HTTP response, provider SDK object hoặc durable local path.

Definition thiếu bất kỳ field policy trên không được activate. Timeout, retry, progress và checkpoint là data của Pipeline version, không hard-code theo workflow trong scheduler.

### Idempotency

Input fingerprint = hash của node config + pipeline version + exact input IDs/revisions + provider/model snapshot. Nếu fingerprint trùng output committed, runtime có thể reuse output. Re-run chỉ force khi user explicit hoặc source revision đổi.

### Soft/hard semantics

- Hard node fail → Job fail nếu retry không thành công.
- Soft node fail/skip → Job tiếp tục với status `skipped`/`degraded`, nhưng output contract phải cho biết capability mất.
- `strict=true` đổi soft failure thành Job failure.
- Human gate không phải failure; JobStep chuyển `waiting_for_review` và giữ checkpoint.

## 5. Independent movie-recap baseline graph

The built-in graph is an NH-Media-native workflow. Reference observations informed capability
coverage, but node names, contracts, state and APIs belong to NH-Media.

```text
resolve_source_asset → prepare_media_assets
  ├→ research_metadata → generate_script → review_script
  ├→ detect_scenes → analyze_scenes
  └→ extract_transcript

review_script → generate_narration → align_audio
script + scenes + analysis + alignment → generate_match_candidates
generate_match_candidates → evaluate_candidates → select_candidate
selection + narration + subtitle cues → build_timeline → review_timeline
review_timeline → mix_audio → run_qa_gate → render_timeline
→ validate_deliverable → export_clips
```

The earliest vertical slice may use a smaller graph (`resolve_source_asset → generate_thumbnail →
commit Artifact`) while preserving the same Job/worker contracts. It must not execute upstream.

## 6. NH-Media node catalog

Policy profiles dùng trong bảng:

| Profile | Timeout | Retry | Progress | Checkpoint / idempotency |
|---|---|---|---|---|
| `P-SYSTEM` | wall 120s, grace 10s | 1 attempt; không retry validation/review | item/phase, emit ≤ 1/s | terminal checkpoint; deterministic fingerprint/reuse |
| `P-PROBE` | wall 600s, idle 120s, grace 15s | 2 attempts cho storage/worker transient | bytes/streams/frames | terminal; side-effect-free before Artifact commit |
| `P-AI` | wall 300s, idle 120s, grace 10s | 3 attempts cho timeout/rate-limit/unavailable; circuit/fallback policy | request/item count, ≤ 1/s | terminal or batch chunk; fingerprint includes provider/model/prompt schema |
| `P-ML` | wall 3600s, idle 300s, grace 30s | 2 attempts; worker lost/transient/OOM-on-other-capability only | media time/frame/item | chunk checkpoint when executor declares support; model/source revision in fingerprint |
| `P-MEDIA` | wall 1800s, idle 300s, grace 30s | 2 attempts cho storage/process interruption, không retry invalid media | media time/frame | stage output then atomic commit; exact inputs/profile in fingerprint |
| `P-RENDER` | wall 14400s, idle 600s, grace 60s | 2 attempts cho worker/storage/transient encoder failure | encoded frame/media time | segment checkpoint optional; incomplete file never canonical; timeline/profile hash in fingerprint |

`timeout` cụ thể có thể override trong Pipeline version nhưng không được vô hạn. Bảng dưới là required contract của built-in `movie_recap_v2`; “soft” nghĩa là output consequence phải hiện trong Job warnings, không nghĩa nuốt mọi exception.

Every retry decision classifies the failure as `transient`, `permanent`, `cancellation`, `timeout`,
`provider_throttling`, `resource_exhaustion` or `invalid_user_input`. Only categories explicitly
declared retryable use bounded exponential backoff with jitter; permanent/security/invalid-input
failures do not consume repeated attempts. Resource exhaustion may retry only on a compatible worker
when the node policy declares that route.

| Node | Inputs / required artifacts | Produced domain/output artifacts | Dependencies | Policy | Failure semantics |
|---|---|---|---|---|---|
| `resolve_source_asset` | Asset ID, committed original Artifact | validated source ref + probe Artifact | entry | `P-PROBE` | hard; security/invalid media non-retryable |
| `prepare_media_assets` | source ref; optional BGM/font/image Assets | normalized proxy/audio/thumb Artifacts | `resolve_source_asset` | `P-MEDIA` | hard for required proxy; optional variants soft |
| `research_metadata` | project/source metadata; optional external title IDs | ResearchAnalysis/Artifact | `resolve_source_asset` | `P-AI` | soft by default, hard in strict pipeline |
| `generate_script` | research Artifact optional, style/language/duration snapshot | Script + proposed ScriptVersion; export-ready content | `research_metadata` or source metadata fallback | `P-AI` | hard; invalid structured script non-retryable after provider budget |
| `review_script` | proposed ScriptVersion | approved ScriptVersion ref/review event | `generate_script` | `P-SYSTEM` + `approval_completes_node` | automatic mode completes by policy; studio waits; reject follows declared correction/fail edge |
| `generate_narration` | exact approved/allowed ScriptVersion; Voice snapshot | Narration + audio Artifact + optional provider timing | `review_script` or `generate_script` in automatic mode | `P-AI` | hard; cache/reuse by exact TTS fingerprint |
| `align_audio` | narration audio Artifact + ScriptVersion | timing/transcript alignment Artifact | `generate_narration` | `P-ML` | soft fallback to segment timing if declared; hard in strict mode |
| `detect_scenes` | prepared source video Artifact | Scene records, scene index + thumbnail Artifacts | `prepare_media_assets` | `P-ML` | soft fallback one full-length Scene only if pipeline explicitly allows |
| `extract_transcript` | prepared source audio Artifact | Transcript Artifact | `prepare_media_assets` | `P-ML` | soft if matching can proceed without dialogue; fallback chain is node config |
| `analyze_scenes` | Scene refs + keyframe Artifacts; optional transcript | Analysis records for caption/entity/action/location/emotion | `detect_scenes`; optional `extract_transcript` | `P-AI` | soft per-scene partial result only with missing-item list/provenance |
| `detect_characters` | Scene/keyframe Artifacts | Character/Appearance records + embedding/profile Artifacts | `detect_scenes` | `P-ML` | optional/soft; never overwrite user-confirmed identity |
| `embed_media` | captions/text/keyframes as declared | embedding Artifact + item index/model metadata | `analyze_scenes`; optional `generate_script`/`detect_characters` | `P-ML` | optional/soft if matcher has declared lexical fallback |
| `generate_match_candidates` | ScriptVersion, Scenes, alignment; optional transcript/analysis/characters/embeddings | one or more GenerationCandidates/MatchProposal Artifacts | `generate_script`, `detect_scenes`, `align_audio`; optional intelligence nodes | `P-ML` or `P-AI` | hard if no valid coverage; degraded inputs are explicit |
| `evaluate_candidates` | candidate group + selection policy snapshot | EvaluationResults with component scores/provenance | `generate_match_candidates` | `P-AI` or `P-SYSTEM` | optional in first workflow; deterministic policy errors are hard |
| `select_candidate` | candidates + EvaluationResults + user/policy constraints | selected candidate ref and audit record | `evaluate_candidates` or direct single-candidate path | `P-SYSTEM` | must never overwrite candidate payloads or bypass review policy |
| `coverage_feedback` | ScriptVersion + selected match proposal | coverage report Artifact; optional new ScriptVersion proposal | `select_candidate` | `P-AI` | optional/soft; cannot mutate approved ScriptVersion |
| `analyze_reference_style` | authorized reference Asset/Scenes/audio/subtitles | ReferenceStyleAnalysis abstract metrics | independent analysis workflow | `P-ML`/`P-AI` | later-phase; never outputs copied footage as style result |
| `translate_subtitles` | timed script/alignment + target language | translated timed-text Artifact | `align_audio` | `P-AI` | soft when source-language subtitle allowed; hard when target required |
| `generate_subtitle` | alignment + ScriptVersion + optional translation | subtitle cue data + SRT/VTT/ASS Artifacts | `align_audio`; optional `translate_subtitles` | `P-SYSTEM` | hard when profile requires subtitle, otherwise soft |
| `build_timeline` | ScriptVersion, selected match proposal, Narration, subtitle cues, source refs | proposed Timeline + TimelineVersion document | `select_candidate`, `generate_narration`, `generate_subtitle` | `P-SYSTEM` | hard; full Timeline schema/cross-ref validation |
| `review_timeline` | proposed TimelineVersion | approved TimelineVersion ref/review event | `build_timeline` | `P-SYSTEM` + `approval_completes_node` | studio waits; automatic mode applies configured approval policy |
| `mix_audio` | TimelineVersion audio tracks, Narration/BGM/SFX Artifacts | mixed audio Artifact + loudness report | `build_timeline`; approval requirement follows render mode | `P-MEDIA` | hard for required narration; optional BGM may soft-skip |
| `run_qa_gate` | TimelineVersion and declared intermediate artifacts | QA report Artifact + pass/warnings | `build_timeline`, `mix_audio` | `P-SYSTEM` | hard/soft thresholds declared; security/integrity fail hard |
| `render_timeline` | exact validated TimelineVersion, RenderProfile snapshot, source/mixed audio Artifacts | staged then committed video/audio/metadata Artifacts | `review_timeline` or automatic approval, `run_qa_gate` | `P-RENDER` | hard; renderer may not rematch scenes |
| `validate_deliverable` | committed render candidate Artifacts + profile | deliverable QA Artifact and canonicalization decision | `render_timeline` | `P-PROBE` | hard for missing streams/profile violation; warning thresholds explicit |
| `export_clips` | TimelineVersion or declared clip selection + canonical render/source refs | clip Artifacts + manifest | `validate_deliverable` | `P-RENDER` | optional/soft unless clips requested as required output |

`build_timeline` is the sole bridge from a selected proposal to renderer source of truth. No
upstream node aliases or compatibility graph are part of NH-Media.

## 7. Partial execution and checkpoints

### Start/stop contract

```json
{
  "start_from": "generate_narration",
  "stop_after": "select_candidate",
  "resume_checkpoint_id": "ckpt_..."
}
```

- `start_from` marks earlier nodes `skipped` với reason `reused_checkpoint|partial_boundary` chỉ khi declared outputs đã có và fingerprint/checkpoint compatible; nếu thiếu input thì reject command `422`.
- `stop_after` completes target node, commits checkpoint and returns Job `paused`; the API response marks the execution as partial through checkpoint/progress metadata.
- Resume starts at first not-committed node after checkpoint, unless explicit override.
- A changed Script/Timeline/Asset revision invalidates downstream checkpoint by dependency graph, not necessarily all checkpoints.

### Checkpoint contents

- completed node key and canonical status `completed|skipped`;
- serialized `JobContext` references, not secrets/paths;
- input fingerprint and pipeline/provider snapshots;
- output Artifact IDs and domain revision IDs;
- state hash, schema version, created time;
- optional human gate payload and UI instructions.

Checkpoint is persisted before event `node.completed`/`node.skipped` is published. On crash, replay state from DB/object storage; Redis message is not source of truth.

Meaningful pipeline checkpoints use typed stage names where applicable, including `uploaded`,
`probed`, `audio_extracted`, `speech_transcribed`, `scenes_detected`, `script_generated`,
`narration_generated`, `timeline_built`, `rendered` and `validated`. A completed compatible stage is
not rerun after process restart unless an input fingerprint or declared invalidation rule changes.

## 8. Provider interfaces

Canonical provider contract và conformance rules nằm tại `11-PROVIDER-ARCHITECTURE.md`. Phần dưới là summary để node author biết dependency surface; nếu khác nhau thì tài liệu `11` thắng.

Provider interface là port của engine. Provider implementation không được leak SDK-specific response vào domain.

### Common provider metadata

```text
ProviderDescriptor:
  name, kind, version
  capabilities: [streaming, local, batch, multilingual, gpu]
  models: [{id, max_input, languages, modalities}]
  health() -> status
```

Mỗi call nhận `ProviderCallContext` gồm correlation ID, timeout, cancellation, locale, tenant-scoped credential reference (không plaintext), cost policy và request hash.

### LLMProvider

```text
LLMProvider.generate(request: LLMRequest) -> LLMResponse

LLMRequest:
  model, system_prompt, messages, response_schema,
  temperature, max_tokens, language, metadata

LLMResponse:
  text, structured_json?, finish_reason,
  usage {input_tokens, output_tokens}, model,
  provider_request_id, warnings
```

Requirements: structured output phải validate schema; prompt injection từ media/script được đánh dấu untrusted; provider timeout và retry network/rate-limit; không log prompt chứa secret/media content ngoài policy.

### VLMProvider

```text
VLMProvider.describe_frame(request: VLMFrameRequest) -> VLMDescription
VLMProvider.describe_scene(request: VLMSceneRequest) -> VLMDescription

VLMFrameRequest: artifact_ref, timestamp_sec, prompt, language
VLMSceneRequest: scene_id, keyframe_refs, transcript?, prompt, language
VLMDescription:
  text, entities, actions, location?, emotion?, confidence?, usage, model
```

Frame/keyframe bytes được worker resolve từ Artifact; provider không nhận local path. Result phải trace về Scene/Asset revision.

### TTSProvider

```text
TTSProvider.synthesize(request: TTSRequest) -> AudioOutput

TTSRequest:
  text, language, voice, style_prompt?, speed?, pitch?, format,
  output_policy, idempotency_key

AudioOutput:
  audio_artifact_ref, duration_sec, sample_rate, channels,
  provider_request_id, usage/cost, warnings
```

Provider chỉ tạo audio; duration probing và cache orchestration ở engine. TTS cache key phải gồm
schema version, provider/version, model, voice, text, style prompt và relevant audio settings.

### ASRProvider

```text
ASRProvider.transcribe(request: ASRRequest) -> Transcript
ASRRequest: audio_artifact_ref, language?, model?, diarization?, word_timestamps
Transcript:
  segments [{id, text, start_sec, end_sec, confidence, words[]}],
  language, model, provider_request_id, warnings
```

WhisperX, faster-whisper và FunASR are possible independently implemented adapter integrations;
fallback chain phải khai báo trong node policy, không hard-code trong Product API.

### EmbeddingProvider

```text
EmbeddingProvider.embed(request: EmbeddingRequest) -> EmbeddingResult
EmbeddingRequest: items [{id, modality, text?, artifact_ref?}], model, normalize
EmbeddingResult: vectors/artifact_ref, dimension, model, metric, usage
```

Vector lớn phải lưu Artifact hoặc vector store abstraction; DB chỉ giữ reference/metadata ở giai đoạn đầu. Text embedding, visual embedding (CLIP/SigLIP/custom) dùng cùng port nhưng capability khác nhau.

### Provider error contract

```text
ProviderError:
  code, provider, retryable, safe_message, provider_request_id?, cause_category
```

Không retry lỗi schema/auth/content policy; retry có exponential backoff + jitter cho timeout/connection/rate-limit theo budget node. Circuit breaker áp dụng cho endpoint/provider.

## 9. Matching and feedback contract

`match_clips` nhận text embedding, VLM caption/visual embedding nếu có, character appearance, temporal context, diversity history và quality filters. Composite score phải lưu các thành phần:

```text
semantic_score
visual_score
character_score
temporal_score
rhythm_score
diversity_score
quality_score
composite_score
```

`coverage_feedback` báo segment không có footage phù hợp; LLM chỉ được rewrite trong vùng segment được phép và phải giữ semantic/user constraints. Mỗi rewrite là Script version/proposal mới, không silently mutate approved script.

## 10. Render contract

Renderer không nhận “script + matches” làm nguồn chính. Renderer nhận:

```text
Timeline ID + exact revision
source Asset/Artifact refs
render profile
optional preview policy
```

Renderer compile Timeline → deterministic Go media plan/FFmpeg execution → output Artifact(s) → QA.
Profile chỉ thay target constraints (resolution, aspect ratio, codec, bitrate, safe area), không
mutate canonical Timeline. Python ML workers are not used for deterministic media composition.

## 11. First Python worker integration slice

After the deterministic Go probe/thumbnail path passes, a minimal `nh_media` node validates the
same versioned worker protocol. It may perform lightweight audio/image analysis or a deterministic
protocol fixture, but it must create a real result/Artifact through:

```text
Go Product API → PostgreSQL Job → Redis Streams → Python nh_media worker
→ result/Artifact → Product API/SSE
```

This slice must not depend on a hosted LLM, Whisper, CUDA, VLM or TTS provider. Heavy AI capability
work begins only after both worker classes and recovery paths are proven.

## 12. Reference-informed implementation boundary

For a capability identified through upstream research:

1. record the observable purpose and failure/quality expectations;
2. define an NH-Media domain/interface contract;
3. implement it independently in the appropriate Go or `nh_media` module;
4. test the NH-Media contract and security boundary;
5. optionally compare against recorded reference outputs;
6. record intentional divergence and acceptance evidence.

Pipeline activation and normal CI must not import, execute, fetch or materialize Movie Narrator.
The normative classification is in `UPSTREAM-CAPABILITY-MATRIX.md`.

## 13. Gate G implementation evidence (2026-08-18)

The current native implementation keeps provider decisions behind typed LLM, VLM, TTS, ASR and
embedding ports. Provider results carry adapter/configuration/model provenance; normalized errors,
allowlisted fallback and credential redaction are tested. Domain nodes do not call a universal
untyped provider method.

The Go renderer consumes the canonical TimelineVersion document and profile, compiles multiple
video clips with allowlisted transitions, audio mixing and sandboxed subtitle files, and performs
ffprobe QA for non-empty output, profile dimensions/codecs, duration and required audio before an
Artifact commit is offered. Real reviewed FFmpeg/ffprobe tests cover Python scene detection and
frame-derived features, three-clip render with audio/subtitles, clip export, and 16:9/9:16/1:1
profile reuse. This is local deterministic/provider-free acceptance; external provider quality,
GPU model execution and production deployment remain unclaimed.
