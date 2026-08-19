import type { ProductApiClient } from "@nh-media/sdk";

export async function resolveReview(api: ProductApiClient, input: {
  projectId: string; jobId: string; stepId: string; action: "approve" | "reject" | "resume";
  resourceType: string; resourceId: string; resourceRevision: number;
}): Promise<void> {
  if (input.action === "approve") {
    await api.approveReview(input.projectId, input.jobId, input.stepId, input.resourceType, input.resourceId, input.resourceRevision);
  } else if (input.action === "reject") {
    await api.rejectReview(input.projectId, input.jobId, input.stepId, "edit_then_resume", "web-edit");
  } else {
    await api.resumeJob(input.projectId, input.jobId);
  }
}
