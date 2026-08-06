export type UploadCoordinationMessage = {
  uploadId: string;
  type: "progress" | "status" | "deleted";
  payload?: Record<string, unknown>;
};

const leasePrefix = "filedock:upload-lease:";
const leaseDurationMs = 15_000;
const ownerId = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`;
let channel: BroadcastChannel | undefined;

function browserStorage(): Storage | undefined {
  try {
    return typeof window === "undefined" ? undefined : window.localStorage;
  } catch {
    return undefined;
  }
}

function leaseKey(uploadId: string) {
  return `${leasePrefix}${uploadId}`;
}

function readLease(uploadId: string, storage = browserStorage()): { owner: string; expiresAt: number } | undefined {
  if (!storage) return undefined;
  try {
    const value = JSON.parse(storage.getItem(leaseKey(uploadId)) ?? "null") as Partial<{ owner: string; expiresAt: number }> | null;
    return value && typeof value.owner === "string" && typeof value.expiresAt === "number" ? value as { owner: string; expiresAt: number } : undefined;
  } catch {
    return undefined;
  }
}

export function acquireUploadLease(uploadId: string, now = Date.now(), storage = browserStorage()) {
  if (!storage) return true;
  const current = readLease(uploadId, storage);
  if (current && current.owner !== ownerId && current.expiresAt > now) return false;
  try {
    storage.setItem(leaseKey(uploadId), JSON.stringify({ owner: ownerId, expiresAt: now + leaseDurationMs }));
    return readLease(uploadId, storage)?.owner === ownerId;
  } catch {
    return true;
  }
}

export function renewUploadLease(uploadId: string, now = Date.now(), storage = browserStorage()) {
  if (!storage) return true;
  const current = readLease(uploadId, storage);
  if (current?.owner !== ownerId) return false;
  try {
    storage.setItem(leaseKey(uploadId), JSON.stringify({ owner: ownerId, expiresAt: now + leaseDurationMs }));
    return true;
  } catch {
    return true;
  }
}

export function releaseUploadLease(uploadId: string, storage = browserStorage()) {
  if (!storage) return;
  const current = readLease(uploadId, storage);
  if (current?.owner !== ownerId) return;
  try {
    storage.removeItem(leaseKey(uploadId));
  } catch {
    // Ignore unavailable browser storage.
  }
}

export function broadcastUploadMessage(message: UploadCoordinationMessage) {
  if (typeof window === "undefined" || typeof BroadcastChannel === "undefined") return;
  channel ??= new BroadcastChannel("filedock:uploads");
  channel.postMessage(message);
}

export function subscribeUploadMessages(listener: (message: UploadCoordinationMessage) => void) {
  if (typeof window === "undefined" || typeof BroadcastChannel === "undefined") return () => undefined;
  channel ??= new BroadcastChannel("filedock:uploads");
  const handler = (event: MessageEvent<UploadCoordinationMessage>) => {
    if (event.data?.uploadId) listener(event.data);
  };
  channel.addEventListener("message", handler);
  return () => channel?.removeEventListener("message", handler);
}

export function uploadLeaseDuration() {
  return leaseDurationMs;
}
