import { useMemo, useState } from "react";
import {
  appendTimelineHistory, canUndoTimelineHistory, cloneTimelineClip,
  completeTimelineUndo, reconcileTimelineHistory,
} from "@nh-media/sdk";
import type { TimelineHistoryEntry } from "@nh-media/sdk";
import { authenticate, revokeSession } from "../services/auth-service";
import { uploadAsset, issueArtifactDownload } from "../services/asset-service";
import { createProductApiClient } from "../services/api-client";
import { safeMessage } from "../services/error-message";
import { createDeterministicJob, loadReviewableSteps } from "../services/job-service";
import { loadProjectWorkspace, createProject, listProjects } from "../services/project-service";
import { loadScriptState, saveScriptVersion, setScriptStatus } from "../services/script-service";
import { applyTimelineCommand, loadTimelineState, undoTimelineCommand } from "../services/timeline-service";
import { resolveReview } from "../services/review-service";
import type { UnknownRecord, WorkspaceControllerModel } from "../types/workspace";
import { firstClipId, firstTrackId, stringValue } from "../services/value-utils";
import { useJobEvents } from "./use-job-events";

const initialScriptContent = '{"title":"Draft script","blocks":[]}';

export function useWorkspaceController(): WorkspaceControllerModel {
  const api = useMemo(createProductApiClient, []);
  const [endpoint, setEndpoint] = useState(process.env.NEXT_PUBLIC_NH_MEDIA_API_URL || "http://127.0.0.1:8080");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [loggedIn, setLoggedIn] = useState(false);
  const [projects, setProjects] = useState<UnknownRecord[]>([]);
  const [jobs, setJobs] = useState<UnknownRecord[]>([]);
  const [selectedProject, setSelectedProject] = useState<UnknownRecord | null>(null);
  const [selectedJob, setSelectedJob] = useState<UnknownRecord | null>(null);
  const [projectName, setProjectName] = useState("");
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [artifactId, setArtifactId] = useState("");
  const [downloadUrl, setDownloadUrl] = useState("");
  const [scriptId, setScriptId] = useState("");
  const [scriptState, setScriptState] = useState<UnknownRecord | null>(null);
  const [scriptVersion, setScriptVersion] = useState<UnknownRecord | null>(null);
  const [scriptContent, setScriptContent] = useState(initialScriptContent);
  const [timelineId, setTimelineId] = useState("");
  const [timelineVersionId, setTimelineVersionId] = useState("");
  const [timelineClipId, setTimelineClipId] = useState("");
  const [timelineState, setTimelineState] = useState<UnknownRecord | null>(null);
  const [timelineDocument, setTimelineDocument] = useState<UnknownRecord | null>(null);
  const [timelineHistory, setTimelineHistory] = useState<TimelineHistoryEntry[]>([]);
  const [timelineEditKind, setTimelineEditKind] = useState("UpdateScene");
  const [timelineTargetTrack, setTimelineTargetTrack] = useState("");
  const [scenes, setScenes] = useState<UnknownRecord[]>([]);
  const [reviewStepId, setReviewStepId] = useState("");
  const [reviewResourceType, setReviewResourceType] = useState("timeline_version");
  const [reviewResourceId, setReviewResourceId] = useState("");
  const [reviewResourceRevision, setReviewResourceRevision] = useState("1");
  const [steps, setSteps] = useState<UnknownRecord[]>([]);
  const [status, setStatus] = useState("Sign in to connect to Product API.");
  const [error, setError] = useState("");

  const clearError = () => setError("");

  async function selectProject(project: UnknownRecord): Promise<void> {
    setSelectedProject(project);
    setSelectedJob(null);
    setTimelineState(null);
    setTimelineDocument(null);
    setTimelineVersionId("");
    setTimelineHistory([]);
    const loaded = await loadProjectWorkspace(api, stringValue(project.id));
    setJobs(loaded.jobs);
    setScenes(loaded.scenes);
    setStatus("Project loaded from server-authoritative state.");
  }

  async function refreshProjects(): Promise<void> {
    const loaded = await listProjects(api);
    setProjects(loaded);
    if (loaded.length && !selectedProject) await selectProject(loaded[0]);
  }

  async function signIn(): Promise<void> {
    clearError();
    try {
      await authenticate(api, endpoint, username, password);
      setLoggedIn(true);
      setStatus("Authenticated. Loading projects from Product API.");
      await refreshProjects();
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function signOut(): Promise<void> {
    await revokeSession(api);
    setLoggedIn(false);
    setProjects([]);
    setJobs([]);
    setSelectedProject(null);
    setSelectedJob(null);
    setTimelineState(null);
    setTimelineDocument(null);
    setTimelineVersionId("");
    setTimelineHistory([]);
    setStatus("Signed out; session revoked.");
  }

  async function createNewProject(): Promise<void> {
    if (!projectName.trim()) return;
    clearError();
    try {
      const project = await createProject(api, projectName.trim());
      setProjectName("");
      await refreshProjects();
      await selectProject(project);
      setStatus("Project created.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function startJob(): Promise<void> {
    if (!selectedProject) return;
    clearError();
    try {
      const job = await createDeterministicJob(api, stringValue(selectedProject.id));
      setSelectedJob(job);
      setJobs((current) => [job, ...current]);
      setStatus("Job submitted; following durable SSE events.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function uploadSelectedAsset(): Promise<void> {
    if (!selectedProject || !uploadFile) return;
    clearError();
    try {
      await uploadAsset(api, stringValue(selectedProject.id), uploadFile);
      setUploadFile(null);
      setStatus("Asset uploaded directly to approved storage; validation Job created.");
      await selectProject(selectedProject);
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function downloadSelectedArtifact(): Promise<void> {
    if (!selectedProject || !artifactId.trim()) return;
    clearError();
    try {
      setDownloadUrl(await issueArtifactDownload(api, stringValue(selectedProject.id), artifactId.trim()));
      setStatus("Short-lived artifact URL issued by Product API.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function loadScript(): Promise<void> {
    if (!selectedProject || !scriptId.trim()) return;
    clearError();
    try {
      const loaded = await loadScriptState(api, stringValue(selectedProject.id), scriptId.trim());
      setScriptState(loaded.script);
      setScriptVersion(loaded.version);
      setScriptContent(loaded.content);
      setStatus("Script and current immutable ScriptVersion loaded from Product API.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function saveScript(): Promise<void> {
    if (!selectedProject || !scriptId.trim()) return;
    clearError();
    try {
      const value = await saveScriptVersion(api, stringValue(selectedProject.id), scriptId.trim(), scriptContent, scriptVersion, scriptState);
      setScriptVersion(value);
      setStatus("Script draft submitted as an immutable server version; reload is required after conflict.");
      await loadScript();
    } catch (value) {
      setError(value instanceof SyntaxError ? "Script content must be valid JSON." : safeMessage(value));
    }
  }

  async function updateScriptStatus(nextStatus: "approve" | "reject"): Promise<void> {
    if (!selectedProject || !scriptId.trim() || !scriptVersion) return;
    clearError();
    try {
      const value = await setScriptStatus(api, stringValue(selectedProject.id), scriptId.trim(), stringValue(scriptVersion.id), nextStatus);
      setScriptVersion(value);
      await loadScript();
      setStatus(nextStatus === "approve" ? "ScriptVersion approved." : "ScriptVersion rejected as superseded.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function loadTimeline(options: { expectedVersionId?: string; resetHistory?: boolean } = {}): Promise<{ versionId: string }> {
    if (!selectedProject || !timelineId.trim()) return { versionId: "" };
    clearError();
    try {
      const projectId = stringValue(selectedProject.id);
      const loaded = await loadTimelineState(api, projectId, timelineId.trim());
      const history = reconcileTimelineHistory(timelineHistory, {
        projectId, timelineId: timelineId.trim(), currentVersionId: loaded.versionId,
        previousProjectId: stringValue(timelineState?.project_id), previousTimelineId: stringValue(timelineState?.id),
        previousVersionId: timelineVersionId, expectedVersionId: options.expectedVersionId, reset: options.resetHistory,
      });
      setTimelineState(loaded.timeline);
      setTimelineVersionId(loaded.versionId);
      setTimelineDocument(loaded.document);
      setTimelineClipId(firstClipId(loaded.document));
      setTimelineTargetTrack(firstTrackId(loaded.document));
      setTimelineHistory(history);
      setStatus("Timeline, current TimelineVersion and Scenes loaded from server state.");
      return { versionId: loaded.versionId };
    } catch (value) {
      setError(safeMessage(value));
      return { versionId: "" };
    }
  }

  async function applyTimeline(): Promise<void> {
    if (!selectedProject || !timelineId.trim() || !timelineVersionId.trim() || !timelineClipId.trim() || !timelineDocument) return;
    clearError();
    try {
      const previousVersionId = timelineVersionId.trim();
      const previousRevision = Number(timelineState?.revision || 0);
      const result = await applyTimelineCommand(api, {
        projectId: stringValue(selectedProject.id), timelineId: timelineId.trim(), versionId: previousVersionId,
        document: timelineDocument, clipId: timelineClipId.trim(), commandKind: timelineEditKind,
        targetTrack: timelineTargetTrack.trim(), expectedRevision: previousRevision,
      });
      const resultVersionId = stringValue(result.result.id || result.result.version_id);
      if (!resultVersionId) throw new Error("Product API did not return the new TimelineVersion identity.");
      const loaded = await loadTimeline({ expectedVersionId: resultVersionId });
      if (loaded.versionId !== resultVersionId) throw new Error("Server TimelineVersion changed before the edit could be reconciled.");
      setTimelineHistory((current) => appendTimelineHistory(current, {
        projectId: stringValue(selectedProject.id), timelineId: timelineId.trim(), previousVersionId,
        resultVersionId, clipId: timelineClipId.trim(), previousClip: cloneTimelineClip(result.previousClip),
      }));
      setStatus("Timeline command accepted as a new immutable user-origin TimelineVersion.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function undoTimeline(): Promise<void> {
    if (!selectedProject || !timelineDocument || !timelineVersionId || !timelineClipId || !timelineState) return;
    const entry = timelineHistory[timelineHistory.length - 1];
    if (!entry || !canUndoTimelineHistory(timelineHistory, stringValue(selectedProject.id), timelineId.trim(), timelineVersionId)) return;
    clearError();
    try {
      const result = await undoTimelineCommand(api, {
        projectId: stringValue(selectedProject.id), timelineId: timelineId.trim(), versionId: timelineVersionId,
        previousVersionId: entry.previousVersionId, clipId: entry.clipId, previousClip: entry.previousClip,
        expectedVersion: Number(timelineDocument.version || 0), expectedRevision: Number(timelineState.revision || 0),
      });
      const resultVersionId = stringValue(result.id || result.version_id);
      if (!resultVersionId) throw new Error("Product API did not return the undo TimelineVersion identity.");
      const loaded = await loadTimeline({ expectedVersionId: resultVersionId });
      if (loaded.versionId !== resultVersionId) throw new Error("Server TimelineVersion changed before undo could be reconciled.");
      setTimelineHistory((current) => completeTimelineUndo(current, resultVersionId));
      setStatus("Undo was recorded as a new server TimelineVersion.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function loadSteps(): Promise<void> {
    if (!selectedProject || !selectedJob) return;
    clearError();
    try {
      const loaded = await loadReviewableSteps(api, stringValue(selectedProject.id), stringValue(selectedJob.id), stringValue(selectedJob.pipeline_run_id));
      setSteps(loaded);
      if (loaded.length) setReviewStepId(stringValue(loaded[0].id));
      setStatus("Reviewable JobSteps loaded from the durable run.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function performReview(action: "approve" | "reject" | "resume"): Promise<void> {
    if (!selectedProject || !selectedJob || !reviewStepId.trim()) return;
    clearError();
    try {
      const projectId = stringValue(selectedProject.id);
      const jobId = stringValue(selectedJob.id);
      await resolveReview(api, {
        projectId, jobId, stepId: reviewStepId.trim(), action, resourceType: reviewResourceType,
        resourceId: reviewResourceId.trim(), resourceRevision: Number(reviewResourceRevision),
      });
      setSelectedJob(await api.getJob(projectId, jobId));
      setStatus(action === "approve" ? "Review approved with the exact selected resource." : action === "reject" ? "Review rejected for edit_then_resume." : "Paused review descendants resumed from server state.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  useJobEvents({ api, selectedProject, selectedJob, onJob: setSelectedJob, onStatus: setStatus, onError: setError });

  return {
    loggedIn,
    auth: {
      endpoint, username, password, status, error, onEndpointChange: setEndpoint, onUsernameChange: setUsername,
      onPasswordChange: setPassword, onSubmit: signIn,
    },
    projects: {
      projects, selectedProject, projectName, uploadFile, artifactId, downloadUrl,
      onProjectNameChange: setProjectName, onUploadFileChange: setUploadFile, onArtifactIdChange: setArtifactId,
      onCreate: createNewProject, onSelect: selectProject, onUpload: uploadSelectedAsset, onDownload: downloadSelectedArtifact,
    },
    jobs: { jobs, selectedProject, selectedJob, onSelect: setSelectedJob, onStartDeterministic: startJob },
    script: {
      selectedProject, scriptId, scriptState, scriptVersion, scriptContent, onScriptIdChange: setScriptId,
      onScriptContentChange: setScriptContent, onLoad: loadScript, onSave: saveScript,
      onApprove: () => updateScriptStatus("approve"), onReject: () => updateScriptStatus("reject"),
    },
    timeline: {
      selectedProject, timelineId, timelineVersionId, timelineClipId, timelineState, timelineDocument,
      timelineHistory, timelineEditKind, timelineTargetTrack, onTimelineIdChange: setTimelineId,
      onTimelineClipIdChange: setTimelineClipId, onTimelineEditKindChange: setTimelineEditKind,
      onTimelineTargetTrackChange: setTimelineTargetTrack, onLoad: async () => { await loadTimeline(); }, onApply: applyTimeline, onUndo: undoTimeline,
    },
    review: {
      selectedProject, selectedJob, steps, reviewStepId, reviewResourceType, reviewResourceId, reviewResourceRevision,
      onStepChange: setReviewStepId, onResourceTypeChange: setReviewResourceType, onResourceIdChange: setReviewResourceId,
      onResourceRevisionChange: setReviewResourceRevision, onLoad: loadSteps, onApprove: () => performReview("approve"),
      onReject: () => performReview("reject"), onResume: () => performReview("resume"),
    },
    scenes: { scenes }, status, error, onSignOut: signOut,
  };
}
