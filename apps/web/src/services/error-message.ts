import { ProductApiError } from "@nh-media/sdk";

export function safeMessage(value: unknown): string {
  if (value instanceof ProductApiError) return value.message;
  if (value instanceof Error) return value.message;
  return "The request could not be completed.";
}
