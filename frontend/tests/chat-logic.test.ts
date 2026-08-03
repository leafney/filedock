import { describe, expect, test } from "bun:test";

import type { ChatMessage } from "../src/types/domain";
import { chatMessageCapabilities, isChatNearBottom, mergeChatMessages, mergeReadSequence, markChatMessagesRead, shouldMarkChatRead } from "../src/utils/chat";

function message(overrides: Partial<ChatMessage> = {}): ChatMessage {
  return {
    roomCode: "1234",
    conversationId: "conversation-1",
    messageId: "message-1",
    clientMessageId: "client-1",
    senderUserId: "user-1",
    recipientUserId: "user-2",
    senderDisplayName: "甲一",
    contentText: "hello",
    isForwarded: false,
    sequence: 1,
    createdAt: 1,
    read: false,
    canCopy: true,
    canRecall: true,
    canRecallAndEdit: true,
    canForward: true,
    canDelete: true,
    deliveryStatus: "sent",
    ...overrides,
  };
}

describe("聊天纯逻辑", () => {
  test("REST、SSE 和乐观消息按消息 ID 与客户端 ID 去重", () => {
    const optimistic = message({ messageId: "optimistic-client-1", optimistic: true, deliveryStatus: "sending", sequence: Number.MAX_SAFE_INTEGER });
    const server = message({ messageId: "message-1", sequence: 1, deliveryStatus: "sent" });
    const merged = mergeChatMessages([optimistic], [server]);
    expect(merged).toHaveLength(1);
    expect(merged[0]?.messageId).toBe("message-1");
    expect(merged[0]?.optimistic).toBe(false);
  });

  test("已读游标单调合并并更新发送状态", () => {
    const items = [message(), message({ messageId: "message-2", clientMessageId: "client-2", sequence: 2 })];
    expect(mergeReadSequence(4, 2)).toBe(4);
    const updated = markChatMessagesRead(items, 1, "user-1");
    expect(updated[0]?.deliveryStatus).toBe("read");
    expect(updated[1]?.deliveryStatus).toBe("sent");
  });

  test("菜单能力遵循服务端能力和撤回窗口", () => {
    expect(chatMessageCapabilities(message(), "user-1", 1)).toEqual({ copy: true, recall: true, recallAndEdit: true, forward: true, delete: true });
    expect(chatMessageCapabilities(message({ recalledAt: 2, contentText: "" }), "user-1", 2)).toEqual({ copy: false, recall: false, recallAndEdit: false, forward: false, delete: true });
    expect(chatMessageCapabilities(message({ senderUserId: "user-2", canRecall: false, canRecallAndEdit: false }), "user-1", 1).recall).toBe(false);
  });

  test("阅读状态和滚动决策符合页面可见性", () => {
    expect(shouldMarkChatRead(true, true, true)).toBe(true);
    expect(shouldMarkChatRead(true, false, true)).toBe(false);
    expect(shouldMarkChatRead(true, true, false)).toBe(false);
    expect(isChatNearBottom(700, 1000, 300)).toBe(true);
    expect(isChatNearBottom(600, 1000, 300)).toBe(false);
  });
});

