import { create } from "zustand";

import { cancelFileUpload, createUploadBatch, startNativeDownload, uploadFileContent, type UploadProgress } from "../services/api";
import type { DownloadTask, FileScope, UploadBatch } from "../types/domain";

export type UploadTransferStatus = "queued" | "uploading" | "completed" | "failed" | "cancelled";
export type DownloadTransferStatus = "queued" | "starting" | "downloading" | "completed" | "failed";

export interface UploadTransferTask {
  clientId: string;
  roomCode: string;
  fileId: string;
  uploadUrl: string;
  file: File;
  scope: FileScope;
  recipientIds: string[];
  status: UploadTransferStatus;
  progress: number;
  loaded: number;
  total: number;
  speed: number;
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

const maximumConcurrentUploads = 2;
const activeRequests = new Map<string, XMLHttpRequest>();
const speedSamples = new Map<string, { loaded: number; time: number }>();
const activeDownloads = new Set<string>();

function makeClientId() {
  return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

export function enqueueUploadBatch(roomCode: string, batch: UploadBatch, files: File[], recipientIds: string[]) {
  const tasks = batch.files.map((serverFile, index): UploadTransferTask => ({
    clientId: makeClientId(),
    roomCode,
    fileId: serverFile.fileId,
    uploadUrl: serverFile.uploadUrl,
    file: files[index],
    scope: batch.scope,
    recipientIds,
    status: "queued",
    progress: 0,
    loaded: 0,
    total: files[index]?.size ?? serverFile.declaredSize,
    speed: 0,
    createdAt: Date.now(),
  })).filter((task) => task.file != null);
  useTransferStore.getState().addUploads(tasks);
  pumpUploadQueue();
  return tasks;
}

function pumpUploadQueue() {
  while (activeRequests.size < maximumConcurrentUploads) {
    const task = useTransferStore.getState().uploads.find((item) => item.status === "queued" && !activeRequests.has(item.clientId));
    if (!task) return;
    startUpload(task);
  }
}

function startUpload(task: UploadTransferTask) {
  const store = useTransferStore.getState();
  store.patchUpload(task.clientId, { status: "uploading", error: undefined });
  speedSamples.set(task.clientId, { loaded: 0, time: performance.now() });
  const transfer = uploadFileContent(task.uploadUrl, task.file, (progress) => updateProgress(task.clientId, progress));
  activeRequests.set(task.clientId, transfer.request);
  void transfer.promise.then(() => {
    useTransferStore.getState().patchUpload(task.clientId, { status: "completed", progress: 100, loaded: task.total, speed: 0 });
    announceRefresh(task.roomCode);
  }).catch((error: unknown) => {
    const current = useTransferStore.getState().uploads.find((item) => item.clientId === task.clientId);
    if (current?.status !== "cancelled") {
      useTransferStore.getState().patchUpload(task.clientId, { status: "failed", speed: 0, error: error instanceof Error ? error.message : "upload" });
      announceRefresh(task.roomCode);
    }
  }).finally(() => {
    activeRequests.delete(task.clientId);
    speedSamples.delete(task.clientId);
    pumpUploadQueue();
  });
}

function updateProgress(clientId: string, progress: UploadProgress) {
  const now = performance.now();
  const sample = speedSamples.get(clientId);
  const elapsed = sample ? Math.max(1, now - sample.time) : 1;
  const speed = sample ? Math.max(0, (progress.loaded - sample.loaded) * 1000 / elapsed) : 0;
  if (!sample || elapsed >= 400 || progress.percent === 100) speedSamples.set(clientId, { loaded: progress.loaded, time: now });
  useTransferStore.getState().patchUpload(clientId, { progress: progress.percent, loaded: progress.loaded, total: progress.total, speed });
}

export function cancelUploadTransfer(clientId: string) {
  const task = useTransferStore.getState().uploads.find((item) => item.clientId === clientId);
  if (!task || task.status === "completed" || task.status === "cancelled") return;
  useTransferStore.getState().patchUpload(clientId, { status: "cancelled", speed: 0 });
  activeRequests.get(clientId)?.abort();
  void cancelFileUpload(task.roomCode, task.fileId).finally(() => announceRefresh(task.roomCode));
  if (!activeRequests.has(clientId)) pumpUploadQueue();
}

export async function retryUploadTransfer(task: UploadTransferTask) {
  try {
    const batch = await createUploadBatch(task.roomCode, makeClientId(), task.scope, task.recipientIds, [{ originalName: task.file.name, declaredSize: task.file.size, declaredMime: task.file.type || "application/octet-stream" }]);
    enqueueUploadBatch(task.roomCode, batch, [task.file], task.recipientIds);
  } catch (error) {
    useTransferStore.getState().patchUpload(task.clientId, { error: error instanceof Error ? error.message : "upload" });
  }
}

export function applyDownloadProgress(payload: Record<string, unknown>) {
  const taskId = typeof payload.taskId === "string" ? payload.taskId : "";
  if (!taskId) return;
  const progress = typeof payload.progress === "number" ? payload.progress : 0;
  const transferred = typeof payload.transferred === "number" ? payload.transferred : 0;
  useTransferStore.getState().patchDownload(taskId, { status: progress >= 100 ? "completed" : "downloading", progress, transferred });
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
