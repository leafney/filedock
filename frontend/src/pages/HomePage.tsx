import { CloseOutlined, DeleteOutlined, LoginOutlined, PlusOutlined } from "@ant-design/icons";
import { Alert, Button, Divider, Drawer, Empty, Form, Input, Modal, Radio, Space, Tag, Tooltip } from "antd";
import type { OTPRef } from "antd/es/input/OTP";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Dices } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import { AppShell } from "../components/AppShell";
import { ProfileModal } from "../components/ProfileModal";
import { RadarCanvas } from "../components/RadarCanvas";
import { isUnauthorized } from "../components/common";
import { useSessionQuery } from "../hooks/use-session";
import { getApiErrorMessage } from "../lib/api-error";
import { createPinSubmissionGate, normalizePin } from "../utils/room-gate";
import {
  createRoom,
  createSession,
  getRandomNickname,
  listRooms,
  leaveRoom,
  dissolveRoom,
  resetSession,
  updateSession,
} from "../services/api";
import type { JoinMode, RoomSummary } from "../types/domain";

type ModalKind = "create" | "join" | null;

function ErrorAlert({ error }: { error: unknown }) {
  const { t } = useTranslation();
  return <Alert className="home-alert" type="error" showIcon title={getApiErrorMessage(error, t)} />;
}

function HomeLoading() {
  const { t } = useTranslation();
  return <AppShell variant="home"><div className="home-loading">{t("home.loading")}</div></AppShell>;
}

function NicknamePanel({ name, setName, onRandomize, onSubmit, pending, error }: { name: string; setName: (value: string) => void; onRandomize: () => void; onSubmit: (event: FormEvent) => void; pending: boolean; error?: unknown }) {
  const { t } = useTranslation();
  return <section className="home-onboarding home-sidebar-card"><div className="home-eyebrow">{t("home.identitySetup")}</div><h1>{t("home.welcome")}</h1><p>{t("home.intro")}</p><form className="home-name-form" onSubmit={onSubmit}><label htmlFor="home-display-name">{t("session.displayName")}</label><Space.Compact block><Input id="home-display-name" value={name} maxLength={20} placeholder={t("session.displayNamePlaceholder")} onChange={(event) => setName(event.target.value)} autoComplete="nickname" /><Button aria-label={t("session.randomize")} icon={<Dices aria-hidden="true" size={16} />} onClick={onRandomize} /></Space.Compact><span className="home-form-hint">{t("session.nameHint")}</span>{error ? <ErrorAlert error={error} /> : null}<Button type="primary" htmlType="submit" block loading={pending} disabled={!name.trim()} icon={<LoginOutlined />}>{t("session.create")}</Button></form></section>;
}

function RoomCard({ room, onEnter, onAction }: { room: RoomSummary; onEnter: () => void; onAction: (action: "enter" | "leave" | "dissolve") => void }) {
  const { t } = useTranslation();
  const destroying = room.status === "destroying";
  return <div className={`home-room-card${destroying ? " is-destroying" : ""}`} tabIndex={destroying ? -1 : 0} role="button" onClick={onEnter} onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); onEnter(); } }}><div className="home-room-code">{room.roomCode}</div><Tooltip title={room.title} mouseEnterDelay={0.4}><div className="home-room-title">{room.title}</div></Tooltip>{destroying && <Tag color="gold">{t("room.statusDestroying")}</Tag>}<div className="home-room-actions" onClick={(event) => event.stopPropagation()}><Button size="small" type="primary" onClick={() => onAction("enter")}>{t("room.enter")}</Button>{room.role === "owner" ? <Button size="small" danger icon={<DeleteOutlined />} onClick={() => onAction("dissolve")}>{t("room.dissolve")}</Button> : <Button size="small" onClick={() => onAction("leave")}>{t("room.leave")}</Button>}</div></div>;
}

function RoomSection({ ownerRoom, memberRooms, onCreate, onJoin, onEnter, onAction }: { ownerRoom?: RoomSummary; memberRooms: RoomSummary[]; onCreate: () => void; onJoin: () => void; onEnter: (room: RoomSummary) => void; onAction: (room: RoomSummary, action: "enter" | "leave" | "dissolve") => void }) {
  const { t } = useTranslation();
  return <section className="home-rooms-section home-sidebar-card"><div className="home-section-heading"><span>{t("home.rooms")}</span></div><div className="home-owner-slot">{ownerRoom ? <RoomCard room={ownerRoom} onEnter={() => onEnter(ownerRoom)} onAction={(action) => onAction(ownerRoom, action)} /> : <button type="button" className="home-empty-owner" onClick={onCreate}><PlusOutlined /><span>{t("home.noRooms")}</span></button>}</div><Divider /><div className="home-section-heading home-participating-heading"><span>{t("room.participatingRooms")}</span><Button type="link" size="small" icon={<LoginOutlined />} onClick={onJoin}>{t("room.join")}</Button></div>{memberRooms.length === 0 ? <Empty className="home-empty-members" image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("home.noParticipatingRooms")} /> : <div className="home-member-list">{memberRooms.map((room) => <RoomCard key={room.roomId} room={room} onEnter={() => onEnter(room)} onAction={(action) => onAction(room, action)} />)}</div>}</section>;
}

