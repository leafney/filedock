export const resources = {
  "zh-CN": {
    privateFile: "私密文件#{{alias}}",
    shared: "公共",
    direct: "定向",
    privateAudit: "私密审计",
    ready: "可下载",
    uploading: "上传中",
    transferComplete: "传输完成",
    download: "下载",
    acceptDownload: "接受并下载",
    resend: "再次发送",
    recycle: "移入回收站",
    viewTask: "查看任务",
    filesCount: "共 {{count}} 个文件",
  },
  en: {
    privateFile: "Private file #{{alias}}",
    shared: "Shared",
    direct: "Direct",
    privateAudit: "Private audit",
    ready: "Available",
    uploading: "Uploading",
    transferComplete: "Transfer complete",
    download: "Download",
    acceptDownload: "Accept & download",
    resend: "Send again",
    recycle: "Move to Recycle Bin",
    viewTask: "View task",
    filesCount: "{{count}} files",
  },
};

export function translate(locale, key, params = {}) {
  const dictionary = resources[locale] ?? resources["zh-CN"];
  const fallback = resources["zh-CN"];
  const template = dictionary[key] ?? fallback[key] ?? key;
  return Object.entries(params).reduce(
    (result, [name, value]) => result.replaceAll(`{{${name}}}`, String(value)),
    template,
  );
}
