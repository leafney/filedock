import { AlertCircle, ArrowLeft, Check, CheckCheck, Clock3, Copy, Edit3, Forward, History, MessageSquare, MoreHorizontal, Send, Trash2, Undo2, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type FormEvent, type KeyboardEvent, type MouseEvent } from "react";
import { useTranslation } from "react-i18next";

import { ErrorNotice } from "../common";
import { useChatRoom } from "../../hooks/use-chat";
import type { ChatMessage, RoomMember } from "../../types/domain";
import { chatMessageCapabilities, chatDateKey, chatTimeBucket, isChatNearBottom, shouldMarkChatRead } from "../../utils/chat";
import { copyText } from "../../utils/clipboard";

interface Props {
  code: string;
  members: RoomMember[];
  selfId: string;
  initialPeerUserId?: string;
  onInitialPeerConsumed?: () => void;
}

export function ChatWorkspace({ code, members, selfId, initialPeerUserId, onInitialPeerConsumed }: Props) {
  const { t } = useTranslation();
  const chat = useChatRoom(code, selfId);
  const [draft, setDraft] = useState("");
  const [actionError, setActionError] = useState<unknown>();
  const [menu, setMenu] = useState<{ message: ChatMessage; x: number; y: number }>();
  const [copied, setCopied] = useState(false);
  const listRef = useRef<HTMLDivElement>(null);
  const wasNearBottom = useRef(true);

  useEffect(() => {
    if (!initialPeerUserId) return;
    chat.openConversation(initialPeerUserId);
    onInitialPeerConsumed?.();
  }, [chat.openConversation, initialPeerUserId, onInitialPeerConsumed]);

  const peers = useMemo(() => {
    const active = members.filter((member) => member.userId !== selfId && member.status === "active");
    return active.map((member) => ({ member, conversation: chat.conversations.find((item) => item.peerUserId === member.userId) }));
  }, [chat.conversations, members, selfId]);
  const peer = members.find((member) => member.userId === chat.peerUserId);

  useEffect(() => {
    const element = listRef.current;
    if (!element) return;
    if (wasNearBottom.current) element.scrollTop = element.scrollHeight;
  }, [chat.messages.length, chat.peerUserId]);

  useEffect(() => {
    if (!peer || chat.messages.length === 0) return;
    const lastIncoming = [...chat.messages].reverse().find((message) => message.recipientUserId === selfId && !message.recalledAt);
    if (!lastIncoming) return;
    const mark = () => {
      if (shouldMarkChatRead(Boolean(chat.peerUserId), document.visibilityState === "visible", document.hasFocus())) void chat.markRead(lastIncoming.sequence).catch(() => undefined);
    };
    mark();
    window.addEventListener("focus", mark);
    document.addEventListener("visibilitychange", mark);
    return () => { window.removeEventListener("focus", mark); document.removeEventListener("visibilitychange", mark); };
  }, [chat.markRead, chat.messages, chat.peerUserId, peer, selfId]);

  const submit = async (event?: FormEvent) => {
    event?.preventDefault();
    const content = draft.trim();
    if (!content || !chat.peerUserId) return;
    setActionError(undefined);
    setDraft("");
    try {
      await chat.send(content);
    } catch (error) {
      setDraft(content);
      setActionError(error);
    }
  };

  const onInputKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) {
      event.preventDefault();
      void submit();
    }
  };

  const openMenu = (event: MouseEvent, message: ChatMessage) => {
    event.preventDefault();
    const width = 190;
    const height = 250;
    setMenu({ message, x: Math.min(event.clientX, window.innerWidth - width - 8), y: Math.min(event.clientY, window.innerHeight - height - 8) });
  };

  const runRecallAndEdit = async (message: ChatMessage) => {
    const content = message.contentText;
    setMenu(undefined);
    try {
      await chat.recall(message.messageId);
      setDraft(content);
    } catch (error) {
      setActionError(error);
    }
  };

  const copyMessage = async (message: ChatMessage) => {
    setMenu(undefined);
    if (!await copyText(message.contentText)) {
      setActionError(new Error(t("chat.copyFailed")));
      return;
    }
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1_500);
  };

  if (!chat.peerUserId || !peer) {
    return <aside className="room-chat-workspace room-chat-conversations">
      <header className="chat-panel-heading"><div><span>{t("chat.eyebrow")}</span><h2>{t("chat.conversations")}</h2></div><MessageSquare aria-hidden="true" /></header>
      <div className="chat-conversation-list">
        {peers.length === 0 && <p className="chat-empty-state">{t("chat.noMembers")}</p>}
        {peers.map(({ member, conversation }) => <button className="chat-conversation-item" key={member.userId} type="button" onClick={() => chat.openConversation(member.userId)}>
          <span className="room-avatar">{member.displayName.slice(0, 1)}</span>
          <span className="chat-conversation-main"><strong>{member.displayName}</strong><small>{conversation?.lastMessagePreview || t("chat.startConversation")}</small></span>
          <span className="chat-conversation-meta"><i className={`presence ${member.onlineStatus}`} />{conversation?.unreadCount ? <b>{conversation.unreadCount > 99 ? "99+" : conversation.unreadCount}</b> : null}</span>
        </button>)}
      </div>
    </aside>;
  }

  return <aside className="room-chat-workspace room-chat-conversation">
    <header className="chat-panel-heading chat-active-heading">
      <button className="chat-back-button" type="button" aria-label={t("chat.backToConversations")} onClick={() => { setMenu(undefined); chat.openConversation(""); }}><ArrowLeft aria-hidden="true" /></button>
      <span className="room-avatar">{peer.displayName.slice(0, 1)}</span>
      <div className="chat-active-peer"><strong>{peer.displayName}</strong><small><i className={`presence ${peer.onlineStatus}`} />{peer.onlineStatus === "online" ? t("room.online") : peer.onlineStatus === "away" ? t("room.away") : t("room.offline")}</small></div>
      <button className="chat-history-button" type="button" aria-label={t("chat.history")} title={t("chat.history")}><History aria-hidden="true" /></button>
    </header>
    <div ref={listRef} className="chat-message-list" onScroll={(event) => { const element = event.currentTarget; wasNearBottom.current = isChatNearBottom(element.scrollTop, element.scrollHeight, element.clientHeight); }} onClick={() => setMenu(undefined)}>
      {chat.loadingMessages && <p className="chat-state-message">{t("chat.loading")}</p>}
      {chat.messageError != null && <ErrorNotice error={chat.messageError} />}
      {!chat.loadingMessages && chat.messages.length === 0 && <p className="chat-empty-state">{t("chat.empty")}</p>}
      {chat.messagePage?.hasMoreBefore && <button className="chat-load-older" type="button" onClick={() => void chat.loadOlder()}>{t("chat.loadOlder")}</button>}
      {chat.messages.map((message, index) => <ChatMessageRow key={message.messageId || message.clientMessageId} message={message} selfId={selfId} previous={chat.messages[index - 1]} onContextMenu={openMenu} onLongPress={openMenu} />)}
      {copied && <span className="chat-copy-toast">{t("chat.copied")}</span>}
    </div>
    {actionError != null && <div className="chat-action-error"><ErrorNotice error={actionError} /></div>}
    <form className="chat-composer" onSubmit={(event) => void submit(event)}>
      <textarea value={draft} maxLength={2000} placeholder={t("chat.inputPlaceholder")} aria-label={t("chat.inputLabel")} onChange={(event) => setDraft(event.target.value)} onKeyDown={onInputKeyDown} />
      <div><span>{t("chat.inputHint")}</span><button type="submit" disabled={!draft.trim()}><Send aria-hidden="true" />{t("chat.send")}</button></div>
    </form>
    {menu && <ChatContextMenu menu={menu} selfId={selfId} onClose={() => setMenu(undefined)} onCopy={() => void copyMessage(menu.message)} onRecall={() => void (async () => { setMenu(undefined); try { await chat.recall(menu.message.messageId); } catch (error) { setActionError(error); } })()} onRecallAndEdit={() => void runRecallAndEdit(menu.message)} onForward={() => { setMenu(undefined); setActionError(new Error(t("chat.forwardLater"))); }} onDelete={() => void (async () => { setMenu(undefined); try { await chat.remove(menu.message.messageId); } catch (error) { setActionError(error); } })()} />}
  </aside>;
}

