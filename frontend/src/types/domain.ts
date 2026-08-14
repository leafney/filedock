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
  trashBytes?: number;
  reservedBytes?: number;
}

export interface RoomJoinInfo {
  roomCode: string;
  title: string;
  joinMode: JoinMode;
  alreadyMember: boolean;
  pendingRequest: boolean;
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

interface FileNotificationBase extends NotificationBase {
  counterpartUserId: string;
  counterpartDisplayName: string;
  latestFileName: string;
  latestReason?: string;
  fileCount: number;
  latestFileEventAt: number;
}

export interface FileReceivedNotification extends FileNotificationBase {
  type: "file_received";
}

export interface FileDeclinedNotification extends FileNotificationBase {
  type: "file_declined";
  readToken: string;
}

export interface FileDownloadedNotification extends FileNotificationBase {
  type: "file_downloaded";
  readToken: string;
}

export interface FileTrashedByOwnerNotification extends FileNotificationBase {
  type: "file_trashed_by_owner";
  readToken: string;
}

export interface FileRestoreRequestedNotification extends FileNotificationBase {
  type: "file_restore_requested";
  requestId: string;
}

export interface FileRestoredByOwnerNotification extends FileNotificationBase {
  type: "file_restored_by_owner";
  readToken: string;
}

export interface FileRestoreRejectedNotification extends FileNotificationBase {
  type: "file_restore_rejected";
  readToken: string;
}

export interface FilePurgedByOwnerNotification extends FileNotificationBase {
  type: "file_purged_by_owner";
  readToken: string;
}

export type FileNotification =
  | FileReceivedNotification
  | FileDeclinedNotification
  | FileDownloadedNotification
  | FileTrashedByOwnerNotification
  | FileRestoreRequestedNotification
  | FileRestoredByOwnerNotification
  | FileRestoreRejectedNotification
  | FilePurgedByOwnerNotification;

export type NotificationItem = JoinRequestNotification | ChatConversationNotification | FileNotification;

export interface NotificationPage {
  items: NotificationItem[];
  totalCount: number;
  nextCursor?: string;
}

export interface NotificationReadResult {
  updatedCount: number;
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
  nextCursor?: number;
  hasMoreAfter: boolean;
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
export type FileStatus = "reserved" | "uploading" | "available" | "trashed" | "purging" | "purged";
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
  canTrash: boolean;
  canSetTrashReason: boolean;
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

export interface FileListResult {
	items: RoomFile[];
	total: number;
	counts: { all: number; shared: number; direct: number };
}

export interface FileEventItem {
  eventId: string;
  operationId: string;
  operationType: string;
  type: string;
  actorId?: string;
  actorName?: string;
  createdAt: number;
  file?: RoomFile;
  history: FileEventHistory[];
  recipientSummary?: FileRecipientEventSummary;
  skippedRecipients?: FileEventRecipient[];
}

export interface FileEventHistory {
  eventId: string;
  type: string;
  actorId?: string;
  actorName?: string;
  createdAt: number;
  recipients?: FileEventRecipient[];
}

export interface FileEventRecipient {
  userId: string;
  displayName: string;
  status?: string;
}

export interface FileRecipientEventSummary {
  total: number;
  pending: number;
  accepted: number;
  declined: number;
  skipped?: number;
}

export interface FileEventPage {
  items: FileEventItem[];
  nextCursor?: string;
}

export type FileRestoreRequestStatus = "pending" | "approved" | "rejected" | "invalidated";

export interface FileRestoreRequest {
  requestId: string;
  status: FileRestoreRequestStatus;
  requesterUserId: string;
  requesterName: string;
  createdAt: number;
  rejectionReason?: string;
}

export interface FileTrashCapabilities {
  canRestore: boolean;
  canRequestRestore: boolean;
  canPurge: boolean;
}

export interface FileTrashItem {
  file: RoomFile;
  deletedByUserId: string;
  deletedByName: string;
  deletedAt: number;
  deleteReason?: string;
  restoreRequest?: FileRestoreRequest;
  capabilities: FileTrashCapabilities;
}

export interface FileTrashPage {
  items: FileTrashItem[];
  total: number;
  nextCursor?: string;
}

export interface FileRestoreResult {
  status: "restored" | "pending";
  requestId?: string;
}

export interface UploadManifest {
  originalName: string;
  declaredSize: number;
  declaredMime: string;
}

export interface UploadBatchFile {
  fileId: string;
  uploadId: string;
  displayName: string;
  privateCode?: string;
  declaredSize: number;
  status: string;
  chunkSize: number;
  totalParts: number;
  expiresAt: number;
  uploadUrl: string;
}

export interface UploadPart {
  partNumber: number;
  startOffset: number;
  endOffset: number;
  length: number;
  sha256: string;
}

export interface UploadSession {
  uploadId: string;
  fileId: string;
  status: string;
  declaredSize: number;
  chunkSize: number;
  totalParts: number;
  receivedBytes: number;
  expiresAt: number;
  parts: UploadPart[];
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
