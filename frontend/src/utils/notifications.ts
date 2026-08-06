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
  view: "list" | "timeline" | "trash";
  requestId?: string;
}

const notificationFileEvents = new Set([
  "file.available",
  "file.reused",
  "file.recipient_changed",
  "file.download_completed",
  "file.trashed",
  "file.restore_requested",
  "file.restored",
  "file.restore_rejected",
  "file.purged",
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
  return item.type === "file_received"
    || item.type === "file_declined"
    || item.type === "file_downloaded"
    || item.type === "file_trashed_by_owner"
    || item.type === "file_restore_requested"
    || item.type === "file_restored_by_owner"
    || item.type === "file_restore_rejected"
    || item.type === "file_purged_by_owner";
}

export function isFileResultNotification(item: NotificationItem): item is Extract<FileNotification, { readToken: string }> {
  return item.type === "file_declined"
    || item.type === "file_downloaded"
    || item.type === "file_trashed_by_owner"
    || item.type === "file_restored_by_owner"
    || item.type === "file_restore_rejected"
    || item.type === "file_purged_by_owner";
}

export function fileNotificationTarget(item: NotificationItem): ({ roomCode: string } & Omit<NotificationFileLaunch, "token">) | undefined {
  if (!isFileNotification(item)) return undefined;
  if (item.type === "file_trashed_by_owner" || item.type === "file_restore_rejected") return { roomCode: item.roomCode, view: "trash" };
  if (item.type === "file_restore_requested") return { roomCode: item.roomCode, view: "trash", requestId: item.requestId };
  if (item.type === "file_purged_by_owner") return { roomCode: item.roomCode, view: "timeline" };
  return { roomCode: item.roomCode, view: "list" };
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
  const { token, view, requestId } = target as { token?: unknown; view?: unknown; requestId?: unknown };
  if (typeof token !== "string" || !token || (view !== "list" && view !== "timeline" && view !== "trash")) return undefined;
  if (requestId != null && (typeof requestId !== "string" || !requestId || view !== "trash")) return undefined;
  return { token, view, requestId: requestId as string | undefined };
}

export function formatNotificationCount(count: number) {
  if (!Number.isFinite(count) || count <= 0) return "0";
  return count > 99 ? "99+" : String(Math.floor(count));
}

export function shouldConsumeNotificationLaunch(consumedToken: string | undefined, incomingToken: string | undefined) {
  return Boolean(incomingToken) && consumedToken !== incomingToken;
}
