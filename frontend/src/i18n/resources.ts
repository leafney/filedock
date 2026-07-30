import en from "./locales/en";
import zhCN from "./locales/zh-CN";

export const resources = {
  "zh-CN": { translation: zhCN },
  en: { translation: en },
} as const;

export type SupportedLanguage = keyof typeof resources;
