import { useQuery } from "@tanstack/react-query";
import { FileLock2, FilePlus2, Send, Trash2, X } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import { ErrorNotice } from "../common";
import { useDialogFocus } from "../../hooks/use-dialog-focus";
import { listReusablePrivateFiles } from "../../services/api";
import type { FileScope, RoomFile, RoomMember } from "../../types/domain";
import { formatBytes } from "../../utils/format";
import { moveRovingFocus } from "../../utils/keyboard";

interface UploadProps {
  files: File[];
  members: RoomMember[];
  selfId: string;
  pending: boolean;
  error?: unknown;
  onClose: () => void;
  onAdd: () => void;
  onRemove: (index: number) => void;
  onSubmit: (scope: FileScope, recipientIds: string[]) => void;
}

export function UploadComposer(props: UploadProps) {
  const { t } = useTranslation();
  const [scope, setScope] = useState<FileScope>("shared");
  const [recipients, setRecipients] = useState<Set<string>>(new Set());
  const candidates = props.members.filter((member) => member.userId !== props.selfId && member.status === "active");
  const toggle = (id: string) => setRecipients((current) => { const next = new Set(current); if (next.has(id)) next.delete(id); else next.add(id); return next; });
  const total = props.files.reduce((sum, file) => sum + file.size, 0);
  const panelRef = useDialogFocus(props.onClose);
  return <div className="room-overlay upload-composer" role="dialog" aria-modal="true" aria-label={t("room.files.readyToSend")}>
    <section ref={panelRef}><header><div><span>{t("room.files.newUpload")}</span><h2>{t("room.files.readyToSend")}</h2></div><button type="button" aria-label={t("room.workspace.close")} onClick={props.onClose}><X aria-hidden="true" /></button></header>
      <div className="upload-composer-body">
        {props.error != null && <ErrorNotice error={props.error} />}
        <div className="upload-scope" role="radiogroup" aria-label={t("room.files.chooseScope")} onKeyDown={moveRovingFocus}><button className={scope === "shared" ? "active" : ""} role="radio" aria-checked={scope === "shared"} tabIndex={scope === "shared" ? 0 : -1} type="button" onClick={() => { setScope("shared"); setRecipients(new Set()); }}>{t("room.files.shared")}</button><button className={scope === "direct" ? "active" : ""} role="radio" aria-checked={scope === "direct"} tabIndex={scope === "direct" ? 0 : -1} type="button" onClick={() => setScope("direct")}>{t("room.files.private")}</button></div>
        {scope === "direct" && <fieldset className="recipient-picker"><legend>{t("room.files.chooseRecipients")}</legend>{candidates.length === 0 ? <p>{t("room.files.noRecipients")}</p> : candidates.map((member) => <label key={member.userId}><input type="checkbox" checked={recipients.has(member.userId)} onChange={() => toggle(member.userId)} /><span className="room-avatar">{member.displayName.slice(0, 1)}</span>{member.displayName}</label>)}</fieldset>}
        <div className="upload-draft-list">{props.files.map((file, index) => <article key={`${file.name}-${file.lastModified}-${index}`}><FilePlus2 aria-hidden="true" /><div><strong>{file.name}</strong><span>{formatBytes(file.size)}</span></div><button type="button" aria-label={t("room.files.removeFile", { name: file.name })} onClick={() => props.onRemove(index)}><Trash2 aria-hidden="true" /></button></article>)}</div>
        <button className="upload-add-more" type="button" onClick={props.onAdd}><FilePlus2 aria-hidden="true" />{t("room.files.addMore")}</button>
      </div>
      <footer><span>{t("room.files.fileTotal", { count: props.files.length, size: formatBytes(total) })}</span><button type="button" onClick={props.onClose}>{t("room.files.cancel")}</button><button className="primary" type="button" disabled={props.pending || props.files.length === 0 || (scope === "direct" && recipients.size === 0)} onClick={() => props.onSubmit(scope, [...recipients])}><Send aria-hidden="true" />{props.pending ? t("room.files.preparing") : t("room.files.sendFiles")}</button></footer>
    </section>
  </div>;
}

interface ReuseProps {
  code: string;
  members: RoomMember[];
  selfId: string;
  initialFileIds: string[];
  pending: boolean;
  error?: unknown;
  onClose: () => void;
  onSubmit: (fileIds: string[], recipientIds: string[]) => void;
}

