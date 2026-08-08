import { describe, expect, test } from "bun:test";

import { getVersionDetails, getVersionLabel } from "../src/utils/version";

const baseVersion = {
  status: "ok",
  version: "",
  git_branch: "main",
  git_commit: "0123456789abcdef",
  build_time: "2026-08-08 10:20:30",
};

describe("Footer 版本信息", () => {
  test("版本号存在时优先显示版本号，同时保留短 Commit ID", () => {
    const details = getVersionDetails({ ...baseVersion, version: "v1.2.3" });
    expect(details.label).toBe("v1.2.3");
    expect(details.version).toBe("v1.2.3");
    expect(details.shortCommit).toBe("0123456");
    expect(details.commit).toBe("0123456789abcdef");
  });

  test("没有版本号时回退短 Commit ID", () => {
    expect(getVersionLabel(baseVersion)).toBe("0123456");
    expect(getVersionDetails(baseVersion).version).toBeUndefined();
  });

  test("版本号和 Commit ID 都不可用时回退 dev", () => {
    const details = getVersionDetails({ ...baseVersion, git_commit: "-", version: "-" });
    expect(details.label).toBe("dev");
    expect(details.shortCommit).toBeUndefined();
    expect(details.buildTime).toBe("2026-08-08 10:20:30");
  });
});
