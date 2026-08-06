import type { FileTrashItem, RoomFile } from "../types/domain";

export type FilePrimaryAction = "accept" | "download" | "progress" | "none";
export type FileMenuAction = "download" | "reuse" | "publish" | "details" | "trash";
export type FileTrashAction = "restore" | "request_restore" | "purge";

export function primaryFileAction(file: RoomFile): FilePrimaryAction {
  if (file.capabilities.canAccept) return "accept";
  if (file.status === "uploading") return "progress";
  if (file.capabilities.canDownload) return "download";
  return "none";
}

export function fileMenuActions(file: RoomFile): FileMenuAction[] {
  const actions: FileMenuAction[] = [];
  if (file.capabilities.canReuse) actions.push("reuse");
  if (file.capabilities.canPublishShared) actions.push("publish");
  actions.push("details");
  if (file.capabilities.canTrash) actions.push("trash");
  return actions;
}

export function fileTrashActions(item: FileTrashItem): FileTrashAction[] {
  const actions: FileTrashAction[] = [];
  if (item.capabilities.canRestore) actions.push("restore");
  if (item.capabilities.canRequestRestore && (!item.restoreRequest || item.restoreRequest.status === "invalidated")) actions.push("request_restore");
  if (item.capabilities.canPurge) actions.push("purge");
  return actions;
}

export function batchFileCapabilities(files: RoomFile[]) {
  return {
    canDownload: files.length > 0 && files.every((file) => file.capabilities.canDownload),
    canReuse: files.length > 0 && files.every((file) => file.capabilities.canReuse),
  };
}
