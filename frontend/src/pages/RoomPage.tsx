import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Check, Shield, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { ErrorNotice, LanguageSelector, PinInput } from "../components/common";
import { ProfileModal } from "../components/ProfileModal";
import { RoomWorkspace } from "../components/room/RoomWorkspace";
import { streamEventName, type StreamEventMessage } from "../hooks/use-stream";
import { useSessionQuery } from "../hooks/use-session";
import {
  cancelJoinRequest,
  createJoinRequest,
  dissolveRoom,
  extendRoom,
  getJoinInfo,
  getRoom,
  joinRoom,
  kickMember,
  listRooms,
  leaveRoom,
  resetSession,
} from "../services/api";
import type { RoomJoinInfo } from "../types/domain";

export function RoomPage({ sessionQuery }: { sessionQuery: ReturnType<typeof useSessionQuery> }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { code = "" } = useParams();
  const queryClient = useQueryClient();
  const session = sessionQuery.data;
  const [joined, setJoined] = useState(false);
  const [destroyAt, setDestroyAt] = useState<number>();
  const [kicked, setKicked] = useState(false);
  const [profileOpen, setProfileOpen] = useState(false);
  const [actionError, setActionError] = useState<unknown>();
  const validCode = /^\d{4}$/.test(code);
  const roomsQuery = useQuery({ queryKey: ["rooms"], queryFn: listRooms, enabled: Boolean(session), retry: false });
  const ownerRoom = useMemo(() => roomsQuery.data?.items.find((room) => room.role === "owner"), [roomsQuery.data?.items]);
  const joinInfo = useQuery({ queryKey: ["room-join-info", code], queryFn: () => getJoinInfo(code), enabled: Boolean(session && validCode), retry: false });
  useEffect(() => { if (joinInfo.data?.alreadyMember) setJoined(true); }, [joinInfo.data?.alreadyMember]);
  const snapshot = useQuery({ queryKey: ["room", code], queryFn: () => getRoom(code), enabled: Boolean(session && joined), retry: false, refetchInterval: joined && !destroyAt ? 30_000 : false });
  useEffect(() => { if (snapshot.data?.destroyAt) setDestroyAt(snapshot.data.destroyAt); }, [snapshot.data?.destroyAt]);
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
  }, [code, queryClient]);
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
  const dissolve = useMutation({ mutationFn: () => dissolveRoom(code), onSuccess: () => void queryClient.invalidateQueries({ queryKey: ["room", code] }) });
  useEffect(() => {
    const failed = [join, request, cancelRequest, extend, leave, dissolve].find((mutation) => mutation.isError);
    if (failed) setActionError(failed.error);
  }, [join, request, cancelRequest, extend, leave, dissolve]);

  if (sessionQuery.isPending) return <RoomGate>{t("home.loading")}</RoomGate>;
  if (!session) return <RoomGate><p>{t("session.nameRequired")}</p><Link to="/">{t("room.backHome")}</Link></RoomGate>;
  if (!validCode || joinInfo.isError) return <RoomGate><ErrorNotice error={joinInfo.error} /><Link to="/">{t("room.backHome")}</Link></RoomGate>;
  if (joinInfo.isPending) return <RoomGate>{t("room.loading")}</RoomGate>;
  if (!joined && joinInfo.data) return <JoinPanel info={joinInfo.data} onJoin={(confirmed, pin) => join.mutate({ confirmed, pin })} onRequest={() => request.mutate()} onCancelRequest={() => cancelRequest.mutate()} loading={join.isPending || request.isPending || cancelRequest.isPending} error={join.isError ? join.error : request.isError ? request.error : cancelRequest.error} />;
  if (snapshot.isPending) return <RoomGate>{t("room.loading")}</RoomGate>;
  if (snapshot.isError || !snapshot.data) return <RoomGate><ErrorNotice error={snapshot.error} /><Link to="/">{t("room.backHome")}</Link></RoomGate>;
  return <><RoomWorkspace
    room={snapshot.data}
    code={code}
    session={session}
    destroyAt={destroyAt}
    kicked={kicked}
    setKicked={setKicked}
    onExtend={() => { if (!extend.isPending) extend.mutate(); }}
    onLeave={() => { if (window.confirm(t("room.leaveConfirm")) && !leave.isPending) leave.mutate(); }}
    onDissolve={() => { if (window.confirm(t("room.dissolveConfirm")) && !dissolve.isPending) dissolve.mutate(); }}
    onKick={(userId) => { setActionError(undefined); void kickMember(code, userId).then(() => queryClient.invalidateQueries({ queryKey: ["room", code] })).catch(setActionError); }}
    onOpenProfile={() => setProfileOpen(true)}
    actionError={actionError}
  />{profileOpen && <ProfileModal open={profileOpen} onClose={() => setProfileOpen(false)} session={session} ownerRoom={ownerRoom} onReset={() => {
    if (ownerRoom || !window.confirm(t("home.resetConfirm"))) return;
    void resetSession().then(() => {
      queryClient.removeQueries({ queryKey: ["session"] });
      queryClient.removeQueries({ queryKey: ["rooms"] });
      setProfileOpen(false);
      navigate("/", { replace: true });
    }).catch(setActionError);
  }} />}</>;
}

function RoomGate({ children }: { children: ReactNode }) {
  return <main className="room-gate-page"><LanguageSelector /><section>{children}</section></main>;
}

function JoinPanel({ info, onJoin, onRequest, onCancelRequest, loading, error }: { info: RoomJoinInfo; onJoin: (confirmed: boolean, pin?: string) => void; onRequest: () => void; onCancelRequest: () => void; loading: boolean; error?: unknown }) {
  const { t } = useTranslation();
  const [pin, setPin] = useState("");
  const autoSubmitted = useRef(false);
  useEffect(() => {
    if (pin.length === 4 && !autoSubmitted.current) { autoSubmitted.current = true; onJoin(true, pin); }
    if (pin.length < 4) autoSubmitted.current = false;
  }, [onJoin, pin]);
  return <RoomGate><Link className="room-gate-back" to="/">← {t("room.backHome")}</Link><span className="room-gate-code">{t("room.code")} {info.roomCode}</span><h1>{info.title}</h1><p>{info.joinMode === "open" ? t("room.confirmJoin") : info.joinMode === "password" ? t("room.pinHint") : t("room.approvalHint")}</p>{error != null && <ErrorNotice error={error} />}{info.joinMode === "open" && <button disabled={loading} type="button" onClick={() => onJoin(true)}><Check aria-hidden="true" />{t("room.confirm")}</button>}{info.joinMode === "password" && <div className="room-gate-form"><PinInput id="join-pin" label={t("room.pin")} value={pin} onChange={setPin} /><button disabled={loading || pin.length !== 4} type="button" onClick={() => onJoin(true, pin)}><Shield aria-hidden="true" />{t("room.submitPIN")}</button></div>}{info.joinMode === "owner_approval" && (info.pendingRequest ? <button className="secondary" disabled={loading} type="button" onClick={onCancelRequest}><X aria-hidden="true" />{t("room.cancelRequest")}</button> : <button disabled={loading} type="button" onClick={onRequest}><Check aria-hidden="true" />{t("room.requestApproval")}</button>)}</RoomGate>;
}
