import { ProductApiClient } from "@nh-media/sdk";

export const defaultEndpoint = process.env.NEXT_PUBLIC_NH_MEDIA_API_URL || "http://127.0.0.1:8080";

export function createProductApiClient(): ProductApiClient {
  return new ProductApiClient({ baseUrl: defaultEndpoint });
}
