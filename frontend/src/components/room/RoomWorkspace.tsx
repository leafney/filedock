import { useQuery } from "@tanstack/react-query";
import { Modal, Tooltip } from "antd";
import { Copy, Crown, DoorOpen, MessageSquare, RefreshCw, Users, X } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { AppShell } from "../AppShell";
import { ErrorNotice } from "../common";
import { FileWorkspace } from "./FileWorkspace";
import { ChatWorkspace } from "./ChatWorkspace";
import { TransferBar } from "./TransferBar";
import { getRoomQRCode } from "../../services/api";
import type { RoomMember, RoomSnapshot, Session } from "../../types/domain";
import { durationParts, formatBytes } from "../../utils/format";
import { useDialogFocus } from "../../hooks/use-dialog-focus";
import { copyText } from "../../utils/clipboard";
import { roomCapacitySummary } from "../../utils/room-capacity";

interface Props {
  room: RoomSnapshot;
  code: string;
  session: Session;
  destroyAt?: number;
  kicked: boolean;
  setKicked: (value: boolean) => void;
  onExtend: () => void;
  onLeave: () => void;
  onDissolve: () => void;
  onKick: (userId: string) => void;
  onOpenProfile: () => void;
  actionError?: unknown;
  chatLaunch?: { peerUserId: string; token: string };
  fileLaunch?: { token: string; view: "list" | "timeline" | "trash"; requestId?: string };
  onChatLaunchConsumed: (token: string) => void;
  onFileLaunchConsumed: (token: string) => void;
}