export function ReusePrivateDialog(props: ReuseProps) {
  const { t } = useTranslation();
  const reusable = useQuery({ queryKey: ["reusable-private-files", props.code], queryFn: () => listReusablePrivateFiles(props.code) });
  const [files, setFiles] = useState<Set<string>>(new Set(props.initialFileIds));
  const [recipients, setRecipients] = useState<Set<string>>(new Set());
  useEffect(() => setFiles(new Set(props.initialFileIds)), [props.initialFileIds]);
  const selectedFiles = useMemo(() => reusable.data?.items.filter((file) => files.has(file.fileId)) ?? [], [files, reusable.data?.items]);
  const candidates = props.members.filter((member) => member.userId !== props.selfId && member.status === "active");
  const memberDisabled = (memberId: string) => selectedFiles.some((file) => file.recipients?.some((recipient) => recipient.userId === memberId && recipient.status !== "declined"));
  useEffect(() => {
    const blocked = new Set(candidates.filter((member) => memberDisabled(member.userId)).map((member) => member.userId));
    setRecipients((current) => new Set([...current].filter((id) => !blocked.has(id))));
  }, [files, reusable.data?.items]);
  const toggle = (set: Set<string>, id: string, apply: (value: Set<string>) => void) => { const next = new Set(set); if (next.has(id)) next.delete(id); else next.add(id); apply(next); };
  const panelRef = useDialogFocus(props.onClose);
  return <div className="room-overlay upload-composer" role="dialog" aria-modal="true" aria-label={t("room.files.sendExistingTitle")}>
    <section ref={panelRef}><header><div><span>{t("room.files.private")}</span><h2>{t("room.files.sendExistingTitle")}</h2></div><button type="button" aria-label={t("room.workspace.close")} onClick={props.onClose}><X aria-hidden="true" /></button></header>
      <div className="upload-composer-body">
        {props.error != null && <ErrorNotice error={props.error} />}
        {reusable.isPending && <p className="file-state-message">{t("room.files.loading")}</p>}
        {reusable.isError && <ErrorNotice error={reusable.error} onRetry={() => void reusable.refetch()} />}
        <fieldset className="reuse-file-picker"><legend>{t("room.files.chooseExisting")}</legend>{reusable.data?.items.length === 0 && <p>{t("room.files.noReusableFiles")}</p>}{reusable.data?.items.map((file) => <label key={file.fileId}><input type="checkbox" checked={files.has(file.fileId)} onChange={() => toggle(files, file.fileId, setFiles)} /><FileLock2 aria-hidden="true" /><span><strong>{file.displayName}</strong><small>{t("room.files.privateNumber")}#{file.privateCode} · {formatBytes(file.size)}</small></span></label>)}</fieldset>
        <fieldset className="recipient-picker"><legend>{t("room.files.chooseRecipients")}</legend>{candidates.map((member) => { const disabled = files.size === 0 || memberDisabled(member.userId); return <label key={member.userId} className={disabled ? "is-disabled" : ""}><input type="checkbox" disabled={disabled} checked={recipients.has(member.userId)} onChange={() => toggle(recipients, member.userId, setRecipients)} /><span className="room-avatar">{member.displayName.slice(0, 1)}</span>{member.displayName}</label>; })}</fieldset>
      </div>
      <footer><span>{t("room.files.selectedCount", { count: files.size })}</span><button type="button" onClick={props.onClose}>{t("room.files.cancel")}</button><button className="primary" type="button" disabled={props.pending || files.size === 0 || recipients.size === 0} onClick={() => props.onSubmit([...files], [...recipients])}><Send aria-hidden="true" />{t("room.files.sendFiles")}</button></footer>
    </section>
  </div>;
}

export function FileDetailsDialog({ file, onClose }: { file: RoomFile; onClose: () => void }) {
  const { t } = useTranslation();
  const panelRef = useDialogFocus(onClose);
  return <div className="room-overlay file-details" role="dialog" aria-modal="true" aria-label={t("room.files.fileDetails")}><section ref={panelRef}><header><h2>{t("room.files.fileDetails")}</h2><button type="button" aria-label={t("room.workspace.close")} onClick={onClose}><X aria-hidden="true" /></button></header><dl><div><dt>{t("room.files.name")}</dt><dd>{file.displayName}</dd></div>{file.privateCode && <div><dt>{t("room.files.privateNumber")}</dt><dd>#{file.privateCode}</dd></div>}<div><dt>{t("room.files.scope")}</dt><dd>{file.scope === "shared" ? t("room.files.shared") : t("room.files.private")}</dd></div><div><dt>{t("room.files.size")}</dt><dd>{formatBytes(file.size)}</dd></div><div><dt>{t("room.files.uploader")}</dt><dd>{file.uploaderName}</dd></div></dl></section></div>;
}
