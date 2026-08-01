import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Bell, ChevronLeft, Clock3, Copy, DoorOpen, Ellipsis, FileUp, LogOut, MessageSquare, QrCode, RefreshCw, Send, Trash2, Users, X } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { ErrorNotice, LanguageSelector } from "../common";
import { approveJoinRequest, getJoinRequests, getRoomQRCode, rejectJoinRequest } from "../../services/api";
import type { RoomMember, RoomSnapshot, Session } from "../../types/domain";
import { formatBytes, formatDate, formatDuration } from "../../utils/format";

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
  actionError?: unknown;
}

export function RoomWorkspace(props: Props) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [now, setNow] = useState(() => Math.floor(Date.now() / 1000));
  const [membersOpen, setMembersOpen] = useState(false);
  const [qrOpen, setQROpen] = useState(false);
  const [requestsOpen, setRequestsOpen] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [capacityOpen, setCapacityOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Math.floor(Date.now() / 1000)), 1000);
    return () => window.clearInterval(timer);
  }, []);
  const expires = Math.max(0, props.room.expiresAt - now);
  const countdown = props.destroyAt ? Math.max(0, Math.ceil(props.destroyAt - now)) : 0;
  const capacityPercent = props.room.capacity.capacityBytes > 0 ? Math.min(100, props.room.capacity.usedBytes * 100 / props.room.capacity.capacityBytes) : 0;
  const pending = props.room.pendingRequestCount ?? 0;
  const copyCode = async () => {
    await navigator.clipboard?.writeText(props.room.roomCode);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1500);
  };

  return <main className="room-page">
    <LanguageSelector />
    <header className="room-topbar">
      <div className="room-topbar-leading">
        <Link className="room-icon-button" to="/" aria-label={t("room.backHome")}><ChevronLeft aria-hidden="true" /></Link>
        <span className="room-brand-mark" aria-hidden="true">FD</span>
        <div className="room-topbar-identity">
          <button type="button" onClick={() => void copyCode()}><strong>{t("room.code")} {props.room.roomCode}</strong><Copy aria-hidden="true" /></button>
          <span>{copied ? t("room.workspace.copied") : props.room.title}</span>
        </div>
      </div>
      <div className="room-topbar-summary">
        <Summary label={t("room.members")} value={t("room.workspace.people", { count: props.room.members.length })} />
        <Summary label={t("room.workspace.remaining")} value={formatDuration(expires)} />
        <button className="room-capacity-summary" type="button" onClick={() => setCapacityOpen(true)}>
          <span>{t("room.workspace.capacity")}</span><strong>{formatBytes(props.room.capacity.usedBytes)} / {formatBytes(props.room.capacity.capacityBytes)}</strong>
          <i><i style={{ width: `${capacityPercent}%` }} /></i>
        </button>
      </div>
      <nav className="room-topbar-actions" aria-label={t("room.workspace.roomActions")}>
        <button className="room-icon-button room-members-trigger" type="button" aria-label={t("room.workspace.openMembers")} onClick={() => setMembersOpen(true)}><Users aria-hidden="true" /></button>
        <button className="room-icon-button" type="button" aria-label={t("room.qrcode")} onClick={() => setQROpen(true)}><QrCode aria-hidden="true" /></button>
        {props.room.role === "owner" && <button className="room-icon-button" type="button" aria-label={t("room.notifications")} onClick={() => setRequestsOpen(true)}><Bell aria-hidden="true" />{pending > 0 && <b>{pending}</b>}</button>}
        <div className="room-menu-wrap"><button className="room-icon-button" type="button" aria-label={t("room.workspace.more")} aria-expanded={menuOpen} onClick={() => setMenuOpen((value) => !value)}><Ellipsis aria-hidden="true" /></button>{menuOpen && <div className="room-action-menu">
          {props.room.role === "owner" && props.room.canExtend && <button type="button" onClick={props.onExtend}><Clock3 aria-hidden="true" />{t("room.extend")}</button>}
          {props.room.role === "owner" ? <button className="danger" type="button" onClick={props.onDissolve}><Trash2 aria-hidden="true" />{t("room.dissolve")}</button> : <button type="button" onClick={props.onLeave}><LogOut aria-hidden="true" />{t("room.leave")}</button>}
        </div>}</div>
      </nav>
    </header>

    <div className="room-workspace-layout">
      <MemberPanel room={props.room} session={props.session} onKick={props.onKick} />
      <section className="room-files-shell">
        <div className="room-file-toolbar"><div><span>{t("room.workspace.fileWorkspace")}</span><h1>{t("room.workspace.roomFiles")}</h1></div><div><button type="button"><FileUp aria-hidden="true" />{t("room.workspace.addFiles")}</button><button type="button"><Send aria-hidden="true" />{t("room.workspace.sendExisting")}</button></div></div>
        <div className="room-file-tabs" role="tablist"><button className="active" role="tab" aria-selected="true" type="button">{t("room.workspace.fileList")}</button><button role="tab" aria-selected="false" type="button">{t("room.workspace.timeline")}</button></div>
        <div className="room-file-stage-placeholder"><FileUp aria-hidden="true" /><h2>{t("room.workspace.fileStageTitle")}</h2><p>{t("room.workspace.fileStageDescription")}</p></div>
      </section>
      <aside className="room-chat-placeholder"><MessageSquare aria-hidden="true" /><h2>{t("room.workspace.chatLaterTitle")}</h2><p>{t("room.workspace.chatLaterDescription")}</p></aside>
    </div>
    <footer className="room-transfer-placeholder"><span><RefreshCw aria-hidden="true" />{t("room.workspace.transferIdle")}</span></footer>

    {membersOpen && <Overlay title={t("room.members")} onClose={() => setMembersOpen(false)}><MemberPanel room={props.room} session={props.session} onKick={props.onKick} drawer /></Overlay>}
    {qrOpen && <QRCodePanel code={props.code} onClose={() => setQROpen(false)} />}
    {requestsOpen && props.room.role === "owner" && <RequestPanel code={props.code} onClose={() => setRequestsOpen(false)} onChanged={() => void queryClient.invalidateQueries({ queryKey: ["room", props.code] })} />}
    {capacityOpen && <Overlay title={t("room.workspace.capacityDetails")} onClose={() => setCapacityOpen(false)}><CapacityPanel room={props.room} /></Overlay>}
    {props.actionError != null && <div className="room-floating-error"><ErrorNotice error={props.actionError} /></div>}
    {(props.destroyAt || props.room.status === "destroying") && <div className="room-blocking-state"><RefreshCw aria-hidden="true" /><h2>{t("room.destroyingTitle")}</h2><p>{t("room.destroyingHint")}</p><strong>{t("room.destroyCountdown", { seconds: String(countdown) })}</strong></div>}
    {props.kicked && <div className="room-blocking-state"><DoorOpen aria-hidden="true" /><h2>{t("room.kickedTitle")}</h2><p>{t("room.kicked")}</p><button type="button" onClick={() => { props.setKicked(false); window.location.replace("/"); }}>{t("room.confirmOnly")}</button></div>}
  </main>;
}

