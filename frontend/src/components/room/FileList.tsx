import { useInfiniteQuery } from "@tanstack/react-query";
import { ChevronDown, ChevronRight, Download, Ellipsis, File, FileLock2, FileUp, FolderOpen, Send, Share2 } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { ErrorNotice } from "../common";
import { MobileActionSheet } from "./MobileActionSheet";
import { listRoomFiles } from "../../services/api";
import type { FileIdentity, FileRange, FileSort, RoomFile } from "../../types/domain";
import { batchFileCapabilities, fileMenuActions, primaryFileAction } from "../../utils/file-permissions";
import { formatBytes, formatDate } from "../../utils/format";

interface Props {
  code: string;
  range: FileRange;
  identity: FileIdentity;
  search: string;
  sort: FileSort;
  batchMode: boolean;
  selected: Set<string>;
  onSelectedChange: (selected: Set<string>) => void;
  onAddFiles: () => void;
  onDropFiles: (files: File[]) => void;
  onDownload: (file: RoomFile) => void;
  onAccept: (file: RoomFile) => void;
  onDecline: (file: RoomFile) => void;
  onReuse: (files: RoomFile[]) => void;
  onPublish: (file: RoomFile) => void;
  onDetails: (file: RoomFile) => void;
  onBatchDownload: (files: RoomFile[]) => void;
}

export function FileList(props: Props) {
  const { t } = useTranslation();
  const [sharedOpen, setSharedOpen] = useState(true);
  const [directOpen, setDirectOpen] = useState(true);
  const [dragging, setDragging] = useState(false);
  const shared = useFileGroup(props, "shared");
  const direct = useFileGroup(props, "direct");
  const files = [...shared.items, ...direct.items];
  const selectedFiles = files.filter((file) => props.selected.has(file.fileId));
  const batch = selectedFiles.length === props.selected.size ? batchFileCapabilities(selectedFiles) : { canDownload: false, canReuse: false };
  const showShared = props.range === "all" || props.range === "shared";
  const showDirect = props.range === "all" || props.range === "direct";

  const drop = (event: React.DragEvent) => {
    event.preventDefault();
    setDragging(false);
    props.onDropFiles(Array.from(event.dataTransfer.files));
  };
  return <div className="file-list-view">
    <button className={`file-dropzone ${dragging ? "is-dragging" : ""}`} type="button" onClick={props.onAddFiles} onDragEnter={(event) => { event.preventDefault(); setDragging(true); }} onDragOver={(event) => event.preventDefault()} onDragLeave={() => setDragging(false)} onDrop={drop}>
      <FileUp aria-hidden="true" /><span><strong>{t("room.files.dropTitle")}</strong><small>{t("room.files.dropAction")}</small></span>
    </button>
    {props.batchMode && <div className="file-batch-bar"><strong>{t("room.files.selectedCount", { count: props.selected.size })}</strong><button type="button" disabled={!batch.canDownload} onClick={() => props.onBatchDownload(selectedFiles)}><Download aria-hidden="true" />{t("room.files.batchDownload")}</button><button type="button" disabled={!batch.canReuse} onClick={() => props.onReuse(selectedFiles)}><Send aria-hidden="true" />{t("room.files.batchSend")}</button></div>}
    <div className="file-groups">
      {showShared && <FileGroupView title={t("room.files.sharedFiles")} icon={<FolderOpen aria-hidden="true" />} open={sharedOpen} onToggle={() => setSharedOpen((value) => !value)} query={shared} batchMode={props.batchMode} selected={props.selected} onSelectedChange={props.onSelectedChange} {...fileActions(props)} />}
      {showDirect && <FileGroupView title={t("room.files.privateFiles")} icon={<FileLock2 aria-hidden="true" />} open={directOpen} onToggle={() => setDirectOpen((value) => !value)} query={direct} batchMode={props.batchMode} selected={props.selected} onSelectedChange={props.onSelectedChange} {...fileActions(props)} />}
    </div>
  </div>;
}

function fileActions(props: Props) {
  return { onDownload: props.onDownload, onAccept: props.onAccept, onDecline: props.onDecline, onReuse: (file: RoomFile) => props.onReuse([file]), onPublish: props.onPublish, onDetails: props.onDetails };
}

function useFileGroup(props: Props, scope: "shared" | "direct") {
  const enabled = props.range === "all" || props.range === scope;
  const query = useInfiniteQuery({
    queryKey: ["room-files", props.code, props.range, scope, props.identity, props.search, props.sort],
    enabled,
    initialPageParam: "",
    queryFn: ({ pageParam }) => listRoomFiles(props.code, { range: scope, identity: props.identity, search: props.search, sort: props.sort, limit: 30, sharedCursor: scope === "shared" ? pageParam : undefined, directCursor: scope === "direct" ? pageParam : undefined }),
    getNextPageParam: (page) => (scope === "shared" ? page.shared?.nextCursor : page.direct?.nextCursor) || undefined,
  });
  const groups = query.data?.pages.map((page) => scope === "shared" ? page.shared : page.direct).filter((group) => group != null) ?? [];
  return { query, items: groups.flatMap((group) => group.items), total: groups[0]?.total ?? 0 };
}

