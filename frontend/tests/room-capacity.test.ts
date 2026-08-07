import { describe, expect, it } from "bun:test";

import { roomCapacitySummary } from "../src/utils/room-capacity";

describe("房间容量展示", () => {
  it("房主顶部容量包含未完成上传预留", () => {
    expect(roomCapacitySummary({ capacityBytes: 1000, usedBytes: 400, reservedBytes: 200 }, "owner")).toEqual({ occupiedBytes: 600, percent: 60 });
  });

  it("普通成员顶部容量不暴露上传预留", () => {
    expect(roomCapacitySummary({ capacityBytes: 1000, usedBytes: 400, reservedBytes: 200 }, "member")).toEqual({ occupiedBytes: 400, percent: 40 });
  });

  it("容量进度限制在有效显示范围", () => {
    expect(roomCapacitySummary({ capacityBytes: 100, usedBytes: 80, reservedBytes: 40 }, "owner")).toEqual({ occupiedBytes: 120, percent: 100 });
    expect(roomCapacitySummary({ capacityBytes: 0, usedBytes: 10 }, "owner").percent).toBe(0);
  });
});
