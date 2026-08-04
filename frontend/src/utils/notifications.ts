import type { ChatConversationNotification, NotificationItem } from "../types/domain";

export interface ChatNotificationTarget {
  roomCode: string;
  peerUserId: string;
}

export interface NotificationChatLaunch {
  peerUserId: string;
  token: string;
}

export function shouldRefreshNotifications(eventType: string) {
  return eventType.startsWith("chat.") || eventType.startsWith("room.") || eventType === "user.profile_changed";
}

export function chatNotificationTarget(item: NotificationItem): ChatNotificationTarget | undefined {
  if (item.type !== "chat_conversation") return undefined;
  return { roomCode: item.roomCode, peerUserId: item.peerUserId };
}

export function isChatNotification(item: NotificationItem): item is ChatConversationNotification {
  return item.type === "chat_conversation";
}

export function readNotificationChatLaunch(state: unknown): NotificationChatLaunch | undefined {
  if (!state || typeof state !== "object") return undefined;
  const target = (state as { notificationChatTarget?: unknown }).notificationChatTarget;
  if (!target || typeof target !== "object") return undefined;
  const { peerUserId, token } = target as { peerUserId?: unknown; token?: unknown };
  if (typeof peerUserId !== "string" || !peerUserId || typeof token !== "string" || !token) return undefined;
  return { peerUserId, token };
}

export function formatNotificationCount(count: number) {
  if (!Number.isFinite(count) || count <= 0) return "0";
  return count > 99 ? "99+" : String(Math.floor(count));
}
