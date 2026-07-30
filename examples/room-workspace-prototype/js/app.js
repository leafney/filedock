import { canRestoreFile } from "./capacity.js";
import { renderModel } from "./render.js";
import { createStore } from "./state.js";

export const store = createStore();
let dragDepth = 0;

store.subscribe((state) => renderModel(state));
renderModel(store.getState());

document.addEventListener("click", (event) => {
  const tab = event.target.closest("[data-tab]");
  if (tab) {
    store.dispatch({ type: "ui/set-tab", value: tab.dataset.tab });
    return;
  }
  const target = event.target.closest("[data-action]");
  if (!target || target.disabled) return;
  handleAction(target.dataset.action, target);
});

document.addEventListener("input", (event) => {
  if (event.target.matches('[data-input="file-search"]')) {
    store.dispatch({ type: "ui/set-file-search", value: event.target.value });
  }
});

document.addEventListener("change", (event) => {
  if (event.target.matches('[data-input="file-scope"]')) store.dispatch({ type: "ui/set-file-scope", value: event.target.value });
  if (event.target.matches('[data-input="file-sort"]')) store.dispatch({ type: "ui/set-file-sort", value: event.target.value });
});

const fileInput = document.querySelector("#prototype-file-input");
fileInput.addEventListener("change", () => {
  addBrowserFiles(fileInput.files);
  fileInput.value = "";
});

document.addEventListener("dragenter", (event) => {
  if (!hasFiles(event)) return;
  event.preventDefault();
  dragDepth += 1;
  document.body.classList.add("is-dragging-files");
});
document.addEventListener("dragover", (event) => { if (hasFiles(event)) event.preventDefault(); });
document.addEventListener("dragleave", (event) => {
  if (!hasFiles(event)) return;
  dragDepth = Math.max(0, dragDepth - 1);
  if (dragDepth === 0) document.body.classList.remove("is-dragging-files");
});
document.addEventListener("drop", (event) => {
  if (!hasFiles(event)) return;
  event.preventDefault();
  dragDepth = 0;
  document.body.classList.remove("is-dragging-files");
  addBrowserFiles(event.dataTransfer.files);
});

setInterval(() => {
  if (store.getState().tasks.some((task) => task.status === "active")) {
    store.dispatch({ type: "tasks/tick", step: 7 });
  }
}, 420);

function handleAction(action, target) {
  const state = store.getState();
  const now = new Date().toISOString();
  const common = { actorId: state.ui.currentUserId, occurredAt: now, eventId: uniqueId("event") };
  if (action === "open-upload") return openFilePicker("shared");
  if (action === "open-direct") return openFilePicker("direct");
  if (action === "add-files") return fileInput.click();
  if (action === "open-existing") return store.dispatch({ type: "ui/open-composer", mode: "direct", existingFileId: "choose" });
  if (action === "choose-existing") return store.dispatch({ type: "ui/open-composer", mode: "direct", existingFileId: target.dataset.fileId });
  if (action === "reuse-file") return store.dispatch({ type: "ui/open-composer", mode: "direct", existingFileId: target.dataset.fileId });
  if (action === "close-composer") return store.dispatch({ type: "ui/close-composer" });
  if (action === "remove-pending") return store.dispatch({ type: "composer/remove-file", value: target.dataset.pendingId });
  if (action === "set-send-mode") return store.dispatch({ type: "composer/set-mode", value: target.dataset.mode });
  if (action === "toggle-recipient") return store.dispatch({ type: "composer/toggle-recipient", value: target.dataset.userId });
  if (action === "select-all-recipients") {
    const existing = state.files.find((file) => file.id === state.ui.composer?.existingFileId);
    const ids = state.users.filter((user) => user.id !== state.ui.currentUserId && user.presence === "online" && !existing?.recipientIds.includes(user.id)).map((user) => user.id);
    return store.dispatch({ type: "composer/select-all-recipients", value: ids });
  }
  if (action === "confirm-send") return confirmSend();
  if (action === "download") {
    const file = state.files.find((item) => item.id === target.dataset.fileId);
    if (!file) return;
    store.dispatch({ type: "tasks/start-download", taskId: uniqueId("download"), fileId: file.id, name: file.name ?? `私密文件#${file.alias}`, sizeBytes: file.sizeBytes, ...common });
    return showToast("已开始模拟接收，可在底部查看进度");
  }
  if (action === "recycle") {
    store.dispatch({ type: "files/recycle", fileId: target.dataset.fileId, ...common });
    return showToast("文件已移入回收站");
  }
  if (action === "permanent-delete") {
    if (!window.confirm("永久删除后无法恢复。继续？")) return;
    store.dispatch({ type: "files/permanent-delete", fileId: target.dataset.fileId, ...common });
    return showToast("文件已永久删除");
  }
  if (action === "restore" || action === "approve-restore") {
    const result = canRestoreFile({ fileId: target.dataset.fileId, files: state.files, roomCapacityBytes: state.room.capacityBytes, recycleConfig: state.recycleConfig });
    if (!result.allowed) return showToast("容量不足，无法恢复", true);
    store.dispatch({ type: action === "restore" ? "files/restore" : "files/approve-restore", fileId: target.dataset.fileId, ...common });
    return showToast("文件已恢复");
  }
  if (action === "request-restore") {
    store.dispatch({ type: "files/request-restore", fileId: target.dataset.fileId, ...common });
    return showToast("恢复申请已提交给房主");
  }
  if (action === "publish-shared") {
    if (!window.confirm("发布后房间当前及后加入成员都可查看。继续？")) return;
    store.dispatch({ type: "files/publish-shared", fileId: target.dataset.fileId, ...common });
    return showToast("已发布到共享目录，总物理占用不变");
  }
  if (action === "show-tasks") {
    const summary = state.tasks.length ? state.tasks.map((task) => `${task.name}：${task.progress}%`).join("；") : "暂无传输任务";
    return showToast(summary);
  }
}

