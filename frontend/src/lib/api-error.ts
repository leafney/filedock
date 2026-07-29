import axios from "axios";
import type { TFunction } from "i18next";

import type { ApiResponse } from "../types/api";

export function getApiErrorMessage(error: unknown, t: TFunction): string {
  if (!axios.isAxiosError<ApiResponse<unknown>>(error)) {
    return t("error.unavailable");
  }
  const backendMessage = error.response?.data?.message;
  if (typeof backendMessage === "string" && backendMessage.trim() !== "") {
    return backendMessage;
  }
  if (error.code === "ECONNABORTED") {
    return t("error.timeout");
  }
  if (!error.response) {
    return t("error.network");
  }
  return t("error.unavailable");
}
