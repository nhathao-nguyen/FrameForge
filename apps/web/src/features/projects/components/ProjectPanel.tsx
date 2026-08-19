import type { FormEvent } from "react";
import type { ProjectViewModel } from "../../../types/workspace";
import { stringValue } from "../../../services/value-utils";

export function ProjectPanel({ model }: { model: ProjectViewModel }) {
  function create(event: FormEvent) {
    event.preventDefault();
    void model.onCreate();
  }

  return <section className="card" aria-labelledby="projects-title">
    <h2 id="projects-title">Projects</h2>
    <form onSubmit={create} className="inline-form">
      <label htmlFor="project-name" className="sr-only">Project name</label>
      <input id="project-name" value={model.projectName} onChange={(event) => model.onProjectNameChange(event.target.value)} placeholder="New project" />
      <button type="submit">Create</button>
    </form>
    <ul className="resource-list">
      {model.projects.map((project) => <li key={stringValue(project.id)}><button type="button" className="list-button" onClick={() => void model.onSelect(project)} aria-pressed={model.selectedProject?.id === project.id}>{stringValue(project.name)}</button></li>)}
    </ul>
    <div className="stack compact-form">
      <h3>Direct asset upload</h3>
      <input type="file" onChange={(event) => model.onUploadFileChange(event.target.files?.[0] || null)} disabled={!model.selectedProject} />
      <button type="button" onClick={() => void model.onUpload()} disabled={!model.selectedProject || !model.uploadFile}>Upload to approved storage</button>
      <p className="muted">Media bytes go to the signed storage URL; Product API receives metadata only.</p>
      <h3>Artifact download</h3>
      <input value={model.artifactId} onChange={(event) => model.onArtifactIdChange(event.target.value)} placeholder="Committed artifact UUID" disabled={!model.selectedProject} />
      <button type="button" onClick={() => void model.onDownload()} disabled={!model.selectedProject || !model.artifactId.trim()}>Issue download URL</button>
      {model.downloadUrl ? <a href={model.downloadUrl} target="_blank" rel="noreferrer">Open signed artifact download</a> : null}
    </div>
  </section>;
}
