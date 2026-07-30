import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Bell, Check, ChevronRight, Dice5, DoorOpen, LogOut, RefreshCw, Shield, Trash2, UserRound, Users, X } from "lucide-react";
import { FormEvent, useEffect, useState } from "react";
import { Link, Route, Routes, useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import axios from "axios";

import { normalizeLanguage } from "./i18n";
import { getApiErrorMessage } from "./lib/api-error";
import { streamEventName, useGlobalStream, type StreamEventMessage } from "./hooks/use-stream";
import {
  approveJoinRequest,
  cancelJoinRequest,
  createJoinRequest,
  createRoom,
  createSession,
  dissolveRoom,
  extendRoom,
  getCurrentSession,
  getJoinInfo,
  getJoinRequests,
  getRandomNickname,
  getRoom,
  getRoomQRCode,
  joinRoom,
  kickMember,
  listRooms,
  leaveRoom,
  rejectJoinRequest,
  resetSession,
  updateSession,
} from "./services/api";
import { getVersion } from "./services/version";
import type { JoinMode, RoomMember, RoomSnapshot, Session } from "./types/domain";

function LanguageSelector() {
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

function PageFrame({ children }: { children: React.ReactNode }) {
  return (
    <main className="min-h-screen bg-[radial-gradient(circle_at_top,_#12304a_0,_#07111f_42%,_#020617_100%)] px-4 py-20 text-slate-100 sm:px-8">
      <LanguageSelector />
      <div className="mx-auto w-full max-w-6xl">{children}</div>
    </main>
  );
}

function LoadingPage({ label }: { label: string }) {
  return (
    <PageFrame>
      <div className="flex min-h-[50vh] items-center justify-center text-slate-300">
        <RefreshCw className="mr-3 animate-spin" size={18} aria-hidden="true" />
        {label}
      </div>
    </PageFrame>
  );
}

function ErrorNotice({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
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

function isUnauthorized(error: unknown) {
  return axios.isAxiosError(error) && error.response?.status === 401;
}

function SessionGate({ onCreated }: { onCreated?: (session: Session) => void }) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const randomQuery = useQuery({ queryKey: ["random-nickname"], queryFn: getRandomNickname, enabled: false, retry: false });
  const mutation = useMutation({
    mutationFn: () => createSession(name.trim()),
    onSuccess: onCreated,
  });

  useEffect(() => {
    void randomQuery.refetch().then((result) => {
      if (result.data?.displayName && !name) setName(result.data.displayName);
    });
    // Only choose a starting suggestion once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const randomize = () => {
    void randomQuery.refetch().then((result) => {
      if (result.data?.displayName) setName(result.data.displayName);
    });
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!name.trim()) return;
    mutation.mutate();
  };

  return (
    <PageFrame>
      <section className="mx-auto max-w-xl rounded-[2rem] border border-cyan-300/10 bg-slate-900/80 p-7 shadow-2xl shadow-cyan-950/30 backdrop-blur sm:p-10">
        <div className="mb-8 flex items-center gap-3 text-cyan-300">
          <div className="rounded-2xl bg-cyan-400/10 p-3"><DoorOpen size={24} aria-hidden="true" /></div>
          <span className="text-sm font-semibold uppercase tracking-[0.3em]">FileDock</span>
        </div>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">{t("home.welcome")}</h1>
        <p className="mt-3 text-slate-400">{t("home.intro")}</p>
        <form className="mt-9 space-y-5" onSubmit={submit}>
          <div>
            <label className="mb-2 block text-sm font-medium text-slate-300" htmlFor="display-name">{t("session.displayName")}</label>
            <div className="flex gap-2">
              <input
                id="display-name"
                className="min-w-0 flex-1 rounded-xl border border-slate-700 bg-slate-950/70 px-4 py-3 text-base outline-none transition placeholder:text-slate-600 focus:border-cyan-400 focus:ring-2 focus:ring-cyan-400/20"
                value={name}
                maxLength={20}
                placeholder={t("session.displayNamePlaceholder")}
                onChange={(event) => setName(event.target.value)}
                autoComplete="nickname"
              />
              <button className="rounded-xl border border-slate-700 px-3 text-slate-300 transition hover:border-cyan-400 hover:text-cyan-300" type="button" aria-label={t("session.randomize")} onClick={randomize}>
                <Dice5 size={21} aria-hidden="true" />
              </button>
            </div>
            <p className="mt-2 text-xs text-slate-500">{t("session.nameHint")}</p>
          </div>
          {mutation.isError && <ErrorNotice error={mutation.error} />}
          <button className="flex w-full items-center justify-center gap-2 rounded-xl bg-cyan-400 px-4 py-3 font-semibold text-slate-950 transition hover:bg-cyan-300 disabled:cursor-not-allowed disabled:opacity-60" disabled={!name.trim() || mutation.isPending} type="submit">
            {mutation.isPending ? <RefreshCw className="animate-spin" size={18} aria-hidden="true" /> : <ChevronRight size={18} aria-hidden="true" />}
            {t("session.create")}
          </button>
        </form>
      </section>
    </PageFrame>
  );
}

function Header({ session, onReset }: { session: Session; onReset: () => void }) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(session.displayName);
  const queryClient = useQueryClient();
  const update = useMutation({
    mutationFn: () => updateSession(name.trim()),
    onSuccess: (next) => {
      queryClient.setQueryData(["session"], next);
      setEditing(false);
    },
  });
  return (
    <header className="mb-8 flex flex-wrap items-center justify-between gap-4">
      <Link to="/" className="flex items-center gap-3 text-cyan-300">
        <span className="rounded-xl bg-cyan-400/10 p-2"><DoorOpen size={20} aria-hidden="true" /></span>
        <span className="font-semibold tracking-[0.2em]">FileDock</span>
      </Link>
      <div className="flex items-center gap-2 text-sm">
        {editing ? (
          <form className="flex gap-2" onSubmit={(event) => { event.preventDefault(); if (name.trim()) update.mutate(); }}>
            <input className="w-36 rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 outline-none focus:border-cyan-400" value={name} onChange={(event) => setName(event.target.value)} maxLength={20} autoFocus />
            <button className="rounded-lg bg-cyan-400 px-3 py-2 font-medium text-slate-950" type="submit">{t("home.saveName")}</button>
          </form>
        ) : (
          <button className="flex items-center gap-2 rounded-lg border border-slate-700 bg-slate-900/70 px-3 py-2 text-slate-200 hover:border-cyan-400" type="button" onClick={() => setEditing(true)}>
            <UserRound size={16} aria-hidden="true" />{session.displayName}
          </button>
        )}
        <button className="rounded-lg border border-slate-700 p-2 text-slate-400 hover:border-rose-400 hover:text-rose-300" type="button" aria-label={t("session.reset")} onClick={onReset}>
          <LogOut size={17} aria-hidden="true" />
        </button>
      </div>
    </header>
  );
}

function StatusBadge({ status }: { status: string }) {
  const { t } = useTranslation();
  const label = status === "active" ? t("room.statusActive") : status === "destroying" ? t("room.statusDestroying") : t("room.statusDestroyed");
  return <span className="rounded-full bg-slate-800 px-2.5 py-1 text-xs text-slate-300">{label}</span>;
}

function HomePage({ sessionQuery }: { sessionQuery: ReturnType<typeof useSessionQuery> }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [joinCode, setJoinCode] = useState("");
  const [joinMode, setJoinMode] = useState<JoinMode>("open");
  const [pin, setPin] = useState("");
  const [pinConfirmation, setPinConfirmation] = useState("");
  const [notice, setNotice] = useState("");
  const session = sessionQuery.data;
  const roomsQuery = useQuery({ queryKey: ["rooms"], queryFn: listRooms, enabled: Boolean(session), retry: false });
  const create = useMutation({
    mutationFn: () => createRoom(joinMode, joinMode === "password" ? pin : "", joinMode === "password" ? pinConfirmation : ""),
    onSuccess: (room) => { void queryClient.invalidateQueries({ queryKey: ["rooms"] }); navigate(`/rooms/${room.roomCode}`); },
  });
  const reset = useMutation({
    mutationFn: resetSession,
    onSuccess: () => { queryClient.removeQueries({ queryKey: ["session"] }); queryClient.removeQueries({ queryKey: ["rooms"] }); setNotice(t("home.signedOut")); },
  });

  if (sessionQuery.isPending) return <LoadingPage label={t("home.loading")} />;
  if (!session) {
    if (sessionQuery.error && !isUnauthorized(sessionQuery.error)) return <PageFrame><ErrorNotice error={sessionQuery.error} onRetry={() => void sessionQuery.refetch()} /></PageFrame>;
    return <SessionGate onCreated={(next) => queryClient.setQueryData(["session"], next)} />;
  }

  const submitJoin = (event: FormEvent) => {
    event.preventDefault();
    const code = joinCode.trim();
    if (/^\d{4}$/.test(code)) navigate(`/rooms/${code}`);
  };

  return (
    <PageFrame>
      <Header session={session} onReset={() => { if (window.confirm(t("home.resetConfirm"))) reset.mutate(); }} />
      <section className="grid gap-6 lg:grid-cols-[1.25fr_0.75fr]">
        <div className="space-y-6">
          <div className="rounded-[2rem] border border-cyan-300/10 bg-slate-900/75 p-6 shadow-xl shadow-slate-950/30 sm:p-8">
            <div className="flex items-start justify-between gap-4">
              <div><p className="text-sm uppercase tracking-[0.25em] text-cyan-300">{t("home.rooms")}</p><h1 className="mt-2 text-3xl font-semibold">{t("home.welcome")}</h1></div>
              <Users className="text-slate-600" size={30} aria-hidden="true" />
            </div>
            {roomsQuery.isPending && <p className="mt-8 text-sm text-slate-400">{t("home.loading")}</p>}
            {roomsQuery.isError && <div className="mt-6"><ErrorNotice error={roomsQuery.error} onRetry={() => void roomsQuery.refetch()} /></div>}
            {roomsQuery.data?.items.length === 0 && <p className="mt-8 rounded-xl border border-dashed border-slate-700 px-4 py-8 text-center text-slate-500">{t("home.noRooms")}</p>}
            <div className="mt-6 grid gap-3 sm:grid-cols-2">
              {roomsQuery.data?.items.map((room) => (
                <button key={room.roomId} type="button" onClick={() => navigate(`/rooms/${room.roomCode}`)} className="group rounded-2xl border border-slate-800 bg-slate-950/60 p-4 text-left transition hover:border-cyan-400/50 hover:bg-slate-950">
                  <div className="flex items-center justify-between gap-2"><span className="font-mono text-2xl tracking-[0.18em] text-cyan-300">{room.roomCode}</span><StatusBadge status={room.status} /></div>
                  <p className="mt-3 text-sm text-slate-300">{room.title}</p>
                  <p className="mt-1 text-xs text-slate-500">{room.role === "owner" ? t("room.roleOwner") : t("room.roleMember")}</p>
                  <ChevronRight className="mt-3 text-slate-600 transition group-hover:translate-x-1 group-hover:text-cyan-300" size={17} aria-hidden="true" />
                </button>
              ))}
            </div>
          </div>
          <CreateRoomCard joinMode={joinMode} setJoinMode={setJoinMode} pin={pin} setPin={setPin} pinConfirmation={pinConfirmation} setPinConfirmation={setPinConfirmation} mutation={create} />
        </div>
        <aside className="space-y-6">
          <section className="rounded-[2rem] border border-slate-800 bg-slate-900/70 p-6">
            <div className="mb-5 flex items-center gap-3"><Shield className="text-cyan-300" size={20} aria-hidden="true" /><h2 className="font-semibold">{t("home.joinByCode")}</h2></div>
            <form className="space-y-3" onSubmit={submitJoin}>
              <label className="sr-only" htmlFor="join-code">{t("room.code")}</label>
              <input id="join-code" className="w-full rounded-xl border border-slate-700 bg-slate-950 px-4 py-3 font-mono tracking-[0.28em] outline-none focus:border-cyan-400" value={joinCode} onChange={(event) => setJoinCode(event.target.value.replace(/\D/g, "").slice(0, 4))} placeholder={t("home.roomCodePlaceholder")} inputMode="numeric" />
              <button className="flex w-full items-center justify-center gap-2 rounded-xl border border-cyan-400/50 px-4 py-3 font-medium text-cyan-200 transition hover:bg-cyan-400/10 disabled:opacity-40" disabled={!/^\d{4}$/.test(joinCode)} type="submit"><DoorOpen size={17} aria-hidden="true" />{t("room.join")}</button>
            </form>
          </section>
          <ServiceInfo />
          {notice && <p className="rounded-xl border border-emerald-400/30 bg-emerald-950/30 px-4 py-3 text-sm text-emerald-200">{notice}</p>}
          {reset.isError && <ErrorNotice error={reset.error} />}
        </aside>
      </section>
    </PageFrame>
  );
}

function CreateRoomCard({ joinMode, setJoinMode, pin, setPin, pinConfirmation, setPinConfirmation, mutation }: { joinMode: JoinMode; setJoinMode: (value: JoinMode) => void; pin: string; setPin: (value: string) => void; pinConfirmation: string; setPinConfirmation: (value: string) => void; mutation: { mutate: () => void; isPending: boolean; isError: boolean; error: unknown } }) {
  const { t } = useTranslation();
  return (
    <section className="rounded-[2rem] border border-slate-800 bg-slate-900/70 p-6 sm:p-8">
      <div className="mb-5 flex items-center gap-3"><DoorOpen className="text-cyan-300" size={20} aria-hidden="true" /><h2 className="font-semibold">{t("room.create")}</h2></div>
      <div className="grid gap-2 sm:grid-cols-3">
        {(["open", "password", "owner_approval"] as JoinMode[]).map((mode) => (
          <button key={mode} type="button" onClick={() => setJoinMode(mode)} className={`rounded-xl border px-3 py-3 text-sm transition ${joinMode === mode ? "border-cyan-400 bg-cyan-400/10 text-cyan-200" : "border-slate-700 text-slate-400 hover:border-slate-500"}`}>
            {mode === "open" ? t("room.open") : mode === "password" ? t("room.password") : t("room.ownerApproval")}
          </button>
        ))}
      </div>
      {joinMode === "password" && <div className="mt-4 grid gap-3 sm:grid-cols-2"><PinInput id="create-pin" label={t("room.pin")} value={pin} onChange={setPin} /><PinInput id="create-pin-confirm" label={t("room.pinConfirmation")} value={pinConfirmation} onChange={setPinConfirmation} /></div>}
      {mutation.isError && <div className="mt-4"><ErrorNotice error={mutation.error} /></div>}
      <button className="mt-5 flex w-full items-center justify-center gap-2 rounded-xl bg-cyan-400 px-4 py-3 font-semibold text-slate-950 transition hover:bg-cyan-300 disabled:cursor-not-allowed disabled:opacity-50" type="button" disabled={mutation.isPending || (joinMode === "password" && (pin.length !== 4 || pinConfirmation.length !== 4))} onClick={() => mutation.mutate()}>
        {mutation.isPending ? <RefreshCw className="animate-spin" size={17} aria-hidden="true" /> : <Check size={17} aria-hidden="true" />}{t("room.create")}
      </button>
    </section>
  );
}

function PinInput({ id, label, value, onChange }: { id: string; label: string; value: string; onChange: (value: string) => void }) {
  return <label className="block text-sm text-slate-400" htmlFor={id}>{label}<input id={id} className="mt-2 w-full rounded-xl border border-slate-700 bg-slate-950 px-4 py-3 text-center font-mono text-xl tracking-[0.65em] outline-none focus:border-cyan-400" value={value} onChange={(event) => onChange(event.target.value.replace(/\D/g, "").slice(0, 4))} inputMode="numeric" maxLength={4} autoComplete="off" /></label>;
}

function ServiceInfo() {
  const { t } = useTranslation();
  const versionQuery = useQuery({ queryKey: ["version"], queryFn: getVersion, retry: false });
  return <section className="rounded-[2rem] border border-slate-800 bg-slate-900/70 p-6"><h2 className="mb-4 text-sm uppercase tracking-[0.2em] text-slate-400">{t("home.serviceInfo")}</h2>{versionQuery.data ? <dl className="grid grid-cols-[auto_1fr] gap-x-5 gap-y-2 text-sm"><dt className="text-slate-500">{t("home.fields.status")}</dt><dd className="text-emerald-300">{versionQuery.data.status}</dd><dt className="text-slate-500">{t("home.fields.version")}</dt><dd>{versionQuery.data.version}</dd><dt className="text-slate-500">{t("home.fields.buildTime")}</dt><dd className="truncate text-slate-300">{versionQuery.data.build_time}</dd></dl> : <p className="text-sm text-slate-500">{t("home.loading")}</p>}</section>;
}

function RoomPage({ sessionQuery }: { sessionQuery: ReturnType<typeof useSessionQuery> }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { code = "" } = useParams();
  const queryClient = useQueryClient();
  const session = sessionQuery.data;
  const [joined, setJoined] = useState(false);
  const [destroyAt, setDestroyAt] = useState<number | undefined>();
  const [kicked, setKicked] = useState(false);
  const [actionError, setActionError] = useState<unknown>();
  const joinInfo = useQuery({ queryKey: ["room-join-info", code], queryFn: () => getJoinInfo(code), enabled: Boolean(session && /^\d{4}$/.test(code)), retry: false });
  useEffect(() => { if (joinInfo.data?.alreadyMember) setJoined(true); }, [joinInfo.data?.alreadyMember]);
  const snapshot = useQuery({ queryKey: ["room", code], queryFn: () => getRoom(code), enabled: Boolean(session && joined), retry: false, refetchInterval: joined && !destroyAt ? 30_000 : false });
  useEffect(() => { if (snapshot.data?.destroyAt && !destroyAt) setDestroyAt(snapshot.data.destroyAt); }, [destroyAt, snapshot.data?.destroyAt]);
  useEffect(() => {
    const listener = (raw: Event) => {
      const event = (raw as CustomEvent<StreamEventMessage>).detail?.event;
      if (!event) return;
      const payload = (event.payload ?? {}) as Record<string, unknown>;
      if (payload.roomCode !== code) return;
      if (event.type === "room.member_kicked") setKicked(true);
      if (event.type === "room.destroying" && typeof payload.destroyAt === "number") setDestroyAt(payload.destroyAt);
      if (event.type === "room.destroyed") setDestroyAt(Math.floor(Date.now() / 1000));
      void queryClient.invalidateQueries({ queryKey: ["room", code] });
      void queryClient.invalidateQueries({ queryKey: ["room-join-info", code] });
    };
    window.addEventListener(streamEventName, listener);
    return () => window.removeEventListener(streamEventName, listener);
  }, [code, queryClient, session?.userId]);
  useEffect(() => {
    if (!destroyAt) return undefined;
    const timer = window.setInterval(() => {
      if (destroyAt <= Math.floor(Date.now() / 1000)) {
        window.clearInterval(timer);
        navigate("/", { replace: true });
      }
    }, 250);
    return () => window.clearInterval(timer);
  }, [destroyAt, navigate]);

  const join = useMutation({ mutationFn: ({ confirmed, pin }: { confirmed: boolean; pin?: string }) => joinRoom(code, confirmed, pin), onSuccess: () => { setJoined(true); void queryClient.invalidateQueries({ queryKey: ["rooms"] }); } });
  const request = useMutation({ mutationFn: () => createJoinRequest(code), onSuccess: () => void queryClient.invalidateQueries({ queryKey: ["room-join-info", code] }) });
  const cancelRequest = useMutation({ mutationFn: () => cancelJoinRequest(code), onSuccess: () => void queryClient.invalidateQueries({ queryKey: ["room-join-info", code] }) });
  const extend = useMutation({ mutationFn: () => extendRoom(code), onSuccess: () => void queryClient.invalidateQueries({ queryKey: ["room", code] }) });
  const leave = useMutation({ mutationFn: () => leaveRoom(code), onSuccess: () => navigate("/", { replace: true }) });
  const dissolve = useMutation({ mutationFn: () => dissolveRoom(code), onSuccess: () => navigate("/", { replace: true }) });
  const run = (mutation: { mutate: () => void; isPending: boolean; isError: boolean; error: unknown }) => { setActionError(undefined); if (mutation.isPending) return; mutation.mutate(); };
  useEffect(() => { const errors = [join, request, cancelRequest, extend, leave, dissolve].find((item) => item.isError); if (errors) setActionError(errors.error); }, [join, request, cancelRequest, extend, leave, dissolve]);

  if (sessionQuery.isPending) return <LoadingPage label={t("home.loading")} />;
  if (!session) return <PageFrame><div className="mx-auto max-w-xl space-y-4"><p className="text-slate-300">{t("session.nameRequired")}</p><Link className="inline-flex rounded-xl bg-cyan-400 px-4 py-3 font-semibold text-slate-950" to="/">{t("room.backHome")}</Link></div></PageFrame>;
  if (joinInfo.isPending) return <LoadingPage label={t("room.loading")} />;
  if (joinInfo.isError) return <PageFrame><div className="mx-auto max-w-xl space-y-4"><ErrorNotice error={joinInfo.error} /><Link className="inline-flex rounded-xl border border-slate-700 px-4 py-3" to="/">{t("room.backHome")}</Link></div></PageFrame>;
  if (!joined && joinInfo.data) return <JoinPanel info={joinInfo.data} onJoin={(confirmed, pin) => join.mutate({ confirmed, pin })} onRequest={() => request.mutate()} onCancelRequest={() => cancelRequest.mutate()} loading={join.isPending || request.isPending || cancelRequest.isPending} error={join.isError ? join.error : request.isError ? request.error : cancelRequest.error} />;
  if (snapshot.isPending) return <LoadingPage label={t("room.loading")} />;
  if (snapshot.isError || !snapshot.data) return <PageFrame><div className="mx-auto max-w-xl space-y-4"><ErrorNotice error={snapshot.error} /><Link className="inline-flex rounded-xl border border-slate-700 px-4 py-3" to="/">{t("room.backHome")}</Link></div></PageFrame>;
  const room = snapshot.data;
  return <><RoomWorkspace room={room} code={code} session={session} destroyAt={destroyAt} kicked={kicked} setKicked={setKicked} onExtend={() => run(extend)} onLeave={() => { if (window.confirm(t("room.leaveConfirm"))) run(leave); }} onDissolve={() => { if (window.confirm(t("room.dissolveConfirm"))) run(dissolve); }} onKick={(userId) => kickMember(code, userId).then(() => void queryClient.invalidateQueries({ queryKey: ["room", code] })).catch(setActionError)} actionError={actionError} /><RoomQRCode code={code} /></>;
}

function JoinPanel({ info, onJoin, onRequest, onCancelRequest, loading, error }: { info: { title: string; roomCode: string; joinMode: JoinMode; pendingRequest: boolean }; onJoin: (confirmed: boolean, pin?: string) => void; onRequest: () => void; onCancelRequest: () => void; loading: boolean; error?: unknown }) {
  const { t } = useTranslation();
  const [pin, setPin] = useState("");
  return <PageFrame><section className="mx-auto max-w-xl rounded-[2rem] border border-slate-800 bg-slate-900/80 p-7 shadow-2xl sm:p-10"><Link className="text-sm text-slate-500 hover:text-cyan-300" to="/">← {t("room.backHome")}</Link><p className="mt-8 text-sm uppercase tracking-[0.25em] text-cyan-300">{t("room.code")} {info.roomCode}</p><h1 className="mt-2 text-3xl font-semibold">{info.title}</h1><p className="mt-3 text-slate-400">{info.joinMode === "open" ? t("room.confirmJoin") : info.joinMode === "password" ? t("room.pinHint") : t("room.approvalHint")}</p>{error != null && <div className="mt-5"><ErrorNotice error={error} /></div>}{info.joinMode === "open" && <button className="mt-8 flex w-full items-center justify-center gap-2 rounded-xl bg-cyan-400 px-4 py-3 font-semibold text-slate-950 disabled:opacity-50" disabled={loading} type="button" onClick={() => onJoin(true)}><Check size={18} aria-hidden="true" />{t("room.confirm")}</button>}{info.joinMode === "password" && <div className="mt-8 space-y-4"><PinInput id="join-pin" label={t("room.pin")} value={pin} onChange={setPin} /><button className="flex w-full items-center justify-center gap-2 rounded-xl bg-cyan-400 px-4 py-3 font-semibold text-slate-950 disabled:opacity-50" disabled={loading || pin.length !== 4} type="button" onClick={() => onJoin(true, pin)}><Shield size={18} aria-hidden="true" />{t("room.submitPIN")}</button></div>}{info.joinMode === "owner_approval" && (info.pendingRequest ? <button className="mt-8 flex w-full items-center justify-center gap-2 rounded-xl border border-slate-700 px-4 py-3 font-medium text-slate-200 disabled:opacity-50" disabled={loading} type="button" onClick={onCancelRequest}><X size={18} aria-hidden="true" />{t("room.cancelRequest")}</button> : <button className="mt-8 flex w-full items-center justify-center gap-2 rounded-xl bg-cyan-400 px-4 py-3 font-semibold text-slate-950 disabled:opacity-50" disabled={loading} type="button" onClick={onRequest}><Check size={18} aria-hidden="true" />{t("room.requestApproval")}</button>)}</section></PageFrame>;
}

function RoomQRCode({ code }: { code: string }) {
  const { t } = useTranslation();
  const qrCode = useQuery({ queryKey: ["room-qrcode", code], queryFn: () => getRoomQRCode(code), retry: false });
  return <section className="mx-auto -mt-10 w-full max-w-6xl px-4 pb-10 sm:px-8"><div className="rounded-2xl border border-slate-800 bg-slate-900/75 p-5"><h2 className="text-sm font-medium text-slate-300">{t("room.qrcode")}</h2>{qrCode.data?.svg ? <div className="mt-4 inline-block rounded-lg bg-white p-3" dangerouslySetInnerHTML={{ __html: qrCode.data.svg }} /> : <p className="mt-3 text-xs text-slate-500">{qrCode.isError ? t("room.qrcodeUnavailable") : t("room.loading")}</p>}</div></section>;
}

function RoomWorkspace({ room, code, session, destroyAt, kicked, setKicked, onExtend, onLeave, onDissolve, onKick, actionError }: { room: RoomSnapshot; code: string; session: Session; destroyAt?: number; kicked: boolean; setKicked: (value: boolean) => void; onExtend: () => void; onLeave: () => void; onDissolve: () => void; onKick: (userId: string) => void; actionError?: unknown }) {
  const { t } = useTranslation();
  const [now, setNow] = useState(() => Math.floor(Date.now() / 1000));
  const [showRequests, setShowRequests] = useState(false);
  const requests = useQuery({ queryKey: ["join-requests", code], queryFn: () => getJoinRequests(code), enabled: room.role === "owner" && showRequests, retry: false });
  const queryClient = useQueryClient();
  const approve = useMutation({ mutationFn: (id: string) => approveJoinRequest(code, id), onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ["join-requests", code] }); void queryClient.invalidateQueries({ queryKey: ["room", code] }); } });
  const reject = useMutation({ mutationFn: (id: string) => rejectJoinRequest(code, id), onSuccess: () => void queryClient.invalidateQueries({ queryKey: ["join-requests", code] }) });
  useEffect(() => { const timer = window.setInterval(() => setNow(Math.floor(Date.now() / 1000)), 1000); return () => window.clearInterval(timer); }, []);
  const countdown = destroyAt ? Math.max(0, Math.ceil(destroyAt - now)) : 0;
  const expires = Math.max(0, room.expiresAt - now);
  const statusText = room.status === "destroying" ? t("room.destroying") : room.status === "destroyed" ? t("room.destroyed") : "";
  return <PageFrame><header className="mb-7 flex flex-wrap items-center justify-between gap-4"><Link to="/" className="text-sm text-slate-400 hover:text-cyan-300">← {t("room.backHome")}</Link><div className="flex items-center gap-2">{room.role === "owner" && <button className="relative rounded-xl border border-slate-700 p-2.5 text-slate-300 hover:border-cyan-400 hover:text-cyan-300" type="button" aria-label={t("room.notifications")} onClick={() => setShowRequests((value) => !value)}><Bell size={19} aria-hidden="true" />{(room.pendingRequestCount ?? requests.data?.pendingCount ?? 0) > 0 && <span className="absolute -right-1 -top-1 min-w-5 rounded-full bg-rose-400 px-1 text-center text-[10px] font-bold text-slate-950">{room.pendingRequestCount ?? requests.data?.pendingCount}</span>}</button>}<span className="rounded-full border border-slate-700 px-3 py-1 text-xs text-slate-400">{room.role === "owner" ? t("room.roleOwner") : t("room.roleMember")}</span></div></header><div className="grid gap-6 lg:grid-cols-[1fr_0.34fr]"><section className="rounded-[2rem] border border-slate-800 bg-slate-900/75 p-6 shadow-xl sm:p-8"><div className="flex flex-wrap items-start justify-between gap-4"><div><p className="font-mono text-sm tracking-[0.25em] text-cyan-300">{t("room.code")} {room.roomCode}</p><h1 className="mt-2 text-3xl font-semibold">{room.title}</h1></div><StatusBadge status={room.status} /></div><div className="mt-7 grid gap-3 sm:grid-cols-2"><div className="rounded-xl bg-slate-950/60 p-4"><p className="text-xs text-slate-500">{t("room.expiresAt")}</p><p className="mt-1 text-lg text-slate-200">{formatDuration(expires)}</p></div><div className="rounded-xl bg-slate-950/60 p-4"><p className="text-xs text-slate-500">{t("room.joinMode")}</p><p className="mt-1 text-lg text-slate-200">{room.joinMode === "open" ? t("room.open") : room.joinMode === "password" ? t("room.password") : t("room.ownerApproval")}</p></div></div><div className="mt-8"><h2 className="mb-3 flex items-center gap-2 text-sm uppercase tracking-[0.2em] text-slate-400"><Users size={17} aria-hidden="true" />{t("room.members")}</h2><div className="divide-y divide-slate-800 rounded-xl border border-slate-800">{room.members.map((member) => <MemberRow key={member.userId} member={member} self={member.userId === session.userId} owner={room.role === "owner"} onKick={onKick} />)}</div></div><div className="mt-7 flex flex-wrap gap-3">{room.role === "owner" && room.canExtend && <button className="rounded-xl border border-cyan-400/50 px-4 py-2.5 text-sm text-cyan-200 hover:bg-cyan-400/10" type="button" onClick={onExtend}>{t("room.extend")}</button>}{room.role === "owner" ? <button className="rounded-xl border border-rose-400/40 px-4 py-2.5 text-sm text-rose-200 hover:bg-rose-400/10" type="button" onClick={onDissolve}><Trash2 className="mr-2 inline" size={16} aria-hidden="true" />{t("room.dissolve")}</button> : <button className="rounded-xl border border-slate-700 px-4 py-2.5 text-sm text-slate-300 hover:border-slate-500" type="button" onClick={onLeave}><LogOut className="mr-2 inline" size={16} aria-hidden="true" />{t("room.leave")}</button>}</div>{actionError != null && <div className="mt-4"><ErrorNotice error={actionError} /></div>}</section><aside className="space-y-6">{showRequests && room.role === "owner" && <RequestPanel requests={requests.data?.items ?? []} loading={requests.isPending} error={requests.error} approve={(id) => approve.mutate(id)} reject={(id) => reject.mutate(id)} />}</aside></div>{(destroyAt || room.status === "destroying") && <div className="fixed inset-0 z-30 flex items-center justify-center bg-slate-950/90 p-6 text-center backdrop-blur-sm"><div className="max-w-lg"><RefreshCw className="mx-auto text-amber-300" size={40} aria-hidden="true" /><h2 className="mt-6 text-3xl font-semibold">{room.status === "destroyed" ? t("room.destroyedTitle") : t("room.destroyingTitle")}</h2><p className="mt-3 text-slate-300">{room.status === "destroyed" ? t("room.destroyed") : t("room.destroyingHint")}</p><p className="mt-7 font-mono text-5xl text-amber-300">{t("room.destroyCountdown", { seconds: String(countdown) })}</p></div></div>}{kicked && <div className="fixed inset-0 z-40 flex items-center justify-center bg-slate-950/90 p-6 backdrop-blur-sm"><div className="w-full max-w-md rounded-2xl border border-rose-400/30 bg-slate-900 p-7 text-center shadow-2xl"><X className="mx-auto text-rose-300" size={35} aria-hidden="true" /><h2 className="mt-4 text-2xl font-semibold">{t("room.kickedTitle")}</h2><p className="mt-3 text-slate-300">{t("room.kicked")}</p><button className="mt-7 w-full rounded-xl bg-cyan-400 px-4 py-3 font-semibold text-slate-950" type="button" onClick={() => { setKicked(false); window.location.replace("/"); }}>{t("room.confirmOnly")}</button></div></div>}</PageFrame>;
}

