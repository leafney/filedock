import { useInfiniteQuery } from "@tanstack/react-query";
import { Activity, ChevronDown, MoreHorizontal, X } from "lucide-react";
import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { ErrorNotice } from "../common";
import { listFileEvents } from "../../services/api";
import type { FileEventHistory, FileEventItem } from "../../types/domain";
import { formatBytes, formatDate } from "../../utils/format";

export function FileTimeline({ code }: { code: string }) {
  const { t } = useTranslation();
  const events = useInfiniteQuery({
    queryKey: ["file-events", code],
    initialPageParam: "",
    queryFn: ({ pageParam }) => listFileEvents(code, pageParam),
    getNextPageParam: (page) => page.nextCursor || undefined,
  });
  const items = events.data?.pages.flatMap((page) => page.items) ?? [];
  return <section className="file-timeline" aria-label={t("room.workspace.timeline")}>
    {events.isPending && <p className="file-state-message">{t("room.files.loading")}</p>}
    {events.isError && <ErrorNotice error={events.error} onRetry={() => void events.refetch()} />}
    {!events.isPending && items.length === 0 && <div className="file-empty-state"><Activity aria-hidden="true" /><strong>{t("room.files.noActivity")}</strong></div>}
    {items.length > 0 && <div className="file-timeline-axis">{items.map((item) => <FileEventCard key={`${item.operationId}-${item.eventId}`} item={item} />)}</div>}
    {events.hasNextPage && <button className="file-load-more" type="button" disabled={events.isFetchingNextPage} onClick={() => void events.fetchNextPage()}><ChevronDown aria-hidden="true" />{events.isFetchingNextPage ? t("room.files.loading") : t("room.files.loadMore")}</button>}
  </section>;
}

function FileEventCard({ item }: { item: FileEventItem }) {
  const { t } = useTranslation();
  const [historyOpen, setHistoryOpen] = useState(false);
  const [pinned, setPinned] = useState(false);
  const closeTimer = useRef<number | undefined>(undefined);
  const clearCloseTimer = () => {
    if (closeTimer.current !== undefined) window.clearTimeout(closeTimer.current);
  };
  const openHistory = () => {
    clearCloseTimer();
    setHistoryOpen(true);
  };
  const closeHistory = () => {
    clearCloseTimer();
    if (!pinned) closeTimer.current = window.setTimeout(() => setHistoryOpen(false), 120);
  };
  const toggleHistory = () => {
    clearCloseTimer();
    setPinned((current) => !current);
    setHistoryOpen((current) => !current);
  };
  const fileName = item.file?.displayName || t("room.files.fileUnavailable");
  const actor = item.actorName || t("room.files.systemActor");
  return <article className={`file-event-card operation-${item.operationType} ${item.recipientSummary ? "has-recipients" : ""}`}>
    <div className="file-event-card-main">
      <span className="file-event-status">{t(eventTranslationKey(item.type))}</span>
      <span className="file-event-subject" title={fileName}>{actor} · {fileName}</span>
      {item.file && <span className="file-event-meta">{formatBytes(item.file.size)} · {item.file.scope === "shared" ? t("room.files.shared") : t("room.files.private")}</span>}
      {item.recipientSummary && <span className="file-event-recipients">{t("room.files.recipientSummary", { total: String(item.recipientSummary.total), accepted: String(item.recipientSummary.accepted), pending: String(item.recipientSummary.pending), declined: String(item.recipientSummary.declined) })}</span>}
      <time className="file-event-time">{formatDate(item.createdAt)}</time>
      <div className="file-event-history-host" onMouseEnter={openHistory} onMouseLeave={closeHistory} onFocus={openHistory} onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget)) closeHistory(); }}>
        <button className="file-event-history-trigger" type="button" aria-label={t("room.files.viewHistory")} aria-expanded={historyOpen} onClick={toggleHistory} onKeyDown={(event) => { if (event.key === "Escape") { setPinned(false); setHistoryOpen(false); } }}><MoreHorizontal aria-hidden="true" /></button>
        {historyOpen && <HistoryPanel item={item} fileName={fileName} pinned={pinned} onClose={() => { setPinned(false); setHistoryOpen(false); }} />}
      </div>
    </div>
  </article>;
}

function HistoryPanel({ item, fileName, pinned, onClose }: { item: FileEventItem; fileName: string; pinned: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  return <div className={`file-event-history-layer ${pinned ? "is-pinned" : ""}`}>
    <button className="file-event-history-backdrop" type="button" aria-label={t("room.files.closeHistory")} onClick={onClose} />
    <section className="file-event-history" role="dialog" aria-label={t("room.files.historyTitle", { name: fileName })} onMouseEnter={() => undefined}>
      <header><strong>{t("room.files.historyTitle", { name: fileName })}</strong><button type="button" aria-label={t("room.workspace.close")} onClick={onClose}><X aria-hidden="true" /></button></header>
      <ol>{item.history.map((history) => <HistoryEntry key={history.eventId} history={history} />)}</ol>
      {item.skippedRecipients && item.skippedRecipients.length > 0 && <p className="file-event-history-skipped">{t("room.files.skippedRecipients", { count: item.skippedRecipients.length })}</p>}
    </section>
  </div>;
}

function HistoryEntry({ history }: { history: FileEventHistory }) {
  const { t } = useTranslation();
  return <li><div><strong>{t(eventTranslationKey(history.type))}</strong><time>{formatDate(history.createdAt)}</time></div>{history.actorName && <p>{history.actorName}</p>}{history.recipients?.map((recipient) => <span key={`${history.eventId}-${recipient.userId}`} className={`file-event-recipient-status ${recipient.status || "pending"}`}>{recipient.displayName} · {t(recipientStatusTranslationKey(recipient.status))}</span>)}</li>;
}

const eventKeys = {
  unknown: "room.files.events.unknown",
  batch_created: "room.files.events.batch_created",
  upload_started: "room.files.events.upload_started",
  upload_completed: "room.files.events.upload_completed",
  upload_failed: "room.files.events.upload_failed",
  upload_cancelled: "room.files.events.upload_cancelled",
  direct_sent: "room.files.events.direct_sent",
  reused: "room.files.events.reused",
  published_shared: "room.files.events.published_shared",
  accepted: "room.files.events.accepted",
  declined: "room.files.events.declined",
  download_started: "room.files.events.download_started",
  download_completed: "room.files.events.download_completed",
  download_failed: "room.files.events.download_failed",
  download_cancelled: "room.files.events.download_cancelled",
  trashed: "room.files.events.trashed",
  restore_requested: "room.files.events.restore_requested",
  restore_rejected: "room.files.events.restore_rejected",
  restored: "room.files.events.restored",
  purged: "room.files.events.purged",
} as const;

function eventTranslationKey(type: string): (typeof eventKeys)[keyof typeof eventKeys] {
  return eventKeys[type as keyof typeof eventKeys] ?? eventKeys.unknown;
}

const recipientStatusKeys = {
  pending: "room.files.recipientStatus.pending",
  accepted: "room.files.recipientStatus.accepted",
  declined: "room.files.recipientStatus.declined",
  downloaded: "room.files.recipientStatus.downloaded",
  skipped: "room.files.recipientStatus.skipped",
} as const;

function recipientStatusTranslationKey(status?: string): (typeof recipientStatusKeys)[keyof typeof recipientStatusKeys] {
  return recipientStatusKeys[status as keyof typeof recipientStatusKeys] ?? recipientStatusKeys.pending;
}
