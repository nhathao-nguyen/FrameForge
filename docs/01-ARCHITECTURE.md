# 01 — Architecture Specification

## 1. Architectural shape

Hệ thống bắt đầu là modular monorepo/deployment đơn giản, không tách thành nhiều microservice độc lập ngay. Các boundary logic phải rõ để sau này scale thành worker pool.

```text
apps/web (browser) ──HTTPS──┐
apps/desktop ───────HTTPS───┴──> apps/api ──SQL──> PostgreSQL
                         │
                         ├── Redis: queue, lease, pub/sub
                         ├── S3/MinIO: presigned upload/download
                         └── Trusted Orchestrator / EngineGateway
                                │
                 ┌──────────────┴──────────────┐
                 ▼                             ▼
        LegacyMovieNarratorAdapter       V2 Pipeline Runtime
                 │                             │
                 └──────────────┬──────────────┘
                                ▼
                       Engine Worker
                    (single pool initially)
                                │
                                ▼
                    disposable media executor
```

## 2. Ownership boundaries

| Boundary | Sở hữu | Không được sở hữu |
|---|---|---|
| Product API | auth, workspace, project, asset registration, job command, DB transaction, API version | scene matching, TTS call, FFmpeg, provider-specific prompt |
| Engine | domain-neutral media/AI execution, node contracts, artifact outputs, provider calls | user/session/billing, HTTP response shape, UI state |
| Worker controller | queue consumption, execution lease, heartbeat, sandbox launch, result report | business authorization, arbitrary product-table mutation, permanent project truth |
| Media executor sandbox | chạy đúng một node với scoped inputs/capability | DB/Redis credentials, product/user secrets, durable state |
| Storage | byte durability, metadata persistence, retention/lifecycle | AI decisions, timeline editing semantics |
| Frontend | view/edit commands, local draft UX, progress presentation | provider secret, direct storage credential, render orchestration |

### Dependency direction

```text
web/desktop → packages/sdk + contracts → Product API → domain/application ports
                    ├── repository ports → PostgreSQL
                    ├── blob ports → object storage
                    ├── queue ports → Redis
                    └── engine port → VideoEngine

engine → contracts / provider ports / media adapters
worker controller → trusted execution-state port + engine
media executor → engine node contract + scoped artifact/provider ports
legacy adapter → movie_narrator.contract (stable surface)
```

Core domain không import FastAPI, Pydantic HTTP models, Redis client, boto3 hay UI code. Adapter/infrastructure là outer layer.

## 3. Proposed repository layout

```text
apps/
  web/                         # Next.js/React, UI only
  desktop/                     # Tauri 2 shell + shared client SDK, no business backend
  api/                         # FastAPI routes, auth, application services
services/
  engine/
    core/                      # engine-neutral IDs, context, errors
    pipeline/                  # DAG runtime, node registry, checkpoint
    media/                     # ffprobe/FFmpeg/MoviePy/PySceneDetect adapters
    ai/                        # prompt, matching, scene intelligence
    speech/                    # ASR/alignment orchestration
    tts/                       # narration orchestration
    rendering/                 # timeline compiler and render profiles
    providers/                 # LLM/VLM/TTS/ASR/Embedding ports and registry
  worker/                      # queue consumer, lease, sandbox, heartbeat
packages/
  contracts/                   # versioned JSON/Pydantic contracts
  sdk/                         # shared web/desktop Product API client
  shared/                      # IDs, time, error codes, validation helpers
infra/
  postgres/
  redis/
  minio/
  docker/
  reverse-proxy/
tests/
  unit/
  integration/
  regression/
  e2e/
docs/
tools/
```

`movie_narrator` vẫn là legacy namespace/reference trong thời gian migration; không đổi namespace hàng loạt.

## 4. Runtime flows

### Project and upload

```text
POST /projects
POST /projects/{id}/assets/upload-session
  → API trả presigned multipart URLs
Browser hoặc desktop upload trực tiếp object storage
POST /projects/{id}/assets/{asset_id}/complete
  → API enqueue probe/ingest job
```

Video 5–50 GB không đi xuyên qua FastAPI.

### Job execution

```text
API transaction:
  create jobs + job_steps + outbox event
  commit
outbox publisher → Redis queue
worker claims lease
  → loads project snapshot and asset/artifact refs
  → engine.execute()
  → persists checkpoint/artifact/event
  → terminal transition
```

### Studio edit

Frontend đọc version hiện tại, gửi `If-Match: <revision>`, API tạo revision mới cho Script/Timeline. Job render tiếp theo tham chiếu exact version, không đọc trạng thái UI tạm thời.

### Multi-output

Một project giữ canonical timeline. Mỗi `Render` chọn `timeline_version_id` + `render_profile`; không chạy lại research/script/scene pipeline chỉ vì đổi aspect ratio.

## 5. Deployment modes

### Development/local — initial architecture

Web dev server và desktop dev shell đều gọi một API process; phía server có Redis, PostgreSQL,
MinIO và **một Engine Worker pool**. Worker controller có service identity giới hạn; node media
chạy trong temp workspace/sandbox và không giữ DB/Redis credential. Có thể dùng
LocalStorage/LocalQueue adapter cho unit test nhưng interface không đổi.

