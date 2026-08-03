import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Copy, Crown, DoorOpen, MessageSquare, RefreshCw, Users, X } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { GlobalHeader } from "../GlobalHeader";
import { ErrorNotice } from "../common";
import { FileWorkspace } from "./FileWorkspace";
import { ChatWorkspace } from "./ChatWorkspace";
import { TransferBar } from "./TransferBar";
import { approveJoinRequest, getJoinRequests, getRoomQRCode, rejectJoinRequest } from "../../services/api";
import type { RoomMember, RoomSnapshot, Session } from "../../types/domain";
import { durationParts, formatBytes, formatDate } from "../../utils/format";
import { useDialogFocus } from "../../hooks/use-dialog-focus";
import { copyText } from "../../utils/clipboard";

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
}

export function RoomWorkspace(props: Props) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [now, setNow] = useState(() => Math.floor(Date.now() / 1000));
  const [membersOpen, setMembersOpen] = useState(false);
  const [shareOpen, setShareOpen] = useState(false);
  const [requestsOpen, setRequestsOpen] = useState(false);
  const [capacityOpen, setCapacityOpen] = useState(false);
  const [chatTarget, setChatTarget] = useState<string>();
  const [chatMobileOpen, setChatMobileOpen] = useState(false);
  const [chatUnread, setChatUnread] = useState(0);
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Math.floor(Date.now() / 1000)), 1000);
    return () => window.clearInterval(timer);
  }, []);
  const expires = Math.max(0, props.room.expiresAt - now);
  const duration = durationParts(expires);
  const countdown = props.destroyAt ? Math.max(0, Math.ceil(props.destroyAt - now)) : 0;
  const capacityPercent = props.room.capacity.capacityBytes > 0 ? Math.min(100, props.room.capacity.usedBytes * 100 / props.room.capacity.capacityBytes) : 0;
  const pending = props.room.pendingRequestCount ?? 0;
  return <main className="room-page">
    <GlobalHeader
      variant="room"
      session={props.session}
      center={<div className="room-topbar-summary">
        <Summary label={t("room.members")} value={t("room.workspace.people", { count: props.room.members.length })} />
        <Summary label={t("room.workspace.remaining")} value={t("room.workspace.duration", { hours: String(duration.hours), minutes: String(duration.minutes), seconds: String(duration.seconds) })} />
        <button className="room-capacity-summary" type="button" onClick={() => setCapacityOpen(true)}>
          <span>{t("room.workspace.capacity")}</span><strong>{formatBytes(props.room.capacity.usedBytes)} / {formatBytes(props.room.capacity.capacityBytes)}</strong>
          <i><i style={{ width: `${capacityPercent}%` }} /></i>
        </button>
      </div>}
      extraActions={<><button className="room-icon-button room-chat-mobile-trigger" type="button" aria-label={t("chat.openMobile")} onClick={() => setChatMobileOpen(true)}><MessageSquare aria-hidden="true" />{chatUnread > 0 && <b>{chatUnread > 99 ? "99+" : chatUnread}</b>}</button><button className="room-icon-button room-members-trigger" type="button" aria-label={t("room.workspace.openMembers")} onClick={() => setMembersOpen(true)}><Users aria-hidden="true" /></button></>}
      pendingCount={pending}
      notificationDisabled={props.room.role !== "owner"}
      onOpenNotifications={() => { if (props.room.role === "owner") setRequestsOpen(true); }}
      onOpenProfile={props.onOpenProfile}
      onShare={() => setShareOpen(true)}
      roomActions={props.room.role === "owner" ? { role: "owner", canExtend: props.room.canExtend, onExtend: props.onExtend, onDissolve: props.onDissolve } : { role: "member", onLeave: props.onLeave }}
    />

    <div className="room-workspace-layout">
      <MemberPanel room={props.room} session={props.session} onKick={props.onKick} onChat={(userId) => setChatTarget(userId)} />
      <FileWorkspace code={props.code} members={props.room.members} selfId={props.session.userId} />
      <ChatWorkspace code={props.code} members={props.room.members} selfId={props.session.userId} initialPeerUserId={chatTarget} onInitialPeerConsumed={() => setChatTarget(undefined)} onUnreadCount={setChatUnread} />
    </div>
    <TransferBar roomCode={props.code} />

    {membersOpen && <Overlay title={t("room.members")} onClose={() => setMembersOpen(false)}><MemberPanel room={props.room} session={props.session} onKick={props.onKick} drawer /></Overlay>}
    {shareOpen && <ShareRoomPanel room={props.room} onClose={() => setShareOpen(false)} />}
    {requestsOpen && props.room.role === "owner" && <RequestPanel code={props.code} onClose={() => setRequestsOpen(false)} onChanged={() => void queryClient.invalidateQueries({ queryKey: ["room", props.code] })} />}
    {capacityOpen && <Overlay title={t("room.workspace.capacityDetails")} onClose={() => setCapacityOpen(false)}><CapacityPanel room={props.room} /></Overlay>}
    {chatMobileOpen && <div className="chat-mobile-overlay"><section><header><strong>{t("chat.conversations")}</strong><button type="button" aria-label={t("chat.closeMobile")} onClick={() => setChatMobileOpen(false)}><X aria-hidden="true" /></button></header><ChatWorkspace code={props.code} members={props.room.members} selfId={props.session.userId} onUnreadCount={setChatUnread} /></section></div>}
    {props.actionError != null && <div className="room-floating-error"><ErrorNotice error={props.actionError} /></div>}
    {(props.destroyAt || props.room.status === "destroying") && <div className="room-blocking-state"><RefreshCw aria-hidden="true" /><h2>{t("room.destroyingTitle")}</h2><p>{t("room.destroyingHint")}</p><strong>{t("room.destroyCountdown", { seconds: String(countdown) })}</strong></div>}
    {props.kicked && <div className="room-blocking-state"><DoorOpen aria-hidden="true" /><h2>{t("room.kickedTitle")}</h2><p>{t("room.kicked")}</p><button type="button" onClick={() => { props.setKicked(false); window.location.replace("/"); }}>{t("room.confirmOnly")}</button></div>}
  </main>;
}

