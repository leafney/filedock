import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CheckSquare, FilePlus2, ListFilter, Search, Send, Square, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { ErrorNotice } from "../common";
import { FileList } from "./FileList";
import { FileTimeline } from "./FileTimeline";
import { FileDetailsDialog, ReusePrivateDialog, UploadComposer } from "./UploadComposer";
import { streamEventName, type StreamEventMessage } from "../../hooks/use-stream";
import { acceptPrivateFile, createFileDownload, createUploadBatch, declinePrivateFile, publishPrivateFile, reusePrivateFiles } from "../../services/api";
import { applyDownloadProgress, enqueueDownload, enqueueUploadBatch, fileRefreshEventName } from "../../stores/transfer-store";
import type { FileIdentity, FileRange, FileScope, FileSort, RoomFile, RoomMember } from "../../types/domain";
import { moveRovingFocus } from "../../utils/keyboard";

export function FileWorkspace({ code, members, selfId }: { code: string; members: RoomMember[]; selfId: string }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const inputRef = useRef<HTMLInputElement>(null);
  const [view, setView] = useState<"list" | "timeline">("list");
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

  const refresh = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ["room-files", code] });
    void queryClient.invalidateQueries({ queryKey: ["file-events", code] });
    void queryClient.invalidateQueries({ queryKey: ["reusable-private-files", code] });
    void queryClient.invalidateQueries({ queryKey: ["room", code] });
  }, [code, queryClient]);

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
    mutationFn: ({ files, scope, recipientIds }: { files: File[]; scope: FileScope; recipientIds: string[] }) => createUploadBatch(code, makeRequestId(), scope, recipientIds, files.map((file) => ({ originalName: file.name, declaredSize: file.size, declaredMime: file.type || "application/octet-stream" }))),
    onSuccess: (batch, variables) => { enqueueUploadBatch(code, batch, variables.files, variables.recipientIds); setDrafts([]); refresh(); },
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
  const batchDownload = async (files: RoomFile[]) => {
    setActionError(undefined);
    try { for (const file of files) enqueueDownload(code, await createFileDownload(code, file.fileId)); } catch (error) { setActionError(error); }
  };
  const changeFilter = <T,>(apply: (value: T) => void, value: T) => {
    if (selected.size > 0 && !window.confirm(t("room.files.keepHiddenSelection"))) setSelected(new Set());
    apply(value);
  };
  const toggleBatch = () => { setBatchMode((value) => !value); setSelected(new Set()); };

  return <section className="room-files-shell">
    <input ref={inputRef} className="visually-hidden" type="file" multiple onChange={(event) => { addFiles(Array.from(event.target.files ?? [])); event.target.value = ""; }} />
    <div className="room-file-toolbar"><div><span>{t("room.workspace.fileWorkspace")}</span><h1>{t("room.workspace.roomFiles")}</h1></div><div><button type="button" onClick={chooseFiles}><FilePlus2 aria-hidden="true" />{t("room.workspace.addFiles")}</button><button type="button" onClick={() => setReuseFiles([])}><Send aria-hidden="true" />{t("room.workspace.sendExisting")}</button></div></div>
    <div className="room-file-tabs" role="tablist" onKeyDown={moveRovingFocus}><button className={view === "list" ? "active" : ""} role="tab" aria-selected={view === "list"} tabIndex={view === "list" ? 0 : -1} type="button" onClick={() => setView("list")}>{t("room.workspace.fileList")}</button><button className={view === "timeline" ? "active" : ""} role="tab" aria-selected={view === "timeline"} tabIndex={view === "timeline" ? 0 : -1} type="button" onClick={() => setView("timeline")}>{t("room.workspace.timeline")}</button></div>
    {view === "list" ? <div className="file-workspace-body">
      <div className="file-filter-bar">
        <div className="file-range-tabs" role="radiogroup" aria-label={t("room.files.scopeFilter")} onKeyDown={moveRovingFocus}>{(["all", "shared", "direct"] as FileRange[]).map((value) => <button className={range === value ? "active" : ""} key={value} role="radio" aria-checked={range === value} tabIndex={range === value ? 0 : -1} type="button" onClick={() => changeFilter(setRange, value)}>{t(`room.files.range.${value}`)}</button>)}</div>
        <label className="file-search"><Search aria-hidden="true" /><span className="visually-hidden">{t("room.files.search")}</span><input value={search} onChange={(event) => setSearch(event.target.value)} placeholder={t("room.files.searchPlaceholder")} />{search && <button type="button" aria-label={t("room.files.clearSearch")} onClick={() => setSearch("")}><X aria-hidden="true" /></button>}</label>
        <label className="file-select-filter"><ListFilter aria-hidden="true" /><span className="visually-hidden">{t("room.files.identityFilter")}</span><select value={identity} onChange={(event) => changeFilter(setIdentity, event.target.value as FileIdentity)}><option value="all">{t("room.files.identity.all")}</option><option value="uploaded">{t("room.files.identity.uploaded")}</option><option value="received">{t("room.files.identity.received")}</option></select></label>
        <label className="file-select-filter"><span className="visually-hidden">{t("room.files.sortLabel")}</span><select value={sort} onChange={(event) => setSort(event.target.value as FileSort)}><option value="newest">{t("room.files.sort.newest")}</option><option value="oldest">{t("room.files.sort.oldest")}</option><option value="size_asc">{t("room.files.sort.sizeAsc")}</option><option value="size_desc">{t("room.files.sort.sizeDesc")}</option></select></label>
        <button className={`file-batch-toggle ${batchMode ? "active" : ""}`} type="button" onClick={toggleBatch}>{batchMode ? <CheckSquare aria-hidden="true" /> : <Square aria-hidden="true" />}{batchMode ? t("room.files.exitBatch") : t("room.files.batchSelect")}</button>
      </div>
      {actionError != null && <div className="file-action-error"><ErrorNotice error={actionError} /></div>}
      <FileList code={code} range={range} identity={identity} search={search} sort={sort} batchMode={batchMode} selected={selected} onSelectedChange={setSelected} onAddFiles={chooseFiles} onDropFiles={addFiles} onDownload={(file) => void download(file)} onAccept={(file) => void accept(file)} onDecline={(file) => void decline(file)} onReuse={(files) => setReuseFiles(files.map((file) => file.fileId))} onPublish={(file) => void publish(file)} onDetails={setDetails} onBatchDownload={(files) => void batchDownload(files)} />
    </div> : <FileTimeline code={code} />}
    {drafts.length > 0 && <UploadComposer files={drafts} members={members} selfId={selfId} pending={upload.isPending} error={upload.error} onClose={() => { if (!upload.isPending) setDrafts([]); }} onAdd={chooseFiles} onRemove={(index) => setDrafts((files) => files.filter((_, current) => current !== index))} onSubmit={(scope, recipientIds) => upload.mutate({ files: drafts, scope, recipientIds })} />}
    {reuseFiles != null && <ReusePrivateDialog code={code} members={members} selfId={selfId} initialFileIds={reuseFiles} pending={reuse.isPending} error={reuse.error} onClose={() => { if (!reuse.isPending) setReuseFiles(null); }} onSubmit={(fileIds, recipientIds) => reuse.mutate({ fileIds, recipientIds })} />}
    {details && <FileDetailsDialog file={details} onClose={() => setDetails(null)} />}
  </section>;
}

function makeRequestId() {
  return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}
