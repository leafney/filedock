import { describe, expect, test } from "bun:test";

import type { NotificationItem } from "../src/types/domain";
import { chatNotificationTarget, fileNotificationTarget, formatNotificationCount, isChatNotification, isFileNotification, isFileResultNotification, readNotificationChatLaunch, readNotificationFileLaunch, shouldRefreshNotifications } from "../src/utils/notifications";

const chatItem: NotificationItem = {
  key: "chat_conversation:one",
  type: "chat_conversation",
  roomCode: "1234",
  roomTitle: "房间 1234",
  conversationId: "conversation-one",
  peerUserId: "user-two",
  peerDisplayName: "乙二",
  latestMessageText: "你好",
  unreadCount: 2,
  latestMessageAt: 1,
};

const fileItem: NotificationItem = {
  key: "file_declined:room-one:user-two",
  type: "file_declined",
  roomCode: "1234",
  roomTitle: "房间 1234",
  counterpartUserId: "user-two",
  counterpartDisplayName: "乙二",
  latestFileName: "资料.pdf",
  fileCount: 2,
  latestFileEventAt: 1,
  readToken: "read-one",
};

describe("全局通知纯逻辑", () => {
  test("聊天、房间、文件结果和昵称事件刷新通知，文件进度事件不刷新", () => {
    expect(shouldRefreshNotifications("chat.message_created")).toBe(true);
    expect(shouldRefreshNotifications("chat.read_updated")).toBe(true);
    expect(shouldRefreshNotifications("room.join_request_changed")).toBe(true);
    expect(shouldRefreshNotifications("room.member_left")).toBe(true);
    expect(shouldRefreshNotifications("user.profile_changed")).toBe(true);
    expect(shouldRefreshNotifications("file.available")).toBe(true);
    expect(shouldRefreshNotifications("file.reused")).toBe(true);
    expect(shouldRefreshNotifications("file.recipient_changed")).toBe(true);
    expect(shouldRefreshNotifications("file.download_completed")).toBe(true);
    expect(shouldRefreshNotifications("notification.changed")).toBe(true);
    expect(shouldRefreshNotifications("file.upload_progress")).toBe(false);
    expect(shouldRefreshNotifications("file.download_progress")).toBe(false);
    expect(shouldRefreshNotifications("file.download_started")).toBe(false);
  });

  test("文件通知区分待接收与结果并只生成房间目标", () => {
    expect(isFileNotification(fileItem)).toBe(true);
    expect(isFileResultNotification(fileItem)).toBe(true);
    expect(fileNotificationTarget(fileItem)).toEqual({ roomCode: "1234" });
    const received: NotificationItem = { ...fileItem, key: "file_received:room-one:user-two", type: "file_received" };
    delete (received as { readToken?: string }).readToken;
    expect(isFileNotification(received)).toBe(true);
    expect(isFileResultNotification(received)).toBe(false);
    expect(fileNotificationTarget(chatItem)).toBeUndefined();
  });

  test("聊天通知生成一次性房间和成员目标", () => {
    expect(isChatNotification(chatItem)).toBe(true);
    expect(chatNotificationTarget(chatItem)).toEqual({ roomCode: "1234", peerUserId: "user-two" });
    const approval: NotificationItem = {
      key: "join_request:one",
      type: "join_request",
      roomCode: "5678",
      roomTitle: "房间 5678",
      requestId: "request-one",
      actorUserId: "user-three",
      actorDisplayName: "丙三",
      createdAt: 1,
      expiresAt: 2,
    };
    expect(isChatNotification(approval)).toBe(false);
    expect(chatNotificationTarget(approval)).toBeUndefined();
  });

  test("只接受完整的一次性聊天导航状态", () => {
    expect(readNotificationChatLaunch({ notificationChatTarget: { peerUserId: "user-two", token: "launch-one" } })).toEqual({ peerUserId: "user-two", token: "launch-one" });
    expect(readNotificationChatLaunch({ notificationChatTarget: { peerUserId: "user-two" } })).toBeUndefined();
    expect(readNotificationChatLaunch(null)).toBeUndefined();
  });

  test("只接受完整的一次性文件导航状态", () => {
    expect(readNotificationFileLaunch({ notificationFileTarget: { token: "file-launch-one" } })).toEqual({ token: "file-launch-one" });
    expect(readNotificationFileLaunch({ notificationFileTarget: {} })).toBeUndefined();
    expect(readNotificationFileLaunch({ notificationFileTarget: { token: 1 } })).toBeUndefined();
    expect(readNotificationFileLaunch(null)).toBeUndefined();
  });

  test("通知数量在九十九条后保持紧凑显示", () => {
    expect(formatNotificationCount(0)).toBe("0");
    expect(formatNotificationCount(99)).toBe("99");
    expect(formatNotificationCount(100)).toBe("99+");
  });
});