export function RoomWorkspace(props: Props) {
  const { t } = useTranslation();
  const [now, setNow] = useState(() => Math.floor(Date.now() / 1000));
  const [membersOpen, setMembersOpen] = useState(false);
  const [shareOpen, setShareOpen] = useState(false);
  const [capacityOpen, setCapacityOpen] = useState(false);
  const [chatTarget, setChatTarget] = useState<string>();
  const [mobileChatLaunch, setMobileChatLaunch] = useState<Props["chatLaunch"]>();
  const [chatMobileOpen, setChatMobileOpen] = useState(false);
  const [chatUnread, setChatUnread] = useState(0);
  const [chatUnreadByPeer, setChatUnreadByPeer] = useState<Record<string, number>>({});
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Math.floor(Date.now() / 1000)), 1000);
    return () => window.clearInterval(timer);
  }, []);
  useEffect(() => {
    if (chatTarget && !props.room.members.some((member) => member.userId === chatTarget && member.status === "active")) setChatTarget(undefined);
  }, [chatTarget, props.room.members]);
  useEffect(() => {
    if (!props.chatLaunch) return;
    const member = props.room.members.find((item) => item.userId === props.chatLaunch?.peerUserId && item.status === "active");
    if (!member || member.userId === props.session.userId) return;
    setChatTarget(member.userId);
    if (window.matchMedia("(max-width: 760px)").matches) {
      setMobileChatLaunch(props.chatLaunch);
      setChatMobileOpen(true);
    } else {
      setMobileChatLaunch(undefined);
    }
    props.onChatLaunchConsumed(props.chatLaunch.token);
  }, [props.chatLaunch, props.onChatLaunchConsumed, props.room.members, props.session.userId]);
  useEffect(() => {
    if (props.fileLaunch) setChatMobileOpen(false);
  }, [props.fileLaunch?.token]);
  const expires = Math.max(0, props.room.expiresAt - now);
  const duration = durationParts(expires);
  const countdown = props.destroyAt ? Math.max(0, Math.ceil(props.destroyAt - now)) : 0;
  const capacity = roomCapacitySummary(props.room.capacity, props.room.role);
  return <AppShell
    variant="room"
    session={props.session}
    onOpenProfile={props.onOpenProfile}
    onShare={() => setShareOpen(true)}
    headerCenter={<div className="room-topbar-summary">
      <Summary label={t("room.members")} value={t("room.workspace.people", { count: props.room.members.length })} />
      <Summary label={t("room.workspace.remaining")} value={t("room.workspace.duration", { hours: String(duration.hours), minutes: String(duration.minutes), seconds: String(duration.seconds) })} />
      <button className="room-capacity-summary" type="button" onClick={() => setCapacityOpen(true)}>
        <span>{t("room.workspace.capacity")}</span><strong title={`${formatBytes(capacity.occupiedBytes)} / ${formatBytes(props.room.capacity.capacityBytes)}`}>{formatBytes(capacity.occupiedBytes)} / {formatBytes(props.room.capacity.capacityBytes)}</strong>
        <i><i style={{ width: `${capacity.percent}%` }} /></i>
      </button>
    </div>}
    headerActions={<><button className="room-icon-button room-chat-mobile-trigger" type="button" aria-label={t("chat.openMobile")} onClick={() => setChatMobileOpen(true)}><MessageSquare aria-hidden="true" />{chatUnread > 0 && <b>{chatUnread > 99 ? "99+" : chatUnread}</b>}</button><button className="room-icon-button room-members-trigger" type="button" aria-label={t("room.workspace.openMembers")} onClick={() => setMembersOpen(true)}><Users aria-hidden="true" /></button></>}
    roomActions={props.room.role === "owner" ? { role: "owner", canExtend: props.room.canExtend, onExtend: props.onExtend, onDissolve: props.onDissolve } : { role: "member", onLeave: props.onLeave }}
    footerOverlay={<TransferBar roomCode={props.code} />}
  >
    <div className="room-workspace-content">
      <div className="room-workspace-layout">
        <MemberPanel room={props.room} session={props.session} onKick={props.onKick} onChat={setChatTarget} selectedUserId={chatTarget} unreadByUser={chatUnreadByPeer} />
        <FileWorkspace code={props.code} members={props.room.members} selfId={props.session.userId} fileLaunch={props.fileLaunch} onFileLaunchConsumed={props.onFileLaunchConsumed} />
        <ChatWorkspace roomId={props.room.roomId} code={props.code} members={props.room.members} selfId={props.session.userId} mode="desktop" selectedPeerUserId={chatTarget} onCloseConversation={() => setChatTarget(undefined)} onUnreadCount={setChatUnread} onUnreadByPeer={setChatUnreadByPeer} />
      </div>

      {membersOpen && <Overlay title={t("room.members")} onClose={() => setMembersOpen(false)}><MemberPanel room={props.room} session={props.session} onKick={props.onKick} drawer /></Overlay>}
      <ShareRoomPanel open={shareOpen} room={props.room} onClose={() => setShareOpen(false)} />
      <Modal className="room-capacity-modal" title={t("room.workspace.capacityDetails")} open={capacityOpen} onCancel={() => setCapacityOpen(false)} footer={null} destroyOnHidden width={460}>
        <CapacityPanel room={props.room} />
      </Modal>
      {chatMobileOpen && <div className="chat-mobile-overlay"><section><header><strong>{t("chat.conversations")}</strong><button type="button" aria-label={t("chat.closeMobile")} onClick={() => setChatMobileOpen(false)}><X aria-hidden="true" /></button></header><ChatWorkspace roomId={props.room.roomId} code={props.code} members={props.room.members} selfId={props.session.userId} mode="mobile" launchPeerUserId={mobileChatLaunch?.peerUserId} launchToken={mobileChatLaunch?.token} onLaunchConsumed={(token) => setMobileChatLaunch((current) => current?.token === token ? undefined : current)} onUnreadCount={setChatUnread} /></section></div>}
      {props.actionError != null && <div className="room-floating-error"><ErrorNotice error={props.actionError} /></div>}
      {(props.destroyAt || props.room.status === "destroying") && <div className="room-blocking-state"><RefreshCw aria-hidden="true" /><h2>{t("room.destroyingTitle")}</h2><p>{t("room.destroyingHint")}</p><strong>{t("room.destroyCountdown", { seconds: String(countdown) })}</strong></div>}
      {props.kicked && <div className="room-blocking-state"><DoorOpen aria-hidden="true" /><h2>{t("room.kickedTitle")}</h2><p>{t("room.kicked")}</p><button type="button" onClick={() => { props.setKicked(false); window.location.replace("/"); }}>{t("room.confirmOnly")}</button></div>}
    </div>
  </AppShell>;
}

