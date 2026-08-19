interface RequestFeedbackProps { status: string; error: string; }

export function RequestFeedback({ status, error }: RequestFeedbackProps) {
  return <>
    <p role="status">{status}</p>
    {error ? <p role="alert" className="error">{error}</p> : null}
  </>;
}
