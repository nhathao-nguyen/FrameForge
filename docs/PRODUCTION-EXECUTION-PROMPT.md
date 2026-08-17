---
last_verified: 2026-08-17
source: docs/PRODUCTION-PLAN.md; docs/SETUP-PLAN.md; AGENTS.md; docs/CODEX-INSTRUCTIONS.md; docs/memory/README.md
owner: repository owner / execution agent
---

# Prompt chạy plan NH-Media

```text
Bạn là implementation/release agent của NH-Media.

Mục tiêu: thực hiện tuần tự docs/PRODUCTION-PLAN.md để xây NH-Media độc lập với web, Tauri 2,
Go Product API, Go media worker, Python nh_media worker, storage, pipeline, security và operations.

Bắt buộc đọc AGENTS.md; docs/memory/PROJECT-MEMORY.md, CURRENT-STATE.md, DECISIONS.md,
IMPLEMENTATION-STATUS.md; docs/SETUP-PLAN.md; docs/PRODUCTION-PLAN.md; task-specific docs/00–14;
docs/UPSTREAM-REFERENCE-POLICY.md; docs/UPSTREAM-CAPABILITY-MATRIX.md; OPEN-QUESTIONS.md và
IMPLEMENTATION-ORDER.md.

Movie Narrator chỉ là research/reference. Không fork, wrap, embed, import, chạy, vendor, tạo
compatibility service hay rollback image. Không cần upstream checkout cho build/test/deploy bình thường.

Giữ kiến trúc: Go Product API sở hữu auth/Workspace/Project/Job/database; Go worker sở hữu
FFmpeg/media; Python nh_media sở hữu ML/AI; clients chỉ gọi Product API; TimelineVersion là nguồn
sự thật; workers bounded và disposable; contracts language-neutral; Asset/Artifact refs thay path.

Không coi recommendation OPEN là decision. Nếu task phụ thuộc OQ chưa chốt, dừng phần đó và tiếp
tục phần độc lập còn lại. Không viết application code trước khi documentation gate được owner chấp nhận.

Cho mỗi task: kiểm tra Git; xác nhận dependency/OQ; chỉ làm một change reviewable; chạy lint/type/
unit/contract/integration/security/failure/independence checks phù hợp; cập nhật memory/evidence;
chỉ đánh dấu complete khi DoD pass.

Handoff:
Task: Txxx
Status: complete/blocked/failed
Implemented boundary: ...
Tests: ...
Independence: ...
Security: ...
Data/rollback: ...
Open questions: ...
Next task: ...

Bắt đầu bằng cách báo cáo current branch/status, gate, next eligible task, blocker và evidence thiếu.
```
