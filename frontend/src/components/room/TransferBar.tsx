import { ChevronUp, CircleCheck, CircleX, Download, Pause, Play, RefreshCw, Trash2, Upload, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { cancelUploadTransfer, pauseUploadTransfer, retryUploadTransfer, resumeUploadTransfer, useTransferStore, type UploadTransferTask } from "../../stores/transfer-store";
import { formatBytes } from "../../utils/format";

export function TransferBar({ roomCode }: { roomCode: string }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const uploads = useTransferStore((state) => state.uploads);
  const downloads = useTransferStore((state) => state.downloads);
  const roomUploads = useMemo(() => uploads.filter((task) => task.roomCode === roomCode), [uploads, roomCode]);
  const roomDownloads = useMemo(() => downloads.filter((task) => task.roomCode === roomCode), [downloads, roomCode]);
  const clearFinished = useTransferStore((state) => state.clearFinished);
  const active = roomUploads.filter((task) => task.status === "queued" || task.status === "uploading").length + roomDownloads.filter((task) => task.status === "queued" || task.status === "starting" || task.status === "downloading").length;
  const failed = roomUploads.filter((task) => task.status === "failed" || task.status === "delete_failed").length + roomDownloads.filter((task) => task.status === "failed").length;
  const latest = roomUploads.find((task) => task.status === "uploading") ?? roomUploads.find((task) => task.status === "queued");
  const previousActive = useRef(0);
  const previousCount = useRef(0);
  const closeTimer = useRef<number>();

  useEffect(() => {
    const total = roomUploads.length + roomDownloads.length;
    if (total > previousCount.current || active > 0) {
      setOpen(true);
      if (closeTimer.current) window.clearTimeout(closeTimer.current);
    } else if (previousActive.current > 0 && active === 0) {
      closeTimer.current = window.setTimeout(() => setOpen(false), 5000);
    }
    previousActive.current = active;
    previousCount.current = total;
    return () => { if (closeTimer.current) window.clearTimeout(closeTimer.current); };
  }, [active, roomDownloads.length, roomUploads.length]);

  return <footer className="room-transfer-bar">
    <button className="transfer-summary" type="button" aria-expanded={open} onClick={() => setOpen((value) => !value)}>
      {active > 0 ? <RefreshCw className="is-spinning" aria-hidden="true" /> : failed > 0 ? <CircleX aria-hidden="true" /> : <CircleCheck aria-hidden="true" />}
      <span>{active > 0 ? t("room.files.transferActive", { count: active }) : failed > 0 ? t("room.files.transferFailed", { count: failed }) : t("room.workspace.transferIdle")}</span>
      {latest && <><progress max={100} value={latest.progress} aria-label={t("room.files.uploadProgress", { percent: String(latest.progress) })} /><b>{latest.progress}%</b></>}
      <ChevronUp className={open ? "is-open" : ""} aria-hidden="true" />
    </button>
    {open && <section className="transfer-panel">
      <header><strong>{t("room.files.transferTasks")}</strong><button type="button" onClick={clearFinished}><Trash2 aria-hidden="true" />{t("room.files.clearFinished")}</button></header>
      <div className="transfer-task-list">
        {roomUploads.map((task) => <article key={task.clientId}>
          <Upload aria-hidden="true" /><div><strong>{task.fileName}</strong><span>{uploadStatusLabel(task, t)} · {task.completedParts.length}/{task.totalParts} · {formatBytes(task.loaded)} / {formatBytes(task.total)}{task.speed > 0 ? ` · ${formatBytes(task.speed)}/s` : ""}</span><progress max={100} value={task.progress} aria-label={t("room.files.uploadProgress", { percent: String(task.progress) })} /></div>
          {task.status === "uploading" && <button type="button" aria-label={t("room.files.pauseTransfer")} onClick={() => pauseUploadTransfer(task.clientId)}><Pause aria-hidden="true" /></button>}
          {task.status === "paused" && task.file && <button type="button" aria-label={t("room.files.resumeTransfer")} onClick={() => resumeUploadTransfer(task.clientId)}><Play aria-hidden="true" /></button>}
          {task.status === "failed" && task.file && <button type="button" onClick={() => void retryUploadTransfer(task)}>{t("room.files.retry")}</button>}
          {task.status === "delete_failed" && <button type="button" onClick={() => cancelUploadTransfer(task.clientId)}>{t("room.files.retryDelete")}</button>}
          {task.status !== "completed" && task.status !== "cancelled" && task.status !== "deleting" && <button type="button" aria-label={t("room.files.deleteTransfer")} onClick={() => { if (window.confirm(t("room.files.deleteTransferConfirm", { name: task.fileName }))) cancelUploadTransfer(task.clientId); }}><X aria-hidden="true" /></button>}
        </article>)}
        {roomDownloads.map((task) => <article key={task.taskId}>
          <Download aria-hidden="true" /><div><strong>{task.fileName}</strong><span>{t(`room.files.downloadStatus.${task.status}`)} · {formatBytes(task.transferred)} / {formatBytes(task.total)}</span><progress max={100} value={task.progress} aria-label={t("room.files.downloadProgress", { percent: String(task.progress) })} /></div>
        </article>)}
        {roomUploads.length === 0 && roomDownloads.length === 0 && <p>{t("room.workspace.transferIdle")}</p>}
      </div>
    </section>}
  </footer>;
}

function uploadStatusLabel(task: UploadTransferTask, t: ReturnType<typeof useTranslation>["t"]) {
  if (task.error === "other_tab") return t("room.files.uploadErrors.otherTab");
  if (task.error === "content_changed") return t("room.files.uploadErrors.contentChanged");
  if (task.error === "delete_failed") return t("room.files.uploadErrors.deleteFailed");
  if (task.error === "expired") return t("room.files.uploadErrors.expired");
  if (task.status === "failed" && task.error && task.error !== "network" && task.error !== "upload") return task.error;
  return t(`room.files.uploadStatus.${task.status}`);
}