function Summary({ label, value }: { label: string; value: string }) { return <div><span>{label}</span><strong>{value}</strong></div>; }

function MemberPanel({ room, session, onKick, onChat, drawer = false }: { room: RoomSnapshot; session: Session; onKick: (id: string) => void; onChat?: (id: string) => void; drawer?: boolean }) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  const [copyError, setCopyError] = useState(false);
  const copyCode = async () => {
    setCopyError(false);
    if (!await copyText(room.roomCode)) { setCopyError(true); return; }
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1500);
  };
  return <aside className={drawer ? "room-member-panel is-drawer" : "room-member-panel"}>
    <div className="room-panel-heading">
      <div className="room-panel-room-info">
        <strong title={room.title}>{room.title}</strong>
        <button type="button" onClick={() => void copyCode()}><span>{t("room.code")} {room.roomCode}</span><Copy aria-hidden="true" /></button>
        {copied && <small>{t("room.workspace.copied")}</small>}
        {copyError && <small>{t("room.workspace.copyFailed")}</small>}
      </div>
    </div>
    <div className="room-member-list">{room.members.map((member) => <MemberItem key={member.userId} member={member} self={member.userId === session.userId} canKick={room.role === "owner" && member.role !== "owner"} onKick={onKick} onChat={onChat} />)}</div>
  </aside>;
}

function MemberItem({ member, self, canKick, onKick, onChat }: { member: RoomMember; self: boolean; canKick: boolean; onKick: (id: string) => void; onChat?: (id: string) => void }) {
  const { t } = useTranslation();
  const ownerLabel = t("room.workspace.owner");
  return <article className={`room-member-item ${!self && member.status === "active" ? "is-chatable" : ""}`} onDoubleClick={() => { if (!self && member.status === "active") onChat?.(member.userId); }}><button className="room-member-chat-hit" type="button" disabled={self || member.status !== "active"} aria-label={!self ? t("chat.openConversation", { name: member.displayName }) : undefined} onClick={() => { if (!self && member.status === "active") onChat?.(member.userId); }}><span className="room-avatar-wrap" title={member.role === "owner" ? ownerLabel : undefined} aria-label={member.role === "owner" ? ownerLabel : undefined}><span className="room-avatar">{member.displayName.slice(0, 1)}</span>{member.role === "owner" && <span className="room-owner-corner"><Crown aria-hidden="true" /></span>}</span><div><strong>{member.displayName}{self && <small>{t("room.workspace.me")}</small>}</strong><span><i className={`presence ${member.onlineStatus}`} />{member.onlineStatus === "online" ? t("room.online") : member.onlineStatus === "away" ? t("room.away") : t("room.offline")}</span></div></button>{canKick && <button type="button" aria-label={t("room.kick")} onClick={() => { if (window.confirm(t("room.kickConfirm", { name: member.displayName }))) onKick(member.userId); }}><X aria-hidden="true" /></button>}</article>;
}

