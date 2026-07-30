import { BellOutlined, CloseOutlined, DeleteOutlined, DownOutlined, EditOutlined, GlobalOutlined, LoginOutlined, PlusOutlined, ReloadOutlined, SettingOutlined } from "@ant-design/icons";
import { Alert, Avatar, Badge, Button, Divider, Drawer, Dropdown, Empty, Form, Input, Modal, Radio, Space, Tag, Tooltip, message } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import { AppFooter } from "../components/AppFooter";
import { RadarCanvas } from "../components/RadarCanvas";
import { isUnauthorized } from "../components/common";
import { useSessionQuery } from "../hooks/use-session";
import { getApiErrorMessage } from "../lib/api-error";
import {
  approveJoinRequest,
  createRoom,
  createSession,
  getJoinRequests,
  getRandomNickname,
  listRooms,
  rejectJoinRequest,
  resetSession,
  updateSession,
} from "../services/api";
import { normalizeLanguage } from "../i18n";
import type { JoinMode, JoinRequest, RoomSummary, Session } from "../types/domain";
import { getAvatarInitial, getStableAvatarColor } from "../utils/avatar";

type ModalKind = "create" | "join" | null;

function formatRegistrationDate(timestamp: number, language: string) {
  if (!timestamp) return "-";
  return new Intl.DateTimeFormat(language, { dateStyle: "medium" }).format(new Date(timestamp * 1000));
}

function ErrorAlert({ error }: { error: unknown }) {
  const { t } = useTranslation();
  return <Alert className="home-alert" type="error" showIcon message={getApiErrorMessage(error, t)} />;
}

function HomeLoading() {
  const { t } = useTranslation();
  return <div className="home-page"><div className="home-loading">{t("home.loading")}</div><AppFooter /></div>;
}

function LanguageMenuItem() {
  const { t, i18n } = useTranslation();
  const language = normalizeLanguage(i18n.resolvedLanguage ?? i18n.language) ?? "zh-CN";
  return {
    key: "language",
    icon: <GlobalOutlined />,
    label: language === "zh-CN" ? t("language.en") : t("language.zhCN"),
  };
}

function HomeHeader({ session, draftName, canNotify, pendingCount, onOpenProfile, onOpenNotifications }: { session?: Session; draftName: string; canNotify: boolean; pendingCount: number; onOpenProfile: () => void; onOpenNotifications: () => void }) {
  const { t, i18n } = useTranslation();
  const displayName = session?.displayName || draftName || "?";
  const menuItems = [
    ...(session ? [{ key: "profile", icon: <SettingOutlined />, label: t("home.profile") }] : []),
    LanguageMenuItem(),
  ];
  return (
    <header className="home-header">
      <div className="home-brand"><span className="home-brand-mark">FD</span><div><div className="home-brand-name">FileDock</div><div className="home-brand-subtitle">{t("brand.description")}</div></div></div>
      <div className="home-header-actions">
        <Badge count={session && canNotify ? pendingCount : 0} size="small" overflowCount={99}>
          <Button className="home-icon-button" type="text" shape="circle" icon={<BellOutlined />} aria-label={t("room.notifications")} disabled={!session || !canNotify} onClick={onOpenNotifications} />
        </Badge>
        <Dropdown menu={{ items: menuItems, onClick: ({ key }) => { if (key === "profile") onOpenProfile(); else void i18n.changeLanguage(normalizeLanguage(i18n.resolvedLanguage ?? i18n.language) === "zh-CN" ? "en" : "zh-CN"); } }} trigger={["click"]} placement="bottomRight">
          <button type="button" className="home-user-button" aria-label={t("home.profile")}>
            <Avatar size={34} style={{ backgroundColor: getStableAvatarColor(displayName) }}>{getAvatarInitial(displayName)}</Avatar>
            <span className="home-user-name">{displayName}</span><DownOutlined className="home-user-chevron" />
          </button>
        </Dropdown>
      </div>
    </header>
  );
}

