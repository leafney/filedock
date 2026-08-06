import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Input, Modal, Radio, Space, message } from "antd";
import { ChevronDown, File, FileLock2, RotateCcw, Search, Trash2, X } from "lucide-react";
import { useDeferredValue, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { ErrorNotice } from "../common";
import { approveFileRestore, listFileTrash, purgeRoomFile, rejectFileRestore, restoreRoomFile } from "../../services/api";
import type { FileTrashItem } from "../../types/domain";
import { fileTrashActions } from "../../utils/file-permissions";
import { formatBytes, formatDate } from "../../utils/format";

interface Props {
  code: string;
  search: string;
  onSearchChange: (value: string) => void;
  launch?: { token: string; requestId?: string };
  onChanged: () => void;
}

export function FileTrash({ code, search, onSearchChange, launch, onChanged }: Props) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const deferredSearch = useDeferredValue(search.trim());
  const [operationError, setOperationError] = useState<unknown>();
  const [pendingAction, setPendingAction] = useState<string>();
  const [purgeItem, setPurgeItem] = useState<FileTrashItem>();
  const [approvalItem, setApprovalItem] = useState<FileTrashItem>();
  const [decision, setDecision] = useState<"approve" | "reject">();
  const [decisionReason, setDecisionReason] = useState("");
  const [approvalError, setApprovalError] = useState<unknown>();
  const handledLaunch = useRef<string>();
  const unavailableLaunch = useRef<string>();

  const trashQuery = useInfiniteQuery({
    queryKey: ["file-trash", code, deferredSearch],
    initialPageParam: "",
    queryFn: ({ pageParam }) => listFileTrash(code, deferredSearch, pageParam, 30),
    getNextPageParam: (page) => page.nextCursor || undefined,
  });
  const items = useMemo(() => trashQuery.data?.pages.flatMap((page) => page.items) ?? [], [trashQuery.data?.pages]);
  const total = trashQuery.data?.pages[0]?.total ?? 0;

  const approvalQuery = useInfiniteQuery({
    queryKey: ["file-trash-approval", code, launch?.requestId],
    enabled: Boolean(launch?.requestId),
    initialPageParam: "",
    queryFn: ({ pageParam }) => listFileTrash(code, "", pageParam, 30),
    getNextPageParam: (page) => page.nextCursor || undefined,
  });
  const approvalItems = useMemo(() => approvalQuery.data?.pages.flatMap((page) => page.items) ?? [], [approvalQuery.data?.pages]);
  const { fetchNextPage: fetchApprovalNextPage, hasNextPage: approvalHasNextPage, isFetchingNextPage: approvalIsFetchingNextPage, isSuccess: approvalIsSuccess } = approvalQuery;

  useEffect(() => {
    const requestId = launch?.requestId;
    const token = launch?.token;
    if (!requestId || !token || handledLaunch.current === token || approvalItem) return;
    const item = approvalItems.find((candidate) => candidate.restoreRequest?.requestId === requestId);
    if (item) {
      handledLaunch.current = token;
      if (item.restoreRequest?.status !== "pending") {
        unavailableLaunch.current = token;
        message.warning(t("notification.unavailable"));
        void queryClient.invalidateQueries({ queryKey: ["notifications"] });
        return;
      }
      setDecision(undefined);
      setDecisionReason("");
      setApprovalError(undefined);
      setApprovalItem(item);
      return;
    }
    if (approvalIsSuccess && approvalHasNextPage && !approvalIsFetchingNextPage) {
      void fetchApprovalNextPage();
      return;
    }
    if (approvalIsSuccess && !approvalHasNextPage && unavailableLaunch.current !== token) {
      unavailableLaunch.current = token;
      handledLaunch.current = token;
      message.warning(t("notification.unavailable"));
      void queryClient.invalidateQueries({ queryKey: ["notifications"] });
    }
  }, [approvalHasNextPage, approvalIsFetchingNextPage, approvalIsSuccess, approvalItem, approvalItems, fetchApprovalNextPage, launch?.requestId, launch?.token, queryClient, t]);

  const refreshAfterAction = () => {
    setPendingAction(undefined);
    onChanged();
  };
  const restore = async (item: FileTrashItem) => {
    const action = `restore:${item.file.fileId}`;
    setPendingAction(action);
    setOperationError(undefined);
    try {
      const result = await restoreRoomFile(code, item.file.fileId);
      message.success(t(result.status === "pending" ? "room.files.restoreRequested" : "room.files.restored"));
      refreshAfterAction();
    } catch (error) {
      setOperationError(error);
      setPendingAction(undefined);
    }
  };
  const purge = async () => {
    if (!purgeItem) return;
    const item = purgeItem;
    setPendingAction(`purge:${item.file.fileId}`);
    setOperationError(undefined);
    try {
      await purgeRoomFile(code, item.file.fileId);
      setPurgeItem(undefined);
      message.success(t("room.files.purged"));
      refreshAfterAction();
    } catch (error) {
      setOperationError(error);
      setPendingAction(undefined);
    }
  };
  const submitApproval = async () => {
    const request = approvalItem?.restoreRequest;
    if (!approvalItem || !request || !decision) return;
    setPendingAction(`approval:${request.requestId}`);
    setApprovalError(undefined);
    try {
      if (decision === "approve") await approveFileRestore(code, request.requestId);
      else await rejectFileRestore(code, request.requestId, decisionReason.trim());
      setApprovalItem(undefined);
      setDecisionReason("");
      message.success(t(decision === "approve" ? "room.files.restoreApproved" : "room.files.restoreRejected"));
      refreshAfterAction();
    } catch (error) {
      setApprovalError(error);
      setPendingAction(undefined);
    }
  };

  const emptyText = deferredSearch ? t("room.files.trashEmptySearch") : t("room.files.trashEmpty");
  return <section className="file-trash-view">
    <div className="file-trash-toolbar">
      <label className="file-search"><Search aria-hidden="true" /><span className="visually-hidden">{t("room.files.trashSearch")}</span><input value={search} onChange={(event) => onSearchChange(event.target.value)} placeholder={t("room.files.trashSearchPlaceholder")} />{search && <button type="button" aria-label={t("room.files.clearSearch")} onClick={() => onSearchChange("")}><X aria-hidden="true" /></button>}</label>
      <span className="file-trash-total">{t("room.files.trashTotal", { count: total })}</span>
    </div>
    {operationError != null && <div className="file-action-error"><ErrorNotice error={operationError} /></div>}
    {trashQuery.isPending && <p className="file-state-message">{t("room.files.loadingTrash")}</p>}
    {trashQuery.isError && <ErrorNotice error={trashQuery.error} onRetry={() => void trashQuery.refetch()} />}
    {!trashQuery.isPending && !trashQuery.isError && items.length === 0 && <div className="file-empty-state"><Trash2 aria-hidden="true" /><strong>{emptyText}</strong></div>}
    {items.length > 0 && <div className="file-trash-list">{items.map((item) => <TrashRow key={item.file.fileId} item={item} pendingAction={pendingAction} onRestore={() => void restore(item)} onRequest={() => void restore(item)} onPurge={() => setPurgeItem(item)} />)}</div>}
    {trashQuery.hasNextPage && <button className="file-load-more" type="button" disabled={trashQuery.isFetchingNextPage} onClick={() => void trashQuery.fetchNextPage()}><ChevronDown aria-hidden="true" />{trashQuery.isFetchingNextPage ? t("room.files.loadingTrash") : t("room.files.loadMore")}</button>}

    <Modal open={Boolean(purgeItem)} title={t("room.files.purgeTitle")} okText={t("room.files.purgeConfirm")} cancelText={t("room.files.cancel")} okButtonProps={{ danger: true, loading: pendingAction?.startsWith("purge:") }} onCancel={() => { if (!pendingAction?.startsWith("purge:")) setPurgeItem(undefined); }} onOk={() => void purge()}>
      {purgeItem && <div className="file-trash-modal-content"><Alert type="warning" showIcon message={t("room.files.purgeWarning")} /><dl><div><dt>{t("room.files.name")}</dt><dd>{displayFileName(purgeItem)}</dd></div><div><dt>{t("room.files.uploader")}</dt><dd>{purgeItem.file.uploaderName}</dd></div><div><dt>{t("room.files.size")}</dt><dd>{formatBytes(purgeItem.file.size)}</dd></div><div><dt>{t("room.files.deletedAt")}</dt><dd>{formatDate(purgeItem.deletedAt)}</dd></div>{purgeItem.deleteReason && <div><dt>{t("room.files.deleteReason")}</dt><dd>{purgeItem.deleteReason}</dd></div>}{purgeItem.restoreRequest && <div><dt>{t("room.files.restoreRequestStatus")}</dt><dd>{t(`room.files.trashStatus.${purgeItem.restoreRequest.status}`)}{purgeItem.restoreRequest.status === "rejected" && `：${purgeItem.restoreRequest.rejectionReason || t("room.files.noRejectionReason")}`}</dd></div>}</dl></div>}
    </Modal>

    <RestoreApprovalModal item={approvalItem} decision={decision} reason={decisionReason} error={approvalError} pending={pendingAction?.startsWith("approval:") ?? false} onDecision={setDecision} onReason={setDecisionReason} onClose={() => { if (!pendingAction?.startsWith("approval:")) setApprovalItem(undefined); }} onSubmit={() => void submitApproval()} />
  </section>;
}

