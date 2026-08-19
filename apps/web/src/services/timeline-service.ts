import type { ProductApiClient } from "@nh-media/sdk";
import type { UnknownRecord } from "../types/workspace";
import { findClip, objectValue, parseObject, stringValue } from "./value-utils";

export interface LoadedTimelineState { timeline: UnknownRecord; versionId: string; document: UnknownRecord; }

export async function loadTimelineState(api: ProductApiClient, projectId: string, timelineId: string): Promise<LoadedTimelineState> {
  const timeline = await api.getTimeline(projectId, timelineId);
  const versionId = stringValue(timeline.current_version_id);
  if (!versionId) throw new Error("Timeline has no current TimelineVersion.");
  const version = await api.getTimelineVersion(projectId, timelineId, versionId);
  return { timeline, versionId, document: parseObject(version.document) };
}

export async function applyTimelineCommand(api: ProductApiClient, input: {
  projectId: string; timelineId: string; versionId: string; document: UnknownRecord;
  clipId: string; commandKind: string; targetTrack: string; expectedRevision: number;
}): Promise<{ result: UnknownRecord; previousClip: UnknownRecord }> {
  const clip = findClip(input.document, input.clipId);
  if (!clip) throw new Error("Selected clip is not present in the loaded TimelineVersion.");
  const payload: UnknownRecord = input.commandKind === "MoveClip"
    ? { clip_id: input.clipId, track_id: input.targetTrack }
    : input.commandKind === "ReplaceClip"
      ? { clip_id: input.clipId, clip: { ...clip, metadata: { ...objectValue(clip.metadata), edited_from: "web" } } }
      : { clip_id: input.clipId, metadata: { edited_from: "web", title: "User scene override" } };
  const result = await api.timelineCommand(input.projectId, input.timelineId, input.versionId, {
    kind: input.commandKind, schema_version: "1.0", expected_version: Number(input.document.version || 0), payload,
  }, input.expectedRevision);
  return { result, previousClip: clip };
}

export async function undoTimelineCommand(api: ProductApiClient, input: {
  projectId: string; timelineId: string; versionId: string; previousVersionId: string;
  clipId: string; previousClip: UnknownRecord; expectedVersion: number; expectedRevision: number;
}): Promise<UnknownRecord> {
  return api.restoreTimelineClip(input.projectId, input.timelineId, input.versionId, input.previousVersionId, input.clipId, input.previousClip, input.expectedVersion, input.expectedRevision);
}
