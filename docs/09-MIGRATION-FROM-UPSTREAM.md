# 09 — Migration from Movie Narrator V1

## 1. Non-negotiable strategy

Movie Narrator V1 là **legacy/reference engine**, không phải architecture V2. Migration dùng Strangler Pattern:

```text
Product API / trusted orchestration
  → VideoEngine port
      → LegacyMovieNarratorAdapter → frozen Movie Narrator V1
      → V2PipelineEngine           → ported nodes over time
```

Không rewrite toàn bộ V1, không đổi namespace `movie_narrator` hàng loạt, không xóa compatibility path trước parity/deprecation gate. “REPLACE” trong audit chỉ nghĩa Product V2 path dùng implementation mới; file V1 vẫn được giữ đến removal criteria.

## 2. Verified upstream baseline

Audit ngày 2026-08-15 xác minh trực tiếp local source và remote:

| Item | Verified value |
|---|---|
| Origin | `https://github.com/zcbacxc/movie-narrator.git` |
| Local branch | `main`, clean worktree tại thời điểm audit |
| Remote `main` | `bc2d276cf477fe3ce1a16f9679dcb1d2978e3a74` |
| Local HEAD | cùng commit `bc2d276`, subject `v1.1.0 community and polish (code changes) (#156)` |
| Annotated tag `v1.1.0` | tag object `c6f30d85...`, peeled commit `bc2d276...` |
| Annotated tag `v1.0.0` | tag object `27e866a7...`, peeled commit `121b883...` |
| License | AGPL-3.0-or-later headers/project metadata |
| V1 contract | `movie_narrator.contract`, `CONTRACT_VERSION = (1,0,0)` |

Remote main đúng local baseline tại thời điểm audit. Phase 0 vẫn phải ghi lại immutable baseline branch/tag/report trong target repo; documentation audit không tạo branch hoặc chạy application tests.

## 3. Classification vocabulary

Mỗi V1 module dùng đúng một disposition:

- **REUSE**: giữ nguyên trong legacy path/tests/assets, không sửa thuật toán.
- **WRAP**: gọi implementation V1 qua adapter/compatibility boundary; local path/model được translate ở biên.
- **PORT**: chuyển behavior/algorithm/test có giá trị sang V2 contract từng module, không copy global/path coupling.
- **REFACTOR**: thay cấu trúc đáng kể nhưng giữ behavior, có side-by-side parity và rollback.
- **REPLACE**: Product path cần implementation khác vì requirement/boundary không thể đáp ứng; V1 compatibility vẫn sống trong window.
- **IGNORE**: không nằm target baseline; chỉ giữ trong legacy/reference, không port nếu chưa có product decision.

Không dùng REPLACE chỉ vì style/code cũ. Module-level evidence và reason nằm ở `UPSTREAM-MODULE-AUDIT.md`.

## 4. Subsystem migration matrix

