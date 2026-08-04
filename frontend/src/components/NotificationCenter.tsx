import { MessageOutlined, UserAddOutlined } from "@ant-design/icons";
import { Alert, Avatar, Button, Drawer, Empty, Space } from "antd";
import { useQueryClient } from "@tanstack/react-query";
import { createContext, useContext, useRef, useState, type MouseEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import { useNotifications } from "../hooks/use-notifications";
import { getApiErrorMessage } from "../lib/api-error";
import { approveJoinRequest, rejectJoinRequest } from "../services/api";
import type { JoinRequestNotification, NotificationItem, Session } from "../types/domain";
import { getAvatarInitial, getStableAvatarColor } from "../utils/avatar";
import { formatDate } from "../utils/format";
import { chatNotificationTarget, formatNotificationCount } from "../utils/notifications";

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
          : <button className="global-notification-item is-chat" type="button" key={item.key} onClick={() => openChat(item)} aria-label={t("notification.openChat", { name: item.peerDisplayName, room: item.roomTitle })}>
            <Avatar size={42} style={{ backgroundColor: getStableAvatarColor(item.peerDisplayName) }}>{getAvatarInitial(item.peerDisplayName)}</Avatar>
            <span className="global-notification-content">
              <span className="global-notification-title"><strong>{item.peerDisplayName}</strong><time>{formatDate(item.latestMessageAt)}</time></span>
              <span className="global-notification-room"><MessageOutlined aria-hidden="true" />{item.roomTitle} · {item.roomCode}</span>
              <span className="global-notification-preview">{item.latestMessageText}</span>
            </span>
            <span className="global-notification-count" aria-label={t("notification.unreadCount", { count: item.unreadCount })}>{formatNotificationCount(item.unreadCount)}</span>
          </button>)}
      </div>
      {notifications.hasNextPage && <Button className="global-notification-more" block loading={notifications.isFetchingNextPage} onClick={() => void notifications.fetchNextPage()}>{t("notification.loadMore")}</Button>}
    </Drawer>
  </NotificationCenterContext.Provider>;
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
