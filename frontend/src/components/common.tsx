import { useQuery } from "@tanstack/react-query";
import { RefreshCw } from "lucide-react";
import { useEffect, useRef, type ReactNode } from "react";
import axios from "axios";
import { useTranslation } from "react-i18next";

import { normalizeLanguage } from "../i18n";
import { getApiErrorMessage } from "../lib/api-error";
import { getVersion } from "../services/version";

export function LanguageSelector() {
  const { t, i18n } = useTranslation();
  const language = normalizeLanguage(i18n.resolvedLanguage ?? i18n.language) ?? "zh-CN";
  return (
    <label className="fixed right-5 top-5 z-20">
      <span className="sr-only">{t("language.label")}</span>
      <select
        aria-label={t("language.label")}
        className="rounded-xl border border-slate-700 bg-slate-900/90 px-3 py-2 text-sm text-slate-200 shadow-lg outline-none backdrop-blur transition focus:border-cyan-500 focus:ring-2 focus:ring-cyan-500/30"
        value={language}
        onChange={(event) => void i18n.changeLanguage(normalizeLanguage(event.target.value) ?? "zh-CN")}
      >
        <option value="zh-CN">{t("language.zhCN")}</option>
        <option value="en">{t("language.en")}</option>
      </select>
    </label>
  );
}

export function PageFrame({ children }: { children: ReactNode }) {
  return (
    <main className="min-h-screen bg-[radial-gradient(circle_at_top,_#12304a_0,_#07111f_42%,_#020617_100%)] px-4 py-20 text-slate-100 sm:px-8">
      <LanguageSelector />
      <div className="mx-auto w-full max-w-6xl">{children}</div>
    </main>
  );
}

export function LoadingPage({ label }: { label: string }) {
  return (
    <PageFrame>
      <div className="flex min-h-[50vh] items-center justify-center text-slate-300">
        <RefreshCw className="mr-3 animate-spin" size={18} aria-hidden="true" />
        {label}
      </div>
    </PageFrame>
  );
}

export function ErrorNotice({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const { t } = useTranslation();
  return (
    <div className="flex items-center justify-between gap-4 rounded-2xl border border-rose-400/30 bg-rose-950/40 px-4 py-3 text-sm text-rose-200">
      <span>{getApiErrorMessage(error, t)}</span>
      {onRetry && (
        <button className="shrink-0 rounded-lg border border-rose-300/40 px-3 py-1.5 hover:bg-rose-400/10" onClick={onRetry} type="button">
          {t("home.retry")}
        </button>
      )}
    </div>
  );
}

export function isUnauthorized(error: unknown) {
  return axios.isAxiosError(error) && error.response?.status === 401;
}

export function StatusBadge({ status }: { status: string }) {
  const { t } = useTranslation();
  const label = status === "active" ? t("room.statusActive") : status === "destroying" ? t("room.statusDestroying") : t("room.statusDestroyed");
  return <span className="rounded-full bg-slate-800 px-2.5 py-1 text-xs text-slate-300">{label}</span>;
}

export function PinInput({ id, label, value, onChange, onComplete, disabled = false }: { id: string; label: string; value: string; onChange: (value: string) => void; onComplete?: (value: string) => void; disabled?: boolean }) {
  const refs = useRef<Array<HTMLInputElement | null>>([]);
  const completed = useRef(false);
  useEffect(() => {
    if (value.length === 4 && !completed.current) {
      completed.current = true;
      onComplete?.(value);
    }
    if (value.length < 4) completed.current = false;
  }, [onComplete, value]);

  return (
    <fieldset className="block text-sm text-slate-400">
      <legend>{label}</legend>
      <div className="mt-2 grid grid-cols-4 gap-2">
        {Array.from({ length: 4 }, (_, index) => (
          <input
            key={`${id}-${index}`}
            ref={(element) => { refs.current[index] = element; }}
            id={`${id}-${index}`}
            aria-label={`${label} ${index + 1}`}
            className="h-14 w-full rounded-xl border border-slate-700 bg-slate-950 text-center font-mono text-2xl outline-none focus:border-cyan-400 disabled:opacity-50"
            value={value[index] ?? ""}
            disabled={disabled}
            onChange={(event) => {
              const digit = event.target.value.replace(/\D/g, "").slice(-1);
              const next = value.split("");
              next[index] = digit;
              onChange(next.join(""));
              if (digit && index < 3) refs.current[index + 1]?.focus();
            }}
            onKeyDown={(event) => {
              if (event.key === "Backspace" && !value[index] && index > 0) refs.current[index - 1]?.focus();
            }}
            inputMode="numeric"
            maxLength={1}
            autoComplete="one-time-code"
          />
        ))}
      </div>
    </fieldset>
  );
}

export function ServiceInfo() {
  const { t } = useTranslation();
  const versionQuery = useQuery({ queryKey: ["version"], queryFn: getVersion, retry: false });
  return (
    <section className="rounded-[2rem] border border-slate-800 bg-slate-900/70 p-6">
      <h2 className="mb-4 text-sm uppercase tracking-[0.2em] text-slate-400">{t("home.serviceInfo")}</h2>
      {versionQuery.data ? (
        <dl className="grid grid-cols-[auto_1fr] gap-x-5 gap-y-2 text-sm">
          <dt className="text-slate-500">{t("home.fields.status")}</dt><dd className="text-emerald-300">{versionQuery.data.status}</dd>
          <dt className="text-slate-500">{t("home.fields.version")}</dt><dd>{versionQuery.data.version}</dd>
          <dt className="text-slate-500">{t("home.fields.buildTime")}</dt><dd className="truncate text-slate-300">{versionQuery.data.build_time}</dd>
        </dl>
      ) : <p className="text-sm text-slate-500">{t("home.loading")}</p>}
    </section>
  );
}
