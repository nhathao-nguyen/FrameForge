import type { UnknownRecord } from "../types/workspace";

export function stringValue(value: unknown): string { return typeof value === "string" ? value : ""; }

export function parseObject(value: unknown): UnknownRecord {
  if (typeof value === "string") {
    const parsed: unknown = JSON.parse(value);
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) return parsed as UnknownRecord;
  }
  if (value && typeof value === "object" && !Array.isArray(value)) return value as UnknownRecord;
  throw new Error("The server returned an invalid canonical document.");
}

export function objectValue(value: unknown): UnknownRecord {
  return value && typeof value === "object" && !Array.isArray(value) ? value as UnknownRecord : {};
}

export function firstTrackId(document: UnknownRecord): string {
  const tracks = Array.isArray(document.tracks) ? document.tracks : [];
  return tracks.length ? stringValue(objectValue(tracks[0]).id) : "";
}

export function firstClipId(document: UnknownRecord): string {
  const tracks = Array.isArray(document.tracks) ? document.tracks : [];
  for (const rawTrack of tracks) {
    const clips = objectValue(rawTrack).clips;
    if (Array.isArray(clips) && clips.length) return stringValue(objectValue(clips[0]).id);
  }
  return "";
}

export function findClip(document: UnknownRecord, id: string): UnknownRecord | null {
  const tracks = Array.isArray(document.tracks) ? document.tracks : [];
  for (const rawTrack of tracks) {
    const clips = objectValue(rawTrack).clips;
    if (!Array.isArray(clips)) continue;
    for (const rawClip of clips) if (stringValue(objectValue(rawClip).id) === id) return objectValue(rawClip);
  }
  return null;
}
