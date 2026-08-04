import { describe, expect, test } from "bun:test";

import type { ChatMessage, ChatMessagePage } from "../src/types/domain";
import { addPendingHistoryMessage, containsHistoryTarget, isIncomingForHistory, mergeChatMessagePage } from "../src/utils/chat-history";

function message(sequence: number, overrides: Partial<ChatMessage> = {}): ChatMessage {
  return {
    roomCode: "1234",
    conversationId: "conversation-1",
    messageId: `message-${sequence}`,
    clientMessageId: `client-${sequence}`,
    senderUserId: "peer",
    recipientUserId: "self",
    senderDisplayName: "成员",
    contentText: `message ${sequence}`,
    isForwarded: false,
    sequence,
    createdAt: sequence,
    read: false,
    canCopy: true,
    canRecall: false,
    canRecallAndEdit: false,
    canForward: true,
    canDelete: true,
    ...overrides,
  };
}

function page(items: ChatMessage[], overrides: Partial<ChatMessagePage> = {}): ChatMessagePage {
  return { items, hasMoreBefore: false, hasMoreAfter: false, currentReadSequence: 0, peerReadSequence: 0, ...overrides };
}

describe("聊天历史定位逻辑", () => {
  test("前后分页分别更新对应游标并合并去重", () => {
    const current = page([message(3), message(4)], { previousCursor: 3, hasMoreBefore: true, nextCursor: 4, hasMoreAfter: true });
    const before = mergeChatMessagePage(current, page([message(1), message(2), message(3)], { hasMoreBefore: false }), "before");
    expect(before.items.map((item) => item.sequence)).toEqual([1, 2, 3, 4]);
    expect(before.hasMoreBefore).toBe(false);
    expect(before.nextCursor).toBe(4);
    expect(before.hasMoreAfter).toBe(true);

    const after = mergeChatMessagePage(before, page([message(4), message(5)], { hasMoreAfter: false }), "after");
    expect(after.items.map((item) => item.sequence)).toEqual([1, 2, 3, 4, 5]);
    expect(after.hasMoreBefore).toBe(false);
    expect(after.hasMoreAfter).toBe(false);
  });

  test("历史新消息按消息标识去重且只统计当前对方来信", () => {
    expect(addPendingHistoryMessage(["message-1"], "message-1")).toEqual(["message-1"]);
    expect(addPendingHistoryMessage(["message-1"], "message-2")).toEqual(["message-1", "message-2"]);
    expect(isIncomingForHistory(message(1), "peer", "self")).toBe(true);
    expect(isIncomingForHistory(message(1, { senderUserId: "self", recipientUserId: "peer" }), "peer", "self")).toBe(false);
    expect(isIncomingForHistory(message(1), "other", "self")).toBe(false);
  });

  test("定位响应必须包含未撤回的精确目标", () => {
    const target = message(16);
    expect(containsHistoryTarget(page([message(15), target, message(17)]), 16, target.messageId)).toBe(true);
    expect(containsHistoryTarget(page([message(15), message(17)]), 16, target.messageId)).toBe(false);
    expect(containsHistoryTarget(page([target], {}), 16, "different")).toBe(false);
    expect(containsHistoryTarget(page([{ ...target, recalledAt: 20 }]), 16, target.messageId)).toBe(false);
  });
});
