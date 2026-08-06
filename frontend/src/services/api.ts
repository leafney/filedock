import { apiClient } from "../lib/api-client";
import type { ApiResponse } from "../types/api";
import type {
  ChatConversation,
  ChatMessage,
  ChatMessagePage,
  ChatReadState,
  ChatSearchPage,
  DownloadTask,
  FileEventPage,
  FileIdentity,
  FileListResult,
  FileRange,
  FileRestoreResult,
  FileSort,
  FileTrashPage,
  NotificationPage,
  NotificationReadResult,
  RoomFile,
  RoomJoinInfo,
  RoomSnapshot,
  RoomSummary,
  Session,
  UploadBatch,
  UploadManifest,
  UploadSession,
} from "../types/domain";
import { currentLanguage } from "../i18n";

async function unwrap<T>(request: Promise<{ data: ApiResponse<T> }>): Promise<T> {
  const response = await request;
  return response.data.data;
}

export function getCurrentSession() {
  return unwrap<Session>(apiClient.get<ApiResponse<Session>>("/api/v1/sessions/me"));
}

export function getRandomNickname() {
  return unwrap<{ displayName: string }>(apiClient.get<ApiResponse<{ displayName: string }>>("/api/v1/nicknames/random"));
}

export function createSession(displayName: string) {
  return unwrap<Session>(apiClient.post<ApiResponse<Session>>("/api/v1/sessions", { displayName }));
}

export function updateSession(displayName: string) {
  return unwrap<Session>(apiClient.put<ApiResponse<Session>>("/api/v1/sessions/me", { displayName }));
}

export function resetSession() {
  return unwrap<null>(apiClient.delete<ApiResponse<null>>("/api/v1/sessions/me"));
}

export function listRooms() {
  return unwrap<{ items: RoomSummary[] }>(apiClient.get<ApiResponse<{ items: RoomSummary[] }>>("/api/v1/rooms"));
}

export function createRoom(joinMode: string, pin = "", pinConfirmation = "") {
  return unwrap<RoomSnapshot>(apiClient.post<ApiResponse<RoomSnapshot>>("/api/v1/rooms", { joinMode, pin, pinConfirmation }));
}

export function getJoinInfo(code: string) {
  return unwrap<RoomJoinInfo>(apiClient.get<ApiResponse<RoomJoinInfo>>(`/api/v1/rooms/${code}/join-info`));
}

export function joinRoom(code: string, confirmed: boolean, pin = "") {
  return unwrap<RoomSnapshot>(apiClient.post<ApiResponse<RoomSnapshot>>(`/api/v1/rooms/${code}/join`, { confirmed, pin }));
}

export function getRoom(code: string) {
  return unwrap<RoomSnapshot>(apiClient.get<ApiResponse<RoomSnapshot>>(`/api/v1/rooms/${code}`));
}

export function getRoomQRCode(code: string) {
  return unwrap<{ svg: string }>(apiClient.get<ApiResponse<{ svg: string }>>(`/api/v1/rooms/${code}/qrcode`));
}

export function extendRoom(code: string) {
  return unwrap<{ expiresAt: number }>(apiClient.post<ApiResponse<{ expiresAt: number }>>(`/api/v1/rooms/${code}/extend`));
}

export function leaveRoom(code: string) {
  return unwrap<null>(apiClient.post<ApiResponse<null>>(`/api/v1/rooms/${code}/leave`));
}

export function dissolveRoom(code: string) {
  return unwrap<null>(apiClient.delete<ApiResponse<null>>(`/api/v1/rooms/${code}`));
}

export function kickMember(code: string, userId: string) {
  return unwrap<null>(apiClient.delete<ApiResponse<null>>(`/api/v1/rooms/${code}/members/${userId}`));
}

export function createJoinRequest(code: string) {
  return unwrap<unknown>(apiClient.post<ApiResponse<unknown>>(`/api/v1/rooms/${code}/join-requests`, { confirmed: true }));
}

export function cancelJoinRequest(code: string) {
  return unwrap<null>(apiClient.delete<ApiResponse<null>>(`/api/v1/rooms/${code}/join-requests/me`));
}

export function approveJoinRequest(code: string, requestId: string) {
  return unwrap<null>(apiClient.post<ApiResponse<null>>(`/api/v1/rooms/${code}/join-requests/${requestId}/approve`));
}

export function rejectJoinRequest(code: string, requestId: string) {
  return unwrap<null>(apiClient.post<ApiResponse<null>>(`/api/v1/rooms/${code}/join-requests/${requestId}/reject`));
}

export function listNotifications(cursor = "", limit = 30) {
  return unwrap<NotificationPage>(apiClient.get<ApiResponse<NotificationPage>>("/api/v1/notifications", { params: { cursor: cursor || undefined, limit } }));
}

export function markNotificationRead(key: string, readToken: string) {
  return unwrap<NotificationReadResult>(apiClient.post<ApiResponse<NotificationReadResult>>("/api/v1/notifications/read", { key, readToken }));
}

