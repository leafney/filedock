import axios from "axios";
import type { TFunction } from "i18next";

import type { ApiResponse } from "../types/api";

export function getApiErrorCode(error: unknown): number | undefined {
  if (!axios.isAxiosError<ApiResponse<unknown>>(error)) return undefined;
  const code = error.response?.data?.code;
  return typeof code === "number" ? code : undefined;
}

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
