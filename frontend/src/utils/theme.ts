export type ThemeMode = "system" | "dark" | "light";
export type ResolvedTheme = "dark" | "light";

export const themeStorageKey = "filedock.theme";

export function normalizeThemeMode(value: string | null | undefined): ThemeMode | null {
  if (value === "system" || value === "dark" || value === "light") return value;
  return null;
}

export function resolveTheme(mode: ThemeMode, systemPrefersDark: boolean): ResolvedTheme {
  if (mode === "dark") return "dark";
  if (mode === "light") return "light";
  return systemPrefersDark ? "dark" : "light";
}

export function readStoredThemeMode(storage?: Pick<Storage, "getItem">): ThemeMode {
  try {
    return normalizeThemeMode(storage?.getItem(themeStorageKey)) ?? "system";
  } catch {
    return "system";
  }
}

export function writeStoredThemeMode(storage: Pick<Storage, "setItem"> | undefined, mode: ThemeMode): void {
  try {
    storage?.setItem(themeStorageKey, mode);
  } catch {
    // 页面仍可使用当前主题；存储不可用时不持久化。
  }
}

export function readThemeModeFromStorageEvent(event: StorageEvent): ThemeMode | null {
  if (event.key !== themeStorageKey) return null;
  return normalizeThemeMode(event.newValue) ?? "system";
}
