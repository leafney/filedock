import { createMockState } from "./mock-data.js";

export function reduceState(state, action) {
  switch (action.type) {
    case "ui/set-tab":
      return { ...state, ui: { ...state.ui, activeFileTab: action.value } };
    case "ui/set-current-user":
      return {
        ...state,
        ui: {
          ...state.ui,
          currentUserId: action.value,
          selectedFileIds: [],
          selectedRecipientIds: [],
        },
      };
    case "ui/set-language":
      return { ...state, ui: { ...state.ui, language: action.value } };
    case "ui/set-scenario":
      return { ...state, ui: { ...state.ui, scenario: action.value } };
    case "ui/set-file-search":
      return { ...state, ui: { ...state.ui, fileSearch: action.value } };
    case "ui/set-file-scope":
      return { ...state, ui: { ...state.ui, fileScopeFilter: action.value } };
    case "ui/set-file-sort":
      return { ...state, ui: { ...state.ui, fileSort: action.value } };
    case "ui/select-chat":
      return { ...state, ui: { ...state.ui, selectedChatUserId: action.value } };
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
