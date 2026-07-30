import { canRestoreFile } from "./capacity.js";
import { getDefaultComposerMode } from "./file-list.js";
import { renderModel } from "./render.js";
import { createStore } from "./state.js";

export const store = createStore();
let dragDepth = 0;
let lastOverlayTrigger = null;

const savedLanguage = window.localStorage.getItem("filedock-prototype-language");
if (savedLanguage === "zh-CN" || savedLanguage === "en") {
  store.dispatch({ type: "ui/set-language", value: savedLanguage });
}

store.subscribe((state, action) => {
  if (action.type !== "chat/set-draft") renderModel(state);
});
renderModel(store.getState());

document.addEventListener("click", (event) => {
  const tab = event.target.closest("[data-tab]");
  if (tab) {
    store.dispatch({ type: "ui/set-tab", value: tab.dataset.tab });
    return;
  }
  const currentState = store.getState();
  if (currentState.ui.openFileMenuId && !event.target.closest(".file-action-menu") && !event.target.closest('[data-action="open-file-actions"]')) {
    store.dispatch({ type: "ui/close-file-overlays" });
  }
  const target = event.target.closest("[data-action]");
  if (!target || target.disabled) return;
  if (["open-upload", "open-direct", "open-existing", "open-members", "open-requests", "open-shared-reference", "show-capacity", "show-qr", "show-room-menu"].includes(target.dataset.action)) {
    lastOverlayTrigger = target;
  }
  handleAction(target.dataset.action, target);
});

document.addEventListener("input", (event) => {
  if (event.target.matches('[data-input="file-search"]')) {
    store.dispatch({ type: "ui/set-file-search", value: event.target.value });
  }
  if (event.target.matches('[data-input="chat-draft"]')) {
    store.dispatch({ type: "chat/set-draft", value: event.target.value });
    const button = event.target.closest(".chat-composer")?.querySelector('[data-action="send-chat"]');
    if (button) button.disabled = !event.target.value.trim();
  }
});

document.addEventListener("keydown", (event) => {
  if (event.target.matches('[data-input="chat-draft"]') && event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();
    if (event.target.value.trim()) handleAction("send-chat", event.target);
  }
  if (event.key === "Escape") {
    const state = store.getState();
    if (state.ui.composer) store.dispatch({ type: "ui/close-composer" });
    else if (state.ui.openFileMenuId || state.ui.actionSheetFileId || state.ui.rejectFileId) store.dispatch({ type: "ui/close-file-overlays" });
    else if (state.ui.modal) store.dispatch({ type: "ui/close-modal" });
    else if (state.ui.drawer) store.dispatch({ type: "ui/close-drawer" });
  }
  if (event.altKey && event.key.toLocaleLowerCase() === "d") {
    event.preventDefault();
    store.dispatch({ type: "ui/toggle-controller" });
  }
  if (event.target.matches('[role="tab"]') && (event.key === "ArrowLeft" || event.key === "ArrowRight")) {
    event.preventDefault();
    const tabs = [...document.querySelectorAll('[role="tab"]')];
    const offset = event.key === "ArrowRight" ? 1 : -1;
    const next = tabs[(tabs.indexOf(event.target) + offset + tabs.length) % tabs.length];
    next.focus();
    store.dispatch({ type: "ui/set-tab", value: next.dataset.tab });
  }
});

