import { createMockState } from "./mock-data.js";

export function reduceState(state, action) {
  switch (action.type) {
    case "ui/set-tab":
      return {
        ...state,
        ui: {
          ...state.ui,
          activeFileTab: action.value,
          openFileMenuId: null,
          actionSheetFileId: null,
          rejectFileId: null,
        },
      };
    case "ui/set-current-user":
      return {
        ...state,
        ui: {
          ...state.ui,
          currentUserId: action.value,
          selectedChatUserId: state.ui.selectedChatUserId === action.value ? state.room.ownerId : state.ui.selectedChatUserId,
          selectedFileIds: [],
          selectedRecipientIds: [],
          batchMode: false,
          openFileMenuId: null,
          actionSheetFileId: null,
          rejectFileId: null,
        },
      };
    case "ui/set-language":
      return { ...state, ui: { ...state.ui, language: action.value } };
    case "ui/set-scenario":
      return { ...state, ui: { ...state.ui, scenario: action.value, drawer: null, modal: null } };
    case "ui/set-file-search":
      return { ...state, ui: { ...state.ui, fileSearch: action.value } };
    case "ui/set-file-scope":
      return { ...state, ui: { ...state.ui, fileScopeFilter: action.value } };
    case "ui/set-file-scope-view":
      return {
        ...state,
        ui: {
          ...state.ui,
          fileScopeView: action.value,
          openFileMenuId: null,
          actionSheetFileId: null,
          rejectFileId: null,
        },
      };
    case "ui/set-file-identity-filter":
      return { ...state, ui: { ...state.ui, fileIdentityFilter: action.value } };
    case "ui/toggle-file-group":
      return {
        ...state,
        ui: {
          ...state.ui,
          collapsedFileGroups: {
            ...state.ui.collapsedFileGroups,
            [action.value]: !state.ui.collapsedFileGroups[action.value],
          },
        },
      };
    case "ui/enter-batch-mode":
      return { ...state, ui: { ...state.ui, batchMode: true, openFileMenuId: null, actionSheetFileId: null } };
    case "ui/exit-batch-mode":
      return { ...state, ui: { ...state.ui, batchMode: false, selectedFileIds: [] } };
    case "ui/toggle-file-selection": {
      if (!state.ui.batchMode) return state;
      const selected = state.ui.selectedFileIds.includes(action.value);
      return {
        ...state,
        ui: {
          ...state.ui,
          selectedFileIds: selected
            ? state.ui.selectedFileIds.filter((id) => id !== action.value)
            : [...state.ui.selectedFileIds, action.value],
        },
      };
    }
    case "ui/set-file-selection":
      return { ...state, ui: { ...state.ui, selectedFileIds: state.ui.batchMode ? [...action.value] : [] } };
    case "ui/open-file-menu":
      return { ...state, ui: { ...state.ui, openFileMenuId: action.value, actionSheetFileId: null, rejectFileId: null } };
    case "ui/open-action-sheet":
      return { ...state, ui: { ...state.ui, actionSheetFileId: action.value, openFileMenuId: null, rejectFileId: null } };
    case "ui/open-reject-confirm":
      return { ...state, ui: { ...state.ui, rejectFileId: action.value, openFileMenuId: null, actionSheetFileId: null } };
    case "ui/close-file-overlays":
      return { ...state, ui: { ...state.ui, openFileMenuId: null, actionSheetFileId: null, rejectFileId: null } };
    case "ui/set-file-sort":
      return { ...state, ui: { ...state.ui, fileSort: action.value } };
    case "ui/open-composer":
      return {
        ...state,
        ui: {
          ...state.ui,
          composer: {
            mode: action.mode ?? "shared",
            pendingFiles: action.files ?? [],
            existingFileId: action.existingFileId ?? null,
          },
          selectedRecipientIds: action.recipientIds ?? [],
          uploadReturnTab: action.returnTab ?? state.ui.activeFileTab,
        },
      };
    case "ui/close-composer":
      return {
        ...state,
        ui: {
          ...state.ui,
          activeFileTab: state.ui.uploadReturnTab ?? state.ui.activeFileTab,
          composer: null,
          selectedRecipientIds: [],
          uploadReturnTab: null,
        },
      };
    case "composer/add-files":
      return {
        ...state,
        ui: {
          ...state.ui,
          composer: {
            ...(state.ui.composer ?? { mode: "shared", existingFileId: null }),
            pendingFiles: [...(state.ui.composer?.pendingFiles ?? []), ...action.files],
          },
        },
      };
    case "composer/remove-file":
      return {
        ...state,
        ui: {
          ...state.ui,
          composer: {
            ...state.ui.composer,
            pendingFiles: state.ui.composer.pendingFiles.filter((file) => file.id !== action.value),
          },
        },
      };
    case "composer/set-mode":
      return {
        ...state,
        ui: { ...state.ui, composer: { ...state.ui.composer, mode: action.value }, selectedRecipientIds: [] },
      };
    case "composer/toggle-recipient": {
      const selected = state.ui.selectedRecipientIds.includes(action.value);
      return {
        ...state,
        ui: {
          ...state.ui,
          selectedRecipientIds: selected
            ? state.ui.selectedRecipientIds.filter((id) => id !== action.value)
            : [...state.ui.selectedRecipientIds, action.value],
        },
      };
    }
    case "composer/select-all-recipients":
      return { ...state, ui: { ...state.ui, selectedRecipientIds: action.value } };
    case "files/start-upload":
      return startUpload(state, action);
    case "files/send-existing":
      return sendExisting(state, action);
    case "files/recycle":
      return updateFileWithEvent(state, action.fileId, action, (file) => ({
        ...file,
        status: "recycled",
        removedById: state.ui.currentUserId,
        removalKind: file.uploaderId === state.ui.currentUserId ? "self" : "owner_forced",
        recycledAt: action.occurredAt,
      }), "recycled");
    case "files/permanent-delete":
      return updateFileWithEvent(state, action.fileId, action, (file) => ({ ...file, status: "deleted" }), "permanently_deleted");
    case "files/restore":
      return updateFileWithEvent(state, action.fileId, action, (file) => ({ ...file, status: "available", restoreRequested: false }), "restored");
    case "files/request-restore":
      return updateFileWithEvent(state, action.fileId, action, (file) => ({ ...file, restoreRequested: true }), "restore_requested");
    case "files/approve-restore":
      return updateFileWithEvent(state, action.fileId, action, (file) => ({ ...file, status: "available", restoreRequested: false }), "restore_approved");
    case "files/publish-shared":
      return updateFileWithEvent(state, action.fileId, action, (file) => ({ ...file, scope: "shared", recipientIds: [], receiverStates: {} }), "published_shared");
    case "tasks/start-download": {
      const task = {
        id: action.taskId,
        fileId: action.fileId,
        type: "download",
        name: action.name,
        sizeBytes: action.sizeBytes,
        transferredBytes: 0,
        progress: 0,
        speedBytes: 5 * 1024 * 1024,
        status: "active",
      };
      return {
        ...state,
        tasks: [...state.tasks, task],
        events: [createEvent(action, "download_started"), ...state.events],
      };
    }
    case "tasks/tick":
      return tickTasks(state, action.step ?? 8);
    case "ui/select-chat":
      return { ...state, ui: { ...state.ui, selectedChatUserId: action.value, mobilePage: action.mobile ? "chat" : state.ui.mobilePage, drawer: null } };
    case "ui/set-mobile-page":
      return { ...state, ui: { ...state.ui, mobilePage: action.value } };
    case "ui/toggle-controller":
      return { ...state, ui: { ...state.ui, controllerOpen: !state.ui.controllerOpen, controllerHidden: false } };
    case "ui/hide-controller":
      return { ...state, ui: { ...state.ui, controllerOpen: false, controllerHidden: true } };
    case "ui/open-drawer":
      return { ...state, ui: { ...state.ui, drawer: action.value } };
    case "ui/close-drawer":
      return { ...state, ui: { ...state.ui, drawer: null } };
    case "ui/open-modal":
      return { ...state, ui: { ...state.ui, modal: action.value } };
    case "ui/close-modal":
      return { ...state, ui: { ...state.ui, modal: null } };
    case "chat/set-draft":
      return { ...state, ui: { ...state.ui, chatDraft: action.value } };
    case "chat/append-draft":
      return { ...state, ui: { ...state.ui, chatDraft: `${state.ui.chatDraft}${action.value}` } };
    case "chat/send":
      return {
        ...state,
        messages: [...state.messages, action.message],
        ui: { ...state.ui, chatDraft: "", modal: null },
      };
    case "chat/set-status":
      return { ...state, messages: state.messages.map((message) => message.id === action.messageId ? { ...message, status: action.value } : message) };
    case "chat/mark-read": {
      const lastIncoming = [...state.messages].reverse().find((message) => message.fromId === action.peerId && message.toId === state.ui.currentUserId);
      return {
        ...state,
        messages: state.messages.map((message) => message.fromId === action.peerId && message.toId === state.ui.currentUserId && message.status !== "recalled" ? { ...message, status: "read" } : message),
        readCursors: lastIncoming ? { ...state.readCursors, [`${state.ui.currentUserId}:${action.peerId}`]: lastIncoming.id } : state.readCursors,
      };
    }
    case "chat/recall":
      return { ...state, messages: state.messages.map((message) => message.id === action.messageId && message.fromId === state.ui.currentUserId ? { ...message, status: "recalled", text: "" } : message) };
    case "chat/inject-message":
      return { ...state, messages: [...state.messages, action.message] };
    case "chat/mark-outgoing-read":
      return { ...state, messages: state.messages.map((message) => message.fromId === state.ui.currentUserId && message.toId === state.ui.selectedChatUserId && message.status !== "recalled" ? { ...message, status: "read" } : message) };
    case "recycle/set-count-toward-capacity":
      return {
        ...state,
        recycleConfig: { ...state.recycleConfig, countTowardRoomCapacity: Boolean(action.value) },
      };
    case "recycle/set-free-bytes":
      return {
        ...state,
        recycleConfig: { ...state.recycleConfig, freeBytes: Math.max(0, Number(action.value) || 0) },
      };
    case "state/reset":
      return createMockState();
    default:
      return state;
  }
}