| V1 subsystem | Current implementation verified | Target | Strategy | Compatibility layer | Tests required | Removal criteria |
|---|---|---|---|---|---|---|
| Contract/models | `contract.py` export versioned surface; mutable path-heavy `Context`/typed segments/scenes/matches. | Versioned engine/domain contracts with Artifact refs. | `REUSE` contract + `WRAP` model mapper; port value semantics selectively. | Legacy DTO mapper; contract version pin. | import/API contract, field mapping, path scrub, golden model serialization. | All consumers migrated and one announced major deprecation window elapsed. |
| Fixed runner | Exact 16 `STEPS`; StepRegistry hard/soft metadata; `start_step`; local `pause_at`; PipelineStatus for soft steps. | DAG Pipeline/PipelineRun/JobStep runtime. | `WRAP` runner first, V2 runtime new; no big-bang edit. | LegacyMovieNarratorAdapter ordered compatibility Pipeline. | order/aliases, hard-soft/strict, start/pause, output parity, rollback route. | Every required node routed V2 with parity; compatibility suite no direct runner consumer. |
| Workflow loader | YAML `safe_load`, extra-forbid schema, relative path resolution, aliases and merge/precedence. | Product command/Pipeline snapshots. | `WRAP` legacy translator; `PORT` validation behavior where applicable. | Legacy config endpoint/CLI translator. | CLI > YAML > Settings, aliases, unknown keys, relative paths, security overrides. | Legacy config deprecation complete and import tool retained separately. |
| REST daemon | stdlib HTTP TaskAPIServer, X-API-Key, tasks/batches/schedules/DLQ/artifacts/health. | Go Product API/control plane + auth/domain resources. | `REPLACE` product surface for boundary/security reasons; retain V1 listener/gateway. | Versioned route mapper `/tasks...` to V2 or frozen daemon behind internal boundary. | Language-neutral contract/OpenAPI snapshots, route/status/result/auth/limits/filename guards. | Compatibility clients migrated; explicit breaking-change release approved. |
| Queue/task storage | In-process ThreadPoolExecutor `LocalTaskQueue`; JSON model store; replicas do not share a broker/state. | PostgreSQL state/outbox + Redis queue + leases. | `REPLACE` product state/queue; `WRAP` V1 queue only in legacy mode. | Task ↔ Job status/result mapper. | duplicate/reclaim/crash/lease/DLQ; V1 queue regressions. | No Product Job depends on local JSON/thread queue; legacy daemon retired. |
| Checkpoint/retry/DLQ | Per-task atomic JSON checkpoint after completed steps; resume next step; checkpoints retained failure/cancel, removed success; retry/backoff + JSON DLQ. | DB/Artifact checkpoint, canonical retry and DLQ. | `PORT` semantics/evidence; `WRAP` legacy files. | Checkpoint importer/mapper. | crash at each step, fingerprint invalidation, retry exhaustion/replay, missing artifact. | V2 resume parity proven and old checkpoint migration window closed. |
| Artifact/storage | Local/S3 `StorageBackend`, key normalization/path guard, lifecycle sweeper; TaskResult still exposes paths. | StoragePort + Asset/Artifact metadata + Local/S3/MinIO. | `PORT` backend guards/protocol; wrap path materialization; replace TaskResult path identity. | Adapter downloads inputs to sandbox/uploads outputs. | storage conformance, traversal/symlink, checksums, lifecycle/protection, presign ACL. | No domain/API/checkpoint contains V1 raw path. |
| Providers | Global mutable registry; TTS/Vision protocols; LLM/research factories less uniform; remote proxy helpers. | Typed LLM/VLM/TTS/ASR/Embedding ports/resolver. | `REFACTOR` registry and `PORT` adapters. | V1 provider settings translated into ProviderConfiguration snapshots. | provider conformance/error/redaction/cache/fallback. | All pipeline calls use ports; no provider-specific branch outside adapters. |
| TTS | Protocol + Edge/OpenAI/MiMo providers, factory/cache/voice map; outputs to Path. | TTS port + Narration/Artifact. | `PORT`, preserving cache semantics; legacy remains wrapped. | request/result/path mapper. | voices, cache key, async/cancel, provider parity, Edge policy. | V2 selected providers pass golden audio metadata and compatibility fallback no longer needed. |
| ASR/alignment | WhisperX/faster-whisper/FunASR selection/fallback; timed segments; soft degradation. | ASR port + alignment node/timing Artifact. | `PORT`. | V1 align node behind adapter. | backend matrix, word/segment precision, fallback/degraded metadata, optional deps. | Approved V2 backend matrix and timing parity on corpus. |
| Scene detection | PySceneDetect ContentDetector; optional/soft; full-length fallback when zero scenes. | Scene node/records with source revision/artifacts. | `PORT`. | V1 `Scene` mapper and one-scene compatibility behavior. | thresholds, zero-scene fallback, ranges, optional dependency, thumbnails. | Scene corpus parity and migration importer pass. |
| Matching/intelligence | transcript + sentence embeddings + heuristics/top-k/diversity/filters; optional VLM captions/visual feature scaffold. | Multimodal scorer + Character/coverage proposal. | `REFACTOR` behind V2 node; retain V1 fallback. | Match proposal mapper to Timeline builder. | score components, determinism, quality/diversity, degradation, golden matches. | V2 quality gate and user-override tests pass; renderer no longer reads legacy matches. |
| Rendering/subtitles/audio | MoviePy composition + scattered FFmpeg argv calls; subtitles/audio/QA; direct output paths. | Timeline compiler + render/media ports + Artifact commit. | `REFACTOR` incrementally; port utilities; wrap V1 renderer first. | Compile/materialize Timeline into exact V1 inputs where possible. | timeline-only decision, codec/profile, audio/subtitle, timeout/cancel, QA, multi-output. | Three-profile acceptance + parity + no rematching + rollback archived. |
| Plugins | Entry-point auto-discovery executes arbitrary Python; errors warn. | Built-in/allowlisted registry; isolated trusted extension path. | `REPLACE` production loading policy for security; legacy auto-load only explicit compatibility mode. | plugin allowlist/disable switch. | unknown entrypoint disabled, hash/version allowlist, sandbox/secret access tests. | No production path auto-loads unreviewed code. |
| Distributed render/scheduler | Optional best-effort remote render based on health/duration; local fallback; scheduled jobs and batches. Input sharing deployment-specific. | Bounded Go media workers plus isolated Python ML/V1 workers; future capability routing. | `IGNORE` distributed scheduler in early phase; `PORT` useful behavior only after benchmark/decision. | Preserve V1 API if frozen compatibility profile requires. | remote failure/fallback, schedule/batch route compatibility when enabled. | Future design accepted or feature explicitly deprecated. |
| Docker/Compose | Multi-stage CPU/GPU, non-root UID 10001, Python 3.12; Compose uses independent LocalTaskQueue per worker endpoint; optional MinIO examples. | Reproducible API/worker/infra/sandbox deployment. | `PORT` non-root/multi-stage ideas; `REPLACE` Compose topology with shared PostgreSQL/Redis/object storage. | Frozen V1 image for regression. | CPU/GPU build, UID/mount, health/drain, shared queue proof, pinned MinIO. | V2 deployment drills pass and frozen V1 image remains rollback artifact. |
| CI/tests | Python 3.10–3.13 matrix, 90% coverage gate on 3.11, integration/media/plugin/security jobs. | V2 unit/integration/security + V1 regression. | `REUSE` V1 tests; `PORT` fixtures/golden behavior; add V2 suites. | CI runs legacy and new routes. | as named in roadmap; advisory exceptions reviewed. | V1 tests removed only with equivalent archived compatibility evidence after deprecation. |

