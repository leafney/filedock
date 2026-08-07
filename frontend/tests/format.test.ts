import { describe, expect, test } from "bun:test";

import { formatDate } from "../src/utils/format";

describe("时间格式化", () => {
  test("统一显示为完整的年月日时分秒", () => {
    const timestamp = new Date(2026, 7, 7, 15, 48, 10).getTime() / 1000;
    expect(formatDate(timestamp)).toBe("2026-08-07 15:48:10");
  });

  test("无效时间显示占位符", () => {
    expect(formatDate(Number.NaN)).toBe("-");
  });
});