document.addEventListener("change", (event) => {
  if (event.target.matches('[data-input="file-scope"]')) store.dispatch({ type: "ui/set-file-scope", value: event.target.value });
  if (event.target.matches('[data-input="file-identity-filter"]')) {
    resolveHiddenBatchSelection((file) => event.target.value === "all" || (event.target.value === "mine" ? file.uploaderId === store.getState().ui.currentUserId : file.recipientIds?.includes(store.getState().ui.currentUserId)));
    store.dispatch({ type: "ui/set-file-identity-filter", value: event.target.value });
  }
  if (event.target.matches('[data-input="file-sort"]')) store.dispatch({ type: "ui/set-file-sort", value: event.target.value });
  if (event.target.matches('[data-input="file-selection"]')) store.dispatch({ type: "ui/toggle-file-selection", value: event.target.dataset.fileId });
  if (event.target.matches('[data-input="batch-group-selection"]')) {
    const ids = event.target.dataset.fileIds.split(",").filter(Boolean);
    const selected = store.getState().ui.selectedFileIds;
    const next = event.target.checked ? [...new Set([...selected, ...ids])] : selected.filter((id) => !ids.includes(id));
    store.dispatch({ type: "ui/set-file-selection", value: next });
  }
  if (event.target.matches('[data-input="demo-role"]')) store.dispatch({ type: "ui/set-current-user", value: event.target.value });
  if (event.target.matches('[data-input="demo-language"]')) {
    window.localStorage.setItem("filedock-prototype-language", event.target.value);
    store.dispatch({ type: "ui/set-language", value: event.target.value });
  }
  if (event.target.matches('[data-input="demo-scenario"]')) store.dispatch({ type: "ui/set-scenario", value: event.target.value });
  if (event.target.matches('[data-input="recycle-count"]')) store.dispatch({ type: "recycle/set-count-toward-capacity", value: event.target.checked });
  if (event.target.matches('[data-input="recycle-free"]')) store.dispatch({ type: "recycle/set-free-bytes", value: Number(event.target.value) * 1024 * 1024 });
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

document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "visible") markCurrentConversationRead();
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
  if (action === "open-upload") return openFilePicker(getDefaultComposerMode(state.ui.fileScopeView));
  if (action === "open-direct") return openFilePicker("direct");
  if (action === "set-file-scope-view") {
    resolveHiddenBatchSelection((file) => target.dataset.scope === "all" || file.scope === target.dataset.scope);
    return store.dispatch({ type: "ui/set-file-scope-view", value: target.dataset.scope });
  }
  if (action === "toggle-file-group") return store.dispatch({ type: "ui/toggle-file-group", value: target.dataset.scope });
  if (action === "enter-batch-mode") return store.dispatch({ type: "ui/enter-batch-mode" });
  if (action === "exit-batch-mode") return store.dispatch({ type: "ui/exit-batch-mode" });
  if (action === "batch-download") {
    state.files.filter((file) => state.ui.selectedFileIds.includes(file.id)).forEach((file) => store.dispatch({
      type: "tasks/start-download",
      taskId: uniqueId("batch-download"),
      fileId: file.id,
      name: file.name ?? `私密文件#${file.alias}`,
      sizeBytes: file.sizeBytes,
      ...common,
      eventId: uniqueId("event"),
    }));
    store.dispatch({ type: "ui/exit-batch-mode" });
    return showToast(localized(state, "已开始批量接收", "Batch download started"));
  }
  if (action === "batch-private-send") {
    return store.dispatch({ type: "ui/open-composer", mode: "direct", existingFileIds: [...state.ui.selectedFileIds] });
  }
  if (action === "open-file-actions") {
    const value = state.ui.openFileMenuId === target.dataset.fileId ? null : target.dataset.fileId;
    return store.dispatch({ type: "ui/open-file-menu", value });
  }
  if (action === "close-file-overlays") return closeOverlay("ui/close-file-overlays");
  if (action === "open-reject-confirm") return store.dispatch({ type: "ui/open-reject-confirm", value: target.dataset.fileId });
  if (action === "confirm-decline") {
    store.dispatch({ type: "files/decline", fileId: target.dataset.fileId, ...common });
    store.dispatch({ type: "ui/close-file-overlays" });
    return showToast(localized(state, "已拒绝接收该私密文件", "Private file declined"));
  }
  if (action === "file-menu-action") {
    store.dispatch({ type: "ui/close-file-overlays" });
    return handleFileMenuAction(target.dataset.fileOperation, target, state);
  }
  if (action === "add-files") return fileInput.click();
  if (action === "open-existing") return store.dispatch({ type: "ui/open-composer", mode: "direct", choosingExisting: true, existingFileIds: [] });
  if (action === "toggle-existing-file") return store.dispatch({ type: "composer/toggle-existing-file", value: target.dataset.fileId });
  if (action === "confirm-existing-files") return store.dispatch({ type: "composer/confirm-existing-files" });
  if (action === "reuse-file") return store.dispatch({ type: "ui/open-composer", mode: "direct", existingFileId: target.dataset.fileId });
  if (action === "close-composer") return closeOverlay("ui/close-composer");
  if (action === "remove-pending") return store.dispatch({ type: "composer/remove-file", value: target.dataset.pendingId });
  if (action === "set-send-mode") return store.dispatch({ type: "composer/set-mode", value: target.dataset.mode });
  if (action === "toggle-recipient") return store.dispatch({ type: "composer/toggle-recipient", value: target.dataset.userId });
  if (action === "select-all-recipients") {
    const existingFiles = state.files.filter((file) => state.ui.composer?.existingFileIds?.includes(file.id));
    const ids = state.users.filter((user) => user.id !== state.ui.currentUserId && user.presence === "online" && (!existingFiles.length || !existingFiles.every((file) => file.recipientIds.includes(user.id)))).map((user) => user.id);
    return store.dispatch({ type: "composer/select-all-recipients", value: ids });
  }
  if (action === "confirm-send") return confirmSend();
  if (action === "download") {
    const file = state.files.find((item) => item.id === target.dataset.fileId);
    if (!file) return;
    store.dispatch({ type: "tasks/start-download", taskId: uniqueId("download"), fileId: file.id, name: file.name ?? `私密文件#${file.alias}`, sizeBytes: file.sizeBytes, ...common });
    return showToast(localized(state, "已开始模拟接收，可在底部查看进度", "Simulated receive started. Progress is shown below."));
  }
  if (action === "recycle") {
    store.dispatch({ type: "files/recycle", fileId: target.dataset.fileId, ...common });
    return showToast(localized(state, "文件已移入回收站", "File moved to Recycle Bin"));
  }
  if (action === "permanent-delete") {
    if (!window.confirm(localized(state, "永久删除后无法恢复。继续？", "Permanent deletion cannot be undone. Continue?"))) return;
    store.dispatch({ type: "files/permanent-delete", fileId: target.dataset.fileId, ...common });
    return showToast(localized(state, "文件已永久删除", "File deleted permanently"));
  }
  if (action === "restore" || action === "approve-restore") {
    const result = canRestoreFile({ fileId: target.dataset.fileId, files: state.files, roomCapacityBytes: state.room.capacityBytes, recycleConfig: state.recycleConfig });
    if (!result.allowed) return showToast(localized(state, "容量不足，无法恢复", "Not enough capacity to restore"), true);
    store.dispatch({ type: action === "restore" ? "files/restore" : "files/approve-restore", fileId: target.dataset.fileId, ...common });
    return showToast(localized(state, "文件已恢复", "File restored"));
  }
  if (action === "request-restore") {
    store.dispatch({ type: "files/request-restore", fileId: target.dataset.fileId, ...common });
    return showToast(localized(state, "恢复申请已提交给房主", "Restore request sent to owner"));
  }
  if (action === "publish-shared") {
    if (!window.confirm(localized(state, "发布后房间当前及后加入成员都可查看。继续？", "Current and future members can view this file after publishing. Continue?"))) return;
    store.dispatch({ type: "files/publish-shared", fileId: target.dataset.fileId, ...common });
    return showToast(localized(state, "已发布到共享目录，总物理占用不变", "Published to shared files. Physical usage is unchanged."));
  }
  if (action === "show-tasks") {
    const summary = state.tasks.length ? state.tasks.map((task) => `${task.name}: ${task.progress}%`).join(" · ") : localized(state, "暂无传输任务", "No transfers");
    return showToast(summary);
  }
  if (action === "show-capacity") return store.dispatch({ type: "ui/open-modal", value: { type: "capacity" } });
  if (action === "show-qr") return store.dispatch({ type: "ui/open-modal", value: { type: "qr" } });
  if (action === "show-room-menu") return store.dispatch({ type: "ui/open-modal", value: { type: "room-menu" } });
  if (action === "copy-room") {
    navigator.clipboard?.writeText(state.room.code).catch(() => {});
    return showToast(state.ui.language === "en" ? "Room code copied" : "房间号已复制");
  }
  if (action === "extend-room") {
    store.dispatch({ type: "ui/close-modal" });
    return showToast(state.ui.language === "en" ? "Room extended by 2 hours (simulated)" : "房间已模拟延长 2 小时");
  }
  if (action === "dissolve-room") return store.dispatch({ type: "ui/set-scenario", value: "dissolved" });
  if (action === "leave-room") return store.dispatch({ type: "ui/set-scenario", value: "kicked" });
  if (action === "toggle-controller") return store.dispatch({ type: "ui/toggle-controller" });
  if (action === "hide-controller") return store.dispatch({ type: "ui/hide-controller" });
  if (action === "reset-state") {
    const language = state.ui.language;
    store.dispatch({ type: "state/reset" });
    store.dispatch({ type: "ui/set-language", value: language });
    return showToast(language === "en" ? "Fixtures reset" : "夹具已重置");
  }
  if (action === "inject-event") return injectDemoEvent(target.dataset.inject);
  if (action === "select-member") {
    store.dispatch({ type: "ui/select-chat", value: target.dataset.userId, mobile: window.innerWidth < 768 });
    markCurrentConversationRead();
    return;
  }
  if (action === "open-mobile-chat") {
    store.dispatch({ type: "ui/set-mobile-page", value: "chat" });
    markCurrentConversationRead();
    return;
  }
  if (action === "close-mobile-chat") return store.dispatch({ type: "ui/set-mobile-page", value: "files" });
  if (action === "open-members") return store.dispatch({ type: "ui/open-drawer", value: "members" });
  if (action === "open-requests") return store.dispatch({ type: "ui/open-drawer", value: "requests" });
  if (action === "close-drawer") return closeOverlay("ui/close-drawer");
  if (action === "close-modal") return closeOverlay("ui/close-modal");
  if (action === "open-shared-reference") return store.dispatch({ type: "ui/open-modal", value: { type: "shared-reference" } });
  if (action === "append-emoji") {
    store.dispatch({ type: "chat/append-draft", value: "🙂" });
    return;
  }
  if (action === "send-chat") return sendChatMessage("text", { text: state.ui.chatDraft.trim() });
  if (action === "send-shared-reference") return sendChatMessage("shared_reference", { fileId: target.dataset.fileId });
  if (action === "chat-direct-file") return openFilePicker("direct", [state.ui.selectedChatUserId]);
  if (action === "recall-message") {
    store.dispatch({ type: "chat/recall", messageId: target.dataset.messageId });
    return showToast(localized(state, "消息已撤回", "Message recalled"));
  }
  if (action === "show-shared-file") {
    store.dispatch({ type: "ui/set-tab", value: "files" });
    store.dispatch({ type: "ui/set-file-search", value: state.files.find((file) => file.id === target.dataset.fileId)?.name ?? "" });
    store.dispatch({ type: "ui/set-mobile-page", value: "files" });
    return showToast(localized(state, "已定位共享文件", "Shared file located"));
  }
  if (action === "resolve-request") return showToast(state.ui.language === "en" ? `${target.textContent} join request (simulated)` : `${target.textContent}加入申请（原型模拟）`);
  if (action === "set-scenario") return store.dispatch({ type: "ui/set-scenario", value: target.dataset.scenario });
}