## 5. Verified V1 behavior to preserve

### Pipeline order

```text
resolve_video → prepare_assets → research_plot → generate_script
→ export_script_md → generate_voice → align_audio → detect_scenes
→ match_clips → mix_bgm → translate_subtitles → generate_subtitle
→ run_qa_gate → render_video → validate_deliverable → export_clips
```

Short workflow aliases include `research`, `align`, `scene`, `match`, `bgm`, `translate`, `export`. Soft-step result vocabulary is V1-only `disabled|skipped|success|failed`; `strict` escalates soft failure.

### Task and reliability

- TaskStatus: `pending|running|retrying|completed|failed|cancelled|dead`.
- LocalTaskQueue uses in-process threads and disk JSON state; running work does not survive process death automatically.
- Checkpoint JSON uses atomic temp-file replace, saves after completed steps, resumes after last completed step.
- Worker retries retryable exceptions with backoff; exhausted retryable task enters DLQ when enabled.
- `pause_at`/`start_step` are local pipeline controls, not persisted product human-review states.

### REST

Verified OpenAPI routes include tasks, task result/artifacts/download, batch/batches, schedules/runs, deadletters/replay, health, ready, info, metrics and OpenAPI. Public bind without API key is refused unless explicit insecure opt-in; loopback can remain unauthenticated for backward compatibility.

### Storage/output

Legacy logical outputs: `final.mp4`, narration audio, `script.md`, subtitle variants, `matches.json`, `metadata.json`, optional clips. Compatibility downloads keep names; V2 canonical identity is Artifact ID.