function Summary({ label, value }: { label: string; value: string }) { return <div><span>{label}</span><strong>{value}</strong></div>; }

function MemberPanel({ room, session, onKick, drawer = false }: { room: RoomSnapshot; session: Session; onKick: (id: string) => void; drawer?: boolean }) {
  const { t } = useTranslation();
  return <aside className={drawer ? "room-member-panel is-drawer" : "room-member-panel"}><div className="room-panel-heading"><div><span>{t("room.workspace.membersEyebrow")}</span><h2>{t("room.members")}</h2></div><b>{room.members.length}</b></div><div className="room-member-list">{room.members.map((member) => <MemberItem key={member.userId} member={member} self={member.userId === session.userId} canKick={room.role === "owner" && member.role !== "owner"} onKick={onKick} />)}</div></aside>;
}

function MemberItem({ member, self, canKick, onKick }: { member: RoomMember; self: boolean; canKick: boolean; onKick: (id: string) => void }) {
  const { t } = useTranslation();
  return <article className="room-member-item"><span className="room-avatar">{member.displayName.slice(0, 1)}</span><div><strong>{member.displayName}{self && <small>{t("room.workspace.me")}</small>}</strong><span><i className={`presence ${member.onlineStatus}`} />{member.onlineStatus === "online" ? t("room.online") : member.onlineStatus === "away" ? t("room.away") : t("room.offline")}</span></div>{canKick && <button type="button" aria-label={t("room.kick")} onClick={() => { if (window.confirm(t("room.kickConfirm", { name: member.displayName }))) onKick(member.userId); }}><X aria-hidden="true" /></button>}</article>;
}

function Overlay({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  const { t } = useTranslation();
  return <div className="room-overlay" role="dialog" aria-modal="true" aria-label={title}><section><header><h2>{title}</h2><button type="button" aria-label={t("room.workspace.close")} onClick={onClose}><X aria-hidden="true" /></button></header>{children}</section></div>;
}

function QRCodePanel({ code, onClose }: { code: string; onClose: () => void }) {
  const { t } = useTranslation();
  const qr = useQuery({ queryKey: ["room-qrcode", code], queryFn: () => getRoomQRCode(code), retry: false });
  return <Overlay title={t("room.qrcode")} onClose={onClose}>{qr.data?.svg ? <div className="room-qr" dangerouslySetInnerHTML={{ __html: qr.data.svg }} /> : <p className="room-modal-message">{qr.isError ? t("room.qrcodeUnavailable") : t("room.loading")}</p>}</Overlay>;
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
