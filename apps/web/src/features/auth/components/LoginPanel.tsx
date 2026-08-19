import type { FormEvent } from "react";
import type { AuthViewModel } from "../../../types/workspace";

export function LoginPanel({ model }: { model: AuthViewModel }) {
  function submit(event: FormEvent) {
    event.preventDefault();
    void model.onSubmit();
  }

  return <main className="shell">
    <section className="card" aria-labelledby="login-title">
      <p className="eyebrow">NH-Media · Local/LAN client</p>
      <h1 id="login-title">Product workspace</h1>
      <p>Remote-first web shell. Product API remains the only source of truth.</p>
      <form onSubmit={submit} className="stack">
        <label htmlFor="endpoint">Product API endpoint</label>
        <input id="endpoint" value={model.endpoint} onChange={(event) => model.onEndpointChange(event.target.value)} inputMode="url" />
        <label htmlFor="username">Username</label>
        <input id="username" value={model.username} onChange={(event) => model.onUsernameChange(event.target.value)} autoComplete="username" required />
        <label htmlFor="password">Password</label>
        <input id="password" type="password" value={model.password} onChange={(event) => model.onPasswordChange(event.target.value)} autoComplete="current-password" required />
        <button type="submit">Sign in</button>
      </form>
      <p role="status">{model.status}</p>
      {model.error ? <p role="alert" className="error">{model.error}</p> : null}
    </section>
  </main>;
}
