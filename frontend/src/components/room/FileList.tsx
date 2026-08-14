import { useQuery } from "@tanstack/react-query";
import { Button, Dropdown, Empty } from "antd";
import { Download, Ellipsis, File, FileLock2, Send, Share2, Trash2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
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
  onDropFiles: (files: File[]) => void;
  onChooseFiles: () => void;
  onDownload: (file: RoomFile) => void;
  onAccept: (file: RoomFile) => void;
  onDecline: (file: RoomFile) => void;
  onReuse: (files: RoomFile[]) => void;
  onPublish: (file: RoomFile) => void;
  onTrash: (file: RoomFile) => void;
  onDetails: (file: RoomFile) => void;
  onBatchDownload: (files: RoomFile[]) => void;
  onCounts: (counts: { all: number; shared: number; direct: number }) => void;
}

export function FileList(props: Props) {
  const { t } = useTranslation();
  const [dragging, setDragging] = useState(false);
  const dragDepth = useRef(0);
  const query = useQuery({
    queryKey: ["room-files", props.code, props.range, props.identity, props.search, props.sort],
    queryFn: () => listRoomFiles(props.code, { range: props.range, identity: props.identity, search: props.search, sort: props.sort }),
  });
  const files = query.data?.items ?? [];
  useEffect(() => {
    props.onCounts(query.data?.counts ?? { all: 0, shared: 0, direct: 0 });
  }, [props.onCounts, query.data]);
  const selectedFiles = files.filter((file) => props.selected.has(file.fileId));
  const batch = selectedFiles.length === props.selected.size ? batchFileCapabilities(selectedFiles) : { canDownload: false, canReuse: false };

  const hasFiles = (event: React.DragEvent) => event.dataTransfer.types.includes("Files");
  const enter = (event: React.DragEvent) => {
    if (!hasFiles(event)) return;
    event.preventDefault();
    dragDepth.current += 1;
    setDragging(true);
  };
  const leave = (event: React.DragEvent) => {
    if (!hasFiles(event)) return;
    event.preventDefault();
    dragDepth.current = Math.max(0, dragDepth.current - 1);
    if (dragDepth.current === 0) setDragging(false);
  };
  const over = (event: React.DragEvent) => {
    if (!hasFiles(event)) return;
    event.preventDefault();
    event.dataTransfer.dropEffect = "copy";
  };
  const drop = (event: React.DragEvent) => {
    event.preventDefault();
    dragDepth.current = 0;
    setDragging(false);
    if (hasFiles(event)) props.onDropFiles(Array.from(event.dataTransfer.files));
  };
  const hasFilter = Boolean(props.search.trim()) || props.identity !== "all";
  const emptyText = props.search.trim() ? t("room.files.emptySearch") : hasFilter ? t("room.files.emptyIdentity") : t(`room.files.empty${props.range === "all" ? "All" : props.range === "shared" ? "Shared" : "Direct"}`);
  return <div className="file-list-view" onDragEnter={enter} onDragOver={over} onDragLeave={leave} onDrop={drop}>
    {props.batchMode && <div className="file-batch-bar"><strong>{t("room.files.selectedCount", { count: props.selected.size })}</strong><button type="button" disabled={!batch.canDownload} onClick={() => props.onBatchDownload(selectedFiles)}><Download aria-hidden="true" />{t("room.files.batchDownload")}</button><button type="button" disabled={!batch.canReuse} onClick={() => props.onReuse(selectedFiles)}><Send aria-hidden="true" />{t("room.files.batchSend")}</button></div>}
    {query.isPending && <p className="file-state-message">{t("room.files.loading")}</p>}
    {query.isError && <ErrorNotice error={query.error} onRetry={() => void query.refetch()} />}
    {!query.isPending && !query.isError && files.length === 0 && <div className="file-empty-state"><Empty description={emptyText}>{!hasFilter && <Button type="primary" onClick={props.onChooseFiles}>{t("room.workspace.addFiles")}</Button>}</Empty></div>}
    {!query.isPending && !query.isError && files.length > 0 && <div className="file-unified-list">{files.map((file) => <FileRow key={file.fileId} file={file} batchMode={props.batchMode} selected={props.selected} onSelectedChange={props.onSelectedChange} {...fileActions(props)} />)}</div>}
    {dragging && <div className="file-drop-overlay" aria-hidden="true"><strong>{t("room.files.dropOverlay")}</strong></div>}
  </div>;
}

function fileActions(props: Props) {
  return { onDownload: props.onDownload, onAccept: props.onAccept, onDecline: props.onDecline, onReuse: (file: RoomFile) => props.onReuse([file]), onPublish: props.onPublish, onTrash: props.onTrash, onDetails: props.onDetails };
}

