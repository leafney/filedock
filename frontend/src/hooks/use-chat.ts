import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { streamEventName, type StreamEventMessage } from "./use-stream";
import { deleteChatMessage, forwardChatMessage, getChatMessages, listChatConversations, markChatRead, recallChatMessage, searchChatMessages, sendChatMessage } from "../services/api";
import type { ChatConversation, ChatMessage, ChatMessagePage, ChatSearchPage } from "../types/domain";
import { mergeChatMessage, mergeChatMessages, markChatMessagesRead, mergeReadSequence, newChatClientMessageID } from "../utils/chat";

interface ChatPayload extends Record<string, unknown> {
  roomCode?: string;
  conversationId?: string;
  messageId?: string;
  clientMessageId?: string;
  senderUserId?: string;
  recipientUserId?: string;
  senderDisplayName?: string;
  contentText?: string;
  isForwarded?: boolean;
  sequence?: number;
  createdAt?: number;
  recalledAt?: number;
  peerUserId?: string;
  userId?: string;
  lastReadSequence?: number;
}

function eventMessage(payload: ChatPayload): ChatMessage | null {
  if (!payload.messageId || !payload.conversationId || typeof payload.senderUserId !== "string" || typeof payload.recipientUserId !== "string" || typeof payload.sequence !== "number" || typeof payload.createdAt !== "number") return null;
  return {
    roomCode: String(payload.roomCode ?? ""),
    conversationId: payload.conversationId,
    messageId: payload.messageId,
    clientMessageId: String(payload.clientMessageId ?? ""),
    senderUserId: payload.senderUserId,
    recipientUserId: payload.recipientUserId,
    senderDisplayName: String(payload.senderDisplayName ?? ""),
    contentText: String(payload.contentText ?? ""),
    isForwarded: Boolean(payload.isForwarded),
    sequence: payload.sequence,
    createdAt: payload.createdAt,
    recalledAt: payload.recalledAt,
    read: false,
    canCopy: Boolean(payload.contentText),
    canRecall: false,
    canRecallAndEdit: false,
    canForward: Boolean(payload.contentText),
    canDelete: true,
    deliveryStatus: "sent",
  };
}

