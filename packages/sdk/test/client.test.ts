import assert from "node:assert/strict";
import { test } from "node:test";
import { ProductApiClient, ProductApiError } from "../src/client.ts";
import {
  appendTimelineHistory,
  canUndoTimelineHistory,
  completeTimelineUndo,
  makeRestoreClipCommand,
  reconcileTimelineHistory,
} from "../src/timeline-history.ts";
import type { TimelineHistoryEntry } from "../src/timeline-history.ts";

function jsonResponse(status: number, value: unknown, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify(value), { status, headers: { "content-type": "application/json", ...headers } });
}

test("timeline commands send auth and exact aggregate If-Match revision", async () => {
  const requests: Request[] = [];
  const client = new ProductApiClient({
    baseUrl: "http://127.0.0.1:8080",
    fetcher: async (input, init) => {
      requests.push(new Request(input, init));
      return jsonResponse(201, { id: "timeline_version_2" });
    },
  });
  client.setToken("session-token");
  await client.timelineCommand("project_1", "timeline_1", "version_1", { kind: "MoveClip", expected_version: 1, payload: {} }, 7);
  assert.equal(requests[0].headers.get("authorization"), "Bearer session-token");
  assert.equal(requests[0].headers.get("if-match"), '"7"');
  const body = await requests[0].clone().json() as Record<string, unknown>;
  assert.equal(body.based_on_version_id, "version_1");
});

test("SSE parser yields event data and reconnects from Last-Event-ID", async () => {
  const seen: Request[] = [];
  const controller = new AbortController();
  const client = new ProductApiClient({
    baseUrl: "http://127.0.0.1:8080",
    fetcher: async (input, init) => {
      seen.push(new Request(input, init));
      return new Response("id: 12\nevent: stream.snapshot\ndata: {\"job\":{\"status\":\"running\"}}\n\n", { status: 200 });
    },
  });
  const iterator = client.events("project_1", "job_1", controller.signal);
  const first = await iterator.next();
  assert.equal(first.value?.id, 12);
  assert.equal(first.value?.event, "stream.snapshot");
  assert.deepEqual(first.value?.data, { job: { status: "running" } });
  assert.equal(seen[0].headers.get("last-event-id"), "0");
  controller.abort();
  await iterator.return?.();
});

test("non-JSON API errors remain safe ProductApiError values", async () => {
  const client = new ProductApiClient({
    baseUrl: "http://127.0.0.1:8080",
    fetcher: async () => new Response("secret traceback", { status: 503 }),
  });
  await assert.rejects(() => client.getJob("project_1", "job_1"), (error: unknown) => {
    assert.ok(error instanceof ProductApiError);
    assert.equal(error.status, 503);
    assert.equal(error.message, "The Product API request failed.");
    return true;
  });
});

test("timeline edit reload undo uses current V2, creates immutable V3, and preserves provenance", async () => {
  const v1Clip = {
    id: "clip_1", timeline_in_sec: 0, timeline_out_sec: 4,
    source: { type: "none", inline_id: "clip_1" }, origin: "ai",
    proposal_refs: ["proposal_1"], evidence_refs: ["evidence_1"],
  } as Record<string, unknown>;
  const versions: Record<string, { version: number; clip: Record<string, unknown> }> = {
    v1: { version: 1, clip: clone(v1Clip) },
  };
  let currentVersionId = "v1";
  let revision = 1;
  const requests: Request[] = [];
  const client = new ProductApiClient({
    baseUrl: "http://127.0.0.1:8080",
    fetcher: async (input, init) => {
      const request = new Request(input, init);
      requests.push(request);
      const body = await request.clone().json() as Record<string, any>;
      const expectedRevision = Number((request.headers.get("if-match") || "").replaceAll('"', ""));
      if (expectedRevision !== revision) return jsonResponse(412, { error: { code: "VERSION_CONFLICT", message: "stale" } });
      if (body.based_on_version_id !== currentVersionId) return jsonResponse(412, { error: { code: "VERSION_CONFLICT", message: "stale base" } });
      const base = versions[currentVersionId];
      const command = body as any;
      let nextClip: Record<string, unknown>;
      if (command.kind === "RestoreClip") {
        assert.equal(command.payload.restore_from_version_id, "v1");
        nextClip = clone(versions[command.payload.restore_from_version_id].clip);
      } else {
        nextClip = clone(base.clip);
        nextClip.metadata = { edited_from: "web" };
        nextClip.origin = "user";
      }
      const nextId = command.kind === "RestoreClip" ? "v3" : "v2";
      versions[nextId] = { version: base.version + 1, clip: nextClip };
      currentVersionId = nextId;
      revision += 1;
      return jsonResponse(201, { id: nextId, version: base.version + 1 });
    },
  });

  const edit = await client.timelineCommand("project_1", "timeline_1", "v1", {
    kind: "UpdateScene", schema_version: "1.0", expected_version: 1, payload: { clip_id: "clip_1" },
  }, 1);
  const entry: TimelineHistoryEntry = {
    projectId: "project_1", timelineId: "timeline_1", previousVersionId: "v1",
    resultVersionId: String(edit.id), clipId: "clip_1", previousClip: clone(v1Clip),
  };
  let history = appendTimelineHistory([], entry);
  assert.deepEqual(reconcileTimelineHistory(history, {
    projectId: "project_1", timelineId: "timeline_1", currentVersionId: "v2",
    previousProjectId: "project_1", previousTimelineId: "timeline_1", previousVersionId: "v1",
    expectedVersionId: "v2",
  }), history);
  assert.equal(canUndoTimelineHistory(history, "project_1", "timeline_1", "v2"), true);

  const undoCommand = makeRestoreClipCommand(entry, versions.v2.version);
  assert.equal(undoCommand.expected_version, 2);
  const undone = await client.restoreTimelineClip("project_1", "timeline_1", "v2", "v1", "clip_1", entry.previousClip, 2, 2);
  assert.equal(undone.id, "v3");
  assert.deepEqual(versions.v1.clip, v1Clip);
  assert.deepEqual(versions.v2.clip, { ...v1Clip, metadata: { edited_from: "web" }, origin: "user" });
  assert.deepEqual(versions.v3.clip, v1Clip);
  assert.equal(requests[1].headers.get("if-match"), '"2"');
  assert.equal((await requests[1].clone().json() as any).based_on_version_id, "v2");
  assert.deepEqual(completeTimelineUndo(history, "v3"), []);

  // A failed/stale mutation does not consume the local undo entry.
  await assert.rejects(() => client.restoreTimelineClip("project_1", "timeline_1", "v3", "v1", "clip_1", entry.previousClip, 3, 1), (error: unknown) => {
    assert.ok(error instanceof ProductApiError);
    assert.equal(error.status, 412);
    return true;
  });
  assert.equal(history.length, 1);
  history = reconcileTimelineHistory(history, {
    projectId: "project_2", timelineId: "timeline_1", currentVersionId: "v3",
    previousProjectId: "project_1", previousTimelineId: "timeline_1", previousVersionId: "v2",
  });
  assert.deepEqual(history, []);
});

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}
