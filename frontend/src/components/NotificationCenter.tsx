import { CloseCircleOutlined, DeleteOutlined, DownloadOutlined, FileOutlined, MessageOutlined, StopOutlined, UndoOutlined, UserAddOutlined } from "@ant-design/icons";
import { Alert, Avatar, Button, Drawer, Empty, Space } from "antd";
import { useQueryClient } from "@tanstack/react-query";
import { createContext, useContext, useEffect, useRef, useState, type MouseEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import { useNotifications } from "../hooks/use-notifications";
import { getApiErrorMessage } from "../lib/api-error";
import { approveJoinRequest, markNotificationRead, rejectJoinRequest } from "../services/api";
import type { FileNotification, JoinRequestNotification, NotificationItem, Session } from "../types/domain";
import { getAvatarInitial, getStableAvatarColor } from "../utils/avatar";
import { formatDate } from "../utils/format";
import { chatNotificationTarget, fileNotificationTarget, formatNotificationCount, isFileResultNotification } from "../utils/notifications";

interface NotificationCenterValue {
  enabled: boolean;
  totalCount: number;
  open: (trigger: HTMLElement) => void;
}

const NotificationCenterContext = createContext<NotificationCenterValue | undefined>(undefined);

export function useNotificationCenter() {
  const value = useContext(NotificationCenterContext);
  if (!value) throw new Error("notification center provider is required");
  return value;
}

export function NotificationCenterProvider({ session, children }: { session?: Session; children: ReactNode }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const notifications = useNotifications(session);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [processingKey, setProcessingKey] = useState<string>();
  const [actionError, setActionError] = useState<unknown>();
  const triggerRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (session?.userId) return;
    setDrawerOpen(false);
    setProcessingKey(undefined);
    setActionError(undefined);
    triggerRef.current = null;
  }, [session?.userId]);

  const open = (trigger: HTMLElement) => {
    triggerRef.current = trigger;
    setActionError(undefined);
    setDrawerOpen(true);
  };
  const close = () => setDrawerOpen(false);
  const refreshAffectedQueries = () => {
    void queryClient.invalidateQueries({ queryKey: ["notifications"] });
    void queryClient.invalidateQueries({ queryKey: ["rooms"] });
    void queryClient.invalidateQueries({ queryKey: ["room"] });
    void queryClient.invalidateQueries({ queryKey: ["join-requests"] });
  };
  const processApproval = async (item: JoinRequestNotification, approve: boolean) => {
    setProcessingKey(item.key);
    setActionError(undefined);
    try {
      if (approve) await approveJoinRequest(item.roomCode, item.requestId);
      else await rejectJoinRequest(item.roomCode, item.requestId);
      refreshAffectedQueries();
    } catch (error) {
      setActionError(error);
      refreshAffectedQueries();
    } finally {
      setProcessingKey(undefined);
    }
  };
  const openChat = (item: NotificationItem) => {
    const target = chatNotificationTarget(item);
    if (!target) return;
    close();
    navigate(`/rooms/${target.roomCode}`, {
      state: {
        notificationChatTarget: {
          peerUserId: target.peerUserId,
          token: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
        },
      },
    });
  };
  const openFile = async (item: FileNotification) => {
    const target = fileNotificationTarget(item);
    if (!target) return;
    setActionError(undefined);
    if (isFileResultNotification(item)) {
      setProcessingKey(item.key);
      try {
        await markNotificationRead(item.key, item.readToken);
      } catch (error) {
        setActionError(error);
        setProcessingKey(undefined);
        return;
      }
      setProcessingKey(undefined);
    }
    void queryClient.invalidateQueries({ queryKey: ["notifications"] });
    void queryClient.invalidateQueries({ queryKey: ["room-files", target.roomCode] });
    void queryClient.invalidateQueries({ queryKey: ["file-events", target.roomCode] });
    void queryClient.invalidateQueries({ queryKey: ["room", target.roomCode] });
    close();
    navigate(`/rooms/${target.roomCode}`, {
      state: {
        notificationFileTarget: {
          token: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
          view: target.view,
          requestId: target.requestId,
        },
      },
    });
  };

  return <NotificationCenterContext.Provider value={{ enabled: Boolean(session?.userId), totalCount: notifications.totalCount, open }}>
    {children}
    <Drawer
      rootClassName="global-notification-drawer"
      title={t("notification.title")}
      placement="right"
      width={420}
      open={drawerOpen}
      onClose={close}
      afterOpenChange={(nextOpen) => { if (!nextOpen) triggerRef.current?.focus(); }}
    >
      {notifications.isPending && <p className="global-notification-state">{t("notification.loading")}</p>}
      {notifications.isError && <Alert type="error" showIcon title={getApiErrorMessage(notifications.error, t)} action={<Button size="small" onClick={() => void notifications.refetch()}>{t("notification.retry")}</Button>} />}
      {actionError != null && <Alert className="global-notification-error" type="error" showIcon title={getApiErrorMessage(actionError, t)} />}
      {!notifications.isPending && !notifications.isError && notifications.items.length === 0 && <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("notification.empty")} />}
      <div className="global-notification-list">
        {notifications.items.map((item) => item.type === "join_request"
          ? <ApprovalNotificationItem key={item.key} item={item} processing={processingKey === item.key} onProcess={processApproval} />
          : item.type === "chat_conversation" ? <button className="global-notification-item is-chat" type="button" key={item.key} onClick={() => openChat(item)} aria-label={t("notification.openChat", { name: item.peerDisplayName, room: item.roomTitle })}>
            <Avatar size={42} style={{ backgroundColor: getStableAvatarColor(item.peerDisplayName) }}>{getAvatarInitial(item.peerDisplayName)}</Avatar>
            <span className="global-notification-content">
              <span className="global-notification-title"><strong>{item.peerDisplayName}</strong><time>{formatDate(item.latestMessageAt)}</time></span>
              <span className="global-notification-room"><MessageOutlined aria-hidden="true" />{item.roomTitle} · {item.roomCode}</span>
              <span className="global-notification-preview">{item.latestMessageText}</span>
            </span>
            <span className="global-notification-count" aria-label={t("notification.unreadCount", { count: item.unreadCount })}>{formatNotificationCount(item.unreadCount)}</span>
          </button> : <FileNotificationItem key={item.key} item={item} processing={processingKey === item.key} onOpen={openFile} />)}
      </div>
      {notifications.hasNextPage && <Button className="global-notification-more" block loading={notifications.isFetchingNextPage} onClick={() => void notifications.fetchNextPage()}>{t("notification.loadMore")}</Button>}
    </Drawer>
  </NotificationCenterContext.Provider>;
}