interface FileRowProps {
  batchMode: boolean;
  selected: Set<string>;
  onSelectedChange: (selected: Set<string>) => void;
  onDownload: (file: RoomFile) => void;
  onAccept: (file: RoomFile) => void;
  onDecline: (file: RoomFile) => void;
  onReuse: (file: RoomFile) => void;
  onPublish: (file: RoomFile) => void;
  onTrash: (file: RoomFile) => void;
  onDetails: (file: RoomFile) => void;
}

function FileRow({ file, batchMode, selected, onSelectedChange, onDownload, onAccept, onDecline, onReuse, onPublish, onTrash, onDetails }: FileRowProps & { file: RoomFile }) {
  const { t } = useTranslation();
  const [menuOpen, setMenuOpen] = useState(false);
  const menuButtonRef = useRef<HTMLButtonElement>(null);
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
    if (name === "trash") onTrash(file);
  };
  const menuItems = actions.map((name) => ({
    key: name,
    danger: name === "trash",
    icon: name === "reuse" ? <Send aria-hidden="true" /> : name === "publish" ? <Share2 aria-hidden="true" /> : name === "trash" ? <Trash2 aria-hidden="true" /> : <File aria-hidden="true" />,
    label: t(`room.files.actions.${name}`),
    onClick: () => action(name),
  }));
  const title = file.scope === "direct" && file.privateCode ? `${t("room.files.privateNumber")}#${file.privateCode} · ${file.displayName}` : file.displayName;
  const recipientSummary = summarizeRecipients(file.recipients, t);
  const metadata = [file.uploaderName, formatBytes(file.size), formatDate(file.completedAt ?? file.createdAt), recipientSummary].filter(Boolean).join(" · ");
  return <article className={`file-row scope-${file.scope} ${file.projection === "anonymous" ? "is-anonymous" : ""}`}>
    {batchMode && <input type="checkbox" checked={checked} aria-label={file.displayName} onChange={toggle} />}
    <span className="file-type-icon">{file.scope === "direct" ? <FileLock2 aria-hidden="true" /> : <File aria-hidden="true" />}</span>
    <div className="file-row-main"><strong title={title}>{title}</strong><small title={metadata}>{metadata}</small></div>
    <span className={`file-scope-badge ${file.scope}`}>{file.projection === "anonymous" ? t("room.files.anonymous") : file.scope === "shared" ? t("room.files.shared") : t("room.files.private")}</span>
    <div className="file-row-actions">
      {primary === "download" && <button className="primary" type="button" onClick={() => onDownload(file)}><Download aria-hidden="true" />{t("room.files.download")}</button>}
      {primary === "accept" && <><button className="primary" type="button" onClick={() => onAccept(file)}>{t("room.files.acceptDownload")}</button><button type="button" onClick={() => onDecline(file)}>{t("room.files.decline")}</button></>}
      {primary === "progress" && <span className="file-row-progress"><progress max={100} value={file.progress} aria-label={t("room.files.uploadProgress", { percent: String(file.progress) })} />{file.progress}%</span>}
      <div className="file-menu-host"><div className="file-menu-desktop"><Dropdown menu={{ items: menuItems }} trigger={["click"]}><button type="button" aria-label={t("room.files.fileActions")}><Ellipsis aria-hidden="true" /></button></Dropdown></div><div className="file-menu-mobile"><button ref={menuButtonRef} type="button" aria-label={t("room.files.fileActions")} aria-expanded={menuOpen} onClick={() => setMenuOpen((value) => !value)}><Ellipsis aria-hidden="true" /></button><MobileActionSheet title={file.displayName} open={menuOpen} triggerRef={menuButtonRef} onClose={() => setMenuOpen(false)}>{actions.map((name) => <button className={name === "trash" ? "is-danger" : undefined} key={name} type="button" role="menuitem" onClick={() => action(name)}>{name === "reuse" ? <Send aria-hidden="true" /> : name === "publish" ? <Share2 aria-hidden="true" /> : name === "trash" ? <Trash2 aria-hidden="true" /> : <File aria-hidden="true" />}{t(`room.files.actions.${name}`)}</button>)}</MobileActionSheet></div></div>
    </div>
  </article>;
}

function summarizeRecipients(recipients: RoomFile["recipients"], t: ReturnType<typeof useTranslation>["t"]) {
  if (!recipients || recipients.length === 0) return "";
  const counts = recipients.reduce<Record<string, number>>((result, recipient) => {
    const status = recipient.status ?? "pending";
    result[status] = (result[status] ?? 0) + 1;
    return result;
  }, {});
  return Object.entries(counts).map(([status, count]) => `${t(`room.files.recipientStatus.${status}`)} ${count}`).join(" · ");
}