### Staging/production baseline — vẫn một worker class

```text
HTTPS reverse proxy
  ├→ web static/SSR delivery (hoặc CDN)
  └→ FastAPI API replicas
  → PostgreSQL
  → Redis
  → S3/MinIO private bucket
  → Engine Worker replicas
```

Media worker có filesystem tạm riêng, non-root, resource limit và không có product secrets không cần thiết.

Desktop client được phân phối riêng cho máy người dùng. Desktop chỉ lưu endpoint cấu hình, session
được bảo vệ bởi OS credential store và local draft tối thiểu; mọi product state durable, Job và
Artifact vẫn thuộc server. Không expose API/worker port public ngoài HTTPS Product API.

Không yêu cầu Kubernetes, GPU scheduler riêng hoặc ba service worker trong Phase 1–3. Scale ngang cùng worker image trước; chỉ route capability tới pool riêng sau benchmark và operational evidence.

### Future scale-out architecture

Queue routing theo capability:

- `ai`: network-bound, provider credentials cần thiết;
- `ml`: GPU/Whisper/VLM/embedding;
- `render`: CPU/GPU + disk I/O;
- `probe`: media metadata, CPU nhẹ.

Tách pool là deployment concern; pipeline contract không thay đổi.

### Worker trust split

```text
Public API
  → trusted orchestration/application service
      → queue + state transition repository
      → worker controller (scoped service identity)
          → disposable media executor (untrusted-input sandbox)
```

Trusted orchestration là owner duy nhất của Job/JobStep transition. Worker controller gửi claim/heartbeat/progress/result qua execution-state port; media executor chỉ nhận manifest scoped cho một attempt. Cách port được transport bằng internal API hay in-process repository là implementation detail, nhưng media executor không được có broad PostgreSQL, Redis hoặc user credential.

## 6. Engine gateway

Product API chỉ gọi port tương đương:

```text
VideoEngine
  create_execution(command) → pipeline_run_id
  start_execution(pipeline_run_id)
  pause_execution(pipeline_run_id)
  resume_execution(pipeline_run_id, checkpoint_ref)
  cancel_execution(pipeline_run_id)
  get_status(pipeline_run_id)
  list_artifacts(pipeline_run_id)
```

Implementation đầu tiên map tới `LegacyMovieNarratorAdapter`, implementation sau là `V2PipelineEngine`. Gateway chuyển đổi domain command thành engine-neutral command và ngược lại.

Engine không trả raw HTTP response và không ghi trực tiếp vào bảng product nếu chạy remote; worker/reporting layer materialize kết quả.

## 7. Consistency and reliability requirements

- API write có idempotency key cho create job/upload completion.
- Queue message chỉ chứa ID/reference, không chứa video bytes.
- Job claim dùng lease có expiry; worker heartbeat gia hạn.
- Node execution idempotent theo `(pipeline_run_id, job_step_id, input_fingerprint, attempt)`; artifact commit atomic.
- Checkpoint sau mỗi node completed/skipped và trước human gate.
- Retry chỉ áp dụng lỗi transient; không retry validation/security/user input.
- Cancellation cooperative tại node boundary; node media dài phải kiểm tra cancellation giữa các chunk.
- Mọi event có `event_id`, `occurred_at`, `job_id`, `sequence` và correlation ID.
- Tất cả provider calls có timeout, retry policy, circuit breaker và redacted logging.
- Product API phải query được status mà không cần worker còn sống.
- Client reconnect được bằng snapshot + event replay; tab/app đóng không làm mất Job state.
- Desktop offline chỉ là local draft/import mode; không được mô phỏng server Job state.

Chi tiết provider, storage và worker nằm lần lượt ở `11-PROVIDER-ARCHITECTURE.md`, `12-STORAGE-ARCHITECTURE.md` và `13-WORKER-ARCHITECTURE.md`.

## 8. Observability

Bắt buộc:

- structured JSON logs với `request_id`, `correlation_id`, `project_id`, `job_id`, `job_step_id`, `worker_id`;
- metrics queue depth, lease age, node duration, retry count, provider latency/cost, artifact bytes, render QA;
- trace boundary API → queue → worker → provider/media subprocess;
- health/readiness tách biệt; readiness fail khi không nhận job, health vẫn mô tả tình trạng.

Metadata V1 như `metadata.json`, match summary, duration metrics, alignment diagnostics và quality dashboard được lưu như artifact/audit metadata trong migration; không làm API phụ thuộc vào một JSON blob duy nhất.

## 9. Non-goals của architecture hiện tại

- Không đưa users/subscriptions/billing vào engine.
- Không bắt buộc Kubernetes.
- Không chuyển mọi V1 module sang V2 trong một PR.
- Không để frontend giữ API key của engine/provider.
- Không để web/desktop client chứa Product DB credential, provider secret hoặc engine credential.
- Không buộc desktop client cài Python/FFmpeg/ML runtime để dùng remote-server mode.
- Không coi local filesystem là durable storage.
