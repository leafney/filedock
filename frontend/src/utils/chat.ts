import type { ChatDeliveryStatus, ChatMessage } from "../types/domain";

export interface ChatActionCapabilities {
  copy: boolean;
  recall: boolean;
  recallAndEdit: boolean;
  forward: boolean;
  delete: boolean;
}

export function chatMessageCapabilities(message: ChatMessage, selfId: string, now = Math.floor(Date.now() / 1000)): ChatActionCapabilities {
  const visible = !message.recalledAt && message.contentText.length > 0;
  const sender = message.senderUserId === selfId;
  const recall = sender && visible && (message.canRecall || (message.recallDeadline ?? 0) >= now);
  return { copy: visible && message.canCopy, recall, recallAndEdit: recall && message.canRecallAndEdit, forward: visible && message.canForward, delete: message.canDelete };
}

export function mergeChatMessages(current: ChatMessage[], incoming: ChatMessage[]): ChatMessage[] {
  const byID = new Map<string, ChatMessage>();
  const byClient = new Map<string, string>();
  for (const message of current) {
    byID.set(message.messageId, message);
    if (message.clientMessageId) byClient.set(message.clientMessageId, message.messageId);
  }
  for (const message of incoming) {
    const existingID = byID.has(message.messageId) ? message.messageId : byClient.get(message.clientMessageId);
    const previous = existingID ? byID.get(existingID) : undefined;
    if (existingID) byID.delete(existingID);
    byID.set(message.messageId, { ...previous, ...message, optimistic: false, deliveryStatus: message.read ? "read" : "sent" });
    if (message.clientMessageId) byClient.set(message.clientMessageId, message.messageId);
  }
  return [...byID.values()].sort((left, right) => left.sequence - right.sequence || left.createdAt - right.createdAt || left.messageId.localeCompare(right.messageId));
}

export function mergeChatMessage(current: ChatMessage[], incoming: ChatMessage): ChatMessage[] {
  return mergeChatMessages(current, [incoming]);
}

export function markChatMessagesRead(messages: ChatMessage[], peerReadSequence: number, selfId: string): ChatMessage[] {
  return messages.map((message) => {
    if (message.senderUserId !== selfId || message.optimistic) return message;
    const status: ChatDeliveryStatus = message.sequence <= peerReadSequence ? "read" : "sent";
    return { ...message, read: status === "read", deliveryStatus: status };
  });
}

export function mergeReadSequence(current: number, incoming: number): number {
  return Math.max(0, current, incoming);
}

export function chatDateKey(unixSeconds: number, timezone = "zh-CN"): string {
  return new Intl.DateTimeFormat(timezone, { year: "numeric", month: "2-digit", day: "2-digit" }).format(new Date(unixSeconds * 1000));
}

export function chatTimeBucket(unixSeconds: number): string {
  return new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit" }).format(new Date(unixSeconds * 1000));
}

export function isChatNearBottom(scrollTop: number, scrollHeight: number, clientHeight: number, threshold = 80): boolean {
  return scrollHeight - (scrollTop + clientHeight) <= threshold;
}

export function shouldMarkChatRead(open: boolean, visible: boolean, focused: boolean): boolean {
  return open && visible && focused;
}

export function newChatClientMessageID(): string {
  return globalThis.crypto?.randomUUID?.() ?? `chat-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}
