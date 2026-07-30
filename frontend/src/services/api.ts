import { apiClient } from "../lib/api-client";
import type { ApiResponse } from "../types/api";
import type {
  JoinRequestList,
  RoomJoinInfo,
  RoomSnapshot,
  RoomSummary,
  Session,
} from "../types/domain";

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

export function getJoinRequests(code: string) {
  return unwrap<JoinRequestList>(apiClient.get<ApiResponse<JoinRequestList>>(`/api/v1/rooms/${code}/join-requests`));
}

export function approveJoinRequest(code: string, requestId: string) {
  return unwrap<null>(apiClient.post<ApiResponse<null>>(`/api/v1/rooms/${code}/join-requests/${requestId}/approve`));
}

export function rejectJoinRequest(code: string, requestId: string) {
  return unwrap<null>(apiClient.post<ApiResponse<null>>(`/api/v1/rooms/${code}/join-requests/${requestId}/reject`));
}