function openFilePicker(mode, recipientIds = []) {
  store.dispatch({ type: "ui/open-composer", mode, files: [], recipientIds });
  fileInput.click();
}

function resolveHiddenBatchSelection(isVisible) {
  const state = store.getState();
  if (!state.ui.batchMode || !state.ui.selectedFileIds.length) return;
  const hidden = state.files.some((file) => state.ui.selectedFileIds.includes(file.id) && !isVisible(file));
  if (!hidden) return;
  const preserve = window.confirm(localized(state, "切换后部分已选文件会隐藏。确定保留隐藏选择；取消则清空选择。", "Some selected files will be hidden. OK keeps hidden selections; Cancel clears them."));
  if (!preserve) store.dispatch({ type: "ui/set-file-selection", value: [] });
}

function handleFileMenuAction(operation, target, state) {
  const file = state.files.find((item) => item.id === target.dataset.fileId);
  if (!file) return;
  if (operation === "view-details") return showToast(`${file.name} · ${formatCompactBytes(file.sizeBytes)}`);
  if (operation === "view-anonymous-details") return showToast(`${localized(state, "私密文件", "Private file")}#${file.alias} · ${formatCompactBytes(file.sizeBytes)}`);
  if (operation === "receiver-status") {
    const accepted = Object.values(file.receiverStates ?? {}).filter((value) => value === "accepted" || value === "downloaded").length;
    return showToast(localized(state, `${accepted} 人已接收`, `Received by ${accepted}`));
  }
  if (operation === "resend") return store.dispatch({ type: "ui/open-composer", mode: "direct", existingFileId: file.id });
  return handleAction(operation, target);
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
  if (!store.getState().ui.composer || store.getState().ui.composer.existingFileIds?.length) {
    const state = store.getState();
    store.dispatch({ type: "ui/open-composer", mode: getDefaultComposerMode(state.ui.fileScopeView), files, returnTab: state.ui.activeFileTab });
  } else {
    store.dispatch({ type: "composer/add-files", files });
  }
}

