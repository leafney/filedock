import { Routes, Route } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";

import { getVersion } from "./services/version";

function VersionStatus() {
  const versionQuery = useQuery({
    queryKey: ["version"],
    queryFn: getVersion,
    retry: false,
  });

  return (
    <main className="flex min-h-screen items-center justify-center bg-slate-950 px-6 py-12 text-slate-100">
      <section className="w-full max-w-xl rounded-2xl border border-slate-800 bg-slate-900 p-8 shadow-2xl">
        <p className="text-sm font-medium uppercase tracking-[0.3em] text-cyan-400">FileDock</p>
        <h1 className="mt-4 text-4xl font-semibold tracking-tight">码头</h1>
        <p className="mt-3 text-slate-400">浏览器即开即用的文件传输基础服务。</p>

        {versionQuery.isPending && (
          <p className="mt-8 rounded-lg bg-slate-800 px-4 py-3 text-slate-300">正在检查服务状态…</p>
        )}

        {versionQuery.isError && (
          <p className="mt-8 rounded-lg border border-rose-900 bg-rose-950/50 px-4 py-3 text-rose-300">
            服务暂不可用，请确认后端已启动后刷新页面。
          </p>
        )}

        {versionQuery.data && (
          <dl className="mt-8 grid grid-cols-[auto_1fr] gap-x-6 gap-y-3 rounded-lg bg-slate-800 p-4 text-sm">
            <dt className="text-slate-400">状态</dt>
            <dd className="font-medium text-emerald-400">{versionQuery.data.status}</dd>
            <dt className="text-slate-400">版本</dt>
            <dd>{versionQuery.data.version}</dd>
            <dt className="text-slate-400">分支</dt>
            <dd>{versionQuery.data.git_branch}</dd>
            <dt className="text-slate-400">提交</dt>
            <dd>{versionQuery.data.git_commit}</dd>
            <dt className="text-slate-400">构建时间</dt>
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
