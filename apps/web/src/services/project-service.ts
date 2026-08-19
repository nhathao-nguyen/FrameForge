import type { ProductApiClient } from "@nh-media/sdk";
import type { UnknownRecord } from "../types/workspace";

export async function listProjects(api: ProductApiClient): Promise<UnknownRecord[]> {
  const result = await api.listProjects();
  return result.items || [];
}

export async function createProject(api: ProductApiClient, name: string): Promise<UnknownRecord> {
  return api.createProject(name);
}

export async function loadProjectWorkspace(api: ProductApiClient, projectId: string): Promise<{ jobs: UnknownRecord[]; scenes: UnknownRecord[] }> {
  const [jobs, scenes] = await Promise.all([api.listJobs(projectId), api.listScenes(projectId)]);
  return { jobs: jobs.items || [], scenes: scenes.items || [] };
}
