import type { JobViewModel } from "../../../types/workspace";
import { stringValue } from "../../../services/value-utils";

export function JobsPanel({ model }: { model: JobViewModel }) {
  return <section className="card" aria-labelledby="jobs-title">
    <h2 id="jobs-title">Jobs{model.selectedProject ? " · " + stringValue(model.selectedProject.name) : ""}</h2>
    <button type="button" onClick={() => void model.onStartDeterministic()} disabled={!model.selectedProject}>Run deterministic worker path</button>
    <ul className="resource-list">
      {model.jobs.map((job) => <li key={stringValue(job.id)}><button type="button" className="list-button" onClick={() => model.onSelect(job)} aria-pressed={model.selectedJob?.id === job.id}>{stringValue(job.kind)} · {stringValue(job.status)}</button></li>)}
    </ul>
  </section>;
}