function NicknamePanel({ name, setName, onRandomize, onSubmit, pending, error }: { name: string; setName: (value: string) => void; onRandomize: () => void; onSubmit: (event: FormEvent) => void; pending: boolean; error?: unknown }) {
  const { t } = useTranslation();
  return <section className="home-onboarding home-sidebar-card"><div className="home-eyebrow">{t("home.profile")}</div><h1>{t("home.welcome")}</h1><p>{t("home.intro")}</p><form className="home-name-form" onSubmit={onSubmit}><label htmlFor="home-display-name">{t("session.displayName")}</label><Space.Compact block><Input id="home-display-name" value={name} maxLength={20} placeholder={t("session.displayNamePlaceholder")} onChange={(event) => setName(event.target.value)} autoComplete="nickname" /><Button aria-label={t("session.randomize")} icon={<ReloadOutlined />} onClick={onRandomize} /></Space.Compact><span className="home-form-hint">{t("session.nameHint")}</span>{error ? <ErrorAlert error={error} /> : null}<Button type="primary" htmlType="submit" block loading={pending} disabled={!name.trim()} icon={<LoginOutlined />}>{t("session.create")}</Button></form></section>;
}

function RoomCard({ room, onEnter, onAction }: { room: RoomSummary; onEnter: () => void; onAction: (action: "enter" | "leave" | "dissolve") => void }) {
  const { t } = useTranslation();
  const destroying = room.status === "destroying";
  return <div className={`home-room-card${destroying ? " is-destroying" : ""}`} tabIndex={destroying ? -1 : 0} role="button" onClick={onEnter} onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); onEnter(); } }}><div className="home-room-code">{room.roomCode}</div><div className="home-room-role">{room.role === "owner" ? t("room.roleOwner") : t("room.roleMember")}</div>{destroying && <Tag color="gold">{t("room.statusDestroying")}</Tag>}<div className="home-room-actions" onClick={(event) => event.stopPropagation()}><Button size="small" type="primary" onClick={() => onAction("enter")}>{t("room.join")}</Button>{room.role === "owner" ? <Button size="small" danger icon={<DeleteOutlined />} onClick={() => onAction("dissolve")}>{t("room.dissolve")}</Button> : <Button size="small" onClick={() => onAction("leave")}>{t("room.leave")}</Button>}</div></div>;
}

function RoomSection({ ownerRoom, memberRooms, onCreate, onEnter, onAction }: { ownerRoom?: RoomSummary; memberRooms: RoomSummary[]; onCreate: () => void; onEnter: (room: RoomSummary) => void; onAction: (room: RoomSummary, action: "enter" | "leave" | "dissolve") => void }) {
  const { t } = useTranslation();
  return <section className="home-rooms-section home-sidebar-card"><div className="home-section-heading"><span>{t("home.rooms")}</span><Button type="text" size="small" icon={<PlusOutlined />} onClick={onCreate}>{t("room.create")}</Button></div><div className="home-owner-slot">{ownerRoom ? <RoomCard room={ownerRoom} onEnter={() => onEnter(ownerRoom)} onAction={(action) => onAction(ownerRoom, action)} /> : <button type="button" className="home-empty-owner" onClick={onCreate}><PlusOutlined /><span>{t("home.noRooms")}</span></button>}</div><Divider plain>{t("room.roleMember")}</Divider>{memberRooms.length === 0 ? <Empty className="home-empty-members" image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("home.noRooms")} /> : <div className="home-member-list">{memberRooms.map((room) => <RoomCard key={room.roomId} room={room} onEnter={() => onEnter(room)} onAction={(action) => onAction(room, action)} />)}</div>}</section>;
}

