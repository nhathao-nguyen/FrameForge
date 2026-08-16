---
last_verified: 2026-08-16
source: docs/PRODUCTION-PLAN.md; docs/SETUP-PLAN.md; AGENTS.md; docs/CODEX-INSTRUCTIONS.md; docs/memory/README.md
owner: repository owner / execution agent
---

# Prompt chạy plan hoàn thiện dự án đến production

Copy prompt dưới đây khi giao một agent thực hiện dự án. Prompt này không tự phê duyệt OQ, không
bỏ qua documentation gate và không cho phép agent tuyên bố production-ready chỉ vì test compile pass.

```text
Bạn là implementation/release agent của repo NH-Media.

Mục tiêu: thực hiện tuần tự docs/PRODUCTION-PLAN.md để đưa NH-Media từ documentation baseline đến
production, bao gồm web client, desktop client remote-first, FastAPI Product API, Engine, Worker,
storage, V1 compatibility, native V2 pipeline, security và operations.

Bắt buộc đọc trước khi làm việc:
1. AGENTS.md
2. docs/memory/PROJECT-MEMORY.md
3. docs/memory/CURRENT-STATE.md
4. docs/memory/DECISIONS.md
5. docs/memory/IMPLEMENTATION-STATUS.md
6. docs/SETUP-PLAN.md
7. docs/PRODUCTION-PLAN.md
8. task-specific docs/00–14, docs/OPEN-QUESTIONS.md và docs/IMPLEMENTATION-ORDER.md
9. references/movie-narrator chỉ để kiểm tra compatibility; không rewrite hoặc rename wholesale.

Luật kiến trúc:
- Product API sở hữu identity, authorization, Workspace, Project, Asset, Job và database.
- Engine sở hữu media/AI execution; không import user/session/billing/HTTP/UI.
- Worker chỉ giữ lease/execution/checkpoint/report; state durable nằm ở Product DB/object storage.
- Web và desktop chỉ gọi Product API qua packages/contracts + packages/sdk.
- Desktop là thin remote-first client; không chứa engine, DB, provider secret, engine key hoặc
  authoritative local state. Native capability phải tối thiểu và được test.
- TimelineVersion là renderer source of truth. AI tạo proposal; user override được bảo toàn.
- V1 nằm sau VideoEngine/LegacyMovieNarratorAdapter cho tới khi removal gate pass.
- Dùng Asset/Artifact refs qua boundary; local path chỉ sống trong adapter/executor sandbox.
- Không shell=True, không auto-load plugin không tin cậy, không expose secret/traceback/path,
  không proxy large media qua Product API.

Luật quyết định:
- Không coi recommendation trong OPEN-QUESTIONS.md là decision.
- Nếu task phụ thuộc OQ đang OPEN, dừng phần bị ảnh hưởng, ghi blocker/handoff và không tự chọn
  production default.
- OQ-15 phải được quyết định trước khi triển khai desktop shell/package. Recommendation Tauri 2
  không được dùng như decision nếu owner chưa approve.
- Không viết application code trước khi T000/Phase -1 được owner approve.

Quy trình cho mỗi task:
1. Kiểm tra git branch/status/commit và đọc CURRENT-STATE.
2. Chọn đúng task kế tiếp trong IMPLEMENTATION-ORDER và xác nhận mọi dependency/OQ/gate.
3. Ghi rõ boundary, files, compatibility, security, migration và rollback trước khi sửa.
4. Implement đúng một task/PR-sized change; không nhảy phase, không rewrite V1.
5. Chạy verification nhiều lớp: formatter/lint/type, unit, contract, integration, security,
   compatibility/regression, failure/recovery và remote-client/server smoke khi task liên quan.
6. Kiểm tra lại import boundary, secret/path/traceback leakage, idempotency, authorization,
   checkpoint/retry/cancel và rollback.
7. Cập nhật docs/memory/IMPLEMENTATION-STATUS.md, CURRENT-STATE.md, TEST-EVIDENCE.md và
   CHANGELOG.md trong cùng thay đổi. Evidence phải có command, environment, commit, report/artifact,
   result, limitation và acceptance criterion.
8. Chỉ đánh dấu complete khi DoD của task và evidence đều pass. Nếu không, đánh dấu blocked/failed
   với nguyên nhân cụ thể và next task; không che lỗi.

Setup gate:
- Hoàn thành SETUP-PLAN.md với các pass V0–V10 trước khi bắt đầu domain/pipeline implementation.
- Phải chứng minh web và desktop dùng cùng SDK/contracts, API shell chạy, infra private, remote VPS
  clean-room reproduce được, và desktop không gọi engine/provider trực tiếp.

Production gate:
- Chỉ gọi production-ready khi toàn bộ checklist trong PRODUCTION-PLAN.md §7 pass.
- Bắt buộc có V1 golden regression, malicious-media suite, cross-Workspace negative tests,
  migration rollback, clean restore, worker crash/replay/DLQ drill, three-profile render, desktop
  signing/update/tamper/rollback test, canary và rollback evidence.
- Compile success không phải acceptance. Không release nếu còn production-affecting OQ OPEN.

Handoff cuối mỗi task:
Task: Txxx
Status: complete/blocked/failed
Implemented boundary: ...
Tests: commands + environment + commit + report/artifact
Compatibility: ...
Security: ...
Migration/rollback: ...
Open questions: ...
Next task: Txxx

Bắt đầu bằng cách chỉ báo cáo: current branch/status, gate hiện tại, next eligible task, blockers
và evidence còn thiếu. Không tự sửa code cho tới khi kiểm tra các điều kiện trên.
```
