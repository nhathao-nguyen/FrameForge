import type { ReviewViewModel } from "../../../types/workspace";
import { stringValue } from "../../../services/value-utils";

export function JobStepReview({ model }: { model: ReviewViewModel }) {
  return <section className="compact-form" aria-labelledby="review-actions-title">
    <h3 id="review-actions-title">Generic JobStep review</h3>
    <button type="button" onClick={() => void model.onLoad()} disabled={!model.selectedJob}>Load reviewable steps</button>
    <label htmlFor="review-step-id">JobStep</label>
    <select id="review-step-id" value={model.reviewStepId} onChange={(event) => model.onStepChange(event.target.value)}>
      <option value="">Select JobStep</option>
      {model.steps.map((step) => <option key={stringValue(step.id)} value={stringValue(step.id)}>{stringValue(step.node_key)} · {stringValue(step.status)}</option>)}
    </select>
    <label htmlFor="review-resource-type">Exact resource type</label>
    <input id="review-resource-type" value={model.reviewResourceType} onChange={(event) => model.onResourceTypeChange(event.target.value)} />
    <label htmlFor="review-resource-id">Exact resource ID</label>
    <input id="review-resource-id" value={model.reviewResourceId} onChange={(event) => model.onResourceIdChange(event.target.value)} />
    <label htmlFor="review-resource-revision">Exact resource revision</label>
    <input id="review-resource-revision" value={model.reviewResourceRevision} onChange={(event) => model.onResourceRevisionChange(event.target.value)} inputMode="numeric" />
    <div className="inline-form">
      <button type="button" onClick={() => void model.onApprove()} disabled={!model.selectedJob || !model.reviewStepId || !model.reviewResourceId}>Approve exact resource</button>
      <button type="button" onClick={() => void model.onReject()} disabled={!model.selectedJob || !model.reviewStepId}>Reject / edit then resume</button>
      <button type="button" onClick={() => void model.onResume()} disabled={!model.selectedJob}>Resume descendants</button>
    </div>
  </section>;
}