function FileNotificationItem({ item, processing, onOpen }: { item: FileNotification; processing: boolean; onOpen: (item: FileNotification) => Promise<void> }) {
  const { t } = useTranslation();
  const icon = item.type === "file_declined" ? <CloseCircleOutlined aria-hidden="true" /> : item.type === "file_downloaded" ? <DownloadOutlined aria-hidden="true" /> : item.type === "file_trashed_by_owner" ? <DeleteOutlined aria-hidden="true" /> : item.type === "file_restore_requested" ? <UndoOutlined aria-hidden="true" /> : item.type === "file_restore_rejected" ? <StopOutlined aria-hidden="true" /> : item.type === "file_purged_by_owner" ? <DeleteOutlined aria-hidden="true" /> : <FileOutlined aria-hidden="true" />;
  const previewKey = item.type === "file_received" ? "notification.fileReceived" : item.type === "file_declined" ? "notification.fileDeclined" : item.type === "file_downloaded" ? "notification.fileDownloaded" : item.type === "file_trashed_by_owner" ? "notification.fileTrashed" : item.type === "file_restore_requested" ? "notification.fileRestoreRequested" : item.type === "file_restored_by_owner" ? "notification.fileRestored" : item.type === "file_restore_rejected" ? "notification.fileRestoreRejected" : "notification.filePurged";
  const actionable = item.type === "file_restore_requested";
  return <button className={`global-notification-item is-file ${item.type} ${actionable ? "is-action" : ""}`} type="button" disabled={processing} onClick={() => void onOpen(item)} aria-label={t("notification.openFile", { name: item.counterpartDisplayName, room: item.roomTitle, count: item.fileCount })}>
    <Avatar size={42} style={{ backgroundColor: getStableAvatarColor(item.counterpartDisplayName) }}>{getAvatarInitial(item.counterpartDisplayName)}</Avatar>
    <span className="global-notification-content">
      <span className="global-notification-title"><strong>{item.counterpartDisplayName}</strong><time>{formatDate(item.latestFileEventAt)}</time></span>
      <span className="global-notification-room">{icon}{item.roomTitle} · {item.roomCode}</span>
      <span className="global-notification-preview">{t(previewKey, { count: item.fileCount, file: item.latestFileName })}</span>
      {item.latestReason && <span className="global-notification-reason">{t("notification.reason", { reason: item.latestReason })}</span>}
    </span>
    <span className="global-notification-count" aria-label={t("notification.fileCount", { count: item.fileCount })}>{formatNotificationCount(item.fileCount)}</span>
  </button>;
}

function ApprovalNotificationItem({ item, processing, onProcess }: { item: JoinRequestNotification; processing: boolean; onProcess: (item: JoinRequestNotification, approve: boolean) => Promise<void> }) {
  const { t } = useTranslation();
  return <article className="global-notification-item is-approval">
    <Avatar size={42} style={{ backgroundColor: getStableAvatarColor(item.actorDisplayName) }}>{getAvatarInitial(item.actorDisplayName)}</Avatar>
    <div className="global-notification-content">
      <span className="global-notification-title"><strong>{item.actorDisplayName}</strong><time>{formatDate(item.createdAt)}</time></span>
      <span className="global-notification-room"><UserAddOutlined aria-hidden="true" />{item.roomTitle} · {item.roomCode}</span>
      <span className="global-notification-preview">{t("notification.joinRequest")}</span>
      <span className="global-notification-expiry">{t("notification.expiresAt", { time: formatDate(item.expiresAt) })}</span>
      <Space size="small">
        <Button size="small" type="primary" loading={processing} disabled={processing} onClick={() => void onProcess(item, true)}>{t("room.approve")}</Button>
        <Button size="small" disabled={processing} onClick={() => void onProcess(item, false)}>{t("room.reject")}</Button>
      </Space>
    </div>
  </article>;
}

export function openNotificationCenter(event: MouseEvent<HTMLElement>, open: (trigger: HTMLElement) => void) {
  open(event.currentTarget);
}
