import type { SceneViewModel } from "../../../types/workspace";
import { stringValue } from "../../../services/value-utils";

export function SceneList({ model }: { model: SceneViewModel }) {
  return <section className="compact-form" aria-labelledby="scene-list-title">
    <h3 id="scene-list-title">Scenes loaded for this Project</h3>
    {model.scenes.length ? <ul className="resource-list">{model.scenes.map((scene) => <li key={stringValue(scene.id)}>{stringValue(scene.id)} · {stringValue(scene.status)} · {String(scene.source_start_sec || 0)}–{String(scene.source_end_sec || 0)}</li>)}</ul> : <p className="muted">No server Scenes are available yet.</p>}
  </section>;
}
