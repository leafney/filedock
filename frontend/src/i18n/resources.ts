const zhCN = {
  brand: {
    name: "FileDock",
    displayName: "码头",
    description: "浏览器即开即用的文件传输基础服务。",
  },
  language: {
    label: "选择语言",
    zhCN: "简体中文",
    en: "English",
  },
  home: {
    loading: "正在检查服务状态…",
    unavailable: "服务暂不可用，请确认后端已启动后刷新页面。",
    fields: {
      status: "状态",
      version: "版本",
      branch: "分支",
      commit: "提交",
      buildTime: "构建时间",
    },
  },
  error: {
    timeout: "请求超时，请稍后重试。",
    network: "网络连接异常，请检查网络后重试。",
    unavailable: "服务暂不可用，请稍后重试。",
  },
} as const;

type TranslationShape<T> = {
  [K in keyof T]: T[K] extends string ? string : TranslationShape<T[K]>;
};

const en = {
  brand: {
    name: "FileDock",
    displayName: "FileDock",
    description: "A browser-ready foundation for transferring files.",
  },
  language: {
    label: "Select language",
    zhCN: "简体中文",
    en: "English",
  },
  home: {
    loading: "Checking service status…",
    unavailable: "The service is unavailable. Make sure the backend is running, then refresh the page.",
    fields: {
      status: "Status",
      version: "Version",
      branch: "Branch",
      commit: "Commit",
      buildTime: "Build time",
    },
  },
  error: {
    timeout: "The request timed out. Please try again later.",
    network: "A network error occurred. Check your connection and try again.",
    unavailable: "The service is unavailable. Please try again later.",
  },
} as const satisfies TranslationShape<typeof zhCN>;

export const resources = {
  "zh-CN": { translation: zhCN },
  en: { translation: en },
} as const;

export type SupportedLanguage = keyof typeof resources;