function CreateRoomModal({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: (roomCode: string) => void }) {
  const { t } = useTranslation();
  const [joinMode, setJoinMode] = useState<JoinMode>("open");
  const [pin, setPin] = useState("");
  const [pinConfirmation, setPinConfirmation] = useState("");
  const mutation = useMutation({ mutationFn: () => createRoom(joinMode, joinMode === "password" ? pin : "", joinMode === "password" ? pinConfirmation : ""), onSuccess: (room) => { onCreated(room.roomCode); setPin(""); setPinConfirmation(""); } });
  const pinMismatch = joinMode === "password" && pinConfirmation.length === 4 && pin !== pinConfirmation;
  return <Modal title={t("room.create")} open={open} onCancel={onClose} destroyOnClose okText={t("room.create")} confirmLoading={mutation.isPending} onOk={() => mutation.mutate()} okButtonProps={{ disabled: joinMode === "password" && (pin.length !== 4 || pinConfirmation.length !== 4 || pinMismatch) }}><Form layout="vertical"><Form.Item label={t("room.joinMode")}><Radio.Group value={joinMode} onChange={(event) => setJoinMode(event.target.value)} options={[{ value: "open", label: t("room.open") }, { value: "password", label: t("room.password") }, { value: "owner_approval", label: t("room.ownerApproval") }]} /></Form.Item>{joinMode === "password" && <><Form.Item label={t("room.pin")} required><Input.OTP length={4} value={pin} onChange={setPin} inputMode="numeric" /></Form.Item><Form.Item label={t("room.pinConfirmation")} required validateStatus={pinMismatch ? "error" : undefined} help={pinMismatch ? t("room.pinMismatch") : undefined}><Input.OTP length={4} value={pinConfirmation} onChange={setPinConfirmation} inputMode="numeric" /></Form.Item></>}{mutation.isError && <ErrorAlert error={mutation.error} />}</Form></Modal>;
}

function JoinRoomModal({ open, onClose, onJoined }: { open: boolean; onClose: () => void; onJoined: (roomCode: string) => void }) {
  const { t } = useTranslation();
  const [code, setCode] = useState("");
  return <Modal title={t("room.join")} open={open} onCancel={onClose} destroyOnClose okText={t("room.join")} okButtonProps={{ disabled: !/^\d{4}$/.test(code) }} onOk={() => onJoined(code)}><Form layout="vertical"><Form.Item label={t("room.code")} required><Input.OTP length={4} value={code} onChange={setCode} inputMode="numeric" /></Form.Item></Form></Modal>;
}

function ProfileModal({ open, onClose, session, ownerRoom, onReset }: { open: boolean; onClose: () => void; session: Session; ownerRoom?: RoomSummary; onReset: () => void }) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [name, setName] = useState(session.displayName);
  useEffect(() => { if (open) setName(session.displayName); }, [open, session.displayName]);
  const update = useMutation({ mutationFn: () => updateSession(name.trim()), onSuccess: (next) => { queryClient.setQueryData(["session"], next); message.success(t("home.saveName")); } });
  const resetDisabled = Boolean(ownerRoom && (ownerRoom.status === "active" || ownerRoom.status === "destroying"));
  return <Modal title={t("home.profile")} open={open} onCancel={onClose} footer={null} destroyOnClose><div className="home-profile"><Avatar size={72} style={{ backgroundColor: getStableAvatarColor(session.displayName), fontSize: 28 }}>{getAvatarInitial(session.displayName)}</Avatar><Form layout="vertical" className="home-profile-form"><Form.Item label={t("session.displayName")}><Space.Compact block><Input value={name} maxLength={20} onChange={(event) => setName(event.target.value)} /><Button type="primary" icon={<EditOutlined />} loading={update.isPending} disabled={!name.trim()} onClick={() => update.mutate()}>{t("home.saveName")}</Button></Space.Compact></Form.Item><dl className="home-profile-details"><dt>{t("home.registeredAt")}</dt><dd>{formatRegistrationDate(session.createdAt, i18n.language)}</dd></dl>{update.isError && <ErrorAlert error={update.error} />}<Tooltip title={resetDisabled ? t("home.resetBlocked") : undefined}><Button danger block icon={<DeleteOutlined />} disabled={resetDisabled} onClick={onReset}>{t("session.reset")}</Button></Tooltip></Form></div></Modal>;
}

