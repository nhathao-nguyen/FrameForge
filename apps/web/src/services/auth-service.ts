import type { ProductApiClient } from "@nh-media/sdk";

export async function authenticate(api: ProductApiClient, endpoint: string, username: string, password: string): Promise<void> {
  api.setBaseUrl(endpoint);
  await api.login(username, password);
}

export async function revokeSession(api: ProductApiClient): Promise<void> {
  await api.logout();
}
