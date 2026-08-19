interface WorkspaceHeaderProps { onSignOut: () => Promise<void>; }

export function WorkspaceHeader({ onSignOut }: WorkspaceHeaderProps) {
  return <header className="topbar">
    <div><p className="eyebrow">NH-Media · Local/LAN client</p><h1>Workspace dashboard</h1></div>
    <button type="button" onClick={() => void onSignOut()}>Sign out</button>
  </header>;
}
