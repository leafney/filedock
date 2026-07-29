import { apiClient } from "../lib/api-client";
import type { ApiResponse } from "../types/api";
import type { VersionResponse } from "../types/version";

export async function getVersion(): Promise<VersionResponse> {
  const response = await apiClient.get<ApiResponse<VersionResponse>>("/version");
  return response.data.data;
}