export function listChatConversations(code: string) {
  return unwrap<{ items: ChatConversation[] }>(apiClient.get<ApiResponse<{ items: ChatConversation[] }>>(`/api/v1/rooms/${code}/chat/conversations`));
}

export interface ChatHistoryParams {
  beforeSequence?: number;
  afterSequence?: number;
  aroundSequence?: number;
  limit?: number;
}

export function getChatMessages(code: string, peerUserId: string, params: ChatHistoryParams = {}) {
  return unwrap<ChatMessagePage>(apiClient.get<ApiResponse<ChatMessagePage>>(`/api/v1/rooms/${code}/chat/conversations/${peerUserId}/messages`, { params }));
}

export function sendChatMessage(code: string, recipientUserId: string, clientMessageId: string, contentText: string) {
  return unwrap<ChatMessage>(apiClient.post<ApiResponse<ChatMessage>>(`/api/v1/rooms/${code}/chat/messages`, { recipientUserId, clientMessageId, contentText }));
}

export function markChatRead(code: string, peerUserId: string, lastReadSequence: number) {
  return unwrap<ChatReadState>(apiClient.post<ApiResponse<ChatReadState>>(`/api/v1/rooms/${code}/chat/conversations/${peerUserId}/read`, { lastReadSequence }));
}

export function recallChatMessage(code: string, messageId: string) {
  return unwrap<ChatMessage>(apiClient.post<ApiResponse<ChatMessage>>(`/api/v1/rooms/${code}/chat/messages/${messageId}/recall`));
}

export function deleteChatMessage(code: string, messageId: string) {
  return unwrap<null>(apiClient.delete<ApiResponse<null>>(`/api/v1/rooms/${code}/chat/messages/${messageId}`));
}

export function forwardChatMessage(code: string, messageId: string, clientMessageId: string, recipientUserIds: string[]) {
  return unwrap<{ items: ChatMessage[] }>(apiClient.post<ApiResponse<{ items: ChatMessage[] }>>(`/api/v1/rooms/${code}/chat/messages/${messageId}/forward`, { clientMessageId, recipientUserIds }));
}

export interface ChatSearchParams {
  query: string;
  beforeSequence?: number;
  limit?: number;
}

export function searchChatMessages(code: string, peerUserId: string, params: ChatSearchParams) {
  return unwrap<ChatSearchPage>(apiClient.get<ApiResponse<ChatSearchPage>>(`/api/v1/rooms/${code}/chat/conversations/${peerUserId}/search`, { params: { q: params.query, beforeSequence: params.beforeSequence, limit: params.limit } }));
}

export interface FileListParams {
  range: FileRange;
  identity: FileIdentity;
  search: string;
  sort: FileSort;
  limit?: number;
  sharedCursor?: string;
  directCursor?: string;
}

export function listRoomFiles(code: string, params: FileListParams) {
  return unwrap<FileListResult>(apiClient.get<ApiResponse<FileListResult>>(`/api/v1/rooms/${code}/files`, { params }));
}

export function listFileEvents(code: string, cursor = "", limit = 30) {
  return unwrap<FileEventPage>(apiClient.get<ApiResponse<FileEventPage>>(`/api/v1/rooms/${code}/file-events`, { params: { cursor, limit } }));
}

export function listFileTrash(code: string, search = "", cursor = "", limit = 30) {
  return unwrap<FileTrashPage>(apiClient.get<ApiResponse<FileTrashPage>>(`/api/v1/rooms/${code}/file-trash`, { params: { search: search || undefined, cursor: cursor || undefined, limit } }));
}

export function trashRoomFile(code: string, fileId: string, reason = "") {
  return unwrap<{ status: string; deletedAt: number }>(apiClient.post<ApiResponse<{ status: string; deletedAt: number }>>(`/api/v1/rooms/${code}/files/${fileId}/trash`, { reason }));
}

export function restoreRoomFile(code: string, fileId: string) {
  return unwrap<FileRestoreResult>(apiClient.post<ApiResponse<FileRestoreResult>>(`/api/v1/rooms/${code}/files/${fileId}/restore`));
}

export function purgeRoomFile(code: string, fileId: string) {
  return unwrap<null>(apiClient.delete<ApiResponse<null>>(`/api/v1/rooms/${code}/files/${fileId}/purge`));
}

export function approveFileRestore(code: string, requestId: string) {
  return unwrap<null>(apiClient.post<ApiResponse<null>>(`/api/v1/rooms/${code}/file-restore-requests/${requestId}/approve`));
}

export function rejectFileRestore(code: string, requestId: string, reason = "") {
  return unwrap<null>(apiClient.post<ApiResponse<null>>(`/api/v1/rooms/${code}/file-restore-requests/${requestId}/reject`, { reason }));
}

export function listReusablePrivateFiles(code: string) {
  return unwrap<{ items: RoomFile[] }>(apiClient.get<ApiResponse<{ items: RoomFile[] }>>(`/api/v1/rooms/${code}/private-files/reusable`));
}

