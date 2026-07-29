import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import { resources, type SupportedLanguage } from "./resources";

export const defaultLanguage: SupportedLanguage = "zh-CN";
export const languageStorageKey = "filedock.language";

export function normalizeLanguage(value: string | null | undefined): SupportedLanguage | null {
  if (!value) {
    return null;
  }
  const tag = value.trim().replaceAll("_", "-").toLowerCase();
  if (
    tag === "zh" ||
    tag === "zh-cn" ||
    tag.startsWith("zh-cn-") ||
    tag === "zh-sg" ||
    tag.startsWith("zh-sg-") ||
    tag === "zh-hans" ||
    tag.startsWith("zh-hans-")
  ) {
    return "zh-CN";
  }
  if (tag === "en" || tag.startsWith("en-")) {
    return "en";
  }
  return null;
}

function readStoredLanguage(): SupportedLanguage | null {
  try {
    return normalizeLanguage(window.localStorage.getItem(languageStorageKey));
  } catch {
    return null;
  }
}

function detectBrowserLanguage(): SupportedLanguage | null {
  const candidates = navigator.languages?.length ? navigator.languages : [navigator.language];
  for (const candidate of candidates) {
    const language = normalizeLanguage(candidate);
    if (language) {
      return language;
    }
  }
  return null;
}

export function detectInitialLanguage(): SupportedLanguage {
  return readStoredLanguage() ?? detectBrowserLanguage() ?? defaultLanguage;
}

const initialLanguage = detectInitialLanguage();

void i18n.use(initReactI18next).init({
  resources,
  lng: initialLanguage,
  fallbackLng: defaultLanguage,
  supportedLngs: ["zh-CN", "en"],
  load: "currentOnly",
  showSupportNotice: false,
  interpolation: { escapeValue: false },
  initImmediate: false,
});

document.documentElement.lang = initialLanguage;

i18n.on("languageChanged", (value) => {
  const language = normalizeLanguage(value) ?? defaultLanguage;
  document.documentElement.lang = language;
  try {
    window.localStorage.setItem(languageStorageKey, language);
  } catch {
    // 页面仍可使用当前语言；存储不可用时不持久化。
  }
});

export function currentLanguage(): SupportedLanguage {
  return normalizeLanguage(i18n.resolvedLanguage ?? i18n.language) ?? defaultLanguage;
}

export default i18n;
