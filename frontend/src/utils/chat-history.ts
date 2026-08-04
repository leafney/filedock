import type { ChatMessage, ChatMessagePage } from "../types/domain";
import { mergeChatMessages, mergeReadSequence } from "./chat";

export type ChatPageDirection = "replace" | "before" | "after";
export type ChatHistoryMode = "live" | "locating" | "history";

export interface ChatHistoryAnchor {
  messageId: string;
  sequence: number;
  createdAt: number;
}

export function mergeChatMessagePage(current: ChatMessagePage | undefined, incoming: ChatMessagePage, direction: ChatPageDirection): ChatMessagePage {
  if (!current || direction === "replace") return incoming;
  const items = direction === "before" ? mergeChatMessages(incoming.items, current.items) : mergeChatMessages(current.items, incoming.items);
  return {
    ...current,
    items,
    currentReadSequence: mergeReadSequence(current.currentReadSequence, incoming.currentReadSequence),
    peerReadSequence: mergeReadSequence(current.peerReadSequence, incoming.peerReadSequence),
    ...(direction === "before" ? { previousCursor: incoming.previousCursor, hasMoreBefore: incoming.hasMoreBefore } : { nextCursor: incoming.nextCursor, hasMoreAfter: incoming.hasMoreAfter }),
  };
}

export function addPendingHistoryMessage(current: string[], messageId: string): string[] {
  return current.includes(messageId) ? current : [...current, messageId];
}

export function appendUniqueChatMessages(current: ChatMessage[], incoming: ChatMessage[]): ChatMessage[] {
  const seen = new Set(current.map((message) => message.messageId));
  return [...current, ...incoming.filter((message) => !seen.has(message.messageId))];
}

export function historyModeAfterPage(hasMoreAfter: boolean): ChatHistoryMode {
  return hasMoreAfter ? "history" : "live";
}

export function isIncomingForHistory(message: ChatMessage, peerUserId: string | undefined, selfId: string): boolean {
  return Boolean(peerUserId && message.senderUserId === peerUserId && message.recipientUserId === selfId);
}

export function containsHistoryTarget(page: ChatMessagePage, sequence: number, messageId: string): boolean {
  return page.items.some((message) => message.sequence === sequence && message.messageId === messageId && !message.recalledAt);
}