function MemberRow({ member, self, owner, onKick }: { member: RoomMember; self: boolean; owner: boolean; onKick: (userId: string) => void }) {
  const { t } = useTranslation();
  const statusColor = member.onlineStatus === "online" ? "bg-emerald-400" : member.onlineStatus === "away" ? "bg-amber-300" : "bg-slate-600";
  return <div className="flex items-center justify-between gap-3 px-4 py-3"><div className="flex min-w-0 items-center gap-3"><span className={`h-2.5 w-2.5 shrink-0 rounded-full ${statusColor}`} title={member.onlineStatus} /><div className="min-w-0"><p className="truncate text-sm text-slate-200">{member.displayName}{self && <span className="ml-2 text-xs text-cyan-300">({t("session.displayName")})</span>}</p><p className="text-xs text-slate-500">{member.role === "owner" ? t("room.roleOwner") : t("room.roleMember")} · {member.onlineStatus === "online" ? t("room.online") : member.onlineStatus === "away" ? t("room.away") : t("room.offline")}</p></div></div>{owner && member.role !== "owner" && member.status === "active" && <button className="rounded-lg p-2 text-slate-500 hover:bg-rose-400/10 hover:text-rose-300" type="button" aria-label={t("room.kick")} onClick={() => { if (window.confirm(t("room.kickConfirm", { name: member.displayName }))) onKick(member.userId); }}><Trash2 size={16} aria-hidden="true" /></button>}</div>;
}

