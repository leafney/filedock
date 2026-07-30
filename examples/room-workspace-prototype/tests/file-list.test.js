import { describe, expect, test } from "bun:test";
import { deriveBatchCapabilities, deriveFileActions, deriveFileGroups, getDefaultComposerMode } from "../js/file-list.js";
import { createMockState } from "../js/mock-data.js";
import { projectFilesForUser } from "../js/permissions.js";
import { reduceState } from "../js/state.js";

const state = createMockState();
const user = (id) => state.users.find((item) => item.id === id);
const projectedFor = (id) => projectFilesForUser(state.files, user(id), state.room).filter((file) => file.status !== "recycled");

describe("文件范围分组", () => {
  test("房主投影后按共享与私密分组并保持固定顺序", () => {
    const result = deriveFileGroups({ files: projectedFor("user-owner"), userId: "user-owner" });
    expect(result.orderedGroups.map((group) => group.scope)).toEqual(["shared", "direct"]);
    expect(result.shared.every((file) => file.scope === "shared")).toBeTrue();
    expect(result.direct.every((file) => file.scope === "direct")).toBeTrue();
  });

  test("普通成员无关私密文件不进入数量", () => {
    const files = projectedFor("user-su");
    const result = deriveFileGroups({ files, userId: "user-su" });
    expect(result.direct).toHaveLength(0);
    expect(result.totalCount).toBe(files.length);
  });

  test("房主匿名文件进入私密分组且不泄露原名", () => {
    const result = deriveFileGroups({ files: projectedFor("user-owner"), userId: "user-owner" });
    const anonymous = result.direct.find((file) => file.id === "file-direct-audit");
    expect(anonymous.visibility).toBe("anonymous");
    expect(anonymous.name).toBeNull();
  });

  test("排序只改变组内顺序", () => {
    const result = deriveFileGroups({ files: projectedFor("user-owner"), sort: "oldest", userId: "user-owner" });
    expect(result.orderedGroups.map((group) => group.scope)).toEqual(["shared", "direct"]);
    expect(result.shared.map((file) => file.id)).toEqual(["file-shared-guide", "file-shared-design", "file-shared-video"]);
  });

  test("匿名文件隐藏原名不能被搜索命中", () => {
    const files = projectedFor("user-owner");
    expect(deriveFileGroups({ files, search: "候选人资料", userId: "user-owner" }).direct).toHaveLength(0);
    expect(deriveFileGroups({ files, search: "2PX8", userId: "user-owner" }).direct.map((file) => file.id)).toEqual(["file-direct-audit"]);
  });

  test("范围上下文决定待发送默认范围", () => {
    expect(getDefaultComposerMode("direct")).toBe("direct");
    expect(getDefaultComposerMode("shared")).toBe("shared");
    expect(getDefaultComposerMode("all")).toBe("shared");
  });
});

describe("单文件操作派生", () => {
  test("待接收文件只有接收和拒绝且没有菜单", () => {
    const file = projectedFor("user-owner").find((item) => item.id === "file-direct-owner-copy");
    const actions = deriveFileActions(file, user("user-owner"), state.room);
    expect(actions.primaryActions.map((item) => item.id)).toEqual(["accept-download", "decline"]);
    expect(actions.menuActions).toEqual([]);
  });

  test("普通文件显示下载和非空菜单", () => {
    const file = projectedFor("user-owner").find((item) => item.id === "file-shared-design");
    const actions = deriveFileActions(file, user("user-owner"), state.room);
    expect(actions.primaryActions.map((item) => item.id)).toEqual(["download"]);
    expect(actions.menuActions.some((item) => item.id === "view-details")).toBeTrue();
  });

  test("匿名文件不可下载但有安全菜单", () => {
    const file = projectedFor("user-owner").find((item) => item.id === "file-direct-audit");
    const actions = deriveFileActions(file, user("user-owner"), state.room);
    expect(actions.primaryActions).toEqual([]);
    expect(actions.menuActions[0].id).toBe("view-anonymous-details");
  });
});

describe("批量共同权限", () => {
  test("单个可复用私密文件允许私密发送", () => {
    const file = projectedFor("user-owner").find((item) => item.id === "file-direct-budget");
    expect(deriveBatchCapabilities([file], user("user-owner"), state.room).privateSend.enabled).toBeTrue();
  });

  test("混合共享与私密选择禁用私密发送", () => {
    const files = projectedFor("user-owner").filter((item) => ["file-shared-design", "file-direct-budget"].includes(item.id));
    expect(deriveBatchCapabilities(files, user("user-owner"), state.room).privateSend).toEqual({ enabled: false, reason: "contains_non_reusable_private_file" });
  });

  test("任一文件不可下载时禁用批量下载", () => {
    const files = projectedFor("user-owner").filter((item) => ["file-shared-design", "file-direct-audit"].includes(item.id));
    expect(deriveBatchCapabilities(files, user("user-owner"), state.room).download).toEqual({ enabled: false, reason: "contains_unavailable_file" });
  });

  test("未选择文件时两个操作均说明原因", () => {
    expect(deriveBatchCapabilities([], user("user-owner"), state.room)).toEqual({
      download: { enabled: false, reason: "no_selection" },
      privateSend: { enabled: false, reason: "no_selection" },
    });
  });
});

describe("拒绝私密文件", () => {
  test("确认拒绝只更新当前接收者并追加一次事件", () => {
    const current = reduceState(state, {
      type: "files/decline",
      fileId: "file-direct-owner-copy",
      actorId: "user-owner",
      eventId: "event-decline-test",
      occurredAt: "2026-07-31T15:00:00+08:00",
    });
    const file = current.files.find((item) => item.id === "file-direct-owner-copy");
    expect(file.receiverStates).toEqual({ "user-owner": "declined" });
    expect(current.events.filter((event) => event.id === "event-decline-test")).toHaveLength(1);
  });
});

describe("历史私密文件多选复用", () => {
  test("一次发送多个历史文件且不创建新文件", () => {
    const beforeCount = state.files.length;
    const current = reduceState(state, {
      type: "files/send-existing-many",
      fileIds: ["file-direct-budget"],
      recipientIds: ["user-su"],
      actorId: "user-owner",
      eventId: "event-multi-send",
      occurredAt: "2026-07-31T15:10:00+08:00",
    });
    expect(current.files).toHaveLength(beforeCount);
    expect(current.files.find((file) => file.id === "file-direct-budget").recipientIds).toContain("user-su");
    expect(current.events.some((event) => event.fileId === "file-direct-budget" && event.type === "resent")).toBeTrue();
  });

  test("退出批量模式会清空选择", () => {
    const batch = reduceState({ ...state, ui: { ...state.ui, batchMode: true, selectedFileIds: ["file-shared-design"] } }, { type: "ui/exit-batch-mode" });
    expect(batch.ui.batchMode).toBeFalse();
    expect(batch.ui.selectedFileIds).toEqual([]);
  });
});
