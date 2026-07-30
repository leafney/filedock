import { describe, expect, test } from "bun:test";
import { createMockState } from "../js/mock-data.js";
import { getFilePermissions, projectFileForUser, projectFilesForUser } from "../js/permissions.js";

const state = createMockState();
const owner = state.users.find((user) => user.id === "user-owner");
const lin = state.users.find((user) => user.id === "user-lin");
const su = state.users.find((user) => user.id === "user-su");
const byId = (id) => state.files.find((file) => file.id === id);

describe("文件可见性投影", () => {
  test("普通成员看到全部共享文件", () => {
    const projected = projectFilesForUser(state.files, su, state.room);
    expect(projected.filter((file) => file.scope === "shared")).toHaveLength(4);
  });

  test("定向参与者看到完整信息", () => {
    const projected = projectFileForUser(byId("file-direct-audit"), lin, state.room);
    expect(projected.visibility).toBe("full");
    expect(projected.name).toBe("候选人资料.pdf");
    expect(projected.receiverStates).toEqual({ "user-lin": "downloaded" });
  });

  test("普通成员看不到无关定向文件", () => {
    expect(projectFileForUser(byId("file-direct-audit"), su, state.room)).toBeNull();
  });

  test("房主看到无关定向文件匿名投影", () => {
    const projected = projectFileForUser(byId("file-direct-audit"), owner, state.room);
    expect(projected.visibility).toBe("anonymous");
    expect(projected.alias).toBe("2PX8-L6RD");
    expect(projected.sizeBytes).toBe(byId("file-direct-audit").sizeBytes);
  });

  test("匿名投影不泄露原名、类型与接收状态", () => {
    const projected = projectFileForUser(byId("file-direct-audit"), owner, state.room);
    expect(projected.name).toBeNull();
    expect(projected.mimeLabel).toBeNull();
    expect(projected.receiverStates).toBeNull();
    expect(projected).not.toHaveProperty("hash");
    expect(projected.permissions.canDownload).toBeFalse();
  });
});

describe("文件操作权限", () => {
  test("上传者可再次发送和发布定向文件", () => {
    const permissions = getFilePermissions(byId("file-direct-budget"), owner, state.room);
    expect(permissions.canResend).toBeTrue();
    expect(permissions.canPublishShared).toBeTrue();
  });

  test("房主强制回收后上传者只能申请恢复", () => {
    const zhou = state.users.find((user) => user.id === "user-zhou");
    const permissions = getFilePermissions(byId("file-recycled-direct"), zhou, state.room);
    expect(permissions.canRestore).toBeFalse();
    expect(permissions.canRequestRestore).toBeTrue();
  });

  test("上传者主动回收后可直接恢复", () => {
    const permissions = getFilePermissions(byId("file-recycled-shared"), owner, state.room);
    expect(permissions.canRestore).toBeTrue();
    expect(permissions.canRequestRestore).toBeFalse();
  });

  test("接收者拒绝私密文件后不能下载", () => {
    const file = { ...byId("file-direct-owner-copy"), receiverStates: { "user-owner": "declined" } };
    expect(getFilePermissions(file, owner, state.room).canDownload).toBeFalse();
  });
});
