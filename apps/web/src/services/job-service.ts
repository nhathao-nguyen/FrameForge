import type { ProductApiClient } from "@nh-media/sdk";
import type { UnknownRecord } from "../types/workspace";

export async function createDeterministicJob(api: ProductApiClient, projectId: string): Promise<UnknownRecord> {
  return api.createJob(projectId, { kind: "analysis", mode: "automatic", input: { mode: "deterministic" }, auto_start: true });
}

export async function loadReviewableSteps(api: ProductApiClient, projectId: string, jobId: string, runId: string): Promise<UnknownRecord[]> {
  const result = await api.listSteps(projectId, jobId, runId);
  return result.items || [];
}

export async function refreshJob(api: ProductApiClient, projectId: string, jobId: string): Promise<UnknownRecord> {
  return api.getJob(projectId, jobId);
}