function confirmSend() {
  const state = store.getState();
  const composer = state.ui.composer;
  if (!composer) return;
  const now = new Date().toISOString();
  if (composer.existingFileIds?.length) {
    const fileIds = [...composer.existingFileIds];
    const recipients = [...state.ui.selectedRecipientIds];
    store.dispatch({ type: "files/send-existing-many", fileIds, recipientIds: recipients, actorId: state.ui.currentUserId, occurredAt: now, eventId: uniqueId("event") });
    fileIds.forEach((fileId) => recipients.forEach((recipientId) => {
      const file = state.files.find((item) => item.id === fileId);
      if (!file?.recipientIds.includes(recipientId)) store.dispatch({ type: "chat/send", message: { id: uniqueId("message"), fromId: state.ui.currentUserId, toId: recipientId, type: "direct_file", fileId, status: "delivered", createdAt: now } });
    }));
    return showToast(localized(state, "已从服务端发送，无需再次上传", "Sent from server without another upload"));
  }
  const files = composer.pendingFiles.map((file) => ({ ...file, alias: randomAlias() }));
  const batchId = uniqueId("batch");
  const recipients = [...state.ui.selectedRecipientIds];
  store.dispatch({ type: "files/start-upload", batchId, files, mode: composer.mode, recipientIds: recipients, occurredAt: now });
  if (composer.mode === "direct") {
    files.forEach((file, index) => recipients.forEach((recipientId) => store.dispatch({
      type: "chat/send",
      message: {
        id: uniqueId("message"),
        fromId: state.ui.currentUserId,
        toId: recipientId,
        type: "direct_file",
        fileId: `${batchId}-file-${index}`,
        status: "delivered",
        createdAt: now,
      },
    })));
  }
  showToast(localized(state, "模拟上传已开始", "Simulated upload started"));
}

