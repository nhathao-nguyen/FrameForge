"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { ProductApiClient, ProductApiError, SseEnvelope } from "@nh-media/sdk";

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
  const [scriptId, setScriptId] = useState("");
  const [scriptBasedOn, setScriptBasedOn] = useState("");
  const [scriptRevision, setScriptRevision] = useState("0");
  const [scriptContent, setScriptContent] = useState('{"title":"Draft script","blocks":[]}');
  const [timelineId, setTimelineId] = useState("");
  const [timelineVersionId, setTimelineVersionId] = useState("");
  const [timelineClipId, setTimelineClipId] = useState("");
  const [timelineExpectedVersion, setTimelineExpectedVersion] = useState("0");
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
    const result = await api.listJobs(stringValue(project.id));
    setJobs(result.items || []);
    setStatus("Project loaded from server-authoritative state.");
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

  async function saveScriptVersion(event: FormEvent) {
    event.preventDefault();
    if (!selectedProject || !scriptId.trim()) return;
    setError("");
    try {
      const content = JSON.parse(scriptContent) as Record<string, unknown>;
      await api.createScriptVersion(stringValue(selectedProject.id), scriptId.trim(), {
        based_on_version_id: scriptBasedOn.trim(),
        language: "vi",
        origin: "user",
        content,
      }, Number(scriptRevision));
      setStatus("Script draft submitted as an immutable server version.");
    } catch (value) {
      setError(value instanceof SyntaxError ? "Script content must be valid JSON." : safeMessage(value));
    }
  }

  async function applyTimelineEdit(event: FormEvent) {
    event.preventDefault();
    if (!selectedProject || !timelineId.trim() || !timelineVersionId.trim() || !timelineClipId.trim()) return;
    setError("");
    try {
      await api.timelineCommand(stringValue(selectedProject.id), timelineId.trim(), timelineVersionId.trim(), {
        kind: "UpdateScene",
        schema_version: "1.0",
        expected_version: Number(timelineExpectedVersion),
        payload: { clip_id: timelineClipId.trim(), metadata: { edited_from: "web", title: "User scene override" } },
      });
      setStatus("Timeline command accepted as a new user-origin TimelineVersion.");
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
            <input id="script-id" value={scriptId} onChange={(event) => setScriptId(event.target.value)} placeholder="scr_…" />
            <label htmlFor="script-based-on">Based-on version ID</label>
            <input id="script-based-on" value={scriptBasedOn} onChange={(event) => setScriptBasedOn(event.target.value)} placeholder="optional" />
            <label htmlFor="script-revision">Expected script revision</label>
            <input id="script-revision" value={scriptRevision} onChange={(event) => setScriptRevision(event.target.value)} inputMode="numeric" />
            <label htmlFor="script-content">Draft content JSON</label>
            <textarea id="script-content" value={scriptContent} onChange={(event) => setScriptContent(event.target.value)} rows={4} />
            <button type="submit" disabled={!selectedProject || !scriptId.trim()}>Save immutable script version</button>
          </form>
          <form onSubmit={applyTimelineEdit} className="stack compact-form">
            <h3>Timeline / scene editor</h3>
            <label htmlFor="timeline-id">Timeline ID</label>
            <input id="timeline-id" value={timelineId} onChange={(event) => setTimelineId(event.target.value)} placeholder="tl_…" />
            <label htmlFor="timeline-version-id">Based-on TimelineVersion ID</label>
            <input id="timeline-version-id" value={timelineVersionId} onChange={(event) => setTimelineVersionId(event.target.value)} placeholder="tlv_…" />
            <label htmlFor="timeline-clip-id">Scene / clip ID</label>
            <input id="timeline-clip-id" value={timelineClipId} onChange={(event) => setTimelineClipId(event.target.value)} placeholder="clip_…" />
            <label htmlFor="timeline-expected-version">Expected document version</label>
            <input id="timeline-expected-version" value={timelineExpectedVersion} onChange={(event) => setTimelineExpectedVersion(event.target.value)} inputMode="numeric" />
            <button type="submit" disabled={!selectedProject || !timelineId.trim() || !timelineVersionId.trim() || !timelineClipId.trim()}>Apply scene override</button>
          </form>
          {selectedJob ? <pre aria-label="Selected job state">{JSON.stringify(selectedJob, null, 2)}</pre> : <p>Select a Job to watch progress and reconnect.</p>}
        </section>
      </div>
    </main>
  );
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function safeMessage(value: unknown): string {
  if (value instanceof ProductApiError) return value.message;
  if (value instanceof Error) return value.message;
  return "The request could not be completed.";
}
