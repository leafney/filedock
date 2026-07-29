import { apiClient } from "../lib/api-client";
import type { VersionResponse } from "../types/version";

export async function getVersion(): Promise<VersionResponse> {
  const response = await apiClient.get<VersionResponse>("/version");
  return response.data;
}