function Summary({ label, value }: { label: string; value: string }) { return <div><span>{label}</span><strong title={value}>{value}</strong></div>; }

function MemberPanel({ room, session, onKick, onChat, selectedUserId, unreadByUser = {}, drawer = false }: { room: RoomSnapshot; session: Session; onKick: (id: string) => void; onChat?: (id: string) => void; selectedUserId?: string; unreadByUser?: Record<string, number>; drawer?: boolean }) {
  const { t } = useTranslation();
  const [copyFeedback, setCopyFeedback] = useState<{ kind: "success" | "error" }>();
  useEffect(() => {
    if (!copyFeedback) return;
    const timer = window.setTimeout(() => setCopyFeedback(undefined), 1500);
    return () => window.clearTimeout(timer);
  }, [copyFeedback]);
  const copyCode = async () => {
    setCopyFeedback(undefined);
    setCopyFeedback({ kind: await copyText(room.roomCode) ? "success" : "error" });
  };
  return <aside className={drawer ? "room-member-panel is-drawer" : "room-member-panel"}>
    <div className="room-panel-heading">
      <div className="room-panel-room-info">
        <strong title={room.title}>{room.title}</strong>
        <Tooltip title={copyFeedback ? t(copyFeedback.kind === "success" ? "room.workspace.copied" : "room.workspace.copyFailed") : undefined} open={Boolean(copyFeedback)} placement="top">
          <button type="button" onClick={() => void copyCode()}><span>{t("room.code")} {room.roomCode}</span><Copy aria-hidden="true" /></button>
        </Tooltip>
      </div>
    </div>
    <div className="room-member-list">{room.members.map((member) => <MemberItem key={member.userId} member={member} self={member.userId === session.userId} selected={member.userId === selectedUserId} unreadCount={unreadByUser[member.userId] ?? 0} canKick={room.role === "owner" && member.role !== "owner"} onKick={onKick} onChat={onChat} />)}</div>
  </aside>;
}

function MemberItem({ member, self, selected, unreadCount, canKick, onKick, onChat }: { member: RoomMember; self: boolean; selected: boolean; unreadCount: number; canKick: boolean; onKick: (id: string) => void; onChat?: (id: string) => void }) {
  const { t } = useTranslation();
  const ownerLabel = t("room.workspace.owner");
  return <article className={`room-member-item ${!self && member.status === "active" ? "is-chatable" : ""} ${selected ? "is-selected" : ""}`}><button className="room-member-chat-hit" type="button" disabled={self || member.status !== "active"} aria-current={selected ? "true" : undefined} aria-label={!self ? t("chat.openConversation", { name: member.displayName }) : undefined} onClick={() => { if (!self && member.status === "active") onChat?.(member.userId); }}><span className="room-avatar-wrap" title={member.role === "owner" ? ownerLabel : undefined} aria-label={member.role === "owner" ? ownerLabel : undefined}><span className="room-avatar">{member.displayName.slice(0, 1)}</span>{member.role === "owner" && <span className="room-owner-corner"><Crown aria-hidden="true" /></span>}{unreadCount > 0 && <span className="room-member-unread" role="status" aria-label={t("chat.unreadMessages", { name: member.displayName, count: unreadCount })}>{unreadCount > 99 ? "99+" : unreadCount}</span>}</span><div><strong>{member.displayName}{self && <small>{t("room.workspace.me")}</small>}</strong><span><i className={`presence ${member.onlineStatus}`} />{member.onlineStatus === "online" ? t("room.online") : member.onlineStatus === "away" ? t("room.away") : t("room.offline")}</span></div></button>{canKick && <button type="button" aria-label={t("room.kick")} onClick={() => { if (window.confirm(t("room.kickConfirm", { name: member.displayName }))) onKick(member.userId); }}><X aria-hidden="true" /></button>}</article>;
}

