# 00 — Project Context

## Trạng thái tài liệu

Đây là technical specification chuẩn hóa từ `docs/movie-narrator-rebuild-context/PROJECT_REBUILD_PLAN.md`. Tài liệu này là baseline để review trước khi viết application code. Mọi quyết định kiến trúc đã có trong master plan là bất biến trong giai đoạn thiết kế; các điểm chưa đủ rõ được ghi tại [`OPEN-QUESTIONS.md`](OPEN-QUESTIONS.md), không được tự suy diễn khi implement.

## Mục tiêu

Xây một **AI Video Production Engine** có Movie Narrator V1 làm reference implementation và legacy engine, nhưng sản phẩm mới có:

- backend/frontend tách biệt;
- có web client và desktop client dùng chung contract/SDK, đều kết nối Product API;
- project state có thể chỉnh sửa và render lại;
- pipeline dạng node, có checkpoint, pause/resume, partial execution;
- Timeline là nguồn sự thật cho renderer;
- provider abstraction cho LLM, VLM, TTS, ASR, Embedding;
- worker CPU/GPU tách biệt khi cần;
- storage abstraction, không truyền filesystem path qua product API;
- nhiều output format từ cùng một project;
- backward compatibility trong suốt giai đoạn strangler migration.

Movie recap chỉ là workflow đầu tiên. Về sau có thể thêm documentary, shorts, highlights, trailer, commentary, reaction và custom workflow.

## Phạm vi

### Trong phạm vi

- Product backend: project, asset, job, pipeline, artifact, timeline, script, scene, character, render.
- AI/video engine: phân tích media, tạo script, narration, alignment, scene intelligence, matching, timeline proposal, render.
- Worker: lấy job, thực thi node, retry/cancel/checkpoint, phát progress.
- Storage: PostgreSQL cho metadata, Redis cho queue/event coordination, S3-compatible/MinIO cho blob.
- Clients: project dashboard, script/scene/subtitle/voice/timeline editor và render panel trên
  web browser và desktop app; cả hai chỉ gọi Product API.
- Adapter và migration từ Movie Narrator V1.

### Ngoài phạm vi giai đoạn đầu

- billing, subscription, quota thương mại;
- Kubernetes, microservices quá nhỏ, autoscaling phức tạp;
- rewrite toàn bộ V1 trước khi có adapter;
- public plugin marketplace;
- cam kết provider/model cụ thể ngoài interface;
- thay đổi license hoặc xóa attribution của upstream.

## Các quyết định kiến trúc bắt buộc giữ

1. Web browser và desktop app không gọi trực tiếp Movie Narrator API, engine, worker hoặc provider.
2. Product API là FastAPI; web là Next.js/React; desktop là native shell mỏng dùng chung SDK/contracts.
3. Client có thể chạy trên máy người dùng; server/API/worker/storage có thể chạy trên VPS riêng.
4. PostgreSQL là database sản phẩm; Redis là queue/event coordination; S3/MinIO là object storage.
5. Engine được gọi qua `VideoEngine` interface; implementation đầu tiên là `LegacyMovieNarratorAdapter`.
6. V2 dùng pipeline node độc lập thay cho chuỗi function cứng.
7. `Project`, `Asset`, `Job`, `JobStep`, `Artifact`, `Timeline` là nền móng.
8. AI tạo proposal; user hoặc automation editor có thể override.
9. Timeline là nguồn sự thật của renderer, không phải output trực tiếp của AI.
10. Worker càng stateless càng tốt; media không tin cậy phải chạy trong sandbox.
11. Không hard-code provider, model hoặc filesystem path.
12. Giữ namespace `movie_narrator` trong migration; namespace V2 dùng `nh_media`.
13. Không dùng `your_engine`, `video_engine` hoặc `frameforge` làm public V2 namespace ở thời điểm này.
14. Giữ regression test V1 và upstream remote/branch baseline.

## Ranh giới hệ thống

```text
User
  ├─ HTTPS → Web App (Next.js/React) ───────┐
  └─ HTTPS → Desktop App (native shell) ────┴─ product auth/session
                                             ▼
Product API (FastAPI)
  ├── PostgreSQL: metadata, versions, state, audit
  ├── Redis: queue, lease, event fan-out
  └── S3/MinIO: source media, intermediate data, renders
          │ internal command
          ▼
VideoEngine interface
  ├── LegacyMovieNarratorAdapter → Movie Narrator V1
  └── V2 Pipeline Runtime → AI / Media / Timeline / Renderer
          │
          ▼
Worker pools (AI/ML/Render)
```

### Product backend

Sở hữu identity, workspace/project, authorization, lifecycle metadata, API contract, upload orchestration, idempotency, event API và database transaction. Không chứa thuật toán media/AI.

### AI/video engine

Nhận engine command có ID và storage references; xử lý research, script, TTS, ASR, scenes, characters, matching, timeline proposal, audio, subtitle, render và QA. Không biết user, billing, subscription hay UI.

### Worker

Nhận một execution lease, tải input artifact cần thiết, chạy engine node, ghi checkpoint/artifact/event, heartbeat và trả terminal outcome. Worker không sở hữu business state lâu dài.

### Storage

PostgreSQL giữ metadata có cấu trúc và trạng thái; Redis giữ dữ liệu ngắn hạn/coordination; object storage giữ byte lớn. Không dùng raw local path làm API identity.

