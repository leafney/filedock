import type { ChatConversationNotification, FileNotification, NotificationItem } from "../types/domain";

export interface ChatNotificationTarget {
  roomCode: string;
  peerUserId: string;
}

export interface NotificationChatLaunch {
  peerUserId: string;
  token: string;
}

export interface NotificationFileLaunch {
  token: string;
}

const notificationFileEvents = new Set([
  "file.available",
  "file.reused",
  "file.recipient_changed",
  "file.download_completed",
]);

export function shouldRefreshNotifications(eventType: string) {
  return eventType.startsWith("chat.") || eventType.startsWith("room.") || eventType === "user.profile_changed" || eventType === "notification.changed" || notificationFileEvents.has(eventType);
}

export function chatNotificationTarget(item: NotificationItem): ChatNotificationTarget | undefined {
  if (item.type !== "chat_conversation") return undefined;
  return { roomCode: item.roomCode, peerUserId: item.peerUserId };
}

export function isChatNotification(item: NotificationItem): item is ChatConversationNotification {
  return item.type === "chat_conversation";
}

export function isFileNotification(item: NotificationItem): item is FileNotification {
  return item.type === "file_received" || item.type === "file_declined" || item.type === "file_downloaded";
}

export function isFileResultNotification(item: NotificationItem): item is Exclude<FileNotification, { type: "file_received" }> {
  return item.type === "file_declined" || item.type === "file_downloaded";
}

export function fileNotificationTarget(item: NotificationItem): { roomCode: string } | undefined {
  if (!isFileNotification(item)) return undefined;
  return { roomCode: item.roomCode };
}

export function readNotificationChatLaunch(state: unknown): NotificationChatLaunch | undefined {
  if (!state || typeof state !== "object") return undefined;
  const target = (state as { notificationChatTarget?: unknown }).notificationChatTarget;
  if (!target || typeof target !== "object") return undefined;
  const { peerUserId, token } = target as { peerUserId?: unknown; token?: unknown };
  if (typeof peerUserId !== "string" || !peerUserId || typeof token !== "string" || !token) return undefined;
  return { peerUserId, token };
}

export function readNotificationFileLaunch(state: unknown): NotificationFileLaunch | undefined {
  if (!state || typeof state !== "object") return undefined;
  const target = (state as { notificationFileTarget?: unknown }).notificationFileTarget;
  if (!target || typeof target !== "object") return undefined;
  const { token } = target as { token?: unknown };
  if (typeof token !== "string" || !token) return undefined;
  return { token };
}

export function formatNotificationCount(count: number) {
  if (!Number.isFinite(count) || count <= 0) return "0";
  return count > 99 ? "99+" : String(Math.floor(count));
}
