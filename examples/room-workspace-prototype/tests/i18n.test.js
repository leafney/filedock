import { describe, expect, test } from "bun:test";
import { resources, translate } from "../js/i18n.js";

describe("原型双语词典", () => {
  test("中英文翻译键完全一致", () => {
    expect(Object.keys(resources.en).sort()).toEqual(Object.keys(resources["zh-CN"]).sort());
  });

  test("插值参数在两种语言中生效", () => {
    expect(translate("zh-CN", "privateFile", { alias: "7K2M-A9Q4" })).toBe("私密文件#7K2M-A9Q4");
    expect(translate("en", "privateFile", { alias: "7K2M-A9Q4" })).toBe("Private file #7K2M-A9Q4");
  });
});