### Web và desktop clients

Web browser và desktop app hiển thị/chỉnh sửa product state qua Product API, subscribe progress qua
SSE/WebSocket, không gọi provider hoặc worker trực tiếp. Desktop chỉ được cấp quyền native tối
thiểu (file picker, download/export và notification nếu cần); không chứa Product DB, provider
secret hoặc engine runtime. Client không là nguồn sự thật của pipeline/timeline.

## Các mode vận hành

### Automatic mode

```text
Source asset → AI pipeline → timeline → render → artifacts
```

### Studio mode

```text
Source asset → research/script → human review
→ voice/alignment → human review
→ scene/match/timeline → human review
→ render
```

Mỗi human gate phải là trạng thái persisted của Job/JobStep, không phụ thuộc browser còn mở.

### Local/offline mode

Có thể chạy local Whisper, local VLM, local LLM, local TTS và FFmpeg; media không rời máy. Product API/worker vẫn dùng cùng interface, chỉ thay provider và storage backend.

## Nguyên tắc dữ liệu

- Mọi entity dùng opaque ID, không expose đường dẫn vật lý.
- Byte lớn luôn nằm trong object storage và được tham chiếu bằng `Artifact`/`Asset`.
- Entity mutable có `revision`/optimistic concurrency; output đã publish là immutable.
- Pipeline node chỉ đọc input đã khai báo và ghi output artifact/state đã khai báo.
- Mỗi execution có `pipeline_version`, provider/model snapshot và input fingerprint để tái lập.
- Event là append-only; trạng thái hiện tại trong PostgreSQL là materialized view của command execution, không thay thế audit event.

## Nguồn tham chiếu

- Master context: `docs/movie-narrator-rebuild-context/PROJECT_REBUILD_PLAN.md`.
- Upstream/reference: `references/movie-narrator/`.
- V1 pipeline order: `references/movie-narrator/src/movie_narrator/pipeline/runner.py`.
- V1 public contract: `references/movie-narrator/src/movie_narrator/contract.py`.
- V1 task/checkpoint/artifact: `references/movie-narrator/src/movie_narrator/cloud/`.
- V1 typed models: `references/movie-narrator/src/movie_narrator/models.py` và `cloud/models.py`.

## Bản đồ tài liệu

| File | Nội dung |
|---|---|
| `01-ARCHITECTURE.md` | component boundary, deployment, dependency direction |
| `02-DOMAIN-MODEL.md` | domain object, lifecycle, state machine |
| `03-DATABASE-SCHEMA.md` | PostgreSQL tables, fields, relation, index |
| `04-API-CONTRACT.md` | REST resource contract và error/concurrency rules |
| `05-PIPELINE-SPEC.md` | DAG runtime, node catalog và execution policies |
| `06-TIMELINE-SPEC.md` | Timeline JSON schema và validation |
| `07-EVENTS-AND-JOBS.md` | queue, retry, checkpoint, progress events |
| `08-SECURITY.md` | trust boundary và production controls |
| `09-MIGRATION-FROM-UPSTREAM.md` | reuse/adapter/refactor/replace và compatibility |
| `10-DEVELOPMENT-ROADMAP.md` | phase, deliverable, acceptance criteria |
| `11-PROVIDER-ARCHITECTURE.md` | LLM/VLM/TTS/ASR/Embedding ports và provider policy |
| `12-STORAGE-ARCHITECTURE.md` | Asset/Artifact/object storage/local cache/upload |
| `13-WORKER-ARCHITECTURE.md` | initial worker, sandbox, lease và future pools |
| `14-DEVELOPMENT-ENVIRONMENT.md` | Arch Linux, uv/Python/FFmpeg/infra reproducibility |
| `CODEX-INSTRUCTIONS.md` | guardrails cho coding agent |
| `IMPLEMENTATION-ORDER.md` | task nhỏ, thứ tự giao việc |
| `OPEN-QUESTIONS.md` | quyết định chưa đủ rõ, options và recommendation |
| `SETUP-PLAN.md` | setup foundation, client/server profiles và V0–V10 certification |
| `PRODUCTION-PLAN.md` | phase execution, client delivery, staging/canary/production gates |
| `PRODUCTION-EXECUTION-PROMPT.md` | prompt vận hành cho agent thực hiện production plan |
| `GLOSSARY.md` | terminology và canonical status vocabulary |
| `UPSTREAM-MODULE-AUDIT.md` | source-level V1 disposition/evidence |
| `SPEC-CONSISTENCY-MATRIX.md` | master/domain/DB/API/event mapping và scenarios A–H |
| `SPEC-AUDIT-REPORT.md` | kết quả consistency audit và scenarios A–H |

## Definition of specification complete

Specification có thể được đánh dấu READY khi các contract nhất quán và mọi điểm chưa chốt được ghi rõ/block đúng task. Tuy nhiên, chỉ được bắt đầu application code sau khi:

- toàn bộ file trong bộ tài liệu này tồn tại;
- các open question của task sắp làm đã được owner chốt;
- consistency check giữa domain, database, API, pipeline, timeline và event contract pass;
- task/phase prerequisites trong roadmap được duyệt;
- agent xác nhận không có code application nào được viết trong giai đoạn documentation-only.
