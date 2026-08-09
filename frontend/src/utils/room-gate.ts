import type { JoinMode } from "../types/domain";

export type JoinGateView = "open" | "pin" | "approval_request" | "approval_waiting";
export type RoomGateErrorKind = "unavailable" | "transient";

const PERMANENT_ROOM_ERROR_CODES = new Set([
  40_004, // invalid room code
  40_403, // room not found
  40_908, // room is no longer active
]);

export function deriveJoinGateView(joinMode: JoinMode, pendingRequest: boolean): JoinGateView {
  if (joinMode === "password") return "pin";
  if (joinMode === "owner_approval") {
    return pendingRequest ? "approval_waiting" : "approval_request";
  }
  return "open";
}

export function classifyRoomGateError(code?: number): RoomGateErrorKind {
  return code !== undefined && PERMANENT_ROOM_ERROR_CODES.has(code) ? "unavailable" : "transient";
}

export function normalizePin(value: string): string {
  return value.replace(/[^0-9]/g, "").slice(0, 4);
}

export function isPinPairValid(pin: string, confirmation: string): boolean {
  return /^\d{4}$/.test(pin) && /^\d{4}$/.test(confirmation) && pin === confirmation;
}

export interface PinSubmissionGate {
  tryStart: (pin: string) => boolean;
  fail: () => void;
  succeed: () => void;
  reset: () => void;
}

export function createPinSubmissionGate(): PinSubmissionGate {
  let state: "idle" | "pending" | "complete" = "idle";

  return {
    tryStart(pin) {
      if (!/^\d{4}$/.test(pin) || state !== "idle") return false;
      state = "pending";
      return true;
    },
    fail() {
      if (state === "pending") state = "idle";
    },
    succeed() {
      if (state === "pending") state = "complete";
    },
    reset() {
      state = "idle";
    },
  };
}
