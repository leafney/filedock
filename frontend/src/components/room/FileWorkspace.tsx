import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Badge, Button, Dropdown, Input, Modal, Segmented, Select, Tabs, message } from "antd";
import { CheckSquare, ChevronDown, FileLock2, FilePlus2, FolderOpen, ListFilter, Send, Square } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { ErrorNotice } from "../common";
import { FileList } from "./FileList";
import { FileTrash } from "./FileTrash";
import { FileTimeline } from "./FileTimeline";
import { FileDetailsDialog, ReusePrivateDialog, UploadComposer } from "./UploadComposer";
import { streamEventName, type StreamEventMessage } from "../../hooks/use-stream";
import { getApiErrorCode } from "../../lib/api-error";
import { acceptPrivateFile, createFileDownload, createUploadBatch, declinePrivateFile, getUploadStatus, listFileTrash, publishPrivateFile, reusePrivateFiles, trashRoomFile } from "../../services/api";
import { applyDownloadProgress, enqueueDownload, enqueueResumedUpload, enqueueUploadBatch, fileRefreshEventName, markUploadResumeUnavailable, removeUploadTask, restoreUploadPlaceholders } from "../../stores/transfer-store";
import type { FileIdentity, FileRange, FileScope, FileSort, RoomFile, RoomMember } from "../../types/domain";
import { formatBytes } from "../../utils/format";
import { shouldConsumeNotificationLaunch } from "../../utils/notifications";
import { findUploadResume, removeUploadResume } from "../../utils/upload-resume";

