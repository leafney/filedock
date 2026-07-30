import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Bell, Check, LogOut, RefreshCw, Shield, Trash2, Users, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { ErrorNotice, LoadingPage, PageFrame, PinInput, StatusBadge } from "../components/common";
import { streamEventName, type StreamEventMessage } from "../hooks/use-stream";
import { useSessionQuery } from "../hooks/use-session";
import {
  approveJoinRequest,
  cancelJoinRequest,
  createJoinRequest,
  extendRoom,
  getJoinInfo,
  getJoinRequests,
  getRoom,
  getRoomQRCode,
  joinRoom,
  kickMember,
  leaveRoom,
  dissolveRoom,
  rejectJoinRequest,
} from "../services/api";
import type { JoinMode, RoomJoinInfo, RoomMember, RoomSnapshot, Session } from "../types/domain";
import { formatDate, formatDuration } from "../utils/format";

export function RoomPage({ sessionQuery }: { sessionQuery: ReturnType<typeof useSessionQuery> }) {
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
  const run = (mutation: { mutate: () => void; isPending: boolean }) => { setActionError(undefined); if (mutation.isPending) return; mutation.mutate(); };
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

function JoinPanel({ info, onJoin, onRequest, onCancelRequest, loading, error }: { info: RoomJoinInfo; onJoin: (confirmed: boolean, pin?: string) => void; onRequest: () => void; onCancelRequest: () => void; loading: boolean; error?: unknown }) {
  const { t } = useTranslation();
  const [pin, setPin] = useState("");
  const autoSubmitted = useRef(false);
  useEffect(() => {
    if (pin.length === 4 && !autoSubmitted.current) {
      autoSubmitted.current = true;
      onJoin(true, pin);
    }
    if (pin.length < 4) autoSubmitted.current = false;
  }, [onJoin, pin]);
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
