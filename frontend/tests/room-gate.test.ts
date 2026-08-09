import { describe, expect, test } from "bun:test";

import {
  classifyRoomGateError,
  createPinSubmissionGate,
  deriveJoinGateView,
  normalizePin,
} from "../src/utils/room-gate";

describe("房间过渡状态", () => {
  test("加入方式和申请状态映射到稳定视图", () => {
    expect(deriveJoinGateView("open", false)).toBe("open");
    expect(deriveJoinGateView("password", false)).toBe("pin");
    expect(deriveJoinGateView("owner_approval", false)).toBe("approval_request");
    expect(deriveJoinGateView("owner_approval", true)).toBe("approval_waiting");
  });

  test("永久房间错误与临时错误分开处理", () => {
    expect(classifyRoomGateError(40_004)).toBe("unavailable");
    expect(classifyRoomGateError(40_403)).toBe("unavailable");
    expect(classifyRoomGateError(40_908)).toBe("unavailable");
    expect(classifyRoomGateError()).toBe("transient");
    expect(classifyRoomGateError(50_000)).toBe("transient");
  });
});

describe("PIN 过渡交互", () => {
  test("粘贴内容只保留前四位 ASCII 数字", () => {
    expect(normalizePin(" 12-34 ")).toBe("1234");
    expect(normalizePin("12a3456")).toBe("1234");
    expect(normalizePin("１２34")).toBe("34");
    expect(normalizePin("no pin")).toBe("");
  });

  test("自动提交和 Enter 共用提交闸门且请求中去重", () => {
    const gate = createPinSubmissionGate();
    expect(gate.tryStart("123")).toBeFalse();
    expect(gate.tryStart("1234")).toBeTrue();
    expect(gate.tryStart("1234")).toBeFalse();
    gate.fail();
    expect(gate.tryStart("1234")).toBeTrue();
  });

  test("成功后阻止再次提交，重置后可用于新页面", () => {
    const gate = createPinSubmissionGate();
    expect(gate.tryStart("5678")).toBeTrue();
    gate.succeed();
    expect(gate.tryStart("5678")).toBeFalse();
    gate.reset();
    expect(gate.tryStart("5678")).toBeTrue();
  });
});