function CreateRoomModal({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: (roomCode: string) => void }) {
  const { t } = useTranslation();
  const [joinMode, setJoinMode] = useState<JoinMode>("open");
  const [pin, setPin] = useState("");
  const [pinConfirmation, setPinConfirmation] = useState("");
  const mutation = useMutation({ mutationFn: () => createRoom(joinMode, joinMode === "password" ? pin : "", joinMode === "password" ? pinConfirmation : ""), onSuccess: (room) => { onCreated(room.roomCode); setPin(""); setPinConfirmation(""); } });
  const pinMismatch = joinMode === "password" && pinConfirmation.length === 4 && pin !== pinConfirmation;
  return <Modal title={t("room.create")} open={open} onCancel={onClose} destroyOnHidden cancelText={t("room.backHome")} okText={t("room.create")} confirmLoading={mutation.isPending} onOk={() => mutation.mutate()} okButtonProps={{ disabled: joinMode === "password" && (pin.length !== 4 || pinConfirmation.length !== 4 || pinMismatch) }}><Form layout="vertical"><Form.Item label={t("room.joinMode")}><Radio.Group value={joinMode} onChange={(event) => setJoinMode(event.target.value)} options={[{ value: "open", label: t("room.open") }, { value: "password", label: t("room.password") }, { value: "owner_approval", label: t("room.ownerApproval") }]} /></Form.Item>{joinMode === "password" && <><Form.Item label={t("room.pin")} required><Input.OTP length={4} value={pin} onChange={setPin} inputMode="numeric" /></Form.Item><Form.Item label={t("room.pinConfirmation")} required validateStatus={pinMismatch ? "error" : undefined} help={pinMismatch ? t("room.pinMismatch") : undefined}><Input.OTP length={4} value={pinConfirmation} onChange={setPinConfirmation} inputMode="numeric" /></Form.Item></>}{mutation.isError && <ErrorAlert error={mutation.error} />}</Form></Modal>;
}

function JoinRoomModal({ open, onClose, onJoined }: { open: boolean; onClose: () => void; onJoined: (roomCode: string) => void }) {
  const { t } = useTranslation();
  const [code, setCode] = useState("");
  const inputRef = useRef<OTPRef>(null);
  const submissionGate = useRef(createPinSubmissionGate());
  const submitCode = useCallback((value: string) => {
    const normalized = normalizePin(value);
    if (!submissionGate.current.tryStart(normalized)) return;
    try {
      onJoined(normalized);
      submissionGate.current.succeed();
    } catch {
      submissionGate.current.fail();
    }
  }, [onJoined]);
  useEffect(() => {
    if (!open) return;
    setCode("");
    submissionGate.current.reset();
    const frame = window.requestAnimationFrame(() => inputRef.current?.focus());
    return () => window.cancelAnimationFrame(frame);
  }, [open]);
  useEffect(() => {
    if (/^\d{4}$/.test(code)) submitCode(code);
  }, [code, submitCode]);
  const handleClose = () => {
    setCode("");
    submissionGate.current.reset();
    onClose();
  };
  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "Enter" || !/^\d{4}$/.test(code)) return;
    event.preventDefault();
    event.stopPropagation();
    submitCode(code);
  };
  return <Modal className="join-room-modal" title={t("room.join")} open={open} onCancel={handleClose} destroyOnHidden footer={null} width={360}><div className="join-room-modal-content" onKeyDown={handleKeyDown}><Input.OTP ref={inputRef} aria-label={t("room.code")} length={4} value={code} onInput={(values) => setCode(normalizePin(values.join("")))} inputMode="numeric" type="tel" autoComplete="one-time-code" /></div></Modal>;
}

