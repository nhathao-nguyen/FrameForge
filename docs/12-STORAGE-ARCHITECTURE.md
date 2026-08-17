# 12 — Storage Architecture

## 1. Storage layers

```text
PostgreSQL metadata
  Asset / Artifact / state / ownership / checksum / refs
                 │
                 ▼
StoragePort ── LocalStorage | S3Storage | MinIOStorage
                 │
                 ▼
       immutable objects / multipart staging

Worker LocalCache
  lease-scoped resolved files; disposable; never global identity
```

- **Asset**: logical product input.
- **Artifact**: immutable blob manifest.
- **Object Storage**: durable bytes addressed by backend/key/version internally.
- **Local Cache**: disposable materialization for FFmpeg/ML.
- **Database Metadata**: ownership, state, relations, checksums and canonical refs.

## 2. StoragePort

```text
StoragePort
  initiate_upload(scope, constraints) -> UploadSession
  presign_part(upload_id, part_number) -> SignedRequest
  complete_upload(upload_id, parts) -> StagedObject
  abort_upload(upload_id)
  stat(locator) -> ObjectMetadata
  open_read(locator, byte_range?) -> stream
  put_staged(scope, stream/path_handle, metadata) -> StagedObject
  promote(staged, final_key, preconditions) -> ObjectMetadata
  materialize(locator, cache_scope) -> LocalHandle
  presign_download(locator, ttl, disposition) -> SignedURL
  delete(locator, version, preconditions)
  list_staging(prefix, cursor) -> page
```

Domain/API dùng Artifact ID; `StorageLocator {backend, object_key, object_version}` chỉ tồn tại trong infrastructure record/port. `LocalHandle.path` không serialize ra domain, event, checkpoint hoặc HTTP.

## 3. Upload lifecycle

```text
Browser → POST upload-session
API → Asset(uploading) + AssetUpload(active) + presigned URL(s)
Browser → S3/MinIO multipart directly
Browser → POST complete
API verifies provider parts/object metadata
  → Asset(uploaded)
  → staged Artifact(original)
  → validation/probe Job
  → checksum + magic/MIME + ffprobe + security scan
  → Artifact(committed) + Asset(ready)
       or Artifact(quarantined) + Asset(quarantined|failed)
```

Video bytes không đi qua Product API hoặc Redis. Complete là idempotent; request khác parts/checksum cho cùng idempotency key trả conflict.

Upload states:

- AssetUpload: `active|completed|aborted|expired`.
- Asset: `pending_upload|uploading|uploaded|validating|ready|failed|quarantined|deleted`.
- Artifact: `staged|committed|quarantined|expired|deleted`.

Abandoned upload sweeper chọn `asset_uploads.status=active AND expires_at < now`, reconcile object storage multipart state, abort và chuyển upload `expired`; Asset không được `ready`.

## 4. Validation boundary

API chỉ kiểm tra declared size/MIME/quota trước presign. Disposable validation worker kiểm tra actual:

- object exists, expected size/parts and checksum;
- MIME/magic bytes, container/stream metadata bằng pinned ffprobe;
- duration, dimensions, frame rate, stream count và codec allow/deny policy;
- malformed/decompression/resource-bomb limits;
- malware scanner nếu deployment policy yêu cầu;
- filename không ảnh hưởng key hoặc executable selection.

Security violation chuyển `quarantined` và fail closed. Unsupported nhưng benign media chuyển `failed` với safe diagnostic; không soft-skip validation.

## 5. Object key policy

Key do server tạo, ví dụ:

```text
workspaces/{workspace_id}/projects/{project_id}/
  assets/{asset_id}/original/{artifact_id}
  jobs/{job_id}/steps/{node_key}/{artifact_id}
  renders/{render_id}/{artifact_id}
  staging/uploads/{upload_id}/...
```

Rules:

- ID segment validated; không dùng filename trực tiếp.
- Từ chối absolute path, `..`, empty/`.` segment, NUL/control char và backslash normalization ambiguity.
- LocalStorage resolve real path và kiểm tra nằm trong configured root; từ chối symlink escape.
- S3/MinIO key normalization dùng cùng conformance tests.
- Listing không phải authorization; caller phải scope project trước khi presign.

## 6. Artifact commit protocol

1. Node ghi output vào sandbox/local handle hoặc staging object.
2. Tính SHA-256, size, MIME/probe metadata.
3. Promote/copy tới immutable final key với precondition “does not exist” hoặc exact version.
4. Transaction insert Artifact `committed`, output relation, checkpoint/JobStep state và event.
5. Nếu DB transaction fail, object là orphan staging/final candidate; reconciler xóa sau grace period.
6. Nếu promote fail, không publish DB Artifact committed.

Artifact dedupe/reuse chỉ khi semantic role, input fingerprint, schema and checksum compatible. Cùng bytes không tự động có cùng authorization/lifecycle.

## 7. Implementations

### LocalStorage

Dùng cho local/offline/dev và executor sandbox. Root phải explicit, không mặc định home/repo root;
atomic rename chỉ khi same filesystem. Không coi local storage là durable production multi-replica backend.

### S3Storage

AWS S3-compatible adapter với private bucket, versioning/lifecycle theo deployment, multipart, checksum và short-lived presigned URL. Credentials lấy từ workload identity/secret manager.

### MinIOStorage

Cùng S3 contract, endpoint custom. Dev Compose pin image/version và non-default credentials; production không expose console/API public trừ authenticated network policy.

## 8. Worker LocalCache

Cache key gồm Artifact ID + object version + checksum. Mỗi attempt có workspace riêng, permission tối thiểu và quota. Cache reuse giữa jobs chỉ cho immutable committed Artifact, phải revalidate checksum; writable work file không được share. Cleanup khi terminal/timeout và periodic stale sweep. Cache miss không làm mất Job state.

## 9. Retention and deletion

- Active Job, valid checkpoint, current Asset original, approved Script/Timeline dependencies và completed Render output tạo protection reference.
- Delete product resource là soft delete; blob sweeper chỉ xóa khi reference/retention/legal hold cho phép.
- `expired` là lifecycle metadata trước physical delete; delete failure retry idempotent.
- Presigned URL TTL ngắn, exact method/key, optional content length/type; URL không được persist vào event/checkpoint.
- Backup PostgreSQL và object storage phải có consistency inventory bằng Artifact checksum.

Retention defaults còn ở OQ-10; implementation không được tự đặt auto-delete source media.

## 10. External tools and research observations

NH-Media workers materialize Artifact refs only inside a lease-scoped sandbox:

```text
ArtifactRef → LocalCache.materialize → LocalHandle
tool output LocalHandle → validate/stage/commit → ArtifactRef
```

Local paths never leave that executor boundary. Research observations about upstream path guards or
S3 concepts may inform NH-Media conformance tests, but upstream classes, JSON stores and filenames
are not product contracts or implementations.

## 11. Acceptance tests

- Local/S3/MinIO adapters pass cùng contract suite.
- multipart complete/abort/expiry và idempotency;
- checksum mismatch, spoofed MIME, oversized/duration/resource-bomb quarantine;
- path traversal, encoded traversal, symlink escape and cross-project presign denial;
- stage/promote/DB failure reconciliation;
- worker crash leaves no committed metadata for incomplete bytes;
- large upload trace proves bytes bypass API/queue;
- multi-output reuse does not duplicate immutable intermediates unnecessarily.