function Overlay({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  const { t } = useTranslation();
  const panelRef = useDialogFocus(onClose);
  return <div className="room-overlay" role="dialog" aria-modal="true" aria-label={title}><section ref={panelRef}><header><h2>{title}</h2><button type="button" aria-label={t("room.workspace.close")} onClick={onClose}><X aria-hidden="true" /></button></header>{children}</section></div>;
}

function ShareRoomPanel({ room, onClose }: { room: RoomSnapshot; onClose: () => void }) {
  const { t } = useTranslation();
  const qr = useQuery({ queryKey: ["room-qrcode", room.roomCode], queryFn: () => getRoomQRCode(room.roomCode), retry: false });
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
  return <Overlay title={t("room.workspace.shareRoom")} onClose={onClose}>
    <div className="room-share-panel">
      <strong title={room.title}>{room.title}</strong>
      <span>{t("room.code")} {room.roomCode}</span>
      {qr.data?.svg ? <div className="room-qr" dangerouslySetInnerHTML={{ __html: qr.data.svg }} /> : <p className="room-modal-message">{qr.isError ? t("room.qrcodeUnavailable") : t("room.loading")}</p>}
      <label><span>{t("room.workspace.joinLink")}</span><input readOnly value={link} /></label>
      <div className="room-share-actions">
        <button type="button" onClick={() => void copy("code", room.roomCode)}>{copied === "code" ? t("room.workspace.copied") : t("room.workspace.copyRoomCode")}</button>
        <button type="button" onClick={() => void copy("link", link)}>{copied === "link" ? t("room.workspace.copied") : t("room.workspace.copyLink")}</button>
        <button type="button" onClick={() => void copy("invite", invite)}>{copied === "invite" ? t("room.workspace.copied") : t("room.workspace.copyInvite")}</button>
      </div>
      {copyError && <p className="room-modal-message">{t("room.workspace.copyFailed")}</p>}
    </div>
  </Overlay>;
}

function RequestPanel({ code, onClose, onChanged }: { code: string; onClose: () => void; onChanged: () => void }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const requests = useQuery({ queryKey: ["join-requests", code], queryFn: () => getJoinRequests(code), retry: false });
  const changed = () => { void queryClient.invalidateQueries({ queryKey: ["join-requests", code] }); onChanged(); };
  const approve = useMutation({ mutationFn: (id: string) => approveJoinRequest(code, id), onSuccess: changed });
  const reject = useMutation({ mutationFn: (id: string) => rejectJoinRequest(code, id), onSuccess: changed });
  return <Overlay title={t("room.pendingRequests")} onClose={onClose}>{requests.isPending && <p className="room-modal-message">{t("room.loading")}</p>}{requests.isError && <ErrorNotice error={requests.error} />}{requests.data?.items.length === 0 && <p className="room-modal-message">{t("room.noRequests")}</p>}<div className="room-request-list">{requests.data?.items.map((request) => <article key={request.requestId}><div><strong>{request.displayName}</strong><span>{t("room.requestExpires", { time: formatDate(request.expiresAt) })}</span></div><button type="button" onClick={() => approve.mutate(request.requestId)}>{t("room.approve")}</button><button type="button" onClick={() => reject.mutate(request.requestId)}>{t("room.reject")}</button></article>)}</div></Overlay>;
}

function CapacityPanel({ room }: { room: RoomSnapshot }) {
  const { t } = useTranslation();
  return <dl className="room-capacity-details"><div><dt>{t("room.workspace.usedTotal")}</dt><dd>{formatBytes(room.capacity.usedBytes)} / {formatBytes(room.capacity.capacityBytes)}</dd></div>{room.role === "owner" && <><div><dt>{t("room.workspace.sharedUsage")}</dt><dd>{formatBytes(room.capacity.sharedBytes ?? 0)}</dd></div><div><dt>{t("room.workspace.privateUsage")}</dt><dd>{formatBytes(room.capacity.directBytes ?? 0)}</dd></div><div><dt>{t("room.workspace.reservedUsage")}</dt><dd>{formatBytes(room.capacity.reservedBytes ?? 0)}</dd></div></>}</dl>;
}
