import type { RoomFile } from "../types/domain";

export type FilePrimaryAction = "accept" | "download" | "progress" | "none";
export type FileMenuAction = "download" | "reuse" | "publish" | "details";

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
  return actions;
}

export function batchFileCapabilities(files: RoomFile[]) {
  return {
    canDownload: files.length > 0 && files.every((file) => file.capabilities.canDownload),
    canReuse: files.length > 0 && files.every((file) => file.capabilities.canReuse),
  };
}
