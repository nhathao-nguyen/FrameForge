import type { ProductApiClient } from "@nh-media/sdk";
import { objectValue, stringValue } from "./value-utils";

export async function uploadAsset(api: ProductApiClient, projectId: string, file: File): Promise<void> {
  const sessionEnvelope = await api.createUploadSession(projectId, {
    kind: file.type.startsWith("audio/") ? "audio" : "video",
    filename: file.name, content_type: file.type || "application/octet-stream",
    size_bytes: file.size, multipart: true,
  });
  const asset = objectValue(sessionEnvelope.asset);
  const upload = objectValue(sessionEnvelope.upload);
  const parts = Array.isArray(upload.parts) ? upload.parts : [];
  const part = objectValue(parts[0]);
  const url = stringValue(part.url);
  if (!url) throw new Error("The API did not return a direct upload URL.");
  const headers = Object.fromEntries(Object.entries(objectValue(part.headers)).filter(([, value]) => typeof value === "string")) as Record<string, string>;
  const etag = await api.uploadPart(url, file, headers);
  if (!etag) throw new Error("The storage provider did not return an upload ETag.");
  await api.completeUpload(projectId, stringValue(asset.id), stringValue(upload.id), [{ part_number: 1, etag }]);
}

export async function issueArtifactDownload(api: ProductApiClient, projectId: string, artifactId: string): Promise<string> {
  const value = await api.downloadArtifact(projectId, artifactId);
  return value.url;
}
