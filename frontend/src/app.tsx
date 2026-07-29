import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Route, Routes } from "react-router-dom";

import { normalizeLanguage } from "./i18n";
import { getApiErrorMessage } from "./lib/api-error";
import { getVersion } from "./services/version";

function LanguageSelector() {
  const { t, i18n } = useTranslation();
  const language = normalizeLanguage(i18n.resolvedLanguage ?? i18n.language) ?? "zh-CN";

  const changeLanguage = (value: string) => {
    const nextLanguage = normalizeLanguage(value);
    if (nextLanguage) {
      void i18n.changeLanguage(nextLanguage);
    }
  };

  return (
    <label className="absolute right-6 top-6">
      <span className="sr-only">{t("language.label")}</span>
      <select
        aria-label={t("language.label")}
        className="rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-200 shadow-lg outline-none transition focus:border-cyan-500 focus:ring-2 focus:ring-cyan-500/30"
        value={language}
        onChange={(event) => changeLanguage(event.target.value)}
      >
        <option value="zh-CN">{t("language.zhCN")}</option>
        <option value="en">{t("language.en")}</option>
      </select>
    </label>
  );
}

function VersionStatus() {
  const { t } = useTranslation();
  const versionQuery = useQuery({
    queryKey: ["version"],
    queryFn: getVersion,
    retry: false,
  });

  return (
    <main className="relative flex min-h-screen items-center justify-center bg-slate-950 px-6 py-12 text-slate-100">
      <LanguageSelector />
      <section className="w-full max-w-xl rounded-2xl border border-slate-800 bg-slate-900 p-8 shadow-2xl">
        <p className="text-sm font-medium uppercase tracking-[0.3em] text-cyan-400">{t("brand.name")}</p>
        <h1 className="mt-4 text-4xl font-semibold tracking-tight">{t("brand.displayName")}</h1>
        <p className="mt-3 text-slate-400">{t("brand.description")}</p>

        {versionQuery.isPending && (
          <p className="mt-8 rounded-lg bg-slate-800 px-4 py-3 text-slate-300">{t("home.loading")}</p>
        )}

        {versionQuery.isError && (
          <p className="mt-8 rounded-lg border border-rose-900 bg-rose-950/50 px-4 py-3 text-rose-300">
            {getApiErrorMessage(versionQuery.error, t)}
          </p>
        )}

        {versionQuery.data && (
          <dl className="mt-8 grid grid-cols-[auto_1fr] gap-x-6 gap-y-3 rounded-lg bg-slate-800 p-4 text-sm">
            <dt className="text-slate-400">{t("home.fields.status")}</dt>
            <dd className="font-medium text-emerald-400">{versionQuery.data.status}</dd>
            <dt className="text-slate-400">{t("home.fields.version")}</dt>
            <dd>{versionQuery.data.version}</dd>
            <dt className="text-slate-400">{t("home.fields.branch")}</dt>
            <dd>{versionQuery.data.git_branch}</dd>
            <dt className="text-slate-400">{t("home.fields.commit")}</dt>
            <dd>{versionQuery.data.git_commit}</dd>
            <dt className="text-slate-400">{t("home.fields.buildTime")}</dt>
            <dd>{versionQuery.data.build_time}</dd>
          </dl>
        )}
      </section>
    </main>
  );
}

export function App() {
  return (
    <Routes>
      <Route path="*" element={<VersionStatus />} />
    </Routes>
  );
}
