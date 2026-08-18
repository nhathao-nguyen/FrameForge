"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import {
  ProductApiClient, ProductApiError, SseEnvelope,
  appendTimelineHistory, canUndoTimelineHistory, cloneTimelineClip, completeTimelineUndo,
  reconcileTimelineHistory,
} from "@nh-media/sdk";
import type { TimelineHistoryEntry } from "@nh-media/sdk";

const defaultEndpoint = process.env.NEXT_PUBLIC_NH_MEDIA_API_URL || "http://127.0.0.1:8080";

export default function HomePage() {
  const api = useMemo(() => new ProductApiClient({ baseUrl: defaultEndpoint }), []);
  const [endpoint, setEndpoint] = useState(defaultEndpoint);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [loggedIn, setLoggedIn] = useState(false);
  const [projects, setProjects] = useState<Record<string, unknown>[]>([]);
  const [jobs, setJobs] = useState<Record<string, unknown>[]>([]);
  const [selectedProject, setSelectedProject] = useState<Record<string, unknown> | null>(null);
  const [selectedJob, setSelectedJob] = useState<Record<string, unknown> | null>(null);
  const [projectName, setProjectName] = useState("");
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [artifactId, setArtifactId] = useState("");
  const [downloadUrl, setDownloadUrl] = useState("");
  const [scriptId, setScriptId] = useState("");
  const [scriptState, setScriptState] = useState<Record<string, unknown> | null>(null);
  const [scriptVersion, setScriptVersion] = useState<Record<string, unknown> | null>(null);
  const [scriptContent, setScriptContent] = useState('{"title":"Draft script","blocks":[]}');
  const [timelineId, setTimelineId] = useState("");
  const [timelineVersionId, setTimelineVersionId] = useState("");
  const [timelineClipId, setTimelineClipId] = useState("");
  const [timelineState, setTimelineState] = useState<Record<string, unknown> | null>(null);
  const [timelineDocument, setTimelineDocument] = useState<Record<string, unknown> | null>(null);
  const [timelineHistory, setTimelineHistory] = useState<TimelineHistoryEntry[]>([]);
  const [timelineEditKind, setTimelineEditKind] = useState("UpdateScene");
  const [timelineTargetTrack, setTimelineTargetTrack] = useState("");
  const [scenes, setScenes] = useState<Record<string, unknown>[]>([]);
  const [reviewStepId, setReviewStepId] = useState("");
  const [reviewResourceType, setReviewResourceType] = useState("timeline_version");
  const [reviewResourceId, setReviewResourceId] = useState("");
  const [reviewResourceRevision, setReviewResourceRevision] = useState("1");
  const [steps, setSteps] = useState<Record<string, unknown>[]>([]);
  const [status, setStatus] = useState("Sign in to connect to Product API.");
  const [error, setError] = useState("");

  useEffect(() => {
    if (!selectedJob || !selectedProject || !api.getToken()) return;
    const controller = new AbortController();
    const projectId = stringValue(selectedProject.id);
    const jobId = stringValue(selectedJob.id);
    (async () => {
      try {
        for await (const frame of api.events(projectId, jobId, controller.signal)) {
          const event = frame as SseEnvelope<Record<string, unknown>>;
          if (event.event === "stream.reset") {
            const refreshed = await api.getJob(projectId, jobId);
            setSelectedJob(refreshed);
            setStatus("Stream reset; canonical Job state reloaded.");
          } else if (event.event === "stream.snapshot") {
            const snapshot = event.data as { job?: Record<string, unknown> };
            if (snapshot.job) setSelectedJob(snapshot.job);
          } else {
            setStatus("Live event: " + event.event);
            const refreshed = await api.getJob(projectId, jobId);
            setSelectedJob(refreshed);
          }
        }
      } catch (value) {
        if (!controller.signal.aborted) setError(safeMessage(value));
      }
    })();
    return () => controller.abort();
  }, [api, selectedJob?.id, selectedProject?.id]);

  async function signIn(event: FormEvent) {
    event.preventDefault();
    setError("");
    try {
      api.setBaseUrl(endpoint);
      await api.login(username, password);
      setLoggedIn(true);
      setStatus("Authenticated. Loading projects from Product API.");
      await refreshProjects();
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function refreshProjects() {
    const result = await api.listProjects();
    setProjects(result.items || []);
    if (result.items?.length && !selectedProject) {
      await selectProject(result.items[0]);
    }
  }

  async function selectProject(project: Record<string, unknown>) {
    setSelectedProject(project);
    setSelectedJob(null);
    setTimelineState(null);
    setTimelineDocument(null);
    setTimelineVersionId("");
    setTimelineHistory([]);
    const result = await api.listJobs(stringValue(project.id));
    setJobs(result.items || []);
    const sceneResult = await api.listScenes(stringValue(project.id));
    setScenes(sceneResult.items || []);
    setStatus("Project loaded from server-authoritative state.");
  }

  async function loadScriptState() {
    if (!selectedProject || !scriptId.trim()) return;
    setError("");
    try {
      const projectId = stringValue(selectedProject.id);
      const script = await api.getScript(projectId, scriptId.trim());
      setScriptState(script);
      const versionId = stringValue(script.current_version_id);
      if (versionId) {
        const version = await api.getScriptVersion(projectId, scriptId.trim(), versionId);
        setScriptVersion(version);
        setScriptContent(JSON.stringify(version.content || {}, null, 2));
      }
      setStatus("Script and current immutable ScriptVersion loaded from Product API.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function loadTimelineState(options: { expectedVersionId?: string; resetHistory?: boolean } = {}): Promise<{ versionId: string }> {
    if (!selectedProject || !timelineId.trim()) return { versionId: "" };
    setError("");
    try {
      const projectId = stringValue(selectedProject.id);
      const timeline = await api.getTimeline(projectId, timelineId.trim());
      const versionId = stringValue(timeline.current_version_id);
      if (!versionId) throw new Error("Timeline has no current TimelineVersion.");
      const version = await api.getTimelineVersion(projectId, timelineId.trim(), versionId);
      const document = parseObject(version.document);
      const history = reconcileTimelineHistory(timelineHistory, {
        projectId,
        timelineId: timelineId.trim(),
        currentVersionId: versionId,
        previousProjectId: stringValue(timelineState?.project_id),
        previousTimelineId: stringValue(timelineState?.id),
        previousVersionId: timelineVersionId,
        expectedVersionId: options.expectedVersionId,
        reset: options.resetHistory,
      });
      setTimelineState(timeline);
      setTimelineVersionId(versionId);
      setTimelineDocument(document);
      setTimelineClipId(firstClipId(document));
      setTimelineTargetTrack(firstTrackId(document));
      setTimelineHistory(history);
      setStatus("Timeline, current TimelineVersion and Scenes loaded from server state.");
      return { versionId };
    } catch (value) {
      setError(safeMessage(value));
      return { versionId: "" };
    }
  }

  async function loadReviewSteps() {
    if (!selectedProject || !selectedJob) return;
    setError("");
    try {
      const result = await api.listSteps(stringValue(selectedProject.id), stringValue(selectedJob.id), stringValue(selectedJob.pipeline_run_id));
      setSteps(result.items || []);
      if (result.items?.length) setReviewStepId(stringValue(result.items[0].id));
      setStatus("Reviewable JobSteps loaded from the durable run.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function createProject(event: FormEvent) {
    event.preventDefault();
    if (!projectName.trim()) return;
    setError("");
    try {
      const project = await api.createProject(projectName.trim());
      setProjectName("");
      await refreshProjects();
      await selectProject(project);
      setStatus("Project created.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function startDeterministicJob() {
    if (!selectedProject) return;
    setError("");
    try {
      const job = await api.createJob(stringValue(selectedProject.id), {
        kind: "analysis",
        mode: "automatic",
        input: { mode: "deterministic" },
        auto_start: true,
      });
      setSelectedJob(job);
      setJobs((current) => [job, ...current]);
      setStatus("Job submitted; following durable SSE events.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function uploadAsset() {
    if (!selectedProject || !uploadFile) return;
    setError("");
    try {
      const projectId = stringValue(selectedProject.id);
      const sessionEnvelope = await api.createUploadSession(projectId, {
        kind: uploadFile.type.startsWith("audio/") ? "audio" : "video",
        filename: uploadFile.name,
        content_type: uploadFile.type || "application/octet-stream",
        size_bytes: uploadFile.size,
        multipart: true,
      });
      const asset = objectValue(sessionEnvelope.asset);
      const upload = objectValue(sessionEnvelope.upload);
      const parts = Array.isArray(upload.parts) ? upload.parts : [];
      const part = objectValue(parts[0]);
      const url = stringValue(part.url);
      if (!url) throw new Error("The API did not return a direct upload URL.");
      const headers = Object.fromEntries(Object.entries(objectValue(part.headers)).filter(([, value]) => typeof value === "string")) as Record<string, string>;
      const etag = await api.uploadPart(url, uploadFile, headers);
      if (!etag) throw new Error("The storage provider did not return an upload ETag.");
      await api.completeUpload(projectId, stringValue(asset.id), stringValue(upload.id), [{ part_number: 1, etag }]);
      setUploadFile(null);
      setStatus("Asset uploaded directly to approved storage; validation Job created.");
      await selectProject(selectedProject);
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function downloadArtifact() {
    if (!selectedProject || !artifactId.trim()) return;
    setError("");
    try {
      const value = await api.downloadArtifact(stringValue(selectedProject.id), artifactId.trim());
      setDownloadUrl(value.url);
      setStatus("Short-lived artifact URL issued by Product API.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function saveScriptVersion(event: FormEvent) {
    event.preventDefault();
    if (!selectedProject || !scriptId.trim()) return;
    setError("");
    try {
      const content = JSON.parse(scriptContent) as Record<string, unknown>;
      const value = await api.createScriptVersion(stringValue(selectedProject.id), scriptId.trim(), {
        based_on_version_id: stringValue(scriptVersion?.id),
        language: "vi",
        origin: "user",
        content,
      }, Number(scriptState?.revision || 0));
      setScriptVersion(value);
      setStatus("Script draft submitted as an immutable server version; reload is required after conflict.");
      await loadScriptState();
    } catch (value) {
      setError(value instanceof SyntaxError ? "Script content must be valid JSON." : safeMessage(value));
    }
  }

  async function setScriptStatus(status: "approve" | "reject") {
    if (!selectedProject || !scriptId.trim() || !scriptVersion) return;
    setError("");
    try {
      const projectId = stringValue(selectedProject.id);
      const versionId = stringValue(scriptVersion.id);
      const value = status === "approve"
        ? await api.approveScriptVersion(projectId, scriptId.trim(), versionId)
        : await api.rejectScriptVersion(projectId, scriptId.trim(), versionId, "web-review");
      setScriptVersion(value);
      await loadScriptState();
      setStatus(status === "approve" ? "ScriptVersion approved." : "ScriptVersion rejected as superseded.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function applyTimelineEdit(event: FormEvent) {
    event.preventDefault();
    if (!selectedProject || !timelineId.trim() || !timelineVersionId.trim() || !timelineClipId.trim() || !timelineDocument) return;
    setError("");
    try {
      const clip = findClip(timelineDocument, timelineClipId.trim());
      if (!clip) throw new Error("Selected clip is not present in the loaded TimelineVersion.");
      const previousVersionId = timelineVersionId.trim();
      const previousRevision = Number(timelineState?.revision || 0);
      const previousDocumentVersion = Number(timelineDocument.version || 0);
      const payload: Record<string, unknown> = timelineEditKind === "MoveClip"
        ? { clip_id: timelineClipId.trim(), track_id: timelineTargetTrack.trim() }
        : timelineEditKind === "ReplaceClip"
          ? { clip_id: timelineClipId.trim(), clip: { ...clip, metadata: { ...(objectValue(clip.metadata)), edited_from: "web" } } }
          : { clip_id: timelineClipId.trim(), metadata: { edited_from: "web", title: "User scene override" } };
      const result = await api.timelineCommand(stringValue(selectedProject.id), timelineId.trim(), previousVersionId, {
        kind: timelineEditKind,
        schema_version: "1.0",
        expected_version: previousDocumentVersion,
        payload,
      }, previousRevision);
      const resultVersionId = stringValue(result.id || result.version_id);
      if (!resultVersionId) throw new Error("Product API did not return the new TimelineVersion identity.");
      const loaded = await loadTimelineState({ expectedVersionId: resultVersionId });
      if (loaded.versionId !== resultVersionId) throw new Error("Server TimelineVersion changed before the edit could be reconciled.");
      setTimelineHistory((current) => appendTimelineHistory(current, {
        projectId: stringValue(selectedProject.id),
        timelineId: timelineId.trim(),
        previousVersionId,
        resultVersionId,
        clipId: timelineClipId.trim(),
        previousClip: cloneTimelineClip(clip),
      }));
      setStatus("Timeline command accepted as a new immutable user-origin TimelineVersion.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function undoTimelineEdit() {
    if (!selectedProject || !timelineDocument || !timelineVersionId || !timelineClipId || !timelineState) return;
    const entry = timelineHistory[timelineHistory.length - 1];
    if (!entry || !canUndoTimelineHistory(timelineHistory, stringValue(selectedProject.id), timelineId.trim(), timelineVersionId)) return;
    setError("");
    try {
      const result = await api.restoreTimelineClip(
        stringValue(selectedProject.id), timelineId.trim(), timelineVersionId,
        entry.previousVersionId, entry.clipId, entry.previousClip,
        Number(timelineDocument.version || 0), Number(timelineState.revision || 0),
      );
      const resultVersionId = stringValue(result.id || result.version_id);
      if (!resultVersionId) throw new Error("Product API did not return the undo TimelineVersion identity.");
      const loaded = await loadTimelineState({ expectedVersionId: resultVersionId });
      if (loaded.versionId !== resultVersionId) throw new Error("Server TimelineVersion changed before undo could be reconciled.");
      setTimelineHistory((current) => completeTimelineUndo(current, resultVersionId));
      setStatus("Undo was recorded as a new server TimelineVersion.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function resolveReview(action: "approve" | "reject" | "resume") {
    if (!selectedProject || !selectedJob || !reviewStepId.trim()) return;
    setError("");
    try {
      const projectId = stringValue(selectedProject.id);
      const jobId = stringValue(selectedJob.id);
      if (action === "approve") {
        await api.approveReview(projectId, jobId, reviewStepId.trim(), reviewResourceType, reviewResourceId.trim(), Number(reviewResourceRevision));
      } else if (action === "reject") {
        await api.rejectReview(projectId, jobId, reviewStepId.trim(), "edit_then_resume", "web-edit");
      } else {
        await api.resumeJob(projectId, jobId);
      }
      const refreshed = await api.getJob(projectId, jobId);
      setSelectedJob(refreshed);
      setStatus(action === "approve" ? "Review approved with the exact selected resource." : action === "reject" ? "Review rejected for edit_then_resume." : "Paused review descendants resumed from server state.");
    } catch (value) {
      setError(safeMessage(value));
    }
  }

  async function signOut() {
    await api.logout();
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

  if (!loggedIn) {
    return (
      <main className="shell">
        <section className="card" aria-labelledby="login-title">
          <p className="eyebrow">NH-Media · Local/LAN client</p>
          <h1 id="login-title">Product workspace</h1>
          <p>Remote-first web shell. Product API remains the only source of truth.</p>
          <form onSubmit={signIn} className="stack">
            <label htmlFor="endpoint">Product API endpoint</label>
            <input id="endpoint" value={endpoint} onChange={(event) => setEndpoint(event.target.value)} inputMode="url" />
            <label htmlFor="username">Username</label>
            <input id="username" value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" required />
            <label htmlFor="password">Password</label>
            <input id="password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" required />
            <button type="submit">Sign in</button>
          </form>
          <p role="status">{status}</p>
          {error ? <p role="alert" className="error">{error}</p> : null}
        </section>
      </main>
    );
  }

  return (
    <main className="shell">
      <header className="topbar">
        <div><p className="eyebrow">NH-Media · Local/LAN client</p><h1>Workspace dashboard</h1></div>
        <button type="button" onClick={signOut}>Sign out</button>
      </header>
      <p role="status">{status}</p>
      {error ? <p role="alert" className="error">{error}</p> : null}
      <div className="grid">
        <section className="card" aria-labelledby="projects-title">
          <h2 id="projects-title">Projects</h2>
          <form onSubmit={createProject} className="inline-form">
            <label htmlFor="project-name" className="sr-only">Project name</label>
            <input id="project-name" value={projectName} onChange={(event) => setProjectName(event.target.value)} placeholder="New project" />
            <button type="submit">Create</button>
          </form>
          <ul className="resource-list">
            {projects.map((project) => (
              <li key={stringValue(project.id)}><button type="button" className="list-button" onClick={() => void selectProject(project)} aria-pressed={selectedProject?.id === project.id}>{stringValue(project.name)}</button></li>
            ))}
          </ul>
          <div className="stack compact-form">
            <h3>Direct asset upload</h3>
            <input type="file" onChange={(event) => setUploadFile(event.target.files?.[0] || null)} disabled={!selectedProject} />
            <button type="button" onClick={() => void uploadAsset()} disabled={!selectedProject || !uploadFile}>Upload to approved storage</button>
            <p className="muted">Media bytes go to the signed storage URL; Product API receives metadata only.</p>
            <h3>Artifact download</h3>
            <input value={artifactId} onChange={(event) => setArtifactId(event.target.value)} placeholder="Committed artifact UUID" disabled={!selectedProject} />
            <button type="button" onClick={() => void downloadArtifact()} disabled={!selectedProject || !artifactId.trim()}>Issue download URL</button>
            {downloadUrl ? <a href={downloadUrl} target="_blank" rel="noreferrer">Open signed artifact download</a> : null}
          </div>
        </section>
        <section className="card" aria-labelledby="jobs-title">
          <h2 id="jobs-title">Jobs{selectedProject ? " · " + stringValue(selectedProject.name) : ""}</h2>
          <button type="button" onClick={() => void startDeterministicJob()} disabled={!selectedProject}>Run deterministic worker path</button>
          <ul className="resource-list">
            {jobs.map((job) => (
              <li key={stringValue(job.id)}><button type="button" className="list-button" onClick={() => setSelectedJob(job)} aria-pressed={selectedJob?.id === job.id}>{stringValue(job.kind)} · {stringValue(job.status)}</button></li>
            ))}
          </ul>
        </section>
        <section className="card" aria-labelledby="review-title">
          <h2 id="review-title">Review & timeline</h2>
          <p>Script and TimelineVersion edits are submitted as immutable server commands. Local drafts are not authoritative.</p>
          <form onSubmit={saveScriptVersion} className="stack compact-form">
            <h3>Script editor</h3>
            <label htmlFor="script-id">Script ID</label>
            <input id="script-id" value={scriptId} onChange={(event) => setScriptId(event.target.value)} placeholder="script_…" />
            <button type="button" onClick={() => void loadScriptState()} disabled={!selectedProject || !scriptId.trim()}>Load script state</button>
            <p className="muted">Revision {String(scriptState?.revision || "—")} · current {stringValue(scriptState?.current_version_id) || "—"}</p>
            <label htmlFor="script-content">Draft content JSON</label>
            <textarea id="script-content" value={scriptContent} onChange={(event) => setScriptContent(event.target.value)} rows={4} />
            <button type="submit" disabled={!selectedProject || !scriptId.trim() || !scriptState}>Save immutable script version</button>
            <div className="inline-form">
              <button type="button" onClick={() => void setScriptStatus("approve")} disabled={!scriptVersion}>Approve version</button>
              <button type="button" onClick={() => void setScriptStatus("reject")} disabled={!scriptVersion}>Reject version</button>
            </div>
          </form>
          <form onSubmit={applyTimelineEdit} className="stack compact-form">
            <h3>Timeline / scene editor</h3>
            <label htmlFor="timeline-id">Timeline ID</label>
            <input id="timeline-id" value={timelineId} onChange={(event) => setTimelineId(event.target.value)} placeholder="timeline_…" />
            <button type="button" onClick={() => void loadTimelineState()} disabled={!selectedProject || !timelineId.trim()}>Load timeline + current version</button>
            <p className="muted">Aggregate revision {String(timelineState?.revision || "—")} · loaded version {timelineVersionId || "—"}</p>
            <label htmlFor="timeline-clip-id">Scene / clip ID</label>
            <input id="timeline-clip-id" value={timelineClipId} onChange={(event) => setTimelineClipId(event.target.value)} placeholder="clip_…" />
            <label htmlFor="timeline-command-kind">Typed command</label>
            <select id="timeline-command-kind" value={timelineEditKind} onChange={(event) => setTimelineEditKind(event.target.value)}>
              <option value="UpdateScene">UpdateScene</option>
              <option value="MoveClip">MoveClip</option>
              <option value="ReplaceClip">ReplaceClip</option>
            </select>
            {timelineEditKind === "MoveClip" ? <>
              <label htmlFor="timeline-target-track">Target track</label>
              <input id="timeline-target-track" value={timelineTargetTrack} onChange={(event) => setTimelineTargetTrack(event.target.value)} placeholder="track_…" />
            </> : null}
            <button type="submit" disabled={!selectedProject || !timelineDocument || !timelineVersionId || !timelineClipId}>Apply typed timeline command</button>
            <button type="button" onClick={() => void undoTimelineEdit()} disabled={!timelineHistory.length}>Undo as a new server mutation</button>
          </form>
          <section className="compact-form" aria-labelledby="review-actions-title">
            <h3 id="review-actions-title">Generic JobStep review</h3>
            <button type="button" onClick={() => void loadReviewSteps()} disabled={!selectedJob}>Load reviewable steps</button>
            <label htmlFor="review-step-id">JobStep</label>
            <select id="review-step-id" value={reviewStepId} onChange={(event) => setReviewStepId(event.target.value)}>
              <option value="">Select JobStep</option>
              {steps.map((step) => <option key={stringValue(step.id)} value={stringValue(step.id)}>{stringValue(step.node_key)} · {stringValue(step.status)}</option>)}
            </select>
            <label htmlFor="review-resource-type">Exact resource type</label>
            <input id="review-resource-type" value={reviewResourceType} onChange={(event) => setReviewResourceType(event.target.value)} />
            <label htmlFor="review-resource-id">Exact resource ID</label>
            <input id="review-resource-id" value={reviewResourceId} onChange={(event) => setReviewResourceId(event.target.value)} />
            <label htmlFor="review-resource-revision">Exact resource revision</label>
            <input id="review-resource-revision" value={reviewResourceRevision} onChange={(event) => setReviewResourceRevision(event.target.value)} inputMode="numeric" />
            <div className="inline-form">
              <button type="button" onClick={() => void resolveReview("approve")} disabled={!selectedJob || !reviewStepId || !reviewResourceId}>Approve exact resource</button>
              <button type="button" onClick={() => void resolveReview("reject")} disabled={!selectedJob || !reviewStepId}>Reject / edit then resume</button>
              <button type="button" onClick={() => void resolveReview("resume")} disabled={!selectedJob}>Resume descendants</button>
            </div>
          </section>
          <section className="compact-form" aria-labelledby="scene-list-title">
            <h3 id="scene-list-title">Scenes loaded for this Project</h3>
            {scenes.length ? <ul className="resource-list">{scenes.map((scene) => <li key={stringValue(scene.id)}>{stringValue(scene.id)} · {stringValue(scene.status)} · {String(scene.source_start_sec || 0)}–{String(scene.source_end_sec || 0)}</li>)}</ul> : <p className="muted">No server Scenes are available yet.</p>}
          </section>
          {selectedJob ? <pre aria-label="Selected job state">{JSON.stringify(selectedJob, null, 2)}</pre> : <p>Select a Job to watch progress and reconnect.</p>}
        </section>
      </div>
    </main>
  );
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function parseObject(value: unknown): Record<string, unknown> {
  if (typeof value === "string") {
    const parsed: unknown = JSON.parse(value);
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) return parsed as Record<string, unknown>;
  }
  if (value && typeof value === "object" && !Array.isArray(value)) return value as Record<string, unknown>;
  throw new Error("The server returned an invalid canonical document.");
}

function objectValue(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

function firstTrackId(document: Record<string, unknown>): string {
  const tracks = Array.isArray(document.tracks) ? document.tracks : [];
  return tracks.length ? stringValue(objectValue(tracks[0]).id) : "";
}

function firstClipId(document: Record<string, unknown>): string {
  const tracks = Array.isArray(document.tracks) ? document.tracks : [];
  for (const rawTrack of tracks) {
    const clips = objectValue(rawTrack).clips;
    if (Array.isArray(clips) && clips.length) return stringValue(objectValue(clips[0]).id);
  }
  return "";
}

function findClip(document: Record<string, unknown>, id: string): Record<string, unknown> | null {
  const tracks = Array.isArray(document.tracks) ? document.tracks : [];
  for (const rawTrack of tracks) {
    const clips = objectValue(rawTrack).clips;
    if (!Array.isArray(clips)) continue;
    for (const rawClip of clips) if (stringValue(objectValue(rawClip).id) === id) return objectValue(rawClip);
  }
  return null;
}

function safeMessage(value: unknown): string {
  if (value instanceof ProductApiError) return value.message;
  if (value instanceof Error) return value.message;
  return "The request could not be completed.";
}
