import assert from "node:assert/strict";
import { test } from "node:test";
import { ProductApiClient, ProductApiError } from "../src/client.ts";

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
