import zhCN from "./zh-CN";

type TranslationShape<T> = {
  [K in keyof T]: T[K] extends string ? string : TranslationShape<T[K]>;
};

const en = {
  brand: {
    name: "FileDock",
    displayName: "FileDock",
    description: "A browser-ready foundation for transferring files.",
  },
  language: {
    label: "Select language",
    zhCN: "简体中文",
    en: "English",
  },
  home: {
    loading: "Checking service status…",
    unavailable: "The service is unavailable. Make sure the backend is running, then refresh the page.",
    fields: {
      status: "Status",
      version: "Version",
      branch: "Branch",
      commit: "Commit",
      buildTime: "Build time",
    },
  },
  error: {
    timeout: "The request timed out. Please try again later.",
    network: "A network error occurred. Check your connection and try again.",
    unavailable: "The service is unavailable. Please try again later.",
    eventUnknown: "The service state changed. Refresh the page to see the latest state.",
  },
  session: {
    create: "Start using FileDock",
    displayName: "Display name",
    displayNamePlaceholder: "Enter a display name",
    randomize: "Generate a random name",
    reset: "Reset anonymous identity",
  },
  room: {
    title: "Room",
    create: "Create room",
    join: "Join room",
    code: "Room code",
    joinMode: "Verification method",
    open: "No verification",
    password: "PIN verification",
    ownerApproval: "Owner approval",
    pin: "Four-digit PIN",
    pinConfirmation: "Confirm PIN",
    members: "Members",
    online: "Online",
    away: "Away",
    offline: "Offline",
    extend: "Extend room",
    leave: "Leave room",
    dissolve: "Dissolve room",
    kicked: "The owner removed you from this room. Confirm to return home.",
    destroying: "The owner dissolved the room. It will be released in {{seconds}} seconds.",
    expired: "The room expired. It will be released in {{seconds}} seconds.",
    destroyed: "The room was released. Returning home.",
    confirmJoin: "Join this room?",
    waitingApproval: "Your request was submitted. Please wait for the owner.",
    notifications: "Join request notifications",
    pendingCount: "{{count}} pending request(s)",
  },
} as const satisfies TranslationShape<typeof zhCN>;

export default en;
