import { useEffect } from "react";
import type { ProductApiClient, SseEnvelope } from "@nh-media/sdk";
import type { UnknownRecord } from "../types/workspace";
import { refreshJob } from "../services/job-service";
import { safeMessage } from "../services/error-message";

export function useJobEvents(input: {
  api: ProductApiClient; selectedProject: UnknownRecord | null; selectedJob: UnknownRecord | null;
  onJob: (job: UnknownRecord) => void; onStatus: (message: string) => void; onError: (message: string) => void;
}): void {
  const { api, selectedProject, selectedJob, onJob, onStatus, onError } = input;
  useEffect(() => {
    if (!selectedJob || !selectedProject || !api.getToken()) return;
    const controller = new AbortController();
    const projectId = typeof selectedProject.id === "string" ? selectedProject.id : "";
    const jobId = typeof selectedJob.id === "string" ? selectedJob.id : "";
    (async () => {
      try {
        for await (const frame of api.events(projectId, jobId, controller.signal)) {
          const event = frame as SseEnvelope<UnknownRecord>;
          if (event.event === "stream.reset") {
            onJob(await refreshJob(api, projectId, jobId));
            onStatus("Stream reset; canonical Job state reloaded.");
          } else if (event.event === "stream.snapshot") {
            const snapshot = event.data as { job?: UnknownRecord };
            if (snapshot.job) onJob(snapshot.job);
          } else {
            onStatus("Live event: " + event.event);
            onJob(await refreshJob(api, projectId, jobId));
          }
        }
      } catch (value) {
        if (!controller.signal.aborted) onError(safeMessage(value));
      }
    })();
    return () => controller.abort();
  }, [api, onError, onJob, onStatus, selectedJob?.id, selectedProject?.id]);
}