function NotificationPanel({ open, onClose, roomCode, requests, loading, error, onApprove, onReject }: { open: boolean; onClose: () => void; roomCode?: string; requests: JoinRequest[]; loading: boolean; error?: unknown; onApprove: (request: JoinRequest) => void; onReject: (request: JoinRequest) => void }) {
  const { t } = useTranslation();
  return <Drawer title={t("room.pendingRequests")} placement="right" open={open} onClose={onClose} width={360}>{roomCode && <p className="home-notification-room">{t("room.code")} {roomCode}</p>}{loading && <p>{t("room.loading")}</p>}{error ? <ErrorAlert error={error} /> : null}{!loading && !error && requests.length === 0 && <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("room.noRequests")} />}{requests.map((request) => <div className="home-request-item" key={request.requestId}><strong>{request.displayName}</strong><span>{t("room.requestExpires", { time: new Intl.DateTimeFormat(undefined, { dateStyle: "short", timeStyle: "short" }).format(new Date(request.expiresAt * 1000)) })}</span><Space><Button size="small" type="primary" onClick={() => onApprove(request)}>{t("room.approve")}</Button><Button size="small" onClick={() => onReject(request)}>{t("room.reject")}</Button></Space></div>)}</Drawer>;
}

export function HomePage({ sessionQuery }: { sessionQuery: ReturnType<typeof useSessionQuery> }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [draftName, setDraftName] = useState("");
  const [modal, setModal] = useState<ModalKind>(null);
  const [profileOpen, setProfileOpen] = useState(false);
  const [notificationsOpen, setNotificationsOpen] = useState(false);
  const [selectedRoom, setSelectedRoom] = useState<RoomSummary>();
  const [mobileDrawerOpen, setMobileDrawerOpen] = useState(false);
  const session = sessionQuery.data;
  const roomsQuery = useQuery({ queryKey: ["rooms"], queryFn: listRooms, enabled: Boolean(session), retry: false });
  const nicknameQuery = useQuery({ queryKey: ["random-nickname"], queryFn: getRandomNickname, enabled: false, retry: false });
  const createSessionMutation = useMutation({ mutationFn: () => createSession(draftName.trim()), onSuccess: (next) => queryClient.setQueryData(["session"], next) });
  const ownerRoom = useMemo(() => roomsQuery.data?.items.find((room) => room.role === "owner"), [roomsQuery.data?.items]);
  const memberRooms = useMemo(() => roomsQuery.data?.items.filter((room) => room.role === "member"), [roomsQuery.data?.items]);
  const requestsQuery = useQuery({ queryKey: ["join-requests", "home", ownerRoom?.roomCode], queryFn: () => getJoinRequests(ownerRoom!.roomCode), enabled: Boolean(session && ownerRoom && ownerRoom.status === "active"), retry: false });
  const approvalMutation = useMutation({ mutationFn: ({ request, approve }: { request: JoinRequest; approve: boolean }) => approve ? approveJoinRequest(request.roomCode, request.requestId) : rejectJoinRequest(request.roomCode, request.requestId), onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ["join-requests"] }); void queryClient.invalidateQueries({ queryKey: ["rooms"] }); } });
  const resetMutation = useMutation({ mutationFn: resetSession, onSuccess: () => { queryClient.removeQueries({ queryKey: ["session"] }); queryClient.removeQueries({ queryKey: ["rooms"] }); setProfileOpen(false); } });

  useEffect(() => {
    if (!session && !draftName) void nicknameQuery.refetch().then((result) => { if (result.data?.displayName) setDraftName(result.data.displayName); });
  }, [draftName, nicknameQuery, session]);

  if (sessionQuery.isPending) return <HomeLoading />;
  if (sessionQuery.error && !isUnauthorized(sessionQuery.error) && !session) return <div className="home-page"><div className="home-centered-error"><ErrorAlert error={sessionQuery.error} /><Button onClick={() => void sessionQuery.refetch()}>{t("home.retry")}</Button></div><AppFooter /></div>;

  const openRoom = (room: RoomSummary) => {
    if (room.status === "destroying") return;
    if (window.matchMedia("(hover: none)").matches) { setSelectedRoom(room); setMobileDrawerOpen(true); return; }
    navigate(`/rooms/${room.roomCode}`);
  };
  const runRoomAction = (room: RoomSummary, action: "enter" | "leave" | "dissolve") => {
    if (room.status === "destroying") return;
    if (action === "enter") { navigate(`/rooms/${room.roomCode}`); return; }
    const dangerous = action === "dissolve" ? t("room.dissolveConfirm") : t("room.leaveConfirm");
    Modal.confirm({ title: dangerous, okText: t("room.confirm"), cancelText: t("room.backHome"), okButtonProps: action === "dissolve" ? { danger: true } : undefined, onOk: async () => { if (action === "dissolve") { const { dissolveRoom } = await import("../services/api"); await dissolveRoom(room.roomCode); } else { const { leaveRoom } = await import("../services/api"); await leaveRoom(room.roomCode); } void queryClient.invalidateQueries({ queryKey: ["rooms"] }); } });
  };
  const resetAccount = () => Modal.confirm({ title: t("home.resetConfirm"), okText: t("session.reset"), cancelText: t("room.backHome"), okButtonProps: { danger: true }, onOk: () => resetMutation.mutateAsync() });
  const submitName = (event: FormEvent) => { event.preventDefault(); if (draftName.trim()) createSessionMutation.mutate(); };

  return <div className="home-page"><HomeHeader session={session} draftName={draftName} canNotify={Boolean(ownerRoom && ownerRoom.status === "active")} pendingCount={requestsQuery.data?.pendingCount ?? 0} onOpenProfile={() => setProfileOpen(true)} onOpenNotifications={() => setNotificationsOpen(true)} /><main className="home-layout"><section className="home-radar-panel"><RadarCanvas displayName={session?.displayName ?? draftName} /></section><aside className="home-sidebar">{session ? <><div className="home-primary-actions"><Button type="primary" size="large" icon={<PlusOutlined />} onClick={() => setModal("create")}>{t("room.create")}</Button><Button size="large" icon={<LoginOutlined />} onClick={() => setModal("join")}>{t("room.join")}</Button></div>{roomsQuery.isError && <ErrorAlert error={roomsQuery.error} />}{roomsQuery.isPending ? <div className="home-sidebar-card">{t("home.loading")}</div> : <RoomSection ownerRoom={ownerRoom} memberRooms={memberRooms ?? []} onCreate={() => setModal("create")} onEnter={openRoom} onAction={runRoomAction} />}</> : <NicknamePanel name={draftName} setName={setDraftName} onRandomize={() => void nicknameQuery.refetch().then((result) => { if (result.data?.displayName) setDraftName(result.data.displayName); })} onSubmit={submitName} pending={createSessionMutation.isPending} error={createSessionMutation.error} />}</aside></main><AppFooter /><CreateRoomModal open={modal === "create"} onClose={() => setModal(null)} onCreated={(roomCode) => { setModal(null); void queryClient.invalidateQueries({ queryKey: ["rooms"] }); navigate(`/rooms/${roomCode}`); }} /><JoinRoomModal open={modal === "join"} onClose={() => setModal(null)} onJoined={(roomCode) => navigate(`/rooms/${roomCode}`)} />{session && <ProfileModal open={profileOpen} onClose={() => setProfileOpen(false)} session={session} ownerRoom={ownerRoom} onReset={resetAccount} />}{session && <NotificationPanel open={notificationsOpen} onClose={() => setNotificationsOpen(false)} roomCode={ownerRoom?.roomCode} requests={requestsQuery.data?.items ?? []} loading={requestsQuery.isPending} error={requestsQuery.error} onApprove={(request) => approvalMutation.mutate({ request, approve: true })} onReject={(request) => approvalMutation.mutate({ request, approve: false })} />}{selectedRoom && <Drawer title={`${t("room.code")} ${selectedRoom.roomCode}`} placement="bottom" height="auto" open={mobileDrawerOpen} onClose={() => setMobileDrawerOpen(false)} closeIcon={<CloseOutlined />}><Space direction="vertical" className="home-mobile-actions" size="middle"><Button block type="primary" onClick={() => runRoomAction(selectedRoom, "enter")}>{t("room.join")}</Button>{selectedRoom.role === "owner" ? <Button block danger onClick={() => runRoomAction(selectedRoom, "dissolve")}>{t("room.dissolve")}</Button> : <Button block onClick={() => runRoomAction(selectedRoom, "leave")}>{t("room.leave")}</Button>}</Space></Drawer>}</div>;
}
