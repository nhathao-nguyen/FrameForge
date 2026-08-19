import type { FormEvent } from "react";
import type { ScriptViewModel } from "../../../types/workspace";
import { stringValue } from "../../../services/value-utils";

export function ScriptEditor({ model }: { model: ScriptViewModel }) {
  function save(event: FormEvent) {
    event.preventDefault();
    void model.onSave();
  }

  return <form onSubmit={save} className="stack compact-form">
    <h3>Script editor</h3>
    <label htmlFor="script-id">Script ID</label>
    <input id="script-id" value={model.scriptId} onChange={(event) => model.onScriptIdChange(event.target.value)} placeholder="script_…" />
    <button type="button" onClick={() => void model.onLoad()} disabled={!model.selectedProject || !model.scriptId.trim()}>Load script state</button>
    <p className="muted">Revision {String(model.scriptState?.revision || "—")} · current {stringValue(model.scriptState?.current_version_id) || "—"}</p>
    <label htmlFor="script-content">Draft content JSON</label>
    <textarea id="script-content" value={model.scriptContent} onChange={(event) => model.onScriptContentChange(event.target.value)} rows={4} />
    <button type="submit" disabled={!model.selectedProject || !model.scriptId.trim() || !model.scriptState}>Save immutable script version</button>
    <div className="inline-form">
      <button type="button" onClick={() => void model.onApprove()} disabled={!model.scriptVersion}>Approve version</button>
      <button type="button" onClick={() => void model.onReject()} disabled={!model.scriptVersion}>Reject version</button>
    </div>
  </form>;
}