function sendChatMessage(type, payload) {
  const state = store.getState();
  const peerId = state.ui.selectedChatUserId;
  if (!peerId) return;
  const id = uniqueId("message");
  store.dispatch({
    type: "chat/send",
    message: {
      id,
      fromId: state.ui.currentUserId,
      toId: peerId,
      type,
      ...payload,
      status: "sending",
      createdAt: new Date().toISOString(),
    },
  });
  window.setTimeout(() => store.dispatch({ type: "chat/set-status", messageId: id, value: "delivered" }), 650);
}

function markCurrentConversationRead() {
  if (document.visibilityState !== "visible") return;
  const peerId = store.getState().ui.selectedChatUserId;
  if (peerId) store.dispatch({ type: "chat/mark-read", peerId });
}

function injectDemoEvent(kind) {
  const state = store.getState();
  const now = new Date().toISOString();
  const english = state.ui.language === "en";
  if (kind === "upload") {
    store.dispatch({ type: "files/start-upload", batchId: uniqueId("demo"), files: [{ id: uniqueId("pending"), name: "局域网演示文件.bin", sizeBytes: 36 * 1024 * 1024, type: "application/octet-stream", kind: "document", alias: randomAlias() }], mode: "shared", recipientIds: [], occurredAt: now });
    return showToast(english ? "Upload event injected" : "已注入上传事件");
  }
  if (kind === "download") {
    const file = state.files.find((item) => item.status === "available" && item.scope === "shared");
    if (file) store.dispatch({ type: "tasks/start-download", taskId: uniqueId("demo-download"), fileId: file.id, name: file.name, sizeBytes: file.sizeBytes, actorId: state.ui.currentUserId, occurredAt: now, eventId: uniqueId("event") });
    return showToast(english ? "Download event injected" : "已注入接收事件");
  }
  if (kind === "message") {
    const peerId = state.ui.selectedChatUserId;
    store.dispatch({ type: "chat/inject-message", message: { id: uniqueId("demo-message"), fromId: peerId, toId: state.ui.currentUserId, type: "text", text: english ? "New LAN message for prototype review." : "这是一条新注入的局域网消息。", status: "delivered", createdAt: now } });
    return showToast(english ? "Unread message injected" : "已注入未读消息");
  }
  if (kind === "read") {
    store.dispatch({ type: "chat/mark-outgoing-read" });
    return showToast(english ? "Read receipt injected" : "已注入已读回执");
  }
  if (kind === "request") {
    const file = state.files.find((item) => item.status === "recycled" && item.removalKind === "owner_forced");
    if (file) store.dispatch({ type: "files/request-restore", fileId: file.id, actorId: file.uploaderId, occurredAt: now, eventId: uniqueId("event") });
    return showToast(english ? "Restore request injected" : "已注入恢复申请");
  }
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

function localized(state, zh, en) { return state.ui.language === "en" ? en : zh; }

function formatCompactBytes(bytes) {
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(1).replace(".0", "")} GB`;
  return `${(bytes / 1024 ** 2).toFixed(1).replace(".0", "")} MB`;
}

function closeOverlay(type) {
  store.dispatch({ type });
  window.setTimeout(() => lastOverlayTrigger?.focus(), 0);
}
