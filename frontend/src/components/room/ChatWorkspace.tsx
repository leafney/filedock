import { AlertCircle, ArrowLeft, Check, CheckCheck, Clock3, Copy, Edit3, Forward, History, Loader2, MessageSquare, Search, Send, Trash2, Undo2, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type FormEvent, type KeyboardEvent, type MouseEvent } from "react";
import { useTranslation } from "react-i18next";

import { ErrorNotice } from "../common";
import { useChatRoom } from "../../hooks/use-chat";
import type { ChatMessage, RoomMember } from "../../types/domain";
import { chatMessageCapabilities, chatDateKey, chatTimeBucket, isChatNearBottom, shouldMarkChatRead } from "../../utils/chat";
import { readChatDraft, writeChatDraft } from "../../utils/chat-drafts";
import { copyText } from "../../utils/clipboard";

interface Props {
  roomId: string;
  code: string;
  members: RoomMember[];
  selfId: string;
  mode: "desktop" | "mobile";
  selectedPeerUserId?: string;
  launchPeerUserId?: string;
  launchToken?: string;
  onCloseConversation?: () => void;
  onUnreadCount?: (count: number) => void;
  onUnreadByPeer?: (counts: Record<string, number>) => void;
}

type MenuPoint = { clientX: number; clientY: number };