export function useChatRoom(code: string, selfId: string) {
  const queryClient = useQueryClient();
  const [peerUserId, setPeerUserId] = useState<string>();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [messagePage, setMessagePage] = useState<ChatMessagePage>();
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [messageError, setMessageError] = useState<unknown>();
  const [pendingMessages, setPendingMessages] = useState<Record<string, ChatMessage>>({});
  const activePeerRef = useRef(peerUserId);
  activePeerRef.current = peerUserId;
  const conversationsQuery = useQuery({ queryKey: ["chat-conversations", code], queryFn: () => listChatConversations(code), enabled: Boolean(code), retry: false, staleTime: 2_000 });

  const loadMessages = useCallback(async (peer: string, params: { beforeSequence?: number; aroundSequence?: number } = {}) => {
    setLoadingMessages(true);
    setMessageError(undefined);
    try {
      const page = await getChatMessages(code, peer, { ...params, limit: 30 });
      if (params.beforeSequence !== undefined) {
        setMessages((current) => mergeChatMessages(page.items, current));
      } else {
        setMessages(page.items);
      }
      setMessagePage((current) => params.beforeSequence !== undefined && current ? { ...page, items: mergeChatMessages(page.items, current.items) } : page);
      return page;
    } catch (error) {
      setMessageError(error);
      throw error;
    } finally {
      setLoadingMessages(false);
    }
  }, [code]);

  const openConversation = useCallback((peer: string) => {
    setPeerUserId(peer);
    setMessages([]);
    setMessagePage(undefined);
    void loadMessages(peer);
  }, [loadMessages]);

  const send = useCallback(async (contentText: string, recipient = peerUserId) => {
    if (!recipient) throw new Error("chat recipient is required");
    const clientMessageId = newChatClientMessageID();
    const optimistic: ChatMessage = { roomCode: code, conversationId: "", messageId: `optimistic-${clientMessageId}`, clientMessageId, senderUserId: selfId, recipientUserId: recipient, senderDisplayName: "", contentText, isForwarded: false, sequence: Number.MAX_SAFE_INTEGER, createdAt: Math.floor(Date.now() / 1000), read: false, canCopy: true, canRecall: false, canRecallAndEdit: false, canForward: true, canDelete: true, deliveryStatus: "sending", optimistic: true };
    setPendingMessages((current) => ({ ...current, [clientMessageId]: optimistic }));
    setMessages((current) => [...current, optimistic]);
    try {
      const result = await sendChatMessage(code, recipient, clientMessageId, contentText);
      setPendingMessages((current) => { const next = { ...current }; delete next[clientMessageId]; return next; });
      setMessages((current) => mergeChatMessage(current, { ...result, deliveryStatus: result.read ? "read" : "sent" }));
      void queryClient.invalidateQueries({ queryKey: ["chat-conversations", code] });
      return result;
    } catch (error) {
      setPendingMessages((current) => ({ ...current, [clientMessageId]: { ...optimistic, deliveryStatus: "failed" } }));
      setMessages((current) => current.map((message) => message.clientMessageId === clientMessageId ? { ...message, deliveryStatus: "failed" } : message));
      throw error;
    }
  }, [code, peerUserId, queryClient, selfId]);

  const markRead = useCallback(async (sequence: number) => {
    if (!peerUserId) return;
    const state = await markChatRead(code, peerUserId, sequence);
    setMessagePage((current) => current ? { ...current, currentReadSequence: mergeReadSequence(current.currentReadSequence, state.lastReadSequence) } : current);
    void queryClient.invalidateQueries({ queryKey: ["chat-conversations", code] });
  }, [code, peerUserId, queryClient]);

  const recall = useCallback(async (messageId: string) => {
    const result = await recallChatMessage(code, messageId);
    setMessages((current) => current.map((message) => message.messageId === messageId ? { ...message, ...result, deliveryStatus: message.deliveryStatus } : message));
    return result;
  }, [code]);

  const remove = useCallback(async (messageId: string) => {
    await deleteChatMessage(code, messageId);
    setMessages((current) => current.filter((message) => message.messageId !== messageId));
  }, [code]);

  const forward = useCallback(async (messageId: string, recipientUserIds: string[]) => {
    const result = await forwardChatMessage(code, messageId, newChatClientMessageID(), recipientUserIds);
    if (peerUserId) setMessages((current) => mergeChatMessages(current, result.items.filter((item) => item.recipientUserId === peerUserId)));
    void queryClient.invalidateQueries({ queryKey: ["chat-conversations", code] });
    return result.items;
  }, [code, peerUserId, queryClient]);

  const search = useCallback((query: string, beforeSequence?: number): Promise<ChatSearchPage> => {
    if (!peerUserId) return Promise.resolve({ items: [], hasMoreBefore: false });
    return searchChatMessages(code, peerUserId, { query, beforeSequence, limit: 30 });
  }, [code, peerUserId]);

  useEffect(() => {
    const listener = (raw: Event) => {
      const event = (raw as CustomEvent<StreamEventMessage>).detail?.event;
      if (!event) return;
      const payload = (event.payload ?? {}) as ChatPayload;
      if (payload.roomCode !== code) return;
      if (event.type === "chat.message_created") {
        const item = eventMessage(payload);
        const peer = activePeerRef.current;
        const belongsToActiveConversation = Boolean(item && peer && ((item.senderUserId === peer && item.recipientUserId === selfId) || (item.senderUserId === selfId && item.recipientUserId === peer)));
        if (item && belongsToActiveConversation) setMessages((current) => mergeChatMessage(current, item));
        void queryClient.invalidateQueries({ queryKey: ["chat-conversations", code] });
      } else if (event.type === "chat.message_recalled" && payload.messageId) {
        setMessages((current) => current.map((message) => message.messageId === payload.messageId ? { ...message, contentText: "", recalledAt: payload.recalledAt, canCopy: false, canForward: false } : message));
      } else if (event.type === "chat.message_deleted_local" && payload.messageId) {
        setMessages((current) => current.filter((message) => message.messageId !== payload.messageId));
      } else if (event.type === "chat.read_updated" && payload.userId && typeof payload.lastReadSequence === "number") {
        const readUserID = payload.userId;
        const readSequence = payload.lastReadSequence;
        setMessagePage((current) => current ? { ...current, peerReadSequence: readUserID === activePeerRef.current ? mergeReadSequence(current.peerReadSequence, readSequence) : current.peerReadSequence } : current);
        setMessages((current) => readUserID === activePeerRef.current ? markChatMessagesRead(current, readSequence, selfId) : current);
      }
    };
    window.addEventListener(streamEventName, listener);
    return () => window.removeEventListener(streamEventName, listener);
  }, [code, queryClient, selfId]);

  const conversationItems = conversationsQuery.data?.items ?? [];
  const visibleMessages = useMemo(() => mergeChatMessages(messages, Object.values(pendingMessages)), [messages, pendingMessages]);
  return { conversations: conversationItems, conversationsQuery, peerUserId, openConversation, messages: visibleMessages, messagePage, loadingMessages, messageError, loadOlder: () => peerUserId && messagePage?.hasMoreBefore && messagePage.previousCursor ? loadMessages(peerUserId, { beforeSequence: messagePage.previousCursor }) : Promise.resolve(undefined), send, markRead, recall, remove, forward, search, refresh: () => void conversationsQuery.refetch() };
}