function RequestPanel({ requests, loading, error, approve, reject }: { requests: { requestId: string; displayName: string; expiresAt: number }[]; loading: boolean; error: unknown; approve: (id: string) => void; reject: (id: string) => void }) {
  const { t } = useTranslation();
  return <section className="rounded-[2rem] border border-slate-800 bg-slate-900/75 p-6"><h2 className="flex items-center gap-2 font-semibold"><Bell size={18} className="text-cyan-300" aria-hidden="true" />{t("room.pendingRequests")}</h2>{loading && <p className="mt-5 text-sm text-slate-500">{t("room.loading")}</p>}{error != null && <div className="mt-5"><ErrorNotice error={error} /></div>}{!loading && error == null && requests.length === 0 && <p className="mt-5 text-sm text-slate-500">{t("room.noRequests")}</p>}<div className="mt-4 space-y-3">{requests.map((request) => <div key={request.requestId} className="rounded-xl border border-slate-800 bg-slate-950/50 p-3"><p className="font-medium text-slate-200">{request.displayName}</p><p className="mt-1 text-xs text-slate-500">{t("room.requestExpires", { time: formatDate(request.expiresAt) })}</p><div className="mt-3 flex gap-2"><button className="flex-1 rounded-lg bg-cyan-400 px-2 py-2 text-xs font-semibold text-slate-950" type="button" onClick={() => approve(request.requestId)}>{t("room.approve")}</button><button className="flex-1 rounded-lg border border-slate-700 px-2 py-2 text-xs text-slate-300" type="button" onClick={() => reject(request.requestId)}>{t("room.reject")}</button></div></div>)}</div></section>;
}

function formatDuration(seconds: number) {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const rest = seconds % 60;
  return `${hours}h ${minutes}m ${rest}s`;
}

function formatDate(timestamp: number) {
  return new Intl.DateTimeFormat(undefined, { dateStyle: "short", timeStyle: "short" }).format(new Date(timestamp * 1000));
}

function useSessionQuery() {
  return useQuery({ queryKey: ["session"], queryFn: getCurrentSession, retry: false, staleTime: 60_000 });
}

export function App() {
  const sessionQuery = useSessionQuery();
  useGlobalStream(sessionQuery.data);
  return <Routes><Route path="/" element={<HomePage sessionQuery={sessionQuery} />} /><Route path="/rooms/:code" element={<RoomPage sessionQuery={sessionQuery} />} /><Route path="*" element={<HomePage sessionQuery={sessionQuery} />} /></Routes>;
}
