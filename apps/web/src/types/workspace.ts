import type { TimelineHistoryEntry } from "@nh-media/sdk";

export type UnknownRecord = Record<string, unknown>;
export type AsyncAction = () => Promise<void>;

export interface AuthViewModel {
  endpoint: string; username: string; password: string; status: string; error: string;
  onEndpointChange: (value: string) => void; onUsernameChange: (value: string) => void;
  onPasswordChange: (value: string) => void; onSubmit: AsyncAction;
}

export interface ProjectViewModel {
  projects: UnknownRecord[]; selectedProject: UnknownRecord | null; projectName: string;
  uploadFile: File | null; artifactId: string; downloadUrl: string;
  onProjectNameChange: (value: string) => void; onUploadFileChange: (value: File | null) => void;
  onArtifactIdChange: (value: string) => void; onCreate: AsyncAction;
  onSelect: (project: UnknownRecord) => Promise<void>; onUpload: AsyncAction; onDownload: AsyncAction;
}

export interface JobViewModel {
  jobs: UnknownRecord[]; selectedProject: UnknownRecord | null; selectedJob: UnknownRecord | null;
  onSelect: (job: UnknownRecord) => void; onStartDeterministic: AsyncAction;
}

export interface ScriptViewModel {
  selectedProject: UnknownRecord | null; scriptId: string; scriptState: UnknownRecord | null;
  scriptVersion: UnknownRecord | null; scriptContent: string;
  onScriptIdChange: (value: string) => void; onScriptContentChange: (value: string) => void;
  onLoad: AsyncAction; onSave: AsyncAction; onApprove: AsyncAction; onReject: AsyncAction;
}

export interface TimelineViewModel {
  selectedProject: UnknownRecord | null; timelineId: string; timelineVersionId: string;
  timelineClipId: string; timelineState: UnknownRecord | null; timelineDocument: UnknownRecord | null;
  timelineHistory: TimelineHistoryEntry[]; timelineEditKind: string; timelineTargetTrack: string;
  onTimelineIdChange: (value: string) => void; onTimelineClipIdChange: (value: string) => void;
  onTimelineEditKindChange: (value: string) => void; onTimelineTargetTrackChange: (value: string) => void;
  onLoad: AsyncAction; onApply: AsyncAction; onUndo: AsyncAction;
}

export interface ReviewViewModel {
  selectedProject: UnknownRecord | null; selectedJob: UnknownRecord | null; steps: UnknownRecord[];
  reviewStepId: string; reviewResourceType: string; reviewResourceId: string; reviewResourceRevision: string;
  onStepChange: (value: string) => void; onResourceTypeChange: (value: string) => void;
  onResourceIdChange: (value: string) => void; onResourceRevisionChange: (value: string) => void;
  onLoad: AsyncAction; onApprove: AsyncAction; onReject: AsyncAction; onResume: AsyncAction;
}

export interface SceneViewModel { scenes: UnknownRecord[]; }

export interface WorkspaceControllerModel {
  loggedIn: boolean; auth: AuthViewModel; projects: ProjectViewModel; jobs: JobViewModel;
  script: ScriptViewModel; timeline: TimelineViewModel; review: ReviewViewModel;
  scenes: { scenes: UnknownRecord[] }; status: string; error: string; onSignOut: AsyncAction;
}
