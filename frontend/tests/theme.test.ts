import { describe, expect, test } from "bun:test";

import { normalizeThemeMode, readStoredThemeMode, readThemeModeFromStorageEvent, resolveTheme, themeStorageKey, writeStoredThemeMode } from "../src/utils/theme";

describe("主题纯逻辑", () => {
  test("只接受三种主题模式", () => {
    expect(normalizeThemeMode("system")).toBe("system");
    expect(normalizeThemeMode("dark")).toBe("dark");
    expect(normalizeThemeMode("light")).toBe("light");
    expect(normalizeThemeMode("auto")).toBeNull();
    expect(normalizeThemeMode(null)).toBeNull();
  });

  test("跟随系统解析实际主题，手动模式不受系统影响", () => {
    expect(resolveTheme("system", true)).toBe("dark");
    expect(resolveTheme("system", false)).toBe("light");
    expect(resolveTheme("dark", false)).toBe("dark");
    expect(resolveTheme("light", true)).toBe("light");
  });

  test("本地存储缺失或异常时回退跟随系统", () => {
    expect(readStoredThemeMode()).toBe("system");
    expect(readStoredThemeMode({ getItem: () => "invalid" })).toBe("system");
    expect(readStoredThemeMode({ getItem: () => { throw new Error("blocked"); } })).toBe("system");
    expect(readStoredThemeMode({ getItem: () => "dark" })).toBe("dark");
  });

  test("主题模式可以写入固定本地存储键", () => {
    let key = "";
    let value = "";
    writeStoredThemeMode({ setItem: (nextKey, nextValue) => { key = nextKey; value = nextValue; } }, "dark");
    expect(key).toBe(themeStorageKey);
    expect(value).toBe("dark");
    expect(() => writeStoredThemeMode({ setItem: () => { throw new Error("blocked"); } }, "light")).not.toThrow();
  });

  test("跨标签事件只处理主题存储键，非法值回退系统", () => {
    expect(readThemeModeFromStorageEvent({ key: "other", newValue: "dark" } as StorageEvent)).toBeNull();
    expect(readThemeModeFromStorageEvent({ key: themeStorageKey, newValue: "light" } as StorageEvent)).toBe("light");
    expect(readThemeModeFromStorageEvent({ key: themeStorageKey, newValue: "invalid" } as StorageEvent)).toBe("system");
  });
});
