import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Check, ChevronRight, Dice5, DoorOpen, LogOut, RefreshCw, Shield, UserRound, Users } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { ErrorNotice, isUnauthorized, LoadingPage, PageFrame, PinInput, ServiceInfo, StatusBadge } from "../components/common";
import { useSessionQuery } from "../hooks/use-session";
import { getRandomNickname, createRoom, createSession, listRooms, resetSession, updateSession } from "../services/api";
import type { JoinMode, Session } from "../types/domain";

export function SessionGate({ onCreated }: { onCreated?: (session: Session) => void }) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const randomQuery = useQuery({ queryKey: ["random-nickname"], queryFn: getRandomNickname, enabled: false, retry: false });
  const mutation = useMutation({ mutationFn: () => createSession(name.trim()), onSuccess: onCreated });

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

export function Header({ session, onReset }: { session: Session; onReset: () => void }) {
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

export function HomePage({ sessionQuery }: { sessionQuery: ReturnType<typeof useSessionQuery> }) {
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
