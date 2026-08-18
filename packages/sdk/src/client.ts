export type ApiFetcher = typeof fetch;

export interface Page<T> {
  items: T[];
  next_cursor?: string | null;
  next_sequence?: number | null;
}

export interface ApiErrorBody {
  error?: { code?: string; message?: string; request_id?: string; correlation_id?: string };
  serialization_version?: string;
}

export class ProductApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId?: string;

  constructor(status: number, body: ApiErrorBody | undefined) {
    super(body?.error?.message || "The Product API request failed.");
    this.name = "ProductApiError";
    this.status = status;
    this.code = body?.error?.code || "api_request_failed";
    this.requestId = body?.error?.request_id;
  }
}

export interface ProductApiClientOptions {
  baseUrl: string;
  fetcher?: ApiFetcher;
  onToken?: (token: string | null) => void;
}

export interface SseEnvelope<T = unknown> {
  id: number;
  event: string;
  data: T;
}

export class ProductApiClient {
  private readonly fetcher: ApiFetcher;
  private readonly onToken?: (token: string | null) => void;
  private baseUrl: string;
  private token: string | null = null;

  constructor(options: ProductApiClientOptions) {
    this.baseUrl = normalizeBaseUrl(options.baseUrl);
    this.fetcher = options.fetcher || fetch;
    this.onToken = options.onToken;
  }

  setBaseUrl(baseUrl: string): void {
    this.baseUrl = normalizeBaseUrl(baseUrl);
  }

  setToken(token: string | null): void {
    this.token = token;
    this.onToken?.(token);
  }

  getToken(): string | null {
    return this.token;
  }

  async login(username: string, password: string): Promise<{ access_token: string; subject: string }> {
    const value = await this.request<{ access_token: string; subject: string }>("/auth/local/login", {
      method: "POST",
      body: { username, password },
      authenticated: false,
    });
    this.setToken(value.access_token);
    return value;
  }

  async logout(): Promise<void> {
    try {
      await this.request("/auth/logout", { method: "POST" });
    } finally {
      this.setToken(null);
    }
  }