function openFilePicker(mode) {
  store.dispatch({ type: "ui/open-composer", mode, files: [] });
  fileInput.click();
}

function addBrowserFiles(fileList) {
  const files = [...fileList].map((file) => ({
    id: uniqueId("pending"),
    name: file.name,
    sizeBytes: file.size,
    type: file.type,
    kind: inferKind(file),
  }));
  if (!files.length) return;
  if (!store.getState().ui.composer || store.getState().ui.composer.existingFileId) {
    store.dispatch({ type: "ui/open-composer", mode: "shared", files });
  } else {
    store.dispatch({ type: "composer/add-files", files });
  }
}

function confirmSend() {
  const state = store.getState();
  const composer = state.ui.composer;
  if (!composer) return;
  const now = new Date().toISOString();
  if (composer.existingFileId) {
    store.dispatch({ type: "files/send-existing", fileId: composer.existingFileId, recipientIds: state.ui.selectedRecipientIds, actorId: state.ui.currentUserId, occurredAt: now, eventId: uniqueId("event") });
    return showToast("已从服务端发送，无需再次上传");
  }
  const files = composer.pendingFiles.map((file) => ({ ...file, alias: randomAlias() }));
  store.dispatch({ type: "files/start-upload", batchId: uniqueId("batch"), files, mode: composer.mode, recipientIds: state.ui.selectedRecipientIds, occurredAt: now });
  showToast("模拟上传已开始");
}

function inferKind(file) {
  if (file.type.startsWith("video/")) return "video";
  if (/\.(zip|rar|7z|tar|gz)$/i.test(file.name)) return "archive";
  if (/\.(xls|xlsx|csv)$/i.test(file.name)) return "sheet";
  return "document";
}
function hasFiles(event) { return [...(event.dataTransfer?.types ?? [])].includes("Files"); }
function uniqueId(prefix) { return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`; }
function randomAlias() {
  const chars = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ";
  const part = () => Array.from({ length: 4 }, () => chars[Math.floor(Math.random() * chars.length)]).join("");
  return `${part()}-${part()}`;
}
function showToast(message, danger = false) {
  const region = document.querySelector("#toast-region");
  region.textContent = message;
  region.classList.toggle("is-danger", danger);
  region.classList.add("is-visible");
  window.clearTimeout(showToast.timer);
  showToast.timer = window.setTimeout(() => region.classList.remove("is-visible"), 2600);
}
