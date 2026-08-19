import { WorkspaceHeader } from "../../../components/layout/WorkspaceHeader";
import { RequestFeedback } from "../../../components/feedback/RequestFeedback";
import type { WorkspaceControllerModel } from "../../../types/workspace";
import { ProjectPanel } from "../../projects/components/ProjectPanel";
import { JobsPanel } from "../../jobs/components/JobsPanel";
import { ScriptEditor } from "../../script/components/ScriptEditor";
import { TimelineEditor } from "../../timeline/components/TimelineEditor";
import { JobStepReview } from "../../review/components/JobStepReview";
import { SceneList } from "../../scenes/components/SceneList";

export function WorkspaceDashboard({ model }: { model: WorkspaceControllerModel }) {
  return <main className="shell">
    <WorkspaceHeader onSignOut={model.onSignOut} />
    <RequestFeedback status={model.status} error={model.error} />
    <div className="grid">
      <ProjectPanel model={model.projects} />
      <JobsPanel model={model.jobs} />
      <section className="card" aria-labelledby="review-title">
        <h2 id="review-title">Review & timeline</h2>
        <p>Script and TimelineVersion edits are submitted as immutable server commands. Local drafts are not authoritative.</p>
        <ScriptEditor model={model.script} />
        <TimelineEditor model={model.timeline} />
        <JobStepReview model={model.review} />
        <SceneList model={model.scenes} />
        {model.jobs.selectedJob ? <pre aria-label="Selected job state">{JSON.stringify(model.jobs.selectedJob, null, 2)}</pre> : <p>Select a Job to watch progress and reconnect.</p>}
      </section>
    </div>
  </main>;
}