  async session(): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/auth/session");
  }

  async listProjects(cursor?: string): Promise<Page<Record<string, unknown>>> {
    return this.request<Page<Record<string, unknown>>>("/projects" + query({ cursor }));
  }

  async createProject(name: string, workflowKey = "movie_recap"): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects", {
      method: "POST",
      body: { name, workflow_key: workflowKey },
      idempotencyKey: "web-project-" + crypto.randomUUID(),
    });
  }

  async listJobs(projectId: string, cursor?: string): Promise<Page<Record<string, unknown>>> {
    return this.request<Page<Record<string, unknown>>>("/projects/" + encode(projectId) + "/jobs" + query({ cursor }));
  }

  async listSteps(projectId: string, jobId: string, runId?: string): Promise<Page<Record<string, unknown>>> {
    return this.request<Page<Record<string, unknown>>>("/projects/" + encode(projectId) + "/jobs/" + encode(jobId) + "/steps" + query({ pipeline_run_id: runId }));
  }

  async getJob(projectId: string, jobId: string): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/jobs/" + encode(jobId));
  }

  async resumeJob(projectId: string, jobId: string): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/jobs/" + encode(jobId) + "/resume", { method: "POST", body: {} });
  }

  async createJob(projectId: string, input: Record<string, unknown>): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/jobs", {
      method: "POST",
      body: input,
      idempotencyKey: "web-job-" + crypto.randomUUID(),
    });
  }

  async createUploadSession(projectId: string, body: Record<string, unknown>): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/assets/upload-sessions", {
      method: "POST",
      body,
      idempotencyKey: "web-upload-" + crypto.randomUUID(),
    });
  }

  async uploadPart(url: string, data: Blob, headers: Record<string, string> = {}): Promise<string | null> {
    const response = await this.fetcher(url, { method: "PUT", body: data, headers });
    if (!response.ok) throw new ProductApiError(response.status, await safeJson(response));
    return response.headers.get("ETag");
  }

  async completeUpload(projectId: string, assetId: string, uploadId: string, parts: unknown[]): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/assets/" + encode(assetId) + "/upload-sessions/" + encode(uploadId) + "/complete", {
      method: "POST",
      body: { parts },
    });
  }

  async downloadArtifact(projectId: string, artifactId: string): Promise<{ artifact_id: string; url: string; expires_at: string }> {
    return this.request<{ artifact_id: string; url: string; expires_at: string }>("/projects/" + encode(projectId) + "/artifacts/" + encode(artifactId) + "/download");
  }

  async approveReview(projectId: string, jobId: string, stepId: string, selectedResourceType: string, selectedResourceId: string, selectedResourceRevision: number): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/jobs/" + encode(jobId) + "/reviews/" + encode(stepId) + "/approve", {
      method: "POST",
      body: { selected_resource_type: selectedResourceType, selected_resource_id: selectedResourceId, selected_resource_revision: selectedResourceRevision },
    });
  }

  async rejectReview(projectId: string, jobId: string, stepId: string, action: "fail" | "edit_then_resume", reason?: string): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/jobs/" + encode(jobId) + "/reviews/" + encode(stepId) + "/reject", {
      method: "POST",
      body: { action, reason },
    });
  }

  async getTimeline(projectId: string, timelineId: string): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/timelines/" + encode(timelineId));
  }

  async listTimelines(projectId: string, cursor?: string): Promise<Page<Record<string, unknown>>> {
    return this.request<Page<Record<string, unknown>>>("/projects/" + encode(projectId) + "/timelines" + query({ cursor }));
  }

  async listTimelineVersions(projectId: string, timelineId: string, cursor?: string): Promise<Page<Record<string, unknown>>> {
    return this.request<Page<Record<string, unknown>>>("/projects/" + encode(projectId) + "/timelines/" + encode(timelineId) + "/versions" + query({ cursor }));
  }

  async timelineCommand(projectId: string, timelineId: string, versionId: string, command: Record<string, unknown>, expectedRevision?: number): Promise<Record<string, unknown>> {
    const revision = expectedRevision ?? (typeof command.expected_revision === "number" ? command.expected_revision : undefined);
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/timelines/" + encode(timelineId) + "/commands", {
      method: "POST",
      body: { ...command, based_on_version_id: versionId },
      ifMatch: typeof revision === "number" ? "\"" + revision + "\"" : undefined,
    });
  }

  async restoreTimelineClip(projectId: string, timelineId: string, versionId: string, restoreFromVersionId: string, clipId: string, clip: Record<string, unknown>, expectedVersion: number, expectedRevision: number): Promise<Record<string, unknown>> {
    return this.timelineCommand(projectId, timelineId, versionId, {
      kind: "RestoreClip",
      schema_version: "1.0",
      expected_version: expectedVersion,
      payload: { clip_id: clipId, restore_from_version_id: restoreFromVersionId, clip },
    }, expectedRevision);
  }

  async createScript(projectId: string, body: Record<string, unknown>): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/scripts", {
      method: "POST", body, idempotencyKey: "web-script-" + crypto.randomUUID(),
    });
  }

  async getScript(projectId: string, scriptId: string): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/scripts/" + encode(scriptId));
  }

  async createScriptVersion(projectId: string, scriptId: string, body: Record<string, unknown>, expectedRevision?: number): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/scripts/" + encode(scriptId) + "/versions", {
      method: "POST",
      body,
      ifMatch: expectedRevision === undefined ? undefined : `"${expectedRevision}"`,
    });
  }

  async approveScriptVersion(projectId: string, scriptId: string, versionId: string): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/scripts/" + encode(scriptId) + "/versions/" + encode(versionId) + "/approve", { method: "POST" });
  }

  async getScriptVersion(projectId: string, scriptId: string, versionId: string): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/scripts/" + encode(scriptId) + "/versions/" + encode(versionId));
  }

  async rejectScriptVersion(projectId: string, scriptId: string, versionId: string, reason?: string): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/scripts/" + encode(scriptId) + "/versions/" + encode(versionId) + "/reject", { method: "POST", body: { reason } });
  }

  async listScenes(projectId: string, cursor?: string): Promise<Page<Record<string, unknown>>> {
    return this.request<Page<Record<string, unknown>>>("/projects/" + encode(projectId) + "/scenes" + query({ cursor }));
  }

  async getTimelineVersion(projectId: string, timelineId: string, versionId: string): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/timelines/" + encode(timelineId) + "/versions/" + encode(versionId));
  }

  async approveTimelineVersion(projectId: string, timelineId: string, versionId: string): Promise<Record<string, unknown>> {
    return this.request<Record<string, unknown>>("/projects/" + encode(projectId) + "/timelines/" + encode(timelineId) + "/versions/" + encode(versionId) + "/approve", { method: "POST", body: {} });
  }

  async *events<T = unknown>(projectId: string, jobId: string, signal?: AbortSignal): AsyncGenerator<SseEnvelope<T>> {
    let after = 0;
    let reconnectDelay = 500;
    while (!signal?.aborted) {
      let response: Response;
      try {
        response = await this.fetcher(this.baseUrl + "/projects/" + encode(projectId) + "/jobs/" + encode(jobId) + "/events/stream", {
          headers: this.headers({ "Last-Event-ID": String(after) }),
          signal,
        });
      } catch (error) {
        if (signal?.aborted) return;
        await delay(reconnectDelay, signal);
        reconnectDelay = Math.min(reconnectDelay * 2, 8000);
        continue;
      }
      if (!response.ok || !response.body) throw new ProductApiError(response.status, await safeJson(response));
      reconnectDelay = 500;
      try {
        for await (const frame of parseSse(response.body, signal)) {
          if (frame.id > 0) after = frame.id;
          yield frame as SseEnvelope<T>;
        }
      } catch (error) {
        if (signal?.aborted) return;
        await delay(reconnectDelay, signal);
        reconnectDelay = Math.min(reconnectDelay * 2, 8000);
      }
    }
  }

  private headers(extra: Record<string, string> = {}): Record<string, string> {
    return { Accept: "application/json", ...(this.token ? { Authorization: "Bearer " + this.token } : {}), ...extra };
  }

  private async request<T = unknown>(path: string, options: {
    method?: string;
    body?: unknown;
    authenticated?: boolean;
    idempotencyKey?: string;
    ifMatch?: string;
  } = {}): Promise<T> {
    const response = await this.fetcher(this.baseUrl + path, {
      method: options.method || "GET",
      headers: this.headers({
        ...(options.body === undefined ? {} : { "Content-Type": "application/json" }),
        ...(options.idempotencyKey ? { "Idempotency-Key": options.idempotencyKey } : {}),
        ...(options.ifMatch ? { "If-Match": options.ifMatch } : {}),
      }),
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
    });
    if (!response.ok) throw new ProductApiError(response.status, await safeJson(response));
    if (response.status === 204) return undefined as T;
    return (await response.json()) as T;
  }
}