function ChatMessageRow({ message, selfId, previous, onContextMenu, onLongPress }: { message: ChatMessage; selfId: string; previous?: ChatMessage; onContextMenu: (event: MouseEvent, message: ChatMessage) => void; onLongPress: (event: MouseEvent, message: ChatMessage) => void }) {
  const { t } = useTranslation();
  const [longPressTimer, setLongPressTimer] = useState<number>();
  const own = message.senderUserId === selfId;
  const showDate = !previous || chatDateKey(previous.createdAt) !== chatDateKey(message.createdAt);
  const showTime = !previous || message.createdAt - previous.createdAt >= 300;
  const recalled = Boolean(message.recalledAt);
  const status = message.deliveryStatus;
  const beginLongPress = (event: MouseEvent) => { if (event.button !== 0) return; const timer = window.setTimeout(() => onLongPress(event, message), 550); setLongPressTimer(timer); };
  const beginTouchLongPress = (event: React.TouchEvent<HTMLElement>) => { const timer = window.setTimeout(() => onLongPress(event as unknown as MouseEvent, message), 550); setLongPressTimer(timer); };
  const clearLongPress = () => { if (longPressTimer) window.clearTimeout(longPressTimer); setLongPressTimer(undefined); };
  return <>
    {showDate && <div className="chat-date-divider"><span>{chatDateKey(message.createdAt)}</span></div>}
    {showTime && !showDate && <div className="chat-time-divider"><span>{chatTimeBucket(message.createdAt)}</span></div>}
    <article className={`chat-message-row ${own ? "is-own" : "is-peer"}`} onContextMenu={(event) => onContextMenu(event, message)} onMouseDown={beginLongPress} onMouseUp={clearLongPress} onMouseLeave={clearLongPress} onTouchStart={beginTouchLongPress} onTouchEnd={clearLongPress}>
      {!own && <span className="room-avatar chat-message-avatar">{message.senderDisplayName.slice(0, 1)}</span>}
      <div className="chat-message-stack">
        <div className={`chat-bubble ${recalled ? "is-recalled" : ""}`}><span>{recalled ? t("chat.recalled") : message.contentText}</span>{message.isForwarded && !recalled && <small>{t("chat.forwarded")}</small>}</div>
        <div className="chat-message-meta"><time>{chatTimeBucket(message.createdAt)}</time>{own && <span aria-label={status === "read" ? t("chat.read") : status === "sending" ? t("chat.sending") : status === "failed" ? t("chat.failed") : t("chat.sent")}>{status === "read" ? <CheckCheck aria-hidden="true" /> : status === "sent" ? <Check aria-hidden="true" /> : status === "sending" ? <Clock3 aria-hidden="true" /> : status === "failed" ? <AlertCircle aria-hidden="true" /> : null}</span>}</div>
      </div>
    </article>
  </>;
}