export function createUploadBatch(code: string, idempotencyKey: string, scope: "shared" | "direct", recipientIds: string[], files: UploadManifest[]) {
  return unwrap<UploadBatch>(apiClient.post<ApiResponse<UploadBatch>>(`/api/v1/rooms/${code}/file-upload-batches`, { idempotencyKey, scope, recipientIds, files }));
}

export function cancelFileUpload(code: string, fileId: string) {
  return unwrap<null>(apiClient.delete<ApiResponse<null>>(`/api/v1/rooms/${code}/files/${fileId}/upload`));
}

export function getUploadStatus(code: string, fileId: string) {
  return unwrap<UploadSession>(apiClient.get<ApiResponse<UploadSession>>(`/api/v1/rooms/${code}/files/${fileId}/upload`));
}

export function acceptPrivateFile(code: string, fileId: string) {
  return unwrap<DownloadTask>(apiClient.post<ApiResponse<DownloadTask>>(`/api/v1/rooms/${code}/files/${fileId}/accept`));
}

export function declinePrivateFile(code: string, fileId: string) {
  return unwrap<null>(apiClient.post<ApiResponse<null>>(`/api/v1/rooms/${code}/files/${fileId}/decline`));
}

export function reusePrivateFiles(code: string, fileIds: string[], recipientIds: string[]) {
  return unwrap<{ changed: number; skipped: number }>(apiClient.post<ApiResponse<{ changed: number; skipped: number }>>(`/api/v1/rooms/${code}/private-files/reuse`, { fileIds, recipientIds }));
}

export function publishPrivateFile(code: string, fileId: string) {
  return unwrap<null>(apiClient.post<ApiResponse<null>>(`/api/v1/rooms/${code}/files/${fileId}/publish-shared`));
}

export function createFileDownload(code: string, fileId: string) {
  return unwrap<DownloadTask>(apiClient.post<ApiResponse<DownloadTask>>(`/api/v1/rooms/${code}/files/${fileId}/downloads`));
}

export interface UploadProgress {
  loaded: number;
  total: number;
  percent: number;
}

export interface UploadPartProgress {
  loaded: number;
  total: number;
}

export function uploadFileContent(uploadUrl: string, file: File, onProgress: (progress: UploadProgress) => void) {
  const request = new XMLHttpRequest();
  const promise = new Promise<void>((resolve, reject) => {
    request.open("PUT", uploadUrl);
    request.withCredentials = true;
    request.setRequestHeader("Accept-Language", currentLanguage());
    request.setRequestHeader("Content-Type", file.type || "application/octet-stream");
    request.upload.onprogress = (event) => {
      const total = event.lengthComputable ? event.total : file.size;
      onProgress({ loaded: event.loaded, total, percent: total > 0 ? Math.min(100, Math.round(event.loaded * 100 / total)) : 0 });
    };
    request.onload = () => {
      if (request.status >= 200 && request.status < 300) resolve();
      else reject(readXHRFailure(request));
    };
    request.onerror = () => reject(new Error("network"));
    request.onabort = () => reject(new DOMException("Upload cancelled", "AbortError"));
    request.send(file);
  });
  return { request, promise };
}

export function uploadFilePart(uploadUrl: string, part: Blob, partNumber: number, startOffset: number, endOffset: number, totalSize: number, sha256: string, onProgress: (progress: UploadPartProgress) => void) {
  const request = new XMLHttpRequest();
  const promise = new Promise<void>((resolve, reject) => {
    request.open("PUT", uploadUrl);
    request.withCredentials = true;
    request.setRequestHeader("Accept-Language", currentLanguage());
    request.setRequestHeader("Content-Type", "application/octet-stream");
    request.setRequestHeader("Content-Range", `bytes ${startOffset}-${endOffset}/${totalSize}`);
    request.setRequestHeader("X-Chunk-Number", String(partNumber));
    request.setRequestHeader("X-Chunk-SHA256", sha256);
    request.upload.onprogress = (event) => {
      const total = event.lengthComputable ? event.total : part.size;
      onProgress({ loaded: event.loaded, total });
    };
    request.onload = () => {
      if (request.status >= 200 && request.status < 300) resolve();
      else reject(readXHRFailure(request));
    };
    request.onerror = () => reject(new Error("network"));
    request.onabort = () => reject(new DOMException("Upload cancelled", "AbortError"));
    request.send(part);
  });
  return { request, promise };
}

export class UploadRequestError extends Error {
  readonly code?: number;

  constructor(message: string, code?: number) {
    super(message);
    this.name = "UploadRequestError";
    this.code = code;
  }
}

function readXHRFailure(request: XMLHttpRequest) {
  try {
    const response = JSON.parse(request.responseText) as ApiResponse<unknown>;
    if (response.message) return new UploadRequestError(response.message, response.code);
  } catch {
    // Use the HTTP status fallback when the response is not JSON.
  }
  return new UploadRequestError(`HTTP ${request.status}`);
}

export function startNativeDownload(task: DownloadTask) {
  const anchor = document.createElement("a");
  anchor.href = task.downloadUrl;
  anchor.download = task.fileName;
  anchor.hidden = true;
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
}
