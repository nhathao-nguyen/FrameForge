import type { ProductApiClient } from "@nh-media/sdk";
import type { UnknownRecord } from "../types/workspace";
import { stringValue } from "./value-utils";

export interface LoadedScriptState { script: UnknownRecord; version: UnknownRecord | null; content: string; }

export async function loadScriptState(api: ProductApiClient, projectId: string, scriptId: string): Promise<LoadedScriptState> {
  const script = await api.getScript(projectId, scriptId);
  const versionId = stringValue(script.current_version_id);
  if (!versionId) return { script, version: null, content: '{"title":"Draft script","blocks":[]}' };
  const version = await api.getScriptVersion(projectId, scriptId, versionId);
  return { script, version, content: JSON.stringify(version.content || {}, null, 2) };
}

export async function saveScriptVersion(api: ProductApiClient, projectId: string, scriptId: string, content: string, version: UnknownRecord | null, state: UnknownRecord | null): Promise<UnknownRecord> {
  const parsed = JSON.parse(content) as UnknownRecord;
  return api.createScriptVersion(projectId, scriptId, {
    based_on_version_id: stringValue(version?.id), language: "vi", origin: "user", content: parsed,
  }, Number(state?.revision || 0));
}

export async function setScriptStatus(api: ProductApiClient, projectId: string, scriptId: string, versionId: string, status: "approve" | "reject"): Promise<UnknownRecord> {
  return status === "approve"
    ? api.approveScriptVersion(projectId, scriptId, versionId)
    : api.rejectScriptVersion(projectId, scriptId, versionId, "web-review");
}
