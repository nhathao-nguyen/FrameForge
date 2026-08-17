export type SharedPrimitives = {
  schema_version: "shared-primitives/v1";
  ids: { workspace_id: string; project_id: string; artifact_id: string };
  timestamp: string;
  media: { start_sec: number; end_sec: number; duration_sec: number };
  concurrency: { revision: number; etag: string; expected_version: number };
  error: { code: string; category: string; retryable: boolean; safe_message: string };
};

export function validateSharedPrimitives(value: SharedPrimitives): void {
  if (value.schema_version !== "shared-primitives/v1" || !value.timestamp.endsWith("Z")) {
    throw new Error("unsupported shared primitive fixture");
  }
  for (const id of Object.values(value.ids)) {
    if (!/^[a-z][a-z0-9_-]{2,127}$/.test(id)) throw new Error("invalid opaque ID");
  }
  if (value.media.start_sec < 0 || value.media.end_sec < value.media.start_sec) {
    throw new Error("invalid media range");
  }
  if (value.concurrency.revision < 0 || !/^\"[A-Za-z0-9._~-]+\"$/.test(value.concurrency.etag)) {
    throw new Error("invalid concurrency primitive");
  }
  if (!value.error.safe_message || value.error.safe_message.length > 512) throw new Error("unsafe error");
}
