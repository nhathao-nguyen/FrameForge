"use client";

import { LoginPanel } from "../features/auth/components/LoginPanel";
import { WorkspaceDashboard } from "../features/workspace/components/WorkspaceDashboard";
import { useWorkspaceController } from "../hooks/use-workspace-controller";

export default function HomePage() {
  const model = useWorkspaceController();
  return model.loggedIn ? <WorkspaceDashboard model={model} /> : <LoginPanel model={model.auth} />;
}