type GroupQuery = ReturnType<typeof useFileGroup>;
interface GroupProps {
  title: string;
  icon: React.ReactNode;
  open: boolean;
  onToggle: () => void;
  query: GroupQuery;
  batchMode: boolean;
  selected: Set<string>;
  onSelectedChange: (selected: Set<string>) => void;
  onDownload: (file: RoomFile) => void;
  onAccept: (file: RoomFile) => void;
  onDecline: (file: RoomFile) => void;
  onReuse: (file: RoomFile) => void;
  onPublish: (file: RoomFile) => void;
  onDetails: (file: RoomFile) => void;
}

function FileGroupView(props: GroupProps) {
  const { t } = useTranslation();
  return <section className="file-group">
    <button className="file-group-heading" type="button" aria-expanded={props.open} onClick={props.onToggle}>{props.open ? <ChevronDown aria-hidden="true" /> : <ChevronRight aria-hidden="true" />}{props.icon}<strong>{props.title}</strong><span>{props.query.total}</span></button>
    {props.open && <div className="file-group-content">
      {props.query.query.isPending && <p className="file-state-message">{t("room.files.loading")}</p>}
      {props.query.query.isError && <ErrorNotice error={props.query.query.error} onRetry={() => void props.query.query.refetch()} />}
      {!props.query.query.isPending && props.query.items.length === 0 && <p className="file-state-message">{t("room.files.emptyGroup")}</p>}
      {props.query.items.map((file) => <FileRow key={file.fileId} file={file} {...props} />)}
      {props.query.query.hasNextPage && <button className="file-load-more" type="button" disabled={props.query.query.isFetchingNextPage} onClick={() => void props.query.query.fetchNextPage()}>{props.query.query.isFetchingNextPage ? t("room.files.loading") : t("room.files.loadMore")}</button>}
    </div>}
  </section>;
}

function FileRow({ file, batchMode, selected, onSelectedChange, onDownload, onAccept, onDecline, onReuse, onPublish, onDetails }: GroupProps & { file: RoomFile }) {
  const { t } = useTranslation();
  const [menuOpen, setMenuOpen] = useState(false);
  const primary = primaryFileAction(file);
  const actions = fileMenuActions(file);
  const checked = selected.has(file.fileId);
  const toggle = () => {
    const next = new Set(selected);
    if (checked) next.delete(file.fileId); else next.add(file.fileId);
    onSelectedChange(next);
  };
  const action = (name: string) => {
    setMenuOpen(false);
    if (name === "reuse") onReuse(file);
    if (name === "publish") onPublish(file);
    if (name === "details") onDetails(file);
  };
  return <article className={`file-row ${file.projection === "anonymous" ? "is-anonymous" : ""}`}>
    {batchMode && <input type="checkbox" checked={checked} aria-label={file.displayName} onChange={toggle} />}
    <span className="file-type-icon">{file.scope === "direct" ? <FileLock2 aria-hidden="true" /> : <File aria-hidden="true" />}</span>
    <div className="file-row-main"><strong>{file.scope === "direct" && file.privateCode ? `${t("room.files.privateNumber")}#${file.privateCode}` : file.displayName}</strong>{file.scope === "direct" && file.privateCode && file.projection === "full" && <span>{file.displayName}</span>}<small>{file.uploaderName} · {formatBytes(file.size)} · {formatDate(file.completedAt ?? file.createdAt)}</small>{file.recipients && file.recipients.length > 0 && <em>{file.recipients.map((recipient) => `${recipient.displayName}${recipient.status ? ` · ${t(`room.files.recipientStatus.${recipient.status}`)}` : ""}`).join("，")}</em>}</div>
    <span className={`file-scope-badge ${file.scope}`}>{file.projection === "anonymous" ? t("room.files.anonymous") : file.scope === "shared" ? t("room.files.shared") : t("room.files.private")}</span>
    <div className="file-row-actions">
      {primary === "download" && <button className="primary" type="button" onClick={() => onDownload(file)}><Download aria-hidden="true" />{t("room.files.download")}</button>}
      {primary === "accept" && <><button className="primary" type="button" onClick={() => onAccept(file)}>{t("room.files.acceptDownload")}</button><button type="button" onClick={() => onDecline(file)}>{t("room.files.decline")}</button></>}
      {primary === "progress" && <span className="file-row-progress"><progress max={100} value={file.progress} aria-label={t("room.files.uploadProgress", { percent: String(file.progress) })} />{file.progress}%</span>}
      <div className="file-menu-host"><button type="button" aria-label={t("room.files.fileActions")} aria-expanded={menuOpen} onClick={() => setMenuOpen((value) => !value)}><Ellipsis aria-hidden="true" /></button><MobileActionSheet title={file.displayName} open={menuOpen} onClose={() => setMenuOpen(false)}>{actions.map((name) => <button key={name} type="button" role="menuitem" onClick={() => action(name)}>{name === "reuse" ? <Send aria-hidden="true" /> : name === "publish" ? <Share2 aria-hidden="true" /> : <File aria-hidden="true" />}{t(`room.files.actions.${name}`)}</button>)}</MobileActionSheet></div>
    </div>
  </article>;
}