export function HomePage({ sessionQuery }: { sessionQuery: ReturnType<typeof useSessionQuery> }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [draftName, setDraftName] = useState("");
  const [modal, setModal] = useState<ModalKind>(null);
  const [profileOpen, setProfileOpen] = useState(false);
  const [selectedRoom, setSelectedRoom] = useState<RoomSummary>();
  const [mobileDrawerOpen, setMobileDrawerOpen] = useState(false);
  const session = sessionQuery.error && isUnauthorized(sessionQuery.error) ? undefined : sessionQuery.data;
  const roomsQuery = useQuery({ queryKey: ["rooms"], queryFn: listRooms, enabled: Boolean(session), retry: false });
  const nicknameQuery = useQuery({ queryKey: ["random-nickname"], queryFn: getRandomNickname, enabled: false, retry: false });
  const createSessionMutation = useMutation({ mutationFn: () => createSession(draftName.trim()), onSuccess: (next) => queryClient.setQueryData(["session"], next) });
  const ownerRoom = useMemo(() => roomsQuery.data?.items.find((room) => room.role === "owner"), [roomsQuery.data?.items]);
  const memberRooms = useMemo(() => roomsQuery.data?.items.filter((room) => room.role === "member"), [roomsQuery.data?.items]);
  const resetMutation = useMutation({ mutationFn: resetSession, onSuccess: () => { queryClient.removeQueries({ queryKey: ["session"] }); queryClient.removeQueries({ queryKey: ["rooms"] }); queryClient.removeQueries({ queryKey: ["notifications"] }); setProfileOpen(false); } });

  useEffect(() => {
    if (!session && !draftName) void nicknameQuery.refetch().then((result) => { if (result.data?.displayName) setDraftName(result.data.displayName); });
  }, [draftName, nicknameQuery, session]);

  if (sessionQuery.isPending) return <HomeLoading />;
  if (sessionQuery.error && !isUnauthorized(sessionQuery.error) && !session) return <AppShell variant="home"><div className="home-centered-error"><ErrorAlert error={sessionQuery.error} /><Button onClick={() => void sessionQuery.refetch()}>{t("home.retry")}</Button></div></AppShell>;

  const openRoom = (room: RoomSummary) => {
    if (room.status === "destroying") return;
    if (window.matchMedia("(hover: none)").matches) { setSelectedRoom(room); setMobileDrawerOpen(true); return; }
    navigate(`/rooms/${room.roomCode}`);
  };
  const runRoomAction = (room: RoomSummary, action: "enter" | "leave" | "dissolve") => {
    if (room.status === "destroying") return;
    if (action === "enter") { navigate(`/rooms/${room.roomCode}`); return; }
    const dangerous = action === "dissolve" ? t("room.dissolveConfirm") : t("room.leaveConfirm");
    Modal.confirm({ title: dangerous, okText: t("room.confirm"), cancelText: t("room.backHome"), okButtonProps: action === "dissolve" ? { danger: true } : undefined, onOk: async () => { if (action === "dissolve") await dissolveRoom(room.roomCode); else await leaveRoom(room.roomCode); void queryClient.invalidateQueries({ queryKey: ["rooms"] }); } });
  };
  const resetAccount = () => Modal.confirm({ title: t("home.resetConfirm"), okText: t("session.reset"), cancelText: t("room.backHome"), okButtonProps: { danger: true }, onOk: () => resetMutation.mutateAsync() });
  const submitName = (event: FormEvent) => { event.preventDefault(); if (draftName.trim()) createSessionMutation.mutate(); };

  return <AppShell variant="home" session={session} fallbackName={draftName} onOpenProfile={() => setProfileOpen(true)}><div className="home-layout"><aside className="home-sidebar">{session ? <>{roomsQuery.isError && <ErrorAlert error={roomsQuery.error} />}{roomsQuery.isPending ? <div className="home-sidebar-card">{t("home.loading")}</div> : <RoomSection ownerRoom={ownerRoom} memberRooms={memberRooms ?? []} onCreate={() => setModal("create")} onJoin={() => setModal("join")} onEnter={openRoom} onAction={runRoomAction} />}</> : <NicknamePanel name={draftName} setName={setDraftName} onRandomize={() => void nicknameQuery.refetch().then((result) => { if (result.data?.displayName) setDraftName(result.data.displayName); })} onSubmit={submitName} pending={createSessionMutation.isPending} error={createSessionMutation.error} />}</aside><section className="home-radar-panel"><RadarCanvas displayName={session?.displayName ?? draftName} /></section></div><CreateRoomModal open={modal === "create"} onClose={() => setModal(null)} onCreated={(roomCode) => { setModal(null); void queryClient.invalidateQueries({ queryKey: ["rooms"] }); navigate(`/rooms/${roomCode}`); }} /><JoinRoomModal open={modal === "join"} onClose={() => setModal(null)} onJoined={(roomCode) => navigate(`/rooms/${roomCode}`)} />{session && <ProfileModal open={profileOpen} onClose={() => setProfileOpen(false)} session={session} ownerRoom={ownerRoom} onReset={resetAccount} />}{selectedRoom && <Drawer title={`${t("room.code")} ${selectedRoom.roomCode}`} placement="bottom" height="auto" open={mobileDrawerOpen} onClose={() => setMobileDrawerOpen(false)} closeIcon={<CloseOutlined />}><Space direction="vertical" className="home-mobile-actions" size="middle"><Button block type="primary" onClick={() => runRoomAction(selectedRoom, "enter")}>{t("room.enter")}</Button>{selectedRoom.role === "owner" ? <Button block danger onClick={() => runRoomAction(selectedRoom, "dissolve")}>{t("room.dissolve")}</Button> : <Button block onClick={() => runRoomAction(selectedRoom, "leave")}>{t("room.leave")}</Button>}</Space></Drawer>}</AppShell>;
}