export function ChatWorkspace({ roomId, code, members, selfId, mode, selectedPeerUserId, launchPeerUserId, launchToken, onCloseConversation, onUnreadCount, onUnreadByPeer }: Props) {
  const { t } = useTranslation();
  const chat = useChatRoom(code, selfId);
  const desktop = mode === "desktop";
  const [mobileDraft, setMobileDraft] = useState("");
  const [desktopDrafts, setDesktopDrafts] = useState<Record<string, string>>({});
  const desktopDraftsRef = useRef<Record<string, string>>({});
  const loadedDesktopDrafts = useRef(new Set<string>());
  const [actionError, setActionError] = useState<unknown>();
  const [menu, setMenu] = useState<{ message: ChatMessage; x: number; y: number }>();
  const [copied, setCopied] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [historyQuery, setHistoryQuery] = useState("");
  const [historyResults, setHistoryResults] = useState<ChatMessage[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyError, setHistoryError] = useState<unknown>();
  const [forwardMessage, setForwardMessage] = useState<ChatMessage>();
  const [forwardTargets, setForwardTargets] = useState<string[]>([]);
  const [highlightMessageId, setHighlightMessageId] = useState<string>();
  const listRef = useRef<HTMLDivElement>(null);
  const wasNearBottom = useRef(true);
  const handledLaunchToken = useRef<string>();

  useEffect(() => {
    if (!desktop) return;
    if (selectedPeerUserId && selectedPeerUserId !== chat.peerUserId) {
      chat.openConversation(selectedPeerUserId);
      return;
    }
    if (!selectedPeerUserId && chat.peerUserId) chat.openConversation("");
  }, [chat.openConversation, chat.peerUserId, desktop, selectedPeerUserId]);

  useEffect(() => {
    if (desktop || !launchPeerUserId || !launchToken || handledLaunchToken.current === launchToken) return;
    handledLaunchToken.current = launchToken;
    chat.openConversation(launchPeerUserId);
  }, [chat.openConversation, desktop, launchPeerUserId, launchToken]);

  const peers = useMemo(() => {
    const active = members.filter((member) => member.userId !== selfId && member.status === "active");
    return active.map((member) => ({ member, conversation: chat.conversations.find((item) => item.peerUserId === member.userId) }));
  }, [chat.conversations, members, selfId]);
  const activePeerUserId = desktop ? selectedPeerUserId : chat.peerUserId;
  const peer = members.find((member) => member.userId === activePeerUserId && member.status === "active");
  const conversationReady = Boolean(activePeerUserId && chat.peerUserId === activePeerUserId);
  const draft = desktop && activePeerUserId ? desktopDrafts[activePeerUserId] ?? "" : mobileDraft;
  const unreadByPeer = useMemo(() => Object.fromEntries(peers.map(({ member, conversation }) => [member.userId, conversation?.unreadCount ?? 0])), [peers]);
  const activePeerUserIdRef = useRef(activePeerUserId);
  activePeerUserIdRef.current = activePeerUserId;

  useEffect(() => {
    onUnreadCount?.(peers.reduce((total, item) => total + (item.conversation?.unreadCount ?? 0), 0));
    onUnreadByPeer?.(unreadByPeer);
  }, [onUnreadByPeer, onUnreadCount, peers, unreadByPeer]);

  useEffect(() => {
    loadedDesktopDrafts.current.clear();
    desktopDraftsRef.current = {};
    setDesktopDrafts({});
  }, [roomId, selfId]);

  useEffect(() => {
    if (!desktop || !activePeerUserId || loadedDesktopDrafts.current.has(activePeerUserId)) return;
    loadedDesktopDrafts.current.add(activePeerUserId);
    const content = readChatDraft({ roomId, selfId, peerId: activePeerUserId });
    desktopDraftsRef.current = { ...desktopDraftsRef.current, [activePeerUserId]: content };
    setDesktopDrafts((current) => ({ ...current, [activePeerUserId]: content }));
  }, [activePeerUserId, desktop, roomId, selfId]);

  useEffect(() => {
    setMenu(undefined);
    setHistoryOpen(false);
    setHistoryQuery("");
    setHistoryResults([]);
    setHistoryError(undefined);
    setForwardMessage(undefined);
    setForwardTargets([]);
    setActionError(undefined);
  }, [activePeerUserId]);

  useEffect(() => {
    const element = listRef.current;
    if (!element) return;
    if (wasNearBottom.current) element.scrollTop = element.scrollHeight;
  }, [activePeerUserId, chat.messages.length]);

  useEffect(() => {
    if (!peer || !conversationReady || chat.messages.length === 0) return;
    const lastIncoming = [...chat.messages].reverse().find((message) => message.recipientUserId === selfId && !message.recalledAt);
    if (!lastIncoming) return;
    const mark = () => {
      if (shouldMarkChatRead(Boolean(chat.peerUserId), document.visibilityState === "visible", document.hasFocus())) void chat.markRead(lastIncoming.sequence).catch(() => undefined);
    };
    mark();
    window.addEventListener("focus", mark);
    document.addEventListener("visibilitychange", mark);
    return () => { window.removeEventListener("focus", mark); document.removeEventListener("visibilitychange", mark); };
  }, [chat.markRead, chat.messages, chat.peerUserId, conversationReady, peer, selfId]);

  useEffect(() => {
    if (!highlightMessageId) return;
    const timer = window.setTimeout(() => {
      const target = listRef.current?.querySelector<HTMLElement>(`[data-message-id="${highlightMessageId}"]`);
      target?.scrollIntoView({ block: "center", behavior: "smooth" });
      target?.classList.add("is-highlighted");
      window.setTimeout(() => target?.classList.remove("is-highlighted"), 1_600);
      setHighlightMessageId(undefined);
    }, 0);
    return () => window.clearTimeout(timer);
  }, [chat.messages, highlightMessageId]);

  useEffect(() => {
    if (!historyOpen || !chat.peerUserId) return undefined;
    const timer = window.setTimeout(() => {
      const query = historyQuery.trim();
      if (!query) {
        setHistoryResults([]);
        setHistoryLoading(false);
        setHistoryError(undefined);
        return;
      }
      setHistoryLoading(true);
      setHistoryError(undefined);
      void chat.search(query).then((page) => setHistoryResults(page.items)).catch(setHistoryError).finally(() => setHistoryLoading(false));
    }, 300);
    return () => window.clearTimeout(timer);
  }, [chat.peerUserId, chat.search, historyOpen, historyQuery]);

  const updateDraft = (peerUserId: string, value: string) => {
    if (!desktop) {
      setMobileDraft(value);
      return;
    }
    desktopDraftsRef.current = { ...desktopDraftsRef.current, [peerUserId]: value };
    setDesktopDrafts((current) => ({ ...current, [peerUserId]: value }));
    writeChatDraft({ roomId, selfId, peerId: peerUserId }, value);
  };

  const restoreFailedDraft = (peerUserId: string, value: string) => {
    if (!desktop) {
      setMobileDraft((current) => current || value);
      return;
    }
    if (!desktopDraftsRef.current[peerUserId]) updateDraft(peerUserId, value);
  };

  const submit = async (event?: FormEvent) => {
    event?.preventDefault();
    const recipient = activePeerUserId;
    const content = draft.trim();
    if (!content || !recipient || !conversationReady) return;
    setActionError(undefined);
    updateDraft(recipient, "");
    try {
      await chat.send(content, recipient);
    } catch (error) {
      restoreFailedDraft(recipient, content);
      if (activePeerUserIdRef.current === recipient) setActionError(error);
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
    openMenuAt(event, message);
  };

  const openMenuAt = (event: MenuPoint, message: ChatMessage) => {
    const width = 190;
    const height = 250;
    setMenu({ message, x: Math.min(event.clientX, window.innerWidth - width - 8), y: Math.min(event.clientY, window.innerHeight - height - 8) });
  };

  const runRecallAndEdit = async (message: ChatMessage) => {
    if (!activePeerUserId) return;
    const content = message.contentText;
    if (draft.trim() && !window.confirm(t("chat.replaceDraftConfirm"))) return;
    setMenu(undefined);
    try {
      await chat.recall(message.messageId);
      updateDraft(activePeerUserId, content);
    } catch (error) {
      setActionError(error);
    }
  };

  const deleteMessage = async (message: ChatMessage) => {
    setMenu(undefined);
    if (!window.confirm(t("chat.deleteConfirm"))) return;
    try {
      await chat.remove(message.messageId);
    } catch (error) {
      setActionError(error);
    }
  };

  const chooseHistoryResult = async (message: ChatMessage) => {
    if (!chat.peerUserId) return;
    try {
      await chat.loadMessages(chat.peerUserId, { aroundSequence: message.sequence });
      setHistoryOpen(false);
      setHighlightMessageId(message.messageId);
    } catch (error) {
      setHistoryError(error);
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

  if (!activePeerUserId || !peer) {
    if (desktop) {
      return <aside className="room-chat-workspace room-chat-empty">
        <MessageSquare aria-hidden="true" />
        <p>{t("chat.selectMember")}</p>
      </aside>;
    }
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
      {!desktop && <button className="chat-back-button" type="button" aria-label={t("chat.backToConversations")} onClick={() => { setMenu(undefined); chat.openConversation(""); }}><ArrowLeft aria-hidden="true" /></button>}
      <span className="room-avatar">{peer.displayName.slice(0, 1)}</span>
      <div className="chat-active-peer"><strong>{peer.displayName}</strong><small><i className={`presence ${peer.onlineStatus}`} />{peer.onlineStatus === "online" ? t("room.online") : peer.onlineStatus === "away" ? t("room.away") : t("room.offline")}</small></div>
      <div className="chat-heading-actions">
        <button className="chat-history-button" type="button" disabled={!conversationReady} aria-label={t("chat.history")} title={t("chat.history")} onClick={() => setHistoryOpen(true)}><History aria-hidden="true" /></button>
        {desktop && <button className="chat-close-button" type="button" aria-label={t("chat.closeConversation")} title={t("chat.closeConversation")} onClick={() => { setMenu(undefined); setHistoryOpen(false); setForwardMessage(undefined); onCloseConversation?.(); }}><X aria-hidden="true" /></button>}
      </div>
    </header>
    <div ref={listRef} className="chat-message-list" onScroll={(event) => { const element = event.currentTarget; wasNearBottom.current = isChatNearBottom(element.scrollTop, element.scrollHeight, element.clientHeight); }} onClick={() => setMenu(undefined)}>
      {(!conversationReady || chat.loadingMessages) && <p className="chat-state-message">{t("chat.loading")}</p>}
      {conversationReady && chat.messageError != null && <ErrorNotice error={chat.messageError} />}
      {conversationReady && !chat.loadingMessages && chat.messages.length === 0 && <p className="chat-empty-state">{t("chat.empty")}</p>}
      {conversationReady && chat.messagePage?.hasMoreBefore && <button className="chat-load-older" type="button" onClick={() => void chat.loadOlder()}>{t("chat.loadOlder")}</button>}
      {conversationReady && chat.messages.map((message, index) => <ChatMessageRow key={message.messageId || message.clientMessageId} message={message} selfId={selfId} previous={chat.messages[index - 1]} onContextMenu={openMenu} onLongPress={(point, item) => openMenuAt(point, item)} />)}
      {copied && <span className="chat-copy-toast">{t("chat.copied")}</span>}
    </div>
    {actionError != null && <div className="chat-action-error"><ErrorNotice error={actionError} /></div>}
    <form className="chat-composer" onSubmit={(event) => void submit(event)}>
      <textarea value={draft} disabled={!conversationReady} maxLength={2000} placeholder={t("chat.inputPlaceholder")} aria-label={t("chat.inputLabel")} onChange={(event) => { if (activePeerUserId) updateDraft(activePeerUserId, event.target.value); }} onKeyDown={onInputKeyDown} />
      <div><span>{t("chat.inputHint")}</span><button type="submit" disabled={!conversationReady || !draft.trim()}><Send aria-hidden="true" />{t("chat.send")}</button></div>
    </form>
    {menu && <ChatContextMenu menu={menu} selfId={selfId} onClose={() => setMenu(undefined)} onCopy={() => void copyMessage(menu.message)} onRecall={() => void (async () => { setMenu(undefined); try { await chat.recall(menu.message.messageId); } catch (error) { setActionError(error); } })()} onRecallAndEdit={() => void runRecallAndEdit(menu.message)} onForward={() => { setMenu(undefined); setForwardMessage(menu.message); setForwardTargets([]); }} onDelete={() => void deleteMessage(menu.message)} />}
    {historyOpen && <ChatHistoryDrawer query={historyQuery} results={historyResults} loading={historyLoading} error={historyError} onQueryChange={setHistoryQuery} onClose={() => setHistoryOpen(false)} onSelect={(message) => void chooseHistoryResult(message)} />}
    {forwardMessage && <ChatForwardPicker message={forwardMessage} members={members} selfId={selfId} selected={forwardTargets} onSelectedChange={setForwardTargets} onClose={() => setForwardMessage(undefined)} onSubmit={async () => { try { await chat.forward(forwardMessage.messageId, forwardTargets); setForwardMessage(undefined); } catch (error) { setActionError(error); } }} />}
  </aside>;
}

function ChatMessageRow({ message, selfId, previous, onContextMenu, onLongPress }: { message: ChatMessage; selfId: string; previous?: ChatMessage; onContextMenu: (event: MouseEvent, message: ChatMessage) => void; onLongPress: (event: MenuPoint, message: ChatMessage) => void }) {
  const { t } = useTranslation();
  const [longPressTimer, setLongPressTimer] = useState<number>();
  const own = message.senderUserId === selfId;
  const showDate = !previous || chatDateKey(previous.createdAt) !== chatDateKey(message.createdAt);
  const showTime = !previous || message.createdAt - previous.createdAt >= 300;
  const recalled = Boolean(message.recalledAt);
  const status = message.deliveryStatus;
  const beginLongPress = (event: MouseEvent) => { if (event.button !== 0) return; const timer = window.setTimeout(() => onLongPress({ clientX: event.clientX, clientY: event.clientY }, message), 550); setLongPressTimer(timer); };
  const beginTouchLongPress = (event: React.TouchEvent<HTMLElement>) => { const touch = event.touches[0]; const timer = window.setTimeout(() => onLongPress({ clientX: touch?.clientX ?? 0, clientY: touch?.clientY ?? 0 }, message), 550); setLongPressTimer(timer); };
  const clearLongPress = () => { if (longPressTimer) window.clearTimeout(longPressTimer); setLongPressTimer(undefined); };
  return <>
    {showDate && <div className="chat-date-divider"><span>{chatDateKey(message.createdAt)}</span></div>}
    {showTime && !showDate && <div className="chat-time-divider"><span>{chatTimeBucket(message.createdAt)}</span></div>}
    <article data-message-id={message.messageId} className={`chat-message-row ${own ? "is-own" : "is-peer"}`} onContextMenu={(event) => onContextMenu(event, message)} onMouseDown={beginLongPress} onMouseUp={clearLongPress} onMouseLeave={clearLongPress} onTouchStart={beginTouchLongPress} onTouchEnd={clearLongPress}>
      {!own && <span className="room-avatar chat-message-avatar">{message.senderDisplayName.slice(0, 1)}</span>}
      <div className="chat-message-stack">
        <div className={`chat-bubble ${recalled ? "is-recalled" : ""}`}><span>{recalled ? t("chat.recalled") : message.contentText}</span>{message.isForwarded && !recalled && <small>{t("chat.forwarded")}</small>}</div>
        <div className="chat-message-meta"><time>{chatTimeBucket(message.createdAt)}</time>{own && <span aria-label={status === "read" ? t("chat.read") : status === "sending" ? t("chat.sending") : status === "failed" ? t("chat.failed") : t("chat.sent")}>{status === "read" ? <CheckCheck aria-hidden="true" /> : status === "sent" ? <Check aria-hidden="true" /> : status === "sending" ? <Clock3 aria-hidden="true" /> : status === "failed" ? <AlertCircle aria-hidden="true" /> : null}</span>}</div>
      </div>
    </article>
  </>;
}

function ChatHistoryDrawer({ query, results, loading, error, onQueryChange, onClose, onSelect }: { query: string; results: ChatMessage[]; loading: boolean; error?: unknown; onQueryChange: (value: string) => void; onClose: () => void; onSelect: (message: ChatMessage) => void }) {
  const { t } = useTranslation();
  return <div className="chat-history-overlay" role="dialog" aria-modal="true" aria-label={t("chat.history")}>
    <section className="chat-history-drawer">
      <header><div><span>{t("chat.historyEyebrow")}</span><h2>{t("chat.history")}</h2></div><button type="button" aria-label={t("chat.closeHistory")} onClick={onClose}><X aria-hidden="true" /></button></header>
      <label className="chat-history-search"><Search aria-hidden="true" /><span className="visually-hidden">{t("chat.search")}</span><input autoFocus value={query} placeholder={t("chat.searchPlaceholder")} onChange={(event) => onQueryChange(event.target.value)} /></label>
      {loading && <p className="chat-history-state"><Loader2 className="is-spinning" aria-hidden="true" />{t("chat.searching")}</p>}
      {error != null && <div className="chat-history-error"><ErrorNotice error={error} /></div>}
      {!loading && error == null && query.trim() === "" && <p className="chat-history-state">{t("chat.historyHint")}</p>}
      {!loading && error == null && query.trim() !== "" && results.length === 0 && <p className="chat-history-state">{t("chat.searchNoResults")}</p>}
      <div className="chat-history-results">{results.map((message) => <button key={message.messageId} type="button" onClick={() => onSelect(message)}><strong>{message.senderDisplayName}</strong><time>{chatTimeBucket(message.createdAt)}</time><span>{message.recalledAt ? t("chat.messageUnavailable") : message.contentText}</span></button>)}</div>
    </section>
  </div>;
}

function ChatForwardPicker({ message, members, selfId, selected, onSelectedChange, onClose, onSubmit }: { message: ChatMessage; members: RoomMember[]; selfId: string; selected: string[]; onSelectedChange: (values: string[]) => void; onClose: () => void; onSubmit: () => Promise<void> }) {
  const { t } = useTranslation();
  const candidates = members.filter((member) => member.userId !== selfId && member.status === "active");
  const toggle = (userId: string) => onSelectedChange(selected.includes(userId) ? selected.filter((id) => id !== userId) : [...selected, userId]);
  return <div className="chat-forward-overlay" role="dialog" aria-modal="true" aria-label={t("chat.forwardTitle")}>
    <section className="chat-forward-picker"><header><div><span>{t("chat.forwardEyebrow")}</span><h2>{t("chat.forwardTitle")}</h2></div><button type="button" aria-label={t("chat.closeForward")} onClick={onClose}><X aria-hidden="true" /></button></header>
      <p>{t("chat.forwardHint")}</p>
      <div className="chat-forward-preview">{message.contentText}</div>
      <fieldset><legend>{t("chat.forwardRecipients")}</legend>{candidates.map((member) => <label key={member.userId}><input type="checkbox" checked={selected.includes(member.userId)} onChange={() => toggle(member.userId)} /><span className="room-avatar">{member.displayName.slice(0, 1)}</span><strong>{member.displayName}</strong></label>)}</fieldset>
      <footer><button type="button" onClick={onClose}>{t("chat.cancel")}</button><button className="primary" type="button" disabled={selected.length === 0} onClick={() => void onSubmit()}><Forward aria-hidden="true" />{t("chat.forwardConfirm")}</button></footer>
    </section>
  </div>;
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
