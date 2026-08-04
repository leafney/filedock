import type { ChatConversationNotification, NotificationItem } from "../types/domain";

export interface ChatNotificationTarget {
  roomCode: string;
  peerUserId: string;
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
