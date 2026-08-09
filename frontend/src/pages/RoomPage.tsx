import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { message } from "antd";
import { AlertTriangle, Check, Clock3, KeyRound, LoaderCircle, RefreshCw, Send, UserRound, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { AppShell } from "../components/AppShell";
import { PinInput } from "../components/common";
import { ProfileModal } from "../components/ProfileModal";
import { RoomGateCard, type RoomGateAction } from "../components/RoomGateCard";
import { RoomWorkspace } from "../components/room/RoomWorkspace";
import { streamEventName, type StreamEventMessage } from "../hooks/use-stream";
import { useSessionQuery } from "../hooks/use-session";
import { getApiErrorCode, getApiErrorMessage } from "../lib/api-error";
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
import type { RoomJoinInfo, Session } from "../types/domain";
import { readNotificationChatLaunch, readNotificationFileLaunch } from "../utils/notifications";
import { classifyRoomGateError, createPinSubmissionGate, deriveJoinGateView } from "../utils/room-gate";

export function RoomPage({ sessionQuery }: { sessionQuery: ReturnType<typeof useSessionQuery> }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { code = "" } = useParams();
  const queryClient = useQueryClient();
  const session = sessionQuery.data;
  const [joined, setJoined] = useState(false);
  const [destroyAt, setDestroyAt] = useState<number>();
  const [kicked, setKicked] = useState(false);
  const [profileOpen, setProfileOpen] = useState(false);
  const [actionError, setActionError] = useState<unknown>();
  const [chatLaunch, setChatLaunch] = useState<{ roomCode: string; peerUserId: string; token: string }>();
  const [fileLaunch, setFileLaunch] = useState<{ roomCode: string; token: string; view: "list" | "timeline" | "trash"; requestId?: string }>();
  const validCode = /^\d{4}$/.test(code);
  const roomsQuery = useQuery({ queryKey: ["rooms"], queryFn: listRooms, enabled: Boolean(session), retry: false });
  const ownerRoom = useMemo(() => roomsQuery.data?.items.find((room) => room.role === "owner"), [roomsQuery.data?.items]);
  const joinInfo = useQuery({ queryKey: ["room-join-info", code], queryFn: () => getJoinInfo(code), enabled: Boolean(session && validCode), retry: false });
  useEffect(() => { if (joinInfo.data?.alreadyMember) setJoined(true); }, [joinInfo.data?.alreadyMember]);
  const snapshot = useQuery({ queryKey: ["room", code], queryFn: () => getRoom(code), enabled: Boolean(session && joined), retry: false, refetchInterval: joined && !destroyAt ? 30_000 : false });
  const consumeChatLaunch = useCallback((token: string) => {
    setChatLaunch((current) => current?.token === token ? undefined : current);
  }, []);
  const consumeFileLaunch = useCallback((token: string) => {
    setFileLaunch((current) => current?.token === token ? undefined : current);
  }, []);
  useEffect(() => {
    const chatTarget = readNotificationChatLaunch(location.state);
    const fileTarget = readNotificationFileLaunch(location.state);
    if (!chatTarget && !fileTarget) return;
    if (chatTarget) setChatLaunch({ roomCode: code, ...chatTarget });
    if (fileTarget) {
      setFileLaunch({ roomCode: code, ...fileTarget });
      void queryClient.invalidateQueries({ queryKey: ["room-files", code] });
      void queryClient.invalidateQueries({ queryKey: ["file-trash", code] });
      void queryClient.invalidateQueries({ queryKey: ["file-trash-count", code] });
      void queryClient.invalidateQueries({ queryKey: ["file-events", code] });
      void queryClient.invalidateQueries({ queryKey: ["room", code] });
    }
    navigate(location.pathname, { replace: true, state: null });
  }, [code, location.pathname, location.state, navigate, queryClient]);
  useEffect(() => {
    setChatLaunch((current) => current?.roomCode === code ? current : undefined);
    setFileLaunch((current) => current?.roomCode === code ? current : undefined);
  }, [code]);
  useEffect(() => {
    if (!chatLaunch || !snapshot.data || chatLaunch.roomCode !== code) return;
    const target = snapshot.data.members.find((member) => member.userId === chatLaunch.peerUserId && member.status === "active");
    if (target && target.userId !== session?.userId) return;
    setChatLaunch(undefined);
    message.warning(t("notification.unavailable"));
    void queryClient.invalidateQueries({ queryKey: ["notifications"] });
  }, [chatLaunch, code, queryClient, session?.userId, snapshot.data, t]);
  useEffect(() => {
    if (!joined || !session?.displayName) return;
    void queryClient.invalidateQueries({ queryKey: ["room", code] });
  }, [code, joined, queryClient, session?.displayName]);
  useEffect(() => { if (snapshot.data?.destroyAt) setDestroyAt(snapshot.data.destroyAt); }, [snapshot.data?.destroyAt]);
  useEffect(() => {
    const listener = (raw: Event) => {
      const event = (raw as CustomEvent<StreamEventMessage>).detail?.event;
      if (!event) return;
      const payload = (event.payload ?? {}) as Record<string, unknown>;
      if (payload.roomCode !== code) return;
      if (event.type === "room.member_kicked") setKicked(true);
      if (event.type === "room.member_joined" && typeof payload.userId === "string" && payload.userId !== session?.userId && typeof payload.displayName === "string") {
        message.info(t("room.memberJoinedNotice", { name: payload.displayName }));
      }
      if (event.type === "room.member_left" && payload.reason === "left" && typeof payload.displayName === "string") {
        message.info(t("room.memberLeftNotice", { name: payload.displayName }));
      }
      if (event.type === "room.destroying" && typeof payload.destroyAt === "number") setDestroyAt(payload.destroyAt);
      if (event.type === "room.destroyed") setDestroyAt(Math.floor(Date.now() / 1000));
      void queryClient.invalidateQueries({ queryKey: ["room", code] });
      void queryClient.invalidateQueries({ queryKey: ["room-join-info", code] });
    };
    window.addEventListener(streamEventName, listener);
    return () => window.removeEventListener(streamEventName, listener);
  }, [code, queryClient, session?.userId, t]);
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

  const resetAccount = () => {
    if (ownerRoom || !window.confirm(t("home.resetConfirm"))) return;
    void resetSession().then(() => {
      queryClient.removeQueries({ queryKey: ["session"] });
      queryClient.removeQueries({ queryKey: ["rooms"] });
      queryClient.removeQueries({ queryKey: ["notifications"] });
      setProfileOpen(false);
      navigate("/", { replace: true });
    }).catch(setActionError);
  };
  const profileModal = session ? <ProfileModal open={profileOpen} onClose={() => setProfileOpen(false)} session={session} ownerRoom={ownerRoom} onReset={resetAccount} /> : null;
  const renderGate = (children: ReactNode) => <RoomGate session={session} onOpenProfile={session ? () => setProfileOpen(true) : undefined}>{children}{profileModal}</RoomGate>;
  const loadingCard = (roomTitle?: string) => <RoomGateCard
    tone="info"
    icon={<LoaderCircle className="is-spinning" />}
    label={t("room.gate.loadingLabel")}
    title={t("room.gate.loadingTitle")}
    description={t("room.gate.loadingDescription")}
    roomCode={validCode ? code : undefined}
    roomTitle={roomTitle}
    live
  />;
  const unavailableCard = () => <RoomGateCard
    tone="unavailable"
    icon={<AlertTriangle />}
    label={t("room.gate.unavailableLabel")}
    title={t("room.gate.unavailableTitle")}
    description={t("room.gate.unavailableDescription")}
    roomCode={code || undefined}
  />;
  const transientErrorCard = (error: unknown, retry: () => void, roomInfo?: RoomJoinInfo) => <RoomGateCard
    tone="error"
    icon={<AlertTriangle />}
    label={t("room.gate.errorLabel")}
    title={t("room.gate.errorTitle")}
    description={t("room.gate.errorDescription")}
    roomCode={roomInfo?.roomCode ?? (validCode ? code : undefined)}
    roomTitle={roomInfo?.title}
    error={getApiErrorMessage(error, t)}
    actions={[{ key: "retry", label: t("room.gate.retry"), icon: <RefreshCw aria-hidden="true" />, onClick: retry }]}
  />;

  if (sessionQuery.isPending) return renderGate(loadingCard());
  if (!session) return renderGate(<RoomGateCard tone="info" icon={<UserRound />} label={t("room.gate.sessionLabel")} title={t("room.gate.sessionTitle")} description={t("room.gate.sessionDescription")} roomCode={validCode ? code : undefined} />);
  if (!validCode) return renderGate(unavailableCard());
  if (joinInfo.isError) {
    return renderGate(classifyRoomGateError(getApiErrorCode(joinInfo.error)) === "unavailable"
      ? unavailableCard()
      : transientErrorCard(joinInfo.error, () => { void joinInfo.refetch(); }));
  }
  if (joinInfo.isPending) return renderGate(loadingCard());
  if (!joined && joinInfo.data) return <JoinPanel info={joinInfo.data} session={session} onOpenProfile={() => setProfileOpen(true)} profileModal={profileModal} onJoin={(confirmed, pin) => join.mutateAsync({ confirmed, pin }).then(() => undefined)} onRequest={() => request.mutate()} onCancelRequest={() => cancelRequest.mutate()} loading={join.isPending || request.isPending || cancelRequest.isPending} error={join.isError ? join.error : request.isError ? request.error : cancelRequest.error} />;
  if (snapshot.isPending) return renderGate(loadingCard(joinInfo.data?.title));
  if (snapshot.isError || !snapshot.data) return renderGate(transientErrorCard(snapshot.error, () => { void snapshot.refetch(); }, joinInfo.data));
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
    chatLaunch={chatLaunch?.roomCode === code ? chatLaunch : undefined}
    fileLaunch={fileLaunch?.roomCode === code ? { token: fileLaunch.token, view: fileLaunch.view, requestId: fileLaunch.requestId } : undefined}
    onChatLaunchConsumed={consumeChatLaunch}
    onFileLaunchConsumed={consumeFileLaunch}
  />{profileModal}</>;
}

function RoomGate({ children, session, onOpenProfile }: { children: ReactNode; session?: Session; onOpenProfile?: () => void }) {
  return <AppShell variant="room" session={session} onOpenProfile={onOpenProfile}><div className="room-gate-page"><div className="room-gate-content">{children}</div></div></AppShell>;
}

function JoinPanel({ info, session, onOpenProfile, profileModal, onJoin, onRequest, onCancelRequest, loading, error }: { info: RoomJoinInfo; session: Session; onOpenProfile: () => void; profileModal: ReactNode; onJoin: (confirmed: boolean, pin?: string) => Promise<void>; onRequest: () => void; onCancelRequest: () => void; loading: boolean; error?: unknown }) {
  const { t } = useTranslation();
  const [pin, setPin] = useState(["", "", "", ""]);
  const pinSubmission = useRef(createPinSubmissionGate());
  const view = deriveJoinGateView(info.joinMode, info.pendingRequest);

  useEffect(() => {
    pinSubmission.current.reset();
    setPin(["", "", "", ""]);
  }, [info.roomCode]);

  const submitPin = useCallback((value: string) => {
    if (!pinSubmission.current.tryStart(value)) return;
    void onJoin(true, value).then(() => pinSubmission.current.succeed()).catch(() => pinSubmission.current.fail());
  }, [onJoin]);

  let tone: "primary" | "pin" | "waiting" = "primary";
  let icon: ReactNode = <Check />;
  let label: string = t("room.gate.joinLabel");
  let title: string = t("room.gate.joinTitle");
  let description: string = t("room.gate.joinDescription");
  let actions: RoomGateAction[] = [{ key: "join", label: t("room.confirm"), icon: <Check aria-hidden="true" />, loading, onClick: () => { void onJoin(true).catch(() => undefined); } }];

  if (view === "pin") {
    tone = "pin";
    icon = <KeyRound />;
    label = t("room.gate.pinLabel");
    title = t("room.gate.pinTitle");
    description = t("room.gate.pinDescription");
    actions = [];
  } else if (view === "approval_request") {
    icon = <Send />;
    label = t("room.gate.approvalLabel");
    title = t("room.gate.approvalTitle");
    description = t("room.gate.approvalDescription");
    actions = [{ key: "request", label: t("room.requestApproval"), icon: <Send aria-hidden="true" />, loading, onClick: onRequest }];
  } else if (view === "approval_waiting") {
    tone = "waiting";
    icon = <Clock3 />;
    label = t("room.gate.waitingLabel");
    title = t("room.gate.waitingTitle");
    description = t("room.gate.waitingDescription");
    actions = [{ key: "cancel", label: t("room.cancelRequest"), icon: <X aria-hidden="true" />, loading, onClick: onCancelRequest }];
  }

  return <RoomGate session={session} onOpenProfile={onOpenProfile}><RoomGateCard
    tone={tone}
    icon={icon}
    label={label}
    title={title}
    description={description}
    roomCode={info.roomCode}
    roomTitle={info.title}
    live={view === "approval_waiting"}
    error={error != null ? getApiErrorMessage(error, t) : undefined}
    actions={actions}
  >
    {view === "pin" && <div className="room-gate-pin">
      <PinInput
        id="join-pin"
        label={t("room.pin")}
        value={pin}
        onChange={setPin}
        onComplete={submitPin}
        onSubmit={submitPin}
        digitLabel={(position) => t("room.gate.pinDigitLabel", { position: String(position) })}
        disabled={loading}
        invalid={error != null}
        autoFocus
      />
      {loading && <span className="room-gate-pin-status" aria-live="polite"><LoaderCircle className="is-spinning" aria-hidden="true" />{t("room.gate.verifying")}</span>}
    </div>}
  </RoomGateCard>{profileModal}</RoomGate>;
}