function ChatContextMenu({ menu, selfId, onClose, onCopy, onRecall, onRecallAndEdit, onForward, onDelete }: { menu: { message: ChatMessage; x: number; y: number }; selfId: string; onClose: () => void; onCopy: () => void; onRecall: () => void; onRecallAndEdit: () => void; onForward: () => void; onDelete: () => void }) {
  const { t } = useTranslation();
  const capabilities = chatMessageCapabilities(menu.message, selfId);
  return <div className="chat-context-menu" style={{ left: menu.x, top: menu.y }} role="menu" onClick={(event) => event.stopPropagation()}>
    <button type="button" role="menuitem" disabled={!capabilities.copy} onClick={onCopy}><Copy aria-hidden="true" />{t("chat.copy")}</button>
    <button type="button" role="menuitem" disabled={!capabilities.recall} onClick={onRecall}><Undo2 aria-hidden="true" />{t("chat.recall")}</button>
    <button type="button" role="menuitem" disabled={!capabilities.recallAndEdit} onClick={onRecallAndEdit}><Edit3 aria-hidden="true" />{t("chat.recallAndEdit")}</button>
    <button type="button" role="menuitem" disabled={!capabilities.forward} onClick={onForward}><Forward aria-hidden="true" />{t("chat.forward")}</button>
    <button type="button" role="menuitem" disabled={!capabilities.delete} onClick={onDelete}><Trash2 aria-hidden="true" />{t("chat.deleteForMe")}</button>
    <button className="chat-context-close" type="button" aria-label={t("chat.closeMenu")} onClick={onClose}><X aria-hidden="true" /></button>
  </div>;
}