export function FileWorkspace({ code, members, selfId, fileLaunch, onFileLaunchConsumed }: { code: string; members: RoomMember[]; selfId: string; fileLaunch?: { token: string; view: "list" | "timeline" | "trash"; requestId?: string }; onFileLaunchConsumed: (token: string) => void }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const inputRef = useRef<HTMLInputElement>(null);
  const [view, setView] = useState<"list" | "timeline" | "trash">("list");
  const [range, setRange] = useState<FileRange>("all");
  const [identity, setIdentity] = useState<FileIdentity>("all");
  const [sort, setSort] = useState<FileSort>("newest");
  const [search, setSearch] = useState("");
  const [batchMode, setBatchMode] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [drafts, setDrafts] = useState<File[]>([]);
  const [reuseFiles, setReuseFiles] = useState<string[] | null>(null);
  const [details, setDetails] = useState<RoomFile | null>(null);
  const [actionError, setActionError] = useState<unknown>();
  const [trashSearch, setTrashSearch] = useState("");
  const [trashFile, setTrashFile] = useState<RoomFile>();
  const [trashReason, setTrashReason] = useState("");
  const [trashError, setTrashError] = useState<unknown>();
  const [trashPending, setTrashPending] = useState(false);
  const [fileCounts, setFileCounts] = useState({ all: 0, shared: 0, direct: 0 });
  const [trashLaunch, setTrashLaunch] = useState<{ token: string; requestId?: string }>();
  const consumedFileLaunchToken = useRef<string>();
  const trashCountQuery = useQuery({ queryKey: ["file-trash-count", code], queryFn: () => listFileTrash(code, "", "", 1), retry: false });

  useEffect(() => {
    restoreUploadPlaceholders(code);
  }, [code]);

  const refresh = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ["room-files", code] });
    void queryClient.invalidateQueries({ queryKey: ["file-trash", code] });
    void queryClient.invalidateQueries({ queryKey: ["file-trash-count", code] });
    void queryClient.invalidateQueries({ queryKey: ["file-trash-approval", code] });
    void queryClient.invalidateQueries({ queryKey: ["file-events", code] });
    void queryClient.invalidateQueries({ queryKey: ["reusable-private-files", code] });
    void queryClient.invalidateQueries({ queryKey: ["room", code] });
    void queryClient.invalidateQueries({ queryKey: ["notifications"] });
  }, [code, queryClient]);

  useEffect(() => {
    if (!fileLaunch || !shouldConsumeNotificationLaunch(consumedFileLaunchToken.current, fileLaunch.token)) return;
    consumedFileLaunchToken.current = fileLaunch.token;
    setView(fileLaunch.view);
    setTrashLaunch(fileLaunch.view === "trash" ? { token: fileLaunch.token, requestId: fileLaunch.requestId } : undefined);
    onFileLaunchConsumed(fileLaunch.token);
  }, [fileLaunch, onFileLaunchConsumed]);

  useEffect(() => {
    const stream = (raw: Event) => {
      const event = (raw as CustomEvent<StreamEventMessage>).detail?.event;
      if (!event) return;
      const payload = (event.payload ?? {}) as Record<string, unknown>;
      if (event.type === "file.download_progress") { applyDownloadProgress(payload); return; }
      if (event.type.startsWith("file.") && payload.roomCode === code) refresh();
    };
    const local = (raw: Event) => {
      const roomCode = (raw as CustomEvent<{ roomCode?: string }>).detail?.roomCode;
      if (roomCode === code) refresh();
    };
    window.addEventListener(streamEventName, stream);
    window.addEventListener(fileRefreshEventName, local);
    return () => { window.removeEventListener(streamEventName, stream); window.removeEventListener(fileRefreshEventName, local); };
  }, [code, refresh]);

  const upload = useMutation({
    mutationFn: async ({ files, scope, recipientIds }: { files: File[]; scope: FileScope; recipientIds: string[] }) => {
      const freshFiles: File[] = [];
      for (const file of files) {
        const record = findUploadResume(code, file);
        if (!record) {
          freshFiles.push(file);
          continue;
        }
        try {
          const session = await getUploadStatus(code, record.fileId);
          if (session.status === "active" && session.expiresAt * 1000 > Date.now()) {
            enqueueResumedUpload(code, session, record, file);
            message.info(t("room.files.resumeUploadNotice", { name: file.name }));
            continue;
          }
          if (session.status === "completed") {
            removeUploadResume(code, record.uploadId);
            removeUploadTask(record.uploadId);
            continue;
          }
        } catch (error) {
          const errorCode = getApiErrorCode(error);
          if (![40403, 40405, 40921, 40922].includes(errorCode ?? 0)) throw error;
        }
        removeUploadResume(code, record.uploadId);
        markUploadResumeUnavailable(record.uploadId);
        freshFiles.push(file);
      }
      if (freshFiles.length > 0) {
        const batch = await createUploadBatch(code, makeRequestId(), scope, recipientIds, freshFiles.map((file) => ({ originalName: file.name, declaredSize: file.size, declaredMime: file.type || "application/octet-stream" })));
        enqueueUploadBatch(code, batch, freshFiles, recipientIds);
      }
    },
    onSuccess: () => { setDrafts([]); refresh(); },
  });
  const reuse = useMutation({ mutationFn: ({ fileIds, recipientIds }: { fileIds: string[]; recipientIds: string[] }) => reusePrivateFiles(code, fileIds, recipientIds), onSuccess: () => { setReuseFiles(null); refresh(); } });

  const addFiles = (files: File[]) => {
    if (files.length === 0) return;
    setDrafts((current) => [...current, ...files]);
  };
  const chooseFiles = () => inputRef.current?.click();
  const download = async (file: RoomFile) => {
    setActionError(undefined);
    try { enqueueDownload(code, await createFileDownload(code, file.fileId)); } catch (error) { setActionError(error); }
  };
  const accept = async (file: RoomFile) => {
    setActionError(undefined);
    try { enqueueDownload(code, await acceptPrivateFile(code, file.fileId)); refresh(); } catch (error) { setActionError(error); }
  };
  const decline = async (file: RoomFile) => {
    if (!window.confirm(t("room.files.declineConfirm", { name: file.displayName }))) return;
    setActionError(undefined);
    try { await declinePrivateFile(code, file.fileId); refresh(); } catch (error) { setActionError(error); }
  };
  const publish = async (file: RoomFile) => {
    if (!window.confirm(t("room.files.publishConfirm", { name: file.displayName }))) return;
    setActionError(undefined);
    try { await publishPrivateFile(code, file.fileId); refresh(); } catch (error) { setActionError(error); }
  };
  const openTrash = (file: RoomFile) => {
    setTrashFile(file);
    setTrashReason("");
    setTrashError(undefined);
  };
  const confirmTrash = async () => {
    if (!trashFile) return;
    setTrashPending(true);
    setTrashError(undefined);
    try {
      await trashRoomFile(code, trashFile.fileId, trashFile.capabilities.canSetTrashReason ? trashReason.trim() : "");
      setTrashFile(undefined);
      setTrashReason("");
      message.success(t("room.files.trashed"));
      refresh();
    } catch (error) {
      setTrashError(error);
    } finally {
      setTrashPending(false);
    }
  };
  const batchDownload = async (files: RoomFile[]) => {
    setActionError(undefined);
    try { for (const file of files) enqueueDownload(code, await createFileDownload(code, file.fileId)); } catch (error) { setActionError(error); }
  };
  const changeFilter = <T,>(apply: (value: T) => void, value: T) => {
    if (selected.size > 0 && !window.confirm(t("room.files.keepHiddenSelection"))) setSelected(new Set());
    apply(value);
  };
  const toggleBatch = () => { setBatchMode((value) => !value); setSelected(new Set()); };
  useEffect(() => {
    if (range === "shared" && identity === "received") setIdentity("all");
  }, [identity, range]);
  const identityItems = [
    { key: "all", label: t("room.files.identity.all") },
    { key: "uploaded", label: t("room.files.identity.uploaded") },
    ...(range === "shared" ? [] : [{ key: "received", label: t("room.files.identity.received") }]),
  ];
  const identityLabel = identityItems.find((item) => item.key === identity)?.label ?? t("room.files.identity.all");
  const rangeOptions = (["all", "shared", "direct"] as FileRange[]).map((value) => ({
    value,
    label: <span className="file-range-option">{value === "shared" ? <FolderOpen aria-hidden="true" /> : value === "direct" ? <FileLock2 aria-hidden="true" /> : null}{t(`room.files.range.${value}`)} <Badge count={fileCounts[value]} showZero size="small" /></span>,
  }));
  const viewTabs = [
    { key: "list", label: t("room.workspace.fileList") },
    { key: "timeline", label: t("room.workspace.timeline") },
    { key: "trash", label: <span>{t("room.workspace.trash")} <Badge count={trashCountQuery.data?.total ?? 0} showZero size="small" /></span> },
  ];

  return <section className="room-files-shell">
    <input ref={inputRef} className="visually-hidden" type="file" multiple onChange={(event) => { addFiles(Array.from(event.target.files ?? [])); event.target.value = ""; }} />
    <div className="room-file-header">
      <Tabs className="room-file-tabs-ant" activeKey={view} onChange={(key) => setView(key as "list" | "timeline" | "trash")} items={viewTabs} tabBarExtraContent={view === "list" ? <div className="room-file-actions"><Button type="primary" icon={<FilePlus2 aria-hidden="true" />} onClick={chooseFiles}><span className="desktop-action-label">{t("room.workspace.addFiles")}</span><span className="mobile-action-label">{t("room.workspace.addFilesShort")}</span></Button><Button icon={<Send aria-hidden="true" />} onClick={() => setReuseFiles([])}><span className="desktop-action-label">{t("room.workspace.privateForward")}</span><span className="mobile-action-label">{t("room.workspace.privateForwardShort")}</span></Button></div> : undefined} />
    </div>
    {view === "list" ? <div className="file-workspace-body">
      <div className="file-filter-bar">
        <Segmented className="file-range-tabs-ant" aria-label={t("room.files.scopeFilter")} value={range} onChange={(value) => changeFilter(setRange, value as FileRange)} options={rangeOptions} />
        <Input.Search className="file-search-ant" aria-label={t("room.files.search")} value={search} onChange={(event) => setSearch(event.target.value)} placeholder={t("room.files.searchPlaceholder")} allowClear />
        <Dropdown menu={{ items: identityItems, selectable: true, selectedKeys: [identity], onClick: ({ key }) => changeFilter(setIdentity, key as FileIdentity) }} trigger={["click"]}><Button className="file-filter-button" icon={<ListFilter aria-hidden="true" />}>{identityLabel}<ChevronDown aria-hidden="true" size={15} /></Button></Dropdown>
        <Select aria-label={t("room.files.sortLabel")} value={sort} onChange={(value) => setSort(value as FileSort)} options={[{ value: "newest", label: t("room.files.sort.newest") }, { value: "oldest", label: t("room.files.sort.oldest") }, { value: "size_asc", label: t("room.files.sort.sizeAsc") }, { value: "size_desc", label: t("room.files.sort.sizeDesc") }]} />
        <Button className={`file-batch-toggle ${batchMode ? "active" : ""}`} type={batchMode ? "primary" : "default"} icon={batchMode ? <CheckSquare aria-hidden="true" /> : <Square aria-hidden="true" />} onClick={toggleBatch}>{batchMode ? t("room.files.exitBatch") : t("room.files.batchSelect")}</Button>
      </div>
      {actionError != null && <div className="file-action-error"><ErrorNotice error={actionError} /></div>}
      <FileList code={code} range={range} identity={identity} search={search} sort={sort} batchMode={batchMode} selected={selected} onSelectedChange={setSelected} onDropFiles={addFiles} onChooseFiles={chooseFiles} onDownload={(file) => void download(file)} onAccept={(file) => void accept(file)} onDecline={(file) => void decline(file)} onReuse={(files) => setReuseFiles(files.map((file) => file.fileId))} onPublish={(file) => void publish(file)} onTrash={openTrash} onDetails={setDetails} onBatchDownload={(files) => void batchDownload(files)} onCounts={setFileCounts} />
    </div> : view === "timeline" ? <FileTimeline code={code} /> : <FileTrash code={code} search={trashSearch} onSearchChange={setTrashSearch} launch={trashLaunch} onLaunchConsumed={(token) => setTrashLaunch((current) => current?.token === token ? undefined : current)} onChanged={refresh} />}
    {drafts.length > 0 && <UploadComposer files={drafts} members={members} selfId={selfId} pending={upload.isPending} error={upload.error} onClose={() => { if (!upload.isPending) setDrafts([]); }} onAdd={chooseFiles} onRemove={(index) => setDrafts((files) => files.filter((_, current) => current !== index))} onSubmit={(scope, recipientIds) => upload.mutate({ files: drafts, scope, recipientIds })} />}
    {reuseFiles != null && <ReusePrivateDialog code={code} members={members} selfId={selfId} initialFileIds={reuseFiles} pending={reuse.isPending} error={reuse.error} onClose={() => { if (!reuse.isPending) setReuseFiles(null); }} onSubmit={(fileIds, recipientIds) => reuse.mutate({ fileIds, recipientIds })} />}
    {details && <FileDetailsDialog file={details} onClose={() => setDetails(null)} />}
    <Modal open={Boolean(trashFile)} title={t("room.files.trashTitle")} okText={t("room.files.trashConfirm")} cancelText={t("room.files.cancel")} okButtonProps={{ danger: true, loading: trashPending }} onCancel={() => { if (!trashPending) setTrashFile(undefined); }} onOk={() => void confirmTrash()}>
      {trashFile && <div className="file-trash-modal-content"><Alert type="warning" showIcon message={t(trashFile.capabilities.canSetTrashReason ? "room.files.ownerTrashWarning" : "room.files.selfTrashWarning")} /><dl><div><dt>{t("room.files.name")}</dt><dd>{trashFile.displayName}</dd></div><div><dt>{t("room.files.uploader")}</dt><dd>{trashFile.uploaderName}</dd></div><div><dt>{t("room.files.size")}</dt><dd>{formatBytes(trashFile.size)}</dd></div></dl>{trashFile.capabilities.canSetTrashReason && <label className="file-reason-field" htmlFor="trash-reason"><span>{t("room.files.deleteReasonOptional")}</span><Input.TextArea id="trash-reason" value={trashReason} maxLength={200} showCount disabled={trashPending} onChange={(event) => setTrashReason(event.target.value)} rows={3} /></label>}{trashError != null && <ErrorNotice error={trashError} />}</div>}
    </Modal>
  </section>;
}

function makeRequestId() {
  return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}
