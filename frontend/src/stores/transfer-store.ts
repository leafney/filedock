import { create } from "zustand";

import { cancelFileUpload, createUploadBatch, getUploadStatus, startNativeDownload, uploadFilePart, UploadRequestError } from "../services/api";
import type { DownloadTask, FileScope, UploadBatch, UploadBatchFile, UploadSession } from "../types/domain";
import { downloadStatusForProgress, selectQueuedStarts, transferProgress } from "../utils/transfer-queue";
import { broadcastUploadMessage, acquireUploadLease, releaseUploadLease, renewUploadLease, subscribeUploadMessages, uploadLeaseDuration } from "../utils/upload-coordinator";
import { removeUploadResume, saveUploadResume, type UploadResumeRecord } from "../utils/upload-resume";
import { sha256Hex } from "../utils/sha256";

export type UploadTransferStatus = "queued" | "uploading" | "paused" | "completed" | "failed" | "cancelled";
export type DownloadTransferStatus = "queued" | "starting" | "downloading" | "completed" | "failed";

export interface UploadTransferTask {
  clientId: string;
  roomCode: string;
  fileId: string;
  uploadId: string;
  uploadUrl: string;
  file: File;
  scope: FileScope;
  recipientIds: string[];
  status: UploadTransferStatus;
  progress: number;
  loaded: number;
  total: number;
  speed: number;
  chunkSize: number;
  totalParts: number;
  completedParts: number[];
  error?: string;
  createdAt: number;
}

export interface DownloadTransfer {
  taskId: string;
  roomCode: string;
  fileId: string;
  fileName: string;
  status: DownloadTransferStatus;
  progress: number;
  transferred: number;
  total: number;
  downloadUrl: string;
  createdAt: number;
}

interface TransferState {
  uploads: UploadTransferTask[];
  downloads: DownloadTransfer[];
  addUploads: (tasks: UploadTransferTask[]) => void;
  patchUpload: (clientId: string, patch: Partial<UploadTransferTask>) => void;
  addDownload: (roomCode: string, task: DownloadTask) => void;
  patchDownload: (taskId: string, patch: Partial<DownloadTransfer>) => void;
  clearFinished: () => void;
}

export const useTransferStore = create<TransferState>((set) => ({
  uploads: [],
  downloads: [],
  addUploads: (tasks) => set((state) => ({ uploads: [...state.uploads, ...tasks] })),
  patchUpload: (clientId, patch) => set((state) => ({ uploads: state.uploads.map((task) => task.clientId === clientId ? { ...task, ...patch } : task) })),
  addDownload: (roomCode, task) => set((state) => ({
    downloads: [{ taskId: task.taskId, roomCode, fileId: task.fileId, fileName: task.fileName, status: "queued", progress: 0, transferred: 0, total: task.size, downloadUrl: task.downloadUrl, createdAt: Date.now() }, ...state.downloads.filter((item) => item.taskId !== task.taskId)],
  })),
  patchDownload: (taskId, patch) => set((state) => ({ downloads: state.downloads.map((task) => task.taskId === taskId ? { ...task, ...patch } : task) })),
  clearFinished: () => set((state) => ({ uploads: state.uploads.filter((task) => task.status !== "completed" && task.status !== "cancelled"), downloads: state.downloads.filter((task) => task.status !== "completed") })),
}));

const maximumConcurrentFiles = 2;
const maximumConcurrentParts = 2;
const activeRequests = new Map<string, Map<number, XMLHttpRequest>>();
const activeUploadClients = new Set<string>();
const chunkProgress = new Map<string, Map<number, number>>();
const speedSamples = new Map<string, { loaded: number; time: number }>();
const pauseRequested = new Set<string>();
const cancelRequested = new Set<string>();
const leaseTimers = new Map<string, ReturnType<typeof setInterval>>();
const activeDownloads = new Set<string>();

