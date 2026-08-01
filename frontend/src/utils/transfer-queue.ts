export interface QueueItem {
  id: string;
  status: "queued" | "active" | "completed" | "failed" | "cancelled";
}

export function selectQueuedStarts(items: QueueItem[], activeIds: ReadonlySet<string>, maximum: number) {
  const available = Math.max(0, maximum - activeIds.size);
  return items.filter((item) => item.status === "queued" && !activeIds.has(item.id)).slice(0, available).map((item) => item.id);
}

export function transferProgress(transferred: number, total: number) {
  if (!Number.isFinite(transferred) || !Number.isFinite(total) || total <= 0) return 0;
  return Math.max(0, Math.min(100, Math.round(transferred * 100 / total)));
}

export function downloadStatusForProgress(progress: number) {
  if (progress >= 100) return "completed" as const;
  if (progress > 0) return "downloading" as const;
  return "starting" as const;
}
