# Codex Instructions

## 1. Vai trò

Codex làm việc trên hệ thống AI Video Production Engine theo bộ tài liệu trong thư mục `docs/`. `PROJECT_REBUILD_PLAN.md` là master context; bộ technical specification này là bản triển khai có cấu trúc. Khi có khác biệt, không tự đổi quyết định kiến trúc của master plan; ghi điểm khác biệt vào `docs/OPEN-QUESTIONS.md` và dừng phần implementation bị ảnh hưởng.

## 2. Documentation-first gate

Cho tới khi Phase -1 trong `10-DEVELOPMENT-ROADMAP.md` được owner approve:

- không viết application code;
- không tạo migration/runtime/API/frontend/worker implementation;
- chỉ được đọc, phân tích, cập nhật documentation và consistency evidence;
- không dùng “prototype tạm” làm lý do bỏ qua gate.

Sau gate, coding agent vẫn phải làm task theo [`IMPLEMENTATION-ORDER.md`](IMPLEMENTATION-ORDER.md), từng task một, không nhảy phase nếu prerequisite chưa pass.

## 3. Source of truth and scope

Ưu tiên đọc theo thứ tự:

1. user request và quyết định mới đã được approve;
2. root `PROJECT_REBUILD_PLAN.md` pointer and its canonical full document;
3. `GLOSSARY.md`, `00`–`14` và các ADR/open questions đã chốt;
4. `UPSTREAM-MODULE-AUDIT.md` và `references/movie-narrator/` cho V1 behavior/compatibility;
5. implementation hiện tại.

Không lấy assumption trong code V1 làm kiến trúc V2. V1 là legacy/reference; behavior cần preserve phải được ghi trong compatibility contract và regression test.

## 4. Boundary rules

- Product backend sở hữu user/workspace/project/job/database/auth; engine không import chúng.
- Engine sở hữu media/AI/pipeline/timeline compilation; không biết HTTP/session/billing.
- Worker chỉ execute lease và báo progress/checkpoint; không giữ business state duy nhất.
- Frontend không gọi engine/provider trực tiếp, không giữ secret.
- Storage access qua Asset/Artifact refs; không đưa absolute path vào API/domain event.
- Job/JobStep state dùng nguyên canonical vocabulary trong `GLOSSARY.md`; compatibility adapter là nơi duy nhất map `Task/pending/dead/success` của V1.

## 5. Coding rules sau khi được phép implement

1. Dùng interface/port trước implementation và dependency injection cho provider/storage/queue.
2. Giữ `movie_narrator` namespace và public contract trong migration.
3. Một V2 module phải có test và compatibility mapping trước khi route production.
4. Không rewrite nhiều module V1 trong một thay đổi không có parity plan.
5. Node phải idempotent, checkpointable, observable và provider-agnostic.
6. Timeline là nguồn sự thật của renderer; AI chỉ tạo proposal.
7. Artifact có ID/checksum/type/producer; không truyền raw path giữa components.
8. Mọi state transition phải validate và emit audit/progress event.
9. Mọi API mutation có authorization, validation, idempotency hoặc optimistic concurrency phù hợp.
10. Không hard-code model, provider, filesystem path hoặc executable production.
11. Không `shell=True`; subprocess qua wrapper được review.
12. Không auto-load plugin không tin cậy.
13. Secrets không vào logs/events/checkpoints/LLM prompts không cần thiết.

## 6. Migration rules

- Luôn giữ đường rollback về `LegacyMovieNarratorAdapter`.
- Không xóa output/DB/blob legacy trong migration một bước; copy → verify → switch → retention.
- Không mutate approved Script/Timeline; tạo version mới.
- Job snapshot pipeline/provider/input revision tại start; không silently chạy bằng version mới.
- V1 soft-step semantics và short aliases phải được test.

## 7. Open question rule

Nếu gặp decision chưa được chốt hoặc mâu thuẫn:

1. dừng implementation của phần phụ thuộc;
2. ghi vào `OPEN-QUESTIONS.md` với context, 2–3 options, trade-off, recommendation;
3. không tạo default behavior production chỉ vì dễ implement;
4. khi owner chốt, cập nhật docs liên quan và implementation task.

Recommendation trong `OPEN-QUESTIONS.md` không được coi là decision đã approve.

## 8. Required verification per change

Agent phải báo:

- files changed và boundary/module;
- test/validation đã chạy;
- compatibility impact;
- migration/rollback impact;
- security/secret/path review;
- open question mới (nếu có).

Nếu task chỉ là documentation, xác nhận application code không bị thay đổi. Nếu task là migration, test cả old contract và new contract.

## 9. Prohibited shortcuts

- viết code trước documentation gate;
- sửa trực tiếp master plan để làm implementation “khớp”;
- bỏ qua authorization vì endpoint nội bộ;
- upload video qua API request body;
- lấy Redis làm source of truth duy nhất;
- coi worker memory/local disk là durable checkpoint;
- retry mọi exception;
- expose traceback/path/API key;
- xóa upstream license/attribution;
- thay đổi behavior legacy mà không có feature flag/compatibility test.

## 10. Handoff format

Mỗi coding task hoàn thành bằng một handoff ngắn:

```text
Task: Txxx
Status: complete/blocked
Implemented boundary: ...
Tests: ...
Compatibility: ...
Security: ...
Open questions: ...
Next task: ...
```