function makeClientId() {
  return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function buildResumeRecord(roomCode: string, task: Pick<UploadTransferTask, "fileId" | "uploadId" | "uploadUrl" | "file" | "scope" | "recipientIds" | "chunkSize" | "totalParts">, expiresAt: number): UploadResumeRecord {
  return { uploadId: task.uploadId, fileId: task.fileId, roomCode, uploadUrl: task.uploadUrl, fileName: task.file.name, fileSize: task.file.size, lastModified: task.file.lastModified, expiresAt, scope: task.scope, recipientIds: task.recipientIds, chunkSize: task.chunkSize, totalParts: task.totalParts, createdAt: Date.now() };
}

function taskFromBatch(roomCode: string, batchFile: UploadBatchFile, batch: UploadBatch, file: File, recipientIds: string[]): UploadTransferTask {
  return { clientId: makeClientId(), roomCode, fileId: batchFile.fileId, uploadId: batchFile.uploadId || batchFile.fileId, uploadUrl: batchFile.uploadUrl, file, scope: batch.scope, recipientIds, status: "queued", progress: 0, loaded: 0, total: file.size, speed: 0, chunkSize: batchFile.chunkSize, totalParts: batchFile.totalParts, completedParts: [], createdAt: Date.now() };
}

function taskFromResume(roomCode: string, session: UploadSession, record: UploadResumeRecord, file: File): UploadTransferTask {
  const completedParts = session.parts.map((part) => part.partNumber).sort((left, right) => left - right);
  return { clientId: makeClientId(), roomCode, fileId: session.fileId, uploadId: session.uploadId, uploadUrl: record.uploadUrl, file, scope: record.scope, recipientIds: record.recipientIds, status: "queued", progress: session.receivedBytes >= session.declaredSize ? 100 : transferProgress(session.receivedBytes, session.declaredSize), loaded: session.receivedBytes, total: session.declaredSize, speed: 0, chunkSize: session.chunkSize, totalParts: session.totalParts, completedParts, createdAt: record.createdAt };
}

export function enqueueUploadBatch(roomCode: string, batch: UploadBatch, files: File[], recipientIds: string[]) {
  const tasks = batch.files.map((serverFile, index) => taskFromBatch(roomCode, serverFile, batch, files[index], recipientIds)).filter((task) => task.file != null);
  for (const task of tasks) saveUploadResume(buildResumeRecord(roomCode, task, batch.files.find((item) => item.fileId === task.fileId)?.expiresAt ?? 0));
  useTransferStore.getState().addUploads(tasks);
  pumpUploadQueue();
  return tasks;
}

export function enqueueResumedUpload(roomCode: string, session: UploadSession, record: UploadResumeRecord, file: File) {
  const task = taskFromResume(roomCode, session, record, file);
  saveUploadResume({ ...record, expiresAt: session.expiresAt, chunkSize: session.chunkSize, totalParts: session.totalParts });
  useTransferStore.getState().addUploads([task]);
  pumpUploadQueue();
  return task;
}

function pumpUploadQueue() {
  const state = useTransferStore.getState();
  const starts = selectQueuedStarts(state.uploads.map((task) => ({ id: task.clientId, status: task.status === "queued" ? "queued" : activeUploadClients.has(task.clientId) || task.status === "paused" ? "active" : task.status === "completed" ? "completed" : task.status === "failed" ? "failed" : "cancelled" })), new Set(activeUploadClients), maximumConcurrentFiles);
  for (const id of starts) {
    const task = useTransferStore.getState().uploads.find((item) => item.clientId === id);
    if (task) void startUpload(task);
  }
}

async function startUpload(task: UploadTransferTask) {
  if (activeUploadClients.has(task.clientId) || !acquireUploadLease(task.uploadId)) {
    if (!activeUploadClients.has(task.clientId)) useTransferStore.getState().patchUpload(task.clientId, { status: "paused", error: "other_tab" });
    return;
  }
  activeUploadClients.add(task.clientId);
  activeRequests.set(task.clientId, new Map());
  chunkProgress.set(task.clientId, new Map());
  speedSamples.set(task.clientId, { loaded: task.loaded, time: performance.now() });
  useTransferStore.getState().patchUpload(task.clientId, { status: "uploading", error: undefined });
  const timer = setInterval(() => { if (!renewUploadLease(task.uploadId)) pauseUploadTransfer(task.clientId); }, Math.max(1000, uploadLeaseDuration() / 3));
  leaseTimers.set(task.clientId, timer);
  try {
    const session = await getUploadStatus(task.roomCode, task.fileId);
    syncSession(task.clientId, session);
    if (session.status === "completed" || session.receivedBytes >= session.declaredSize) {
      finishUpload(task);
      return;
    }
    const missing = Array.from({ length: session.totalParts }, (_, partNumber) => partNumber).filter((partNumber) => !session.parts.some((part) => part.partNumber === partNumber));
    let cursor = 0;
    const workers = Array.from({ length: Math.min(maximumConcurrentParts, missing.length) }, async () => {
      while (cursor < missing.length && !pauseRequested.has(task.clientId) && !cancelRequested.has(task.clientId)) {
        const partNumber = missing[cursor];
        cursor += 1;
        await uploadPartWithRetry(task.clientId, task, partNumber);
      }
    });
    await Promise.all(workers);
    if (pauseRequested.has(task.clientId) || cancelRequested.has(task.clientId)) return;
    const completed = await getUploadStatus(task.roomCode, task.fileId);
    syncSession(task.clientId, completed);
    if (completed.status === "completed" || completed.receivedBytes >= completed.declaredSize) finishUpload(task);
    else throw new Error("upload");
  } catch (error) {
    if (cancelRequested.has(task.clientId)) return;
    if (pauseRequested.has(task.clientId) || isAbortError(error)) {
      useTransferStore.getState().patchUpload(task.clientId, { status: "paused", speed: 0 });
      return;
    }
    if (error instanceof UploadRequestError && error.code === 40929) {
      await restartUploadTransfer(task);
      return;
    }
    useTransferStore.getState().patchUpload(task.clientId, { status: "failed", speed: 0, error: error instanceof Error ? error.message : "upload" });
    broadcastUploadMessage({ uploadId: task.uploadId, type: "status", payload: { status: "failed" } });
    announceRefresh(task.roomCode);
  } finally {
    clearInterval(timer);
    leaseTimers.delete(task.clientId);
    activeRequests.delete(task.clientId);
    chunkProgress.delete(task.clientId);
    speedSamples.delete(task.clientId);
    activeUploadClients.delete(task.clientId);
    releaseUploadLease(task.uploadId);
    pumpUploadQueue();
  }
}

async function uploadPartWithRetry(clientId: string, task: UploadTransferTask, partNumber: number) {
  const { start, end } = partBounds(task, partNumber);
  const blob = task.file.slice(start, end + 1);
  const sha256 = await digestSHA256(blob);
  for (let attempt = 0; attempt < 3; attempt += 1) {
    if (pauseRequested.has(clientId) || cancelRequested.has(clientId)) throw new DOMException("Upload cancelled", "AbortError");
    const transfer = uploadFilePart(task.uploadUrl, blob, partNumber, start, end, task.total, sha256, (progress) => updateChunkProgress(clientId, task, partNumber, progress.loaded));
    activeRequests.get(clientId)?.set(partNumber, transfer.request);
    try {
      await transfer.promise;
      chunkProgress.get(clientId)?.delete(partNumber);
      markPartCompleted(clientId, task, partNumber);
      return;
    } catch (error) {
      if (isAbortError(error) || error instanceof UploadRequestError && error.code === 40929) throw error;
      if (attempt === 2) throw error;
      await wait(250 * (attempt + 1));
    } finally {
      activeRequests.get(clientId)?.delete(partNumber);
    }
  }
}

function syncSession(clientId: string, session: UploadSession) {
  const task = useTransferStore.getState().uploads.find((item) => item.clientId === clientId);
  if (!task) return;
  const completedParts = session.parts.map((part) => part.partNumber).sort((left, right) => left - right);
  useTransferStore.getState().patchUpload(clientId, { chunkSize: session.chunkSize, totalParts: session.totalParts, completedParts, loaded: session.receivedBytes, total: session.declaredSize, progress: transferProgress(session.receivedBytes, session.declaredSize) });
  saveUploadResume({ ...buildResumeRecord(task.roomCode, task, session.expiresAt), expiresAt: session.expiresAt, chunkSize: session.chunkSize, totalParts: session.totalParts });
}

function markPartCompleted(clientId: string, task: UploadTransferTask, partNumber: number) {
  const current = useTransferStore.getState().uploads.find((item) => item.clientId === clientId);
  if (!current) return;
  const completedParts = current.completedParts.includes(partNumber) ? current.completedParts : [...current.completedParts, partNumber].sort((left, right) => left - right);
  useTransferStore.getState().patchUpload(clientId, { completedParts });
  updateTaskProgress(clientId, task);
  broadcastUploadMessage({ uploadId: task.uploadId, type: "progress", payload: { completedParts, loaded: useTransferStore.getState().uploads.find((item) => item.clientId === clientId)?.loaded ?? 0, progress: useTransferStore.getState().uploads.find((item) => item.clientId === clientId)?.progress ?? 0 } });
}

function updateChunkProgress(clientId: string, task: UploadTransferTask, partNumber: number, loaded: number) {
  chunkProgress.get(clientId)?.set(partNumber, loaded);
  updateTaskProgress(clientId, task);
}

function updateTaskProgress(clientId: string, task: UploadTransferTask) {
  const current = useTransferStore.getState().uploads.find((item) => item.clientId === clientId);
  if (!current) return;
  const completed = current.completedParts.reduce((total, partNumber) => total + partBounds(task, partNumber).length, 0);
  const active = Array.from(chunkProgress.get(clientId)?.values() ?? []).reduce((total, loaded) => total + loaded, 0);
  const loaded = Math.min(task.total, completed + active);
  const now = performance.now();
  const sample = speedSamples.get(clientId);
  const elapsed = sample ? Math.max(1, now - sample.time) : 1;
  const speed = sample ? Math.max(0, (loaded - sample.loaded) * 1000 / elapsed) : 0;
  if (!sample || elapsed >= 400 || loaded >= task.total) speedSamples.set(clientId, { loaded, time: now });
  useTransferStore.getState().patchUpload(clientId, { loaded, progress: transferProgress(loaded, task.total), total: task.total, speed });
}

function finishUpload(task: UploadTransferTask) {
  useTransferStore.getState().patchUpload(task.clientId, { status: "completed", progress: 100, loaded: task.total, speed: 0, completedParts: Array.from({ length: task.totalParts }, (_, index) => index) });
  removeUploadResume(task.roomCode, task.uploadId);
  broadcastUploadMessage({ uploadId: task.uploadId, type: "status", payload: { status: "completed" } });
  announceRefresh(task.roomCode);
}

export function pauseUploadTransfer(clientId: string) {
  const task = useTransferStore.getState().uploads.find((item) => item.clientId === clientId);
  if (!task || task.status === "completed" || task.status === "cancelled") return;
  pauseRequested.add(clientId);
  useTransferStore.getState().patchUpload(clientId, { status: "paused", speed: 0 });
  activeRequests.get(clientId)?.forEach((request) => request.abort());
  releaseUploadLease(task.uploadId);
  broadcastUploadMessage({ uploadId: task.uploadId, type: "status", payload: { status: "paused" } });
}

export function resumeUploadTransfer(clientId: string) {
  const task = useTransferStore.getState().uploads.find((item) => item.clientId === clientId);
  if (!task || task.status === "completed" || task.status === "cancelled") return;
  pauseRequested.delete(clientId);
  cancelRequested.delete(clientId);
  useTransferStore.getState().patchUpload(clientId, { status: "queued", error: undefined });
  pumpUploadQueue();
}

export function cancelUploadTransfer(clientId: string) {
  const task = useTransferStore.getState().uploads.find((item) => item.clientId === clientId);
  if (!task || task.status === "completed" || task.status === "cancelled") return;
  cancelRequested.add(clientId);
  pauseRequested.delete(clientId);
  useTransferStore.getState().patchUpload(clientId, { status: "cancelled", speed: 0 });
  activeRequests.get(clientId)?.forEach((request) => request.abort());
  releaseUploadLease(task.uploadId);
  removeUploadResume(task.roomCode, task.uploadId);
  broadcastUploadMessage({ uploadId: task.uploadId, type: "deleted" });
  void cancelFileUpload(task.roomCode, task.fileId).finally(() => announceRefresh(task.roomCode));
  if (!activeUploadClients.has(clientId)) pumpUploadQueue();
}

export function retryUploadTransfer(task: UploadTransferTask) {
  pauseRequested.delete(task.clientId);
  cancelRequested.delete(task.clientId);
  useTransferStore.getState().patchUpload(task.clientId, { status: "queued", error: undefined, speed: 0 });
  pumpUploadQueue();
}

async function restartUploadTransfer(task: UploadTransferTask) {
  removeUploadResume(task.roomCode, task.uploadId);
  cancelRequested.add(task.clientId);
  useTransferStore.getState().patchUpload(task.clientId, { status: "cancelled", speed: 0, error: "content_changed" });
  try {
    await cancelFileUpload(task.roomCode, task.fileId);
  } catch {
    // The old session can still be cleaned up by its room expiry.
  }
  const batch = await createUploadBatch(task.roomCode, makeClientId(), task.scope, task.recipientIds, [{ originalName: task.file.name, declaredSize: task.file.size, declaredMime: task.file.type || "application/octet-stream" }]);
  enqueueUploadBatch(task.roomCode, batch, [task.file], task.recipientIds);
}

export function applyDownloadProgress(payload: Record<string, unknown>) {
  const taskId = typeof payload.taskId === "string" ? payload.taskId : "";
  if (!taskId) return;
  const reported = typeof payload.progress === "number" ? payload.progress : undefined;
  const transferred = typeof payload.transferred === "number" ? payload.transferred : 0;
  const current = useTransferStore.getState().downloads.find((item) => item.taskId === taskId);
  const progress = reported ?? transferProgress(transferred, current?.total ?? 0);
  useTransferStore.getState().patchDownload(taskId, { status: downloadStatusForProgress(progress), progress, transferred });
  if (progress >= 100) {
    activeDownloads.delete(taskId);
    pumpDownloadQueue();
  }
}

export function enqueueDownload(roomCode: string, task: DownloadTask) {
  useTransferStore.getState().addDownload(roomCode, task);
  pumpDownloadQueue();
}

function pumpDownloadQueue() {
  while (activeDownloads.size < 2) {
    const task = useTransferStore.getState().downloads.find((item) => item.status === "queued");
    if (!task) return;
    activeDownloads.add(task.taskId);
    useTransferStore.getState().patchDownload(task.taskId, { status: "starting" });
    startNativeDownload({ taskId: task.taskId, fileId: task.fileId, fileName: task.fileName, size: task.total, expiresAt: 0, downloadUrl: task.downloadUrl });
  }
}

export const fileRefreshEventName = "filedock:file-refresh";

function announceRefresh(roomCode: string) {
  window.dispatchEvent(new CustomEvent(fileRefreshEventName, { detail: { roomCode } }));
}

function partBounds(task: Pick<UploadTransferTask, "total" | "chunkSize" | "totalParts">, partNumber: number) {
  const start = partNumber * task.chunkSize;
  const end = Math.min(task.total - 1, start + task.chunkSize - 1);
  return { start, end, length: end - start + 1 };
}

async function digestSHA256(blob: Blob) {
  const bytes = new Uint8Array(await blob.arrayBuffer());
  if (globalThis.crypto?.subtle) {
    const digest = await globalThis.crypto.subtle.digest("SHA-256", bytes);
    return Array.from(new Uint8Array(digest), (value) => value.toString(16).padStart(2, "0")).join("");
  }
  return sha256Hex(bytes);
}

function isAbortError(error: unknown) {
  return error instanceof DOMException && error.name === "AbortError";
}

function wait(delay: number) {
  return new Promise<void>((resolve) => window.setTimeout(resolve, delay));
}

subscribeUploadMessages((message) => {
  const task = useTransferStore.getState().uploads.find((item) => item.uploadId === message.uploadId);
  if (!task) return;
  if (message.type === "deleted") {
    useTransferStore.getState().patchUpload(task.clientId, { status: "cancelled", speed: 0 });
    return;
  }
  if (message.type === "status" && message.payload?.status === "paused" && !activeUploadClients.has(task.clientId)) {
    useTransferStore.getState().patchUpload(task.clientId, { status: "paused", speed: 0 });
    return;
  }
  if (message.type === "progress" && Array.isArray(message.payload?.completedParts)) {
    useTransferStore.getState().patchUpload(task.clientId, { completedParts: message.payload.completedParts.filter((value): value is number => typeof value === "number"), loaded: typeof message.payload.loaded === "number" ? message.payload.loaded : task.loaded, progress: typeof message.payload.progress === "number" ? message.payload.progress : task.progress });
  }
});