function startUpload(state, action) {
  const files = action.files.map((pending, index) => ({
    id: `${action.batchId}-file-${index}`,
    scope: action.mode,
    name: pending.name,
    alias: action.mode === "direct" ? pending.alias : null,
    mimeLabel: pending.type || "未知类型",
    kind: pending.kind,
    sizeBytes: pending.sizeBytes,
    uploaderId: state.ui.currentUserId,
    recipientIds: action.mode === "direct" ? [...action.recipientIds] : [],
    receiverStates: action.mode === "direct"
      ? Object.fromEntries(action.recipientIds.map((id) => [id, "pending"]))
      : {},
    status: "uploading",
    uploadProgress: 0,
    createdAt: action.occurredAt,
  }));
  const tasks = files.map((file, index) => ({
    id: `${action.batchId}-task-${index}`,
    batchId: action.batchId,
    fileId: file.id,
    type: "upload",
    name: file.name,
    sizeBytes: file.sizeBytes,
    transferredBytes: 0,
    progress: 0,
    speedBytes: (3 + index) * 1024 * 1024,
    status: "active",
  }));
  const events = files.map((file, index) => ({
    id: `${action.batchId}-event-${index}`,
    type: "upload_started",
    fileId: file.id,
    actorId: state.ui.currentUserId,
    occurredAt: action.occurredAt,
    progress: 0,
  }));
  return {
    ...state,
    files: [...files, ...state.files],
    tasks: [...tasks, ...state.tasks],
    events: [...events, ...state.events],
    ui: { ...state.ui, composer: null, selectedRecipientIds: [], activeFileTab: "files" },
  };
}

