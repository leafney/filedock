import axios from "axios";

import { currentLanguage } from "../i18n";

export const apiClient = axios.create({
  baseURL: "/",
  timeout: 10_000,
});

apiClient.interceptors.request.use((config) => {
  config.headers.set("Accept-Language", currentLanguage());
  return config;
});
