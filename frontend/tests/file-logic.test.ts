import { describe, expect, test } from "bun:test";

import en from "../src/i18n/locales/en";
import zhCN from "../src/i18n/locales/zh-CN";
import type { FileTrashItem, RoomFile } from "../src/types/domain";
import { batchFileCapabilities, fileMenuActions, fileTrashActions, primaryFileAction } from "../src/utils/file-permissions";
import { downloadStatusForProgress, selectQueuedStarts, transferProgress } from "../src/utils/transfer-queue";

function roomFile(overrides: Partial<RoomFile> = {}): RoomFile {
  return {
    projection: "full",
    fileId: "file-1",
    displayName: "report.pdf",
    scope: "shared",
    size: 1024,
    status: "available",
    progress: 100,
    uploaderUserId: "user-1",
    uploaderName: "member",
    createdAt: 1,
    capabilities: { canDownload: true, canAccept: false, canDecline: false, canReuse: false, canPublishShared: false, canTrash: false, canSetTrashReason: false },
    ...overrides,
  };
}

describe("文件操作权限矩阵", () => {
  test("待接收私密文件优先显示接收动作", () => {
    const file = roomFile({ scope: "direct", capabilities: { canDownload: false, canAccept: true, canDecline: true, canReuse: false, canPublishShared: false, canTrash: false, canSetTrashReason: false } });
    expect(primaryFileAction(file)).toBe("accept");
  });

  test("匿名投影只提供安全详情动作", () => {
    const file = roomFile({ projection: "anonymous", scope: "direct", displayName: "7K2M-A9Q4", capabilities: { canDownload: false, canAccept: false, canDecline: false, canReuse: false, canPublishShared: false, canTrash: false, canSetTrashReason: false } });
    expect(primaryFileAction(file)).toBe("none");
    expect(fileMenuActions(file)).toEqual(["details"]);
  });

  test("批量能力要求全部文件同时具备权限", () => {
    const downloadable = roomFile();
    const privateOwned = roomFile({ fileId: "file-2", scope: "direct", capabilities: { canDownload: true, canAccept: false, canDecline: false, canReuse: true, canPublishShared: true, canTrash: true, canSetTrashReason: false } });
    expect(batchFileCapabilities([downloadable, privateOwned])).toEqual({ canDownload: true, canReuse: false });
    expect(batchFileCapabilities([privateOwned])).toEqual({ canDownload: true, canReuse: true });
    expect(batchFileCapabilities([])).toEqual({ canDownload: false, canReuse: false });
  });

  test("移入回收站始终排在安全信息操作之后", () => {
    const file = roomFile({ capabilities: { canDownload: true, canAccept: false, canDecline: false, canReuse: true, canPublishShared: false, canTrash: true, canSetTrashReason: true } });
    expect(fileMenuActions(file)).toEqual(["reuse", "details", "trash"]);
  });

  test("回收站能力映射操作并阻止重复申请", () => {
    const item = (restoreRequest?: FileTrashItem["restoreRequest"]): FileTrashItem => ({
      file: roomFile({ status: "trashed" }), deletedByUserId: "owner", deletedByName: "owner", deletedAt: 2,
      capabilities: { canRestore: false, canRequestRestore: true, canPurge: false },
      restoreRequest,
    });
    expect(fileTrashActions(item())).toEqual(["request_restore"]);
    expect(fileTrashActions({ ...item(), restoreRequest: { requestId: "one", status: "pending", requesterUserId: "user-1", requesterName: "member", createdAt: 3 } })).toEqual([]);
    expect(fileTrashActions({ ...item(), restoreRequest: { requestId: "two", status: "rejected", requesterUserId: "user-1", requesterName: "member", createdAt: 3 } })).toEqual([]);
    expect(fileTrashActions({ ...item(), capabilities: { canRestore: true, canRequestRestore: false, canPurge: true } })).toEqual(["restore", "purge"]);
  });
});

describe("传输队列", () => {
  test("最多补足两个活动上传且跳过终态任务", () => {
    const items = [
      { id: "one", status: "queued" as const },
      { id: "two", status: "failed" as const },
      { id: "three", status: "queued" as const },
      { id: "four", status: "cancelled" as const },
    ];
    expect(selectQueuedStarts(items, new Set(), 2)).toEqual(["one", "three"]);
    expect(selectQueuedStarts(items, new Set(["active"]), 2)).toEqual(["one"]);
    expect(selectQueuedStarts(items, new Set(["a", "b"]), 2)).toEqual([]);
  });

  test("进度被限制在零到一百并驱动下载状态", () => {
    expect(transferProgress(-10, 100)).toBe(0);
    expect(transferProgress(55, 100)).toBe(55);
    expect(transferProgress(150, 100)).toBe(100);
    expect(downloadStatusForProgress(0)).toBe("starting");
    expect(downloadStatusForProgress(55)).toBe("downloading");
    expect(downloadStatusForProgress(100)).toBe("completed");
  });
});

describe("双语词典", () => {
  test("简体中文和英文键完全一致且没有空文案", () => {
    const chinese = flatten(zhCN);
    const english = flatten(en);
    expect([...chinese.keys()].sort()).toEqual([...english.keys()].sort());
    expect([...chinese.values()].every((value) => value.trim().length > 0)).toBe(true);
    expect([...english.values()].every((value) => value.trim().length > 0)).toBe(true);
  });
});

function flatten(value: object, prefix = "", output = new Map<string, string>()) {
  for (const [key, child] of Object.entries(value)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (typeof child === "string") output.set(path, child);
    else if (child && typeof child === "object") flatten(child, path, output);
  }
  return output;
}
