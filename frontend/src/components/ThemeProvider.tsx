import { ConfigProvider, theme as antdTheme } from "antd";
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

import {
  detectSystemPrefersDark,
  readStoredThemeMode,
  readThemeModeFromStorageEvent,
  resolveTheme,
  themeStorageKey,
  writeStoredThemeMode,
  type ResolvedTheme,
  type ThemeMode,
} from "../utils/theme";

interface ThemeContextValue {
  mode: ThemeMode;
  resolvedTheme: ResolvedTheme;
  setMode: (mode: ThemeMode) => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

function getInitialMode(): ThemeMode {
  return readStoredThemeMode(typeof window === "undefined" ? undefined : window.localStorage);
}

function subscribeToSystemTheme(onChange: () => void): () => void {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return () => undefined;
  const media = window.matchMedia("(prefers-color-scheme: dark)");
  if (typeof media.addEventListener === "function") {
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }
  media.addListener(onChange);
  return () => media.removeListener(onChange);
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [mode, setModeState] = useState<ThemeMode>(getInitialMode);
  const [systemPrefersDark, setSystemPrefersDark] = useState(detectSystemPrefersDark);
  const resolvedTheme = resolveTheme(mode, systemPrefersDark);

  const setMode = useCallback((nextMode: ThemeMode) => {
    setModeState(nextMode);
    writeStoredThemeMode(typeof window === "undefined" ? undefined : window.localStorage, nextMode);
  }, []);

  useEffect(() => subscribeToSystemTheme(() => setSystemPrefersDark(detectSystemPrefersDark())), []);

  useEffect(() => {
    const onStorage = (event: StorageEvent) => {
      const nextMode = readThemeModeFromStorageEvent(event);
      if (nextMode) setModeState(nextMode);
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);

  useEffect(() => {
    const root = document.documentElement;
    root.dataset.theme = resolvedTheme;
    root.style.colorScheme = resolvedTheme;
    const metaThemeColor = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]');
    metaThemeColor?.setAttribute("content", resolvedTheme === "dark" ? "#0f172a" : "#f8fafc");
    delete root.dataset.themeInitializing;
  }, [resolvedTheme]);

  const value = useMemo(() => ({ mode, resolvedTheme, setMode }), [mode, resolvedTheme, setMode]);
  return (
    <ThemeContext.Provider value={value}>
      <ConfigProvider
        theme={{
          algorithm: resolvedTheme === "dark" ? antdTheme.darkAlgorithm : undefined,
          token: { colorPrimary: "#0f766e", borderRadius: 10, colorLink: "#0f766e" },
        }}
      >
        {children}
      </ConfigProvider>
    </ThemeContext.Provider>
  );
}

export function useTheme(): ThemeContextValue {
  const value = useContext(ThemeContext);
  if (!value) throw new Error("useTheme must be used within ThemeProvider");
  return value;
}

export { themeStorageKey };
