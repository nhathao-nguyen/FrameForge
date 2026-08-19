import type { FormEvent } from "react";
import type { TimelineViewModel } from "../../../types/workspace";

export function TimelineEditor({ model }: { model: TimelineViewModel }) {
  function apply(event: FormEvent) {
    event.preventDefault();
    void model.onApply();
  }

  return <form onSubmit={apply} className="stack compact-form">
    <h3>Timeline / scene editor</h3>
    <label htmlFor="timeline-id">Timeline ID</label>
    <input id="timeline-id" value={model.timelineId} onChange={(event) => model.onTimelineIdChange(event.target.value)} placeholder="timeline_…" />
    <button type="button" onClick={() => void model.onLoad()} disabled={!model.selectedProject || !model.timelineId.trim()}>Load timeline + current version</button>
    <p className="muted">Aggregate revision {String(model.timelineState?.revision || "—")} · loaded version {model.timelineVersionId || "—"}</p>
    <label htmlFor="timeline-clip-id">Scene / clip ID</label>
    <input id="timeline-clip-id" value={model.timelineClipId} onChange={(event) => model.onTimelineClipIdChange(event.target.value)} placeholder="clip_…" />
    <label htmlFor="timeline-command-kind">Typed command</label>
    <select id="timeline-command-kind" value={model.timelineEditKind} onChange={(event) => model.onTimelineEditKindChange(event.target.value)}>
      <option value="UpdateScene">UpdateScene</option><option value="MoveClip">MoveClip</option><option value="ReplaceClip">ReplaceClip</option>
    </select>
    {model.timelineEditKind === "MoveClip" ? <>
      <label htmlFor="timeline-target-track">Target track</label>
      <input id="timeline-target-track" value={model.timelineTargetTrack} onChange={(event) => model.onTimelineTargetTrackChange(event.target.value)} placeholder="track_…" />
    </> : null}
    <button type="submit" disabled={!model.selectedProject || !model.timelineDocument || !model.timelineVersionId || !model.timelineClipId}>Apply typed timeline command</button>
    <button type="button" onClick={() => void model.onUndo()} disabled={!model.timelineHistory.length}>Undo as a new server mutation</button>
  </form>;
}
