import { describe, expect, test } from "bun:test";

import { chatDraftKey, chatDraftTTL, readChatDraft, removeChatDraft, type ChatDraftScope, type ChatDraftStorage, writeChatDraft } from "../src/utils/chat-drafts";

class MemoryStorage implements ChatDraftStorage {
  readonly values = new Map<string, string>();

  getItem(key: string) { return this.values.get(key) ?? null; }
  setItem(key: string, value: string) { this.values.set(key, value); }
  removeItem(key: string) { this.values.delete(key); }
}

const scope: ChatDraftScope = { roomId: "room-1", selfId: "user-1", peerId: "user-2" };

describe("聊天草稿本地存储", () => {
  test("按房间、当前用户和聊天对象隔离草稿", () => {
    const storage = new MemoryStorage();
    writeChatDraft(scope, "给乙的消息", { storage, now: 1_000 });

    expect(readChatDraft(scope, { storage, now: 2_000 })).toBe("给乙的消息");
    expect(readChatDraft({ ...scope, roomId: "room-2" }, { storage, now: 2_000 })).toBe("");
    expect(readChatDraft({ ...scope, selfId: "user-3" }, { storage, now: 2_000 })).toBe("");
    expect(readChatDraft({ ...scope, peerId: "user-3" }, { storage, now: 2_000 })).toBe("");
  });

  test("每次写入刷新时间并在七天后清理草稿", () => {
    const storage = new MemoryStorage();
    writeChatDraft(scope, "第一版", { storage, now: 1_000 });
    writeChatDraft(scope, "第二版", { storage, now: 2_000 });

    expect(readChatDraft(scope, { storage, now: 2_000 + chatDraftTTL - 1 })).toBe("第二版");
    expect(readChatDraft(scope, { storage, now: 2_000 + chatDraftTTL })).toBe("");
    expect(storage.getItem(chatDraftKey(scope))).toBeNull();
  });

  test("空内容、损坏记录和异常时间不会被恢复", () => {
    const storage = new MemoryStorage();
    writeChatDraft(scope, "内容", { storage, now: 1_000 });
    writeChatDraft(scope, "", { storage, now: 2_000 });
    expect(storage.getItem(chatDraftKey(scope))).toBeNull();

    storage.setItem(chatDraftKey(scope), "not-json");
    expect(readChatDraft(scope, { storage, now: 3_000 })).toBe("");
    expect(storage.getItem(chatDraftKey(scope))).toBeNull();

    storage.setItem(chatDraftKey(scope), JSON.stringify({ content: "未来草稿", updatedAt: 4_000 }));
    expect(readChatDraft(scope, { storage, now: 3_000 })).toBe("");
    expect(storage.getItem(chatDraftKey(scope))).toBeNull();
  });

  test("浏览器存储异常不会阻断聊天逻辑", () => {
    const storage: ChatDraftStorage = {
      getItem() { throw new Error("read unavailable"); },
      setItem() { throw new Error("write unavailable"); },
      removeItem() { throw new Error("remove unavailable"); },
    };

    expect(readChatDraft(scope, { storage, now: 1_000 })).toBe("");
    expect(() => writeChatDraft(scope, "内容", { storage, now: 1_000 })).not.toThrow();
    expect(() => removeChatDraft(scope, { storage })).not.toThrow();
  });
});