function sendExisting(state, action) {
  const file = state.files.find((item) => item.id === action.fileId);
  if (!file) return state;
  const newRecipients = action.recipientIds.filter((id) => !file.recipientIds.includes(id));
  if (newRecipients.length === 0) return { ...state, ui: { ...state.ui, composer: null, selectedRecipientIds: [] } };
  return {
    ...state,
    files: state.files.map((item) => item.id === file.id ? {
      ...item,
      recipientIds: [...item.recipientIds, ...newRecipients],
      receiverStates: { ...item.receiverStates, ...Object.fromEntries(newRecipients.map((id) => [id, "pending"])) },
    } : item),
    events: [createEvent(action, "resent", file.id), ...state.events],
    ui: { ...state.ui, composer: null, selectedRecipientIds: [] },
  };
}

function updateFileWithEvent(state, fileId, action, update, type) {
  const file = state.files.find((item) => item.id === fileId);
  if (!file) return state;
  return {
    ...state,
    files: state.files.map((item) => item.id === fileId ? update(item) : item),
    events: [createEvent(action, type, fileId), ...state.events],
  };
}

function createEvent(action, type, fileId = action.fileId) {
  return {
    id: action.eventId ?? `event-${Date.now()}`,
    type,
    fileId,
    actorId: action.actorId,
    occurredAt: action.occurredAt,
  };
}

function tickTasks(state, step) {
  const completedFileIds = [];
  const tasks = state.tasks.map((task) => {
    if (task.status !== "active") return task;
    const progress = Math.min(100, task.progress + step);
    if (progress === 100) completedFileIds.push(task.fileId);
    return {
      ...task,
      progress,
      transferredBytes: Math.round(task.sizeBytes * progress / 100),
      status: progress === 100 ? "completed" : "active",
    };
  });
  if (completedFileIds.length === 0) {
    return { ...state, tasks };
  }
  const now = new Date().toISOString();
  const completedUploads = tasks.filter((task) => task.type === "upload" && completedFileIds.includes(task.fileId));
  const files = state.files.map((file) => {
    const uploadTask = tasks.find((task) => task.fileId === file.id && task.type === "upload");
    if (!uploadTask || file.status !== "uploading") return file;
    return {
      ...file,
      status: uploadTask.status === "completed" ? "available" : "uploading",
      uploadProgress: uploadTask.progress,
    };
  });
  const events = state.events.map((event) => {
    const task = tasks.find((item) => item.fileId === event.fileId);
    if (event.type === "upload_started" && task) return { ...event, type: "upload_completed", progress: 100, occurredAt: now };
    return event;
  });
  return {
    ...state,
    tasks,
    files,
    events: [
      ...completedFileIds.filter((id) => !completedUploads.some((task) => task.fileId === id)).map((id) => ({
        id: `event-download-${id}-${Date.now()}`,
        type: "download_completed",
        fileId: id,
        actorId: state.ui.currentUserId,
        occurredAt: now,
      })),
      ...events,
    ],
  };
}

export function createStore(initialState = createMockState()) {
  let state = initialState;
  const listeners = new Set();

  return {
    getState: () => state,
    dispatch(action) {
      const nextState = reduceState(state, action);
      if (nextState === state) return;
      state = nextState;
      listeners.forEach((listener) => listener(state, action));
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}
