export interface Session {
  userId: string;
  displayName: string;
  createdAt: number;
  sessionExpiresAt: number;
}

export type JoinMode = "open" | "password" | "owner_approval";
export type RoomStatus = "active" | "destroying" | "destroyed";

export interface RoomSummary {
  roomId: string;
  roomCode: string;
  title: string;
  status: RoomStatus;
  role: "owner" | "member";
  expiresAt: number;
}

export interface RoomMember {
  userId: string;
  displayName: string;
  role: "owner" | "member";
  status: "active" | "left" | "kicked" | "destroyed";
  joinedAt: number;
  onlineStatus: "online" | "away" | "offline";
}

export interface RoomSnapshot {
  roomId: string;
  roomCode: string;
  title: string;
  status: RoomStatus;
  joinMode: JoinMode;
  role: "owner" | "member";
  expiresAt: number;
  canExtend: boolean;
  destroyAt?: number;
  members: RoomMember[];
  pendingRequestCount?: number;
  capacity: RoomCapacity;
}

export interface RoomCapacity {
  capacityBytes: number;
  usedBytes: number;
  sharedBytes?: number;
  directBytes?: number;
  reservedBytes?: number;
}

export interface RoomJoinInfo {
  roomCode: string;
  title: string;
  joinMode: JoinMode;
  alreadyMember: boolean;
  pendingRequest: boolean;
}

export interface JoinRequest {
  requestId: string;
  roomCode: string;
  userId: string;
  displayName: string;
  status: string;
  createdAt: number;
  expiresAt: number;
}

export interface JoinRequestList {
  items: JoinRequest[];
  pendingCount: number;
}

interface NotificationBase {
  key: string;
  roomCode: string;
  roomTitle: string;
}

export interface JoinRequestNotification extends NotificationBase {
  type: "join_request";
  requestId: string;
  actorUserId: string;
  actorDisplayName: string;
  createdAt: number;
  expiresAt: number;
}

export interface ChatConversationNotification extends NotificationBase {
  type: "chat_conversation";
  conversationId: string;
  peerUserId: string;
  peerDisplayName: string;
  latestMessageText: string;
  unreadCount: number;
  latestMessageAt: number;
}

export type NotificationItem = JoinRequestNotification | ChatConversationNotification;

export interface NotificationPage {
  items: NotificationItem[];
  totalCount: number;
  nextCursor?: string;
}

export interface StreamEvent<T = Record<string, unknown>> {
  version: number;
  id: string;
  type: string;
  occurredAt: string;
  payload?: T;
}

export type ChatDeliveryStatus = "sending" | "sent" | "read" | "failed";

export interface ChatConversation {
  peerUserId: string;
  peerDisplayName: string;
  peerRole: "owner" | "member" | string;
  peerStatus: RoomMember["status"] | string;
  peerOnlineStatus: RoomMember["onlineStatus"] | string;
  unreadCount: number;
  lastMessageAt?: number;
  lastMessagePreview?: string;
}

export interface ChatMessage {
  roomCode: string;
  conversationId: string;
  messageId: string;
  clientMessageId: string;
  senderUserId: string;
  recipientUserId: string;
  senderDisplayName: string;
  contentText: string;
  isForwarded: boolean;
  sequence: number;
  createdAt: number;
  recalledAt?: number;
  recallDeadline?: number;
  read: boolean;
  canCopy: boolean;
  canRecall: boolean;
  canRecallAndEdit: boolean;
  canForward: boolean;
  canDelete: boolean;
  deliveryStatus?: ChatDeliveryStatus;
  optimistic?: boolean;
}

export interface ChatMessagePage {
  items: ChatMessage[];
  previousCursor?: number;
  hasMoreBefore: boolean;
  currentReadSequence: number;
  peerReadSequence: number;
}

export interface ChatReadState {
  conversationId: string;
  peerUserId: string;
  userId: string;
  lastReadSequence: number;
  lastReadAt: number;
}

export interface ChatSearchPage {
  items: ChatMessage[];
  previousCursor?: number;
  hasMoreBefore: boolean;
}

export type FileScope = "shared" | "direct";
export type FileProjectionLevel = "full" | "anonymous";
export type FileStatus = "reserved" | "uploading" | "available";
export type FileRecipientStatus = "pending" | "accepted" | "declined" | "downloaded";
export type FileRange = "all" | FileScope;
export type FileIdentity = "all" | "uploaded" | "received";
export type FileSort = "newest" | "oldest" | "size_asc" | "size_desc";

export interface FileCapabilities {
  canDownload: boolean;
  canAccept: boolean;
  canDecline: boolean;
  canReuse: boolean;
  canPublishShared: boolean;
}

export interface FileRecipient {
  userId: string;
  displayName: string;
  status?: FileRecipientStatus;
}

export interface RoomFile {
  projection: FileProjectionLevel;
  fileId: string;
  displayName: string;
  originalName?: string;
  privateCode?: string;
  scope: FileScope;
  size: number;
  declaredMime?: string;
  detectedMime?: string;
  status: FileStatus;
  progress: number;
  uploaderUserId: string;
  uploaderName: string;
  createdAt: number;
  completedAt?: number;
  recipients?: FileRecipient[];
  capabilities: FileCapabilities;
}

export interface FileGroup {
  scope: FileScope;
  items: RoomFile[];
  total: number;
  nextCursor?: string;
}

export interface FileListResult {
  shared?: FileGroup;
  direct?: FileGroup;
}

export interface FileEventItem {
  eventId: string;
  type: string;
  actorId?: string;
  actorName?: string;
  createdAt: number;
  file?: RoomFile;
}

export interface FileEventPage {
  items: FileEventItem[];
  nextCursor?: string;
}

export interface UploadManifest {
  originalName: string;
  declaredSize: number;
  declaredMime: string;
}

export interface UploadBatchFile {
  fileId: string;
  displayName: string;
  privateCode?: string;
  declaredSize: number;
  status: string;
  uploadUrl: string;
}

export interface UploadBatch {
  batchId: string;
  scope: FileScope;
  status: string;
  declaredTotalSize: number;
  files: UploadBatchFile[];
}

export interface DownloadTask {
  taskId: string;
  fileId: string;
  fileName: string;
  size: number;
  expiresAt: number;
  downloadUrl: string;
}
