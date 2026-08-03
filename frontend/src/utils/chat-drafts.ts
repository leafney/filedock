const chatDraftPrefix = "filedock.chat-draft.v1";

export const chatDraftTTL = 7 * 24 * 60 * 60 * 1_000;

export interface ChatDraftScope {
  roomId: string;
  selfId: string;
  peerId: string;
}

export interface ChatDraftStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

interface ChatDraftOptions {
  storage?: ChatDraftStorage;
  now?: number;
}

interface StoredChatDraft {
  content: string;
  updatedAt: number;
}

export function chatDraftKey(scope: ChatDraftScope): string {
  return [chatDraftPrefix, scope.roomId, scope.selfId, scope.peerId].map(encodeURIComponent).join(":");
}

export function readChatDraft(scope: ChatDraftScope, options: ChatDraftOptions = {}): string {
  const storage = options.storage ?? browserStorage();
  if (!storage) return "";
  const key = chatDraftKey(scope);
  let raw: string | null;
  try {
    raw = storage.getItem(key);
  } catch {
    return "";
  }
  if (!raw) return "";

  const now = options.now ?? Date.now();
  try {
    const draft: unknown = JSON.parse(raw);
    if (!isStoredChatDraft(draft, now)) {
      safeRemove(storage, key);
      return "";
    }
    return draft.content;
  } catch {
    safeRemove(storage, key);
    return "";
  }
}

export function writeChatDraft(scope: ChatDraftScope, content: string, options: ChatDraftOptions = {}): void {
  const storage = options.storage ?? browserStorage();
  if (!storage) return;
  const key = chatDraftKey(scope);
  if (content.length === 0) {
    safeRemove(storage, key);
    return;
  }
  const draft: StoredChatDraft = { content, updatedAt: options.now ?? Date.now() };
  try {
    storage.setItem(key, JSON.stringify(draft));
  } catch {
    // Draft persistence is best-effort and must never block chat input.
  }
}

export function removeChatDraft(scope: ChatDraftScope, options: ChatDraftOptions = {}): void {
  const storage = options.storage ?? browserStorage();
  if (!storage) return;
  safeRemove(storage, chatDraftKey(scope));
}

function browserStorage(): ChatDraftStorage | undefined {
  try {
    return typeof window === "undefined" ? undefined : window.localStorage;
  } catch {
    return undefined;
  }
}

function safeRemove(storage: ChatDraftStorage, key: string): void {
  try {
    storage.removeItem(key);
  } catch {
    // Invalid or expired drafts can be ignored when storage is unavailable.
  }
}

function isStoredChatDraft(value: unknown, now: number): value is StoredChatDraft {
  if (value == null || typeof value !== "object") return false;
  const draft = value as Partial<StoredChatDraft>;
  return typeof draft.content === "string"
    && draft.content.length > 0
    && typeof draft.updatedAt === "number"
    && Number.isFinite(draft.updatedAt)
    && draft.updatedAt >= 0
    && draft.updatedAt <= now
    && now - draft.updatedAt < chatDraftTTL;
}
