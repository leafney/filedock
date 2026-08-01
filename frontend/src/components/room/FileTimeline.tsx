import { useInfiniteQuery } from "@tanstack/react-query";
import { Activity, ChevronDown } from "lucide-react";
import { useTranslation } from "react-i18next";

import { ErrorNotice } from "../common";
import { listFileEvents } from "../../services/api";
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
    {items.length > 0 && <div className="file-timeline-axis">{items.map((event) => <article key={event.eventId}>
      <i aria-hidden="true" />
      <div><header><strong>{t(eventTranslationKey(event.type))}</strong><time>{formatDate(event.createdAt)}</time></header><p>{event.actorName || t("room.files.systemActor")}{event.file ? ` · ${event.file.displayName}` : ""}</p>{event.file && <span>{formatBytes(event.file.size)} · {event.file.scope === "shared" ? t("room.files.shared") : t("room.files.private")}</span>}</div>
    </article>)}</div>}
    {events.hasNextPage && <button className="file-load-more" type="button" disabled={events.isFetchingNextPage} onClick={() => void events.fetchNextPage()}><ChevronDown aria-hidden="true" />{events.isFetchingNextPage ? t("room.files.loading") : t("room.files.loadMore")}</button>}
  </section>;
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
} as const;

function eventTranslationKey(type: string): (typeof eventKeys)[keyof typeof eventKeys] {
  return eventKeys[type as keyof typeof eventKeys] ?? eventKeys.unknown;
}