function Overlay({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  const { t } = useTranslation();
  const panelRef = useDialogFocus(onClose);
  return <div className="room-overlay" role="dialog" aria-modal="true" aria-label={title}><section ref={panelRef}><header><h2>{title}</h2><button type="button" aria-label={t("room.workspace.close")} onClick={onClose}><X aria-hidden="true" /></button></header>{children}</section></div>;
}

function ShareRoomPanel({ open, room, onClose }: { open: boolean; room: RoomSnapshot; onClose: () => void }) {
  const { t } = useTranslation();
  const qr = useQuery({ queryKey: ["room-qrcode", room.roomCode], queryFn: () => getRoomQRCode(room.roomCode), retry: false, enabled: open });
  const [copied, setCopied] = useState<string>();
  const [copyError, setCopyError] = useState(false);
  const link = `${window.location.origin}/rooms/${room.roomCode}`;
  const invite = t("room.workspace.inviteTemplate", { title: room.title, code: room.roomCode, link });
  const copy = async (kind: string, value: string) => {
    setCopyError(false);
    if (!await copyText(value)) { setCopyError(true); return; }
    setCopied(kind);
    window.setTimeout(() => setCopied(undefined), 1500);
  };
  return <Modal className="room-share-modal" title={t("room.workspace.shareRoom")} open={open} onCancel={onClose} footer={null} destroyOnHidden width={460}>
    <div className="room-share-panel">
      <strong className="room-share-title" title={room.title}>{room.title}</strong>
      <span className="room-share-code">{t("room.code")} <b>{room.roomCode}</b></span>
      {qr.data?.svg ? <div className="room-qr" dangerouslySetInnerHTML={{ __html: qr.data.svg }} /> : <p className="room-modal-message">{qr.isError ? t("room.qrcodeUnavailable") : t("room.loading")}</p>}
      <div className="room-share-actions">
        <button type="button" onClick={() => void copy("code", room.roomCode)}>{copied === "code" ? t("room.workspace.copied") : t("room.workspace.copyRoomCode")}</button>
        <button type="button" onClick={() => void copy("link", link)}>{copied === "link" ? t("room.workspace.copied") : t("room.workspace.copyLink")}</button>
        <button type="button" onClick={() => void copy("invite", invite)}>{copied === "invite" ? t("room.workspace.copied") : t("room.workspace.copyInvite")}</button>
      </div>
      {copyError && <p className="room-modal-message">{t("room.workspace.copyFailed")}</p>}
    </div>
  </Modal>;
}

function CapacityPanel({ room }: { room: RoomSnapshot }) {
  const { t } = useTranslation();
  return <dl className="room-capacity-details"><div><dt>{t("room.workspace.usedTotal")}</dt><dd>{formatBytes(room.capacity.usedBytes)} / {formatBytes(room.capacity.capacityBytes)}</dd></div>{room.role === "owner" && <><div><dt>{t("room.workspace.sharedUsage")}</dt><dd>{formatBytes(room.capacity.sharedBytes ?? 0)}</dd></div><div><dt>{t("room.workspace.privateUsage")}</dt><dd>{formatBytes(room.capacity.directBytes ?? 0)}</dd></div><div><dt>{t("room.workspace.trashUsage")}</dt><dd>{formatBytes(room.capacity.trashBytes ?? 0)}</dd></div><div><dt>{t("room.workspace.reservedUsage")}</dt><dd>{formatBytes(room.capacity.reservedBytes ?? 0)}</dd></div></>}</dl>;
}
