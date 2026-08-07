import type { FileScope } from "../types/domain";

export interface UploadResumeRecord {
  uploadId: string;
  fileId: string;
  roomCode: string;
  uploadUrl: string;
  fileName: string;
  fileSize: number;
  lastModified: number;
  expiresAt: number;
  scope: FileScope;
  recipientIds: string[];
  chunkSize: number;
  totalParts: number;
  createdAt: number;
  receivedBytes?: number;
  completedParts?: number[];
}

export interface UploadResumeStorage {
  readonly length: number;
  key(index: number): string | null;
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

const keyPrefix = "filedock:upload-resume:";

function browserStorage(): UploadResumeStorage | undefined {
  try {
    return typeof window === "undefined" ? undefined : window.localStorage;
  } catch {
    return undefined;
  }
}

function recordKey(roomCode: string, uploadId: string) {
  return `${keyPrefix}${roomCode}:${uploadId}`;
}

export function saveUploadResume(record: UploadResumeRecord, storage = browserStorage()) {
  if (!storage) return;
  try {
    storage.setItem(recordKey(record.roomCode, record.uploadId), JSON.stringify(record));
  } catch {
    // Uploads keep running when local storage is unavailable; only refresh
    // recovery is disabled.
  }
}

export function removeUploadResume(roomCode: string, uploadId: string, storage = browserStorage()) {
  if (!storage) return;
  try {
    storage.removeItem(recordKey(roomCode, uploadId));
  } catch {
    // Ignore unavailable browser storage.
  }
}

export function findUploadResume(roomCode: string, file: Pick<File, "name" | "size" | "lastModified">, now = Date.now(), storage = browserStorage()): UploadResumeRecord | undefined {
  const records = listUploadResumes(roomCode, now, storage);
  return records.filter((record) => isUploadResumeMatch(record, file)).sort((left, right) => right.createdAt - left.createdAt)[0];
}

export function listUploadResumes(roomCode: string, now = Date.now(), storage = browserStorage()): UploadResumeRecord[] {
  if (!storage) return [];
  const result: UploadResumeRecord[] = [];
  const remove: string[] = [];
  try {
    for (let index = 0; index < storage.length; index += 1) {
      const key = storage.key(index);
      if (!key?.startsWith(`${keyPrefix}${roomCode}:`)) continue;
      const record = parseRecord(storage.getItem(key));
      if (!record || record.expiresAt * 1000 <= now) {
        remove.push(key);
        continue;
      }
      result.push(record);
    }
    remove.forEach((key) => storage.removeItem(key));
  } catch {
    return [];
  }
  return result;
}

export function clearRoomUploadResumes(roomCode: string, storage = browserStorage()) {
  if (!storage) return;
  const remove: string[] = [];
  try {
    for (let index = 0; index < storage.length; index += 1) {
      const key = storage.key(index);
      if (key?.startsWith(`${keyPrefix}${roomCode}:`)) remove.push(key);
    }
    remove.forEach((key) => storage.removeItem(key));
  } catch {
    // Ignore unavailable browser storage.
  }
}

export function isUploadResumeMatch(record: UploadResumeRecord, file: Pick<File, "name" | "size" | "lastModified">) {
  return record.fileName === file.name && record.fileSize === file.size && record.lastModified === file.lastModified;
}

function parseRecord(raw: string | null): UploadResumeRecord | undefined {
  if (!raw) return undefined;
  try {
    const value = JSON.parse(raw) as Partial<UploadResumeRecord>;
    if (typeof value.uploadId !== "string" || typeof value.fileId !== "string" || typeof value.roomCode !== "string" || typeof value.uploadUrl !== "string" || typeof value.fileName !== "string" || typeof value.fileSize !== "number" || typeof value.lastModified !== "number" || typeof value.expiresAt !== "number" || (value.scope !== "shared" && value.scope !== "direct") || !Array.isArray(value.recipientIds) || typeof value.chunkSize !== "number" || typeof value.totalParts !== "number" || typeof value.createdAt !== "number") return undefined;
    const receivedBytes = typeof value.receivedBytes === "number" && value.receivedBytes >= 0 ? value.receivedBytes : 0;
    const completedParts = Array.isArray(value.completedParts) ? value.completedParts.filter((part): part is number => typeof part === "number" && Number.isInteger(part) && part >= 0) : [];
    return { ...value, receivedBytes, completedParts } as UploadResumeRecord;
  } catch {
    return undefined;
  }
}