async function safeJson(response: Response): Promise<ApiErrorBody | undefined> {
  try {
    return (await response.json()) as ApiErrorBody;
  } catch {
    return undefined;
  }
}

async function* parseSse(body: ReadableStream<Uint8Array>, signal?: AbortSignal): AsyncGenerator<SseEnvelope> {
  const reader = body.pipeThrough(new TextDecoderStream()).getReader();
  let buffer = "";
  let id = 0;
  let event = "message";
  let data: string[] = [];
  try {
    while (!signal?.aborted) {
      const next = await reader.read();
      if (next.done) break;
      buffer += String(next.value);
      const lines = buffer.split(/\r?\n/);
      buffer = lines.pop() || "";
      for (const line of lines) {
        if (line === "") {
          if (data.length > 0) {
            let parsed: unknown = data.join("\n");
            try { parsed = JSON.parse(data.join("\n")); } catch { /* safe text is still surfaced */ }
            yield { id, event, data: parsed };
          }
          data = [];
          event = "message";
        } else if (line.startsWith("id:")) {
          const parsed = Number(line.slice(3).trim());
          if (Number.isFinite(parsed)) id = parsed;
        } else if (line.startsWith("event:")) {
          event = line.slice(6).trim();
        } else if (line.startsWith("data:")) {
          data.push(line.slice(5).trimStart());
        }
      }
    }
  } finally {
    await reader.cancel();
  }
}

function normalizeBaseUrl(value: string): string {
  const trimmed = value.trim();
  if (!trimmed || !/^https?:\/\//i.test(trimmed)) throw new Error("Product API endpoint must be an explicit HTTP(S) URL.");
  return trimmed.replace(/\/+$/, "").replace(/\/api\/v1$/i, "") + "/api/v1";
}

function encode(value: string): string {
  return encodeURIComponent(value);
}

function query(values: Record<string, string | undefined>): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(values)) if (value) params.set(key, value);
  const serialized = params.toString();
  return serialized ? "?" + serialized : "";
}

function delay(milliseconds: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(resolve, milliseconds);
    signal?.addEventListener("abort", () => { clearTimeout(timer); reject(new DOMException("Aborted", "AbortError")); }, { once: true });
  });
}
