export interface TimelineHistoryEntry {
  projectId: string;
  timelineId: string;
  previousVersionId: string;
  resultVersionId: string;
  clipId: string;
  previousClip: Record<string, unknown>;
}

export interface TimelineHistoryReloadContext {
  projectId: string;
  timelineId: string;
  currentVersionId: string;
  previousProjectId?: string;
  previousTimelineId?: string;
  previousVersionId?: string;
  expectedVersionId?: string;
  reset?: boolean;
}

export function cloneTimelineClip(clip: Record<string, unknown>): Record<string, unknown> {
  return JSON.parse(JSON.stringify(clip)) as Record<string, unknown>;
}

export function appendTimelineHistory(history: TimelineHistoryEntry[], entry: TimelineHistoryEntry): TimelineHistoryEntry[] {
  return [...history, { ...entry, previousClip: cloneTimelineClip(entry.previousClip) }];
}

export function reconcileTimelineHistory(history: TimelineHistoryEntry[], context: TimelineHistoryReloadContext): TimelineHistoryEntry[] {
  if (context.reset || context.previousProjectId !== context.projectId || context.previousTimelineId !== context.timelineId) {
    return [];
  }
  if (context.expectedVersionId !== undefined && context.currentVersionId !== context.expectedVersionId) {
    return [];
  }
  if (context.expectedVersionId === undefined && context.previousVersionId && context.currentVersionId !== context.previousVersionId) {
    return [];
  }
  return history;
}

export function canUndoTimelineHistory(history: TimelineHistoryEntry[], projectId: string, timelineId: string, currentVersionId: string): boolean {
  const entry = history[history.length - 1];
  return Boolean(entry && entry.projectId === projectId && entry.timelineId === timelineId && entry.resultVersionId === currentVersionId);
}

export function completeTimelineUndo(history: TimelineHistoryEntry[], currentVersionId: string): TimelineHistoryEntry[] {
  const remaining = history.slice(0, -1);
  if (remaining.length === 0) return remaining;
  const last = remaining[remaining.length - 1];
  remaining[remaining.length - 1] = { ...last, resultVersionId: currentVersionId };
  return remaining;
}

export function makeRestoreClipCommand(entry: TimelineHistoryEntry, currentVersion: number): Record<string, unknown> {
  return {
    kind: "RestoreClip",
    schema_version: "1.0",
    expected_version: currentVersion,
    payload: {
      clip_id: entry.clipId,
      restore_from_version_id: entry.previousVersionId,
      // The Product API replaces this with the exact historical server clip.
      // Keeping the snapshot here supports offline request construction and
      // makes the command auditable without making the browser authoritative.
      clip: cloneTimelineClip(entry.previousClip),
    },
  };
}