### Configuration/providers

TaskRequest accepts `format` alias for `video_format`, bounded request fields, workflow step map and params. Workflow YAML uses safe loader and strict schema. Provider registry is global and partially protocol-validated; it is not sufficient as V2 product configuration architecture.

## 6. Backward compatibility policy

| Surface | Preserve during strangler | Explicitly not implied |
|---|---|---|
| CLI | create/config/serve/submit/status/list/cancel/resume and frozen flags/aliases selected in Phase 0 profile. | New Product commands need not mimic V1 internals. |
| REST | Frozen route/method/status/request/result contract including optional batch/schedule/DLQ if profile includes them. | Browser access to engine key; Product API naming based on V1 details. |
| Config | CLI > YAML > Settings, `format` alias, known workflow keys/params. | Project config overriding production executable/secret policy. |
| Pipeline | 16-step order, hard/soft/strict, start/pause semantics. | Fixed order as V2 architecture. |
| Providers | Selected provider names/voices/cache behavior. | Global registry/provider branches in V2 pipeline. |
| Outputs | Logical roles/legacy filenames and metadata evidence. | Absolute paths as API/domain identity. |

Any intentionally removed behavior must be listed in a release compatibility profile as:

```text
BREAKING CHANGE
Surface/behavior:
Reason:
Replacement/migration:
First incompatible version:
Rollback/window:
```

No breaking behavior is approved by this audit.

## 7. Incremental strangler sequence

1. Freeze exact upstream commit, environment, compatibility profile and golden outputs.
2. Add Product API/DB/Redis/Storage boundaries without changing V1 engine.
3. Add VideoEngine port + LegacyMovieNarratorAdapter.
4. Run Product Job end-to-end through V1, converting refs/paths/artifacts/events.
5. Introduce V2 DAG/state/checkpoint while V1 node execution remains adapter-backed.
6. Port one node/subsystem at a time behind Pipeline version/feature route.
7. Build canonical Timeline before changing renderer decision source.
8. Switch renderer only after Timeline/compiler/profile parity.
9. Keep rollback route and old data readable through compatibility window.

Replacement is never a vague “rewrite engine” phase. Every module row in `UPSTREAM-MODULE-AUDIT.md` names tests and removal criteria.

## 8. Data migration

Legacy output import:

1. Normalize/contain source path in importer-only sandbox.
2. Resolve owner/Project per OQ-09; never infer cross-user ownership.
3. Copy to staging, SHA-256 + probe/validation.
4. Commit Artifact and Asset relations.
5. Parse known JSON/subtitle/script schemas; preserve unknown file as Artifact.
6. Create imported ScriptVersion/Timeline proposal only with traceable evidence.
7. Record migration report and source provenance privately.
8. Retain source until verification window; cleanup is separate action.

V1 `RUNNING` record without live process/lease is not V2 running state. V1 checkpoint is resumable only if every referenced output exists and fingerprint/schema/pipeline mapping passes; otherwise create new Job from last trustworthy input.

## 9. Upstream update procedure (Scenario H)

For each upstream release/commit:

1. `git ls-remote`/fetch under reviewed upstream remote; record old/new peeled commit.
2. Compare `git log`, `diff --stat`, changed modules, contract/API/config/status/output/security/dependency files.
3. Re-run V1 compatibility/golden/security suite on candidate in isolated branch.
4. Update module audit disposition only for actual behavior changes.
5. Decide per change: import into frozen legacy branch, port to V2, both, or ignore with reason.
6. Record license/dependency/advisory change and rollback commit.
7. Never merge upstream wholesale over V2 boundaries.

## 10. Migration gates

- Exact baseline/environment and compatibility profile stored.
- Legacy adapter maps command/status/progress/checkpoint/artifact with no path/secret leak.
- V1 regression + V2 contract tests pass side by side.
- Each node switch has parity evidence, feature route and rollback.
- Data copy uses copy → verify → switch → retain; never destructive in one step.
- Compatibility removal has usage evidence, replacement, announced window and owner approval.