function TrashRow({ item, pendingAction, onRestore, onRequest, onPurge }: { item: FileTrashItem; pendingAction?: string; onRestore: () => void; onRequest: () => void; onPurge: () => void }) {
  const { t } = useTranslation();
  const actions = fileTrashActions(item);
  const restoreRequest = item.restoreRequest;
  const restoreStatus = restoreRequest?.status;
  return <article className={`file-trash-row ${item.file.projection === "anonymous" ? "is-anonymous" : ""}`}>
    <span className="file-type-icon">{item.file.scope === "direct" ? <FileLock2 aria-hidden="true" /> : <File aria-hidden="true" />}</span>
    <div className="file-row-main"><strong>{item.file.scope === "direct" && item.file.privateCode ? `${t("room.files.privateNumber")}#${item.file.privateCode}` : item.file.displayName}</strong>{item.file.scope === "direct" && item.file.privateCode && item.file.projection === "full" && <span>{item.file.displayName}</span>}<small>{item.file.uploaderName} · {formatBytes(item.file.size)} · {t("room.files.deletedAtValue", { time: formatDate(item.deletedAt) })}</small><small>{t("room.files.deletedBy", { name: item.deletedByName || t("room.files.systemActor") })}</small>{item.deleteReason && <em>{t("room.files.deleteReason")}：{item.deleteReason}</em>}{restoreStatus && <span className={`file-trash-status ${restoreStatus}`}>{t(`room.files.trashStatus.${restoreStatus}`)}{restoreStatus === "rejected" && `：${restoreRequest?.rejectionReason || t("room.files.noRejectionReason")}`}</span>}</div>
    <span className={`file-scope-badge ${item.file.scope}`}>{item.file.projection === "anonymous" ? t("room.files.anonymous") : item.file.scope === "shared" ? t("room.files.shared") : t("room.files.private")}</span>
    <div className="file-trash-actions">
      {actions.includes("restore") && <Button size="small" type="primary" icon={<RotateCcw aria-hidden="true" />} disabled={Boolean(pendingAction)} loading={pendingAction === `restore:${item.file.fileId}`} onClick={onRestore}>{t("room.files.restore")}</Button>}
      {actions.includes("request_restore") && <Button size="small" disabled={Boolean(pendingAction)} loading={pendingAction === `restore:${item.file.fileId}`} onClick={onRequest}>{t("room.files.requestRestore")}</Button>}
      {actions.includes("purge") && <Button size="small" danger icon={<Trash2 aria-hidden="true" />} disabled={Boolean(pendingAction)} loading={pendingAction === `purge:${item.file.fileId}`} onClick={onPurge}>{t("room.files.purge")}</Button>}
    </div>
  </article>;
}

