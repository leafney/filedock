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

export interface StreamEvent<T = Record<string, unknown>> {
  version: number;
  id: string;
  type: string;
  occurredAt: string;
  payload?: T;
}
