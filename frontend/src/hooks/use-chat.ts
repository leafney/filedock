import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { streamEventName, type StreamEventMessage } from "./use-stream";
import { deleteChatMessage, forwardChatMessage, getChatMessages, listChatConversations, markChatRead, recallChatMessage, searchChatMessages, sendChatMessage } from "../services/api";
import type { ChatConversation, ChatMessage, ChatMessagePage, ChatSearchPage } from "../types/domain";
import { mergeChatMessage, mergeChatMessages, markChatMessagesRead, mergeReadSequence, newChatClientMessageID } from "../utils/chat";
import { addPendingHistoryMessage, containsHistoryTarget, isIncomingForHistory, mergeChatMessagePage, type ChatHistoryAnchor, type ChatHistoryMode, type ChatPageDirection } from "../utils/chat-history";

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

function normalizeMessages(items: ChatMessage[]): ChatMessage[] {
  return items.map((item) => ({ ...item, optimistic: false, deliveryStatus: item.read ? "read" : "sent" }));
}

export function useChatRoom(code: string, selfId: string) {
  const queryClient = useQueryClient();
  const [peerUserId, setPeerUserId] = useState<string>();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [messagePage, setMessagePage] = useState<ChatMessagePage>();
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [messageError, setMessageError] = useState<unknown>();
  const [loadingNewer, setLoadingNewer] = useState(false);
  const [newerError, setNewerError] = useState<unknown>();
  const [pendingMessages, setPendingMessages] = useState<Record<string, ChatMessage>>({});
  const [historyMode, setHistoryMode] = useState<ChatHistoryMode>("live");
  const [historyAnchor, setHistoryAnchor] = useState<ChatHistoryAnchor>();
  const [pendingHistoryMessageIds, setPendingHistoryMessageIds] = useState<string[]>([]);
  const activePeerRef = useRef(peerUserId);
  const historyModeRef = useRef<ChatHistoryMode>("live");
  const historyAnchorRef = useRef<ChatHistoryAnchor>();
  const messageLoadGeneration = useRef(0);
  const loadingNewerRef = useRef(false);
  activePeerRef.current = peerUserId;
  const conversationsQuery = useQuery({ queryKey: ["chat-conversations", code], queryFn: () => listChatConversations(code), enabled: Boolean(code), retry: false, staleTime: 2_000 });

  const updateHistoryMode = useCallback((mode: ChatHistoryMode, anchor?: ChatHistoryAnchor) => {
    historyModeRef.current = mode;
    historyAnchorRef.current = anchor;
    setHistoryMode(mode);
    setHistoryAnchor(anchor);
  }, []);

  const clearPendingHistoryMessages = useCallback(() => setPendingHistoryMessageIds([]), []);

  const loadMessages = useCallback(async (peer: string, params: { beforeSequence?: number; afterSequence?: number; aroundSequence?: number } = {}) => {
    const direction: ChatPageDirection = params.beforeSequence !== undefined ? "before" : params.afterSequence !== undefined ? "after" : "replace";
    if (direction === "replace") messageLoadGeneration.current += 1;
    const generation = messageLoadGeneration.current;
    const isCurrentConversation = () => activePeerRef.current === peer && messageLoadGeneration.current === generation;
    setLoadingMessages(true);
    setMessageError(undefined);
    try {
      const page = await getChatMessages(code, peer, { ...params, limit: params.aroundSequence !== undefined ? 31 : 30 });
      if (!isCurrentConversation()) return page;
      const loadedItems = normalizeMessages(page.items);
      const normalizedPage = { ...page, items: loadedItems };
      setMessages((current) => direction === "before" ? mergeChatMessages(loadedItems, current) : direction === "after" ? mergeChatMessages(current, loadedItems) : loadedItems);
      setMessagePage((current) => mergeChatMessagePage(current, normalizedPage, direction));
      if (direction === "replace" && params.aroundSequence === undefined) {
        updateHistoryMode("live");
        clearPendingHistoryMessages();
      }
      return normalizedPage;
    } catch (error) {
      if (isCurrentConversation()) setMessageError(error);
      throw error;
    } finally {
      if (isCurrentConversation()) setLoadingMessages(false);
    }
  }, [clearPendingHistoryMessages, code, updateHistoryMode]);

  const openConversation = useCallback((peer: string) => {
    messageLoadGeneration.current += 1;
    activePeerRef.current = peer || undefined;
    setMessagePage(undefined);
    setLoadingMessages(false);
    setMessageError(undefined);
    setLoadingNewer(false);
    setNewerError(undefined);
    loadingNewerRef.current = false;
    updateHistoryMode("live");
    clearPendingHistoryMessages();
    if (!peer) {
      setPeerUserId(undefined);
      setMessages([]);
      return;
    }
    setPeerUserId(peer);
    setMessages([]);
    void loadMessages(peer).catch(() => undefined);
  }, [clearPendingHistoryMessages, loadMessages, updateHistoryMode]);

  const locateMessage = useCallback(async (message: ChatMessage) => {
    const peer = activePeerRef.current;
    if (!peer) throw new Error("chat recipient is required");
    const previousMode = historyModeRef.current;
    const previousAnchor = historyAnchorRef.current;
    updateHistoryMode("locating", previousAnchor);
    try {
      const page = await loadMessages(peer, { aroundSequence: message.sequence });
      if (!containsHistoryTarget(page, message.sequence, message.messageId)) throw new Error("chat history target unavailable");
      const anchor = { messageId: message.messageId, sequence: message.sequence, createdAt: message.createdAt };
      if (page.hasMoreAfter) updateHistoryMode("history", anchor);
      else updateHistoryMode("live");
      clearPendingHistoryMessages();
      return page;
    } catch (error) {
      updateHistoryMode(previousMode, previousAnchor);
      throw error;
    }
  }, [clearPendingHistoryMessages, loadMessages, updateHistoryMode]);

  const returnLatest = useCallback(async () => {
    const peer = activePeerRef.current;
    if (!peer) return undefined;
    return loadMessages(peer);
  }, [loadMessages]);

  const loadNewer = useCallback(async () => {
    const peer = activePeerRef.current;
    const page = messagePage;
    if (!peer || historyModeRef.current !== "history" || !page?.hasMoreAfter || !page.nextCursor || loadingNewerRef.current) return undefined;
    loadingNewerRef.current = true;
    setLoadingNewer(true);
    setNewerError(undefined);
    try {
      const result = await loadMessages(peer, { afterSequence: page.nextCursor });
      if (!result.hasMoreAfter) {
        updateHistoryMode("live");
        clearPendingHistoryMessages();
      }
      return result;
    } catch (error) {
      setNewerError(error);
      return undefined;
    } finally {
      loadingNewerRef.current = false;
      setLoadingNewer(false);
    }
  }, [clearPendingHistoryMessages, loadMessages, messagePage, updateHistoryMode]);

  const send = useCallback(async (contentText: string, recipient = peerUserId) => {
    if (!recipient) throw new Error("chat recipient is required");
    const sendingFromHistory = historyModeRef.current === "history" && activePeerRef.current === recipient;
    const clientMessageId = newChatClientMessageID();
    const optimistic: ChatMessage = { roomCode: code, conversationId: "", messageId: `optimistic-${clientMessageId}`, clientMessageId, senderUserId: selfId, recipientUserId: recipient, senderDisplayName: "", contentText, isForwarded: false, sequence: Number.MAX_SAFE_INTEGER, createdAt: Math.floor(Date.now() / 1000), read: false, canCopy: true, canRecall: false, canRecallAndEdit: false, canForward: true, canDelete: true, deliveryStatus: "sending", optimistic: true };
    if (!sendingFromHistory) {
      setPendingMessages((current) => ({ ...current, [clientMessageId]: optimistic }));
      if (activePeerRef.current === recipient) setMessages((current) => [...current, optimistic]);
    }
    try {
      const result = await sendChatMessage(code, recipient, clientMessageId, contentText);
      setPendingMessages((current) => { const next = { ...current }; delete next[clientMessageId]; return next; });
      if (!sendingFromHistory && activePeerRef.current === recipient) setMessages((current) => mergeChatMessage(current, { ...result, deliveryStatus: result.read ? "read" : "sent" }));
      void queryClient.invalidateQueries({ queryKey: ["chat-conversations", code] });
      if (sendingFromHistory) void loadMessages(recipient).catch(setMessageError);
      return result;
    } catch (error) {
      if (!sendingFromHistory) {
        setPendingMessages((current) => ({ ...current, [clientMessageId]: { ...optimistic, deliveryStatus: "failed" } }));
        if (activePeerRef.current === recipient) setMessages((current) => current.map((message) => message.clientMessageId === clientMessageId ? { ...message, deliveryStatus: "failed" } : message));
      }
      throw error;
    }
  }, [code, loadMessages, peerUserId, queryClient, selfId]);

  const markRead = useCallback(async (sequence: number) => {
    if (!peerUserId || historyModeRef.current !== "live") return;
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
    const sourcePeer = peerUserId;
    const result = await forwardChatMessage(code, messageId, newChatClientMessageID(), recipientUserIds);
    if (sourcePeer && activePeerRef.current === sourcePeer) setMessages((current) => mergeChatMessages(current, result.items.filter((item) => item.recipientUserId === sourcePeer)));
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
        if (item && belongsToActiveConversation) {
          if (historyModeRef.current === "history" && isIncomingForHistory(item, peer, selfId)) {
            setPendingHistoryMessageIds((current) => addPendingHistoryMessage(current, item.messageId));
          } else if (historyModeRef.current !== "locating") {
            setMessages((current) => mergeChatMessage(current, item));
          }
        }
        void queryClient.invalidateQueries({ queryKey: ["chat-conversations", code] });
      } else if (event.type === "chat.message_recalled" && payload.messageId) {
        setMessages((current) => current.map((message) => message.messageId === payload.messageId ? { ...message, contentText: "", recalledAt: payload.recalledAt, canCopy: false, canForward: false } : message));
        void queryClient.invalidateQueries({ queryKey: ["chat-conversations", code] });
      } else if (event.type === "chat.message_deleted_local" && payload.messageId) {
        setMessages((current) => current.filter((message) => message.messageId !== payload.messageId));
        void queryClient.invalidateQueries({ queryKey: ["chat-conversations", code] });
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
  const visibleMessages = useMemo(() => mergeChatMessages(messages, Object.values(pendingMessages).filter((message) => message.recipientUserId === peerUserId || message.senderUserId === peerUserId)), [messages, peerUserId, pendingMessages]);
  return { conversations: conversationItems, conversationsQuery, peerUserId, openConversation, messages: visibleMessages, messagePage, loadingMessages, messageError, loadingNewer, newerError, historyMode, historyAnchor, pendingHistoryMessageCount: pendingHistoryMessageIds.length, locateMessage, returnLatest, loadMessages, loadOlder: () => peerUserId && messagePage?.hasMoreBefore && messagePage.previousCursor ? loadMessages(peerUserId, { beforeSequence: messagePage.previousCursor }) : Promise.resolve(undefined), loadNewer, send, markRead, recall, remove, forward, search, refresh: () => void conversationsQuery.refetch() };
}