function RestoreApprovalModal({ item, decision, reason, error, pending, onDecision, onReason, onClose, onSubmit }: { item?: FileTrashItem; decision?: "approve" | "reject"; reason: string; error?: unknown; pending: boolean; onDecision: (decision: "approve" | "reject") => void; onReason: (reason: string) => void; onClose: () => void; onSubmit: () => void }) {
  const { t } = useTranslation();
  const request = item?.restoreRequest;
  return <Modal open={Boolean(item)} title={t("room.files.restoreApprovalTitle")} okText={t("room.files.submitRestoreDecision")} cancelText={t("room.files.cancel")} okButtonProps={{ type: decision === "approve" ? "primary" : "default", danger: decision === "reject", disabled: !request || !decision || pending, loading: pending }} onCancel={onClose} onOk={onSubmit}>
    {item && request && <div className="file-trash-modal-content"><dl><div><dt>{t("room.files.name")}</dt><dd>{displayFileName(item)}</dd></div><div><dt>{t("room.files.requester")}</dt><dd>{request.requesterName}</dd></div><div><dt>{t("room.files.requestedAt")}</dt><dd>{formatDate(request.createdAt)}</dd></div>{item.deleteReason && <div><dt>{t("room.files.deleteReason")}</dt><dd>{item.deleteReason}</dd></div>}</dl><fieldset className="file-restore-decision"><legend>{t("room.files.restoreDecision")}</legend><Radio.Group value={decision} onChange={(event) => onDecision(event.target.value)} disabled={pending}><Space direction="vertical"><Radio value="approve">{t("room.files.approveRestore")}</Radio><Radio value="reject">{t("room.files.rejectRestore")}</Radio></Space></Radio.Group></fieldset><label className="file-reason-field" htmlFor="restore-rejection-reason"><span>{t("room.files.rejectionReasonOptional")}</span><Input.TextArea id="restore-rejection-reason" value={reason} maxLength={200} showCount disabled={decision !== "reject" || pending} onChange={(event) => onReason(event.target.value)} rows={3} /></label>{error != null && <ErrorNotice error={error} />}</div>}
  </Modal>;
}

function displayFileName(item: FileTrashItem) {
  return item.file.displayName;
}
