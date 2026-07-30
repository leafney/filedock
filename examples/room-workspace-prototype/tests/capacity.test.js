import { describe, expect, test } from "bun:test";
import { calculateCapacity, canRestoreFile } from "../js/capacity.js";

const MB = 1024 * 1024;
const file = (id, scope, status, sizeMB) => ({ id, scope, status, sizeBytes: sizeMB * MB });
const files = [
  file("shared", "shared", "available", 100),
  file("direct", "direct", "available", 60),
  file("upload", "shared", "uploading", 40),
  file("recycled", "shared", "recycled", 80),
];

const calculate = (countTowardRoomCapacity, freeMB = 100) => calculateCapacity({
  files,
  roomCapacityBytes: 500 * MB,
  recycleConfig: { countTowardRoomCapacity, freeBytes: freeMB * MB },
});

describe("房间容量", () => {
  test("公共、定向与上传预留分别统计", () => {
    const result = calculate(false);
    expect(result.sharedBytes).toBe(100 * MB);
    expect(result.directBytes).toBe(60 * MB);
    expect(result.reservedBytes).toBe(40 * MB);
    expect(result.occupiedBytes).toBe(200 * MB);
  });

  test("回收站计入时忽略免费额度", () => {
    const result = calculate(true, 999);
    expect(result.freeRecycleBytes).toBe(0);
    expect(result.recycledChargedBytes).toBe(80 * MB);
    expect(result.occupiedBytes).toBe(280 * MB);
  });

  test("回收站未超免费额度时不计费", () => {
    expect(calculate(false).recycledChargedBytes).toBe(0);
  });

  test("回收站超额时只统计超出量", () => {
    const result = calculate(false, 30);
    expect(result.recycledChargedBytes).toBe(50 * MB);
    expect(result.occupiedBytes).toBe(250 * MB);
  });

  test("历史再次发送不会增加物理占用", () => {
    const resent = files.map((item) => item.id === "direct" ? { ...item, recipientIds: ["a", "b", "c"] } : item);
    const after = calculateCapacity({ files: resent, roomCapacityBytes: 500 * MB, recycleConfig: { countTowardRoomCapacity: false, freeBytes: 100 * MB } });
    expect(after.occupiedBytes).toBe(calculate(false).occupiedBytes);
  });
});

describe("恢复容量检查", () => {
  test("容量充足允许恢复", () => {
    expect(canRestoreFile({ fileId: "recycled", files, roomCapacityBytes: 300 * MB, recycleConfig: { countTowardRoomCapacity: false, freeBytes: 100 * MB } }).allowed).toBeTrue();
  });

  test("容量不足阻止恢复", () => {
    const result = canRestoreFile({ fileId: "recycled", files, roomCapacityBytes: 250 * MB, recycleConfig: { countTowardRoomCapacity: true, freeBytes: 0 } });
    expect(result.allowed).toBeFalse();
    expect(result.reason).toBe("capacity_exceeded");
  });

  test("非回收站文件不可恢复", () => {
    expect(canRestoreFile({ fileId: "shared", files, roomCapacityBytes: 500 * MB, recycleConfig: { countTowardRoomCapacity: false, freeBytes: 0 } }).reason).toBe("not_recycled");
  });
});
