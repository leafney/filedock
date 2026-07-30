import { calculateCapacity } from "./capacity.js";
import { deriveBatchCapabilities, deriveFileActions, deriveFileGroups } from "./file-list.js";
import { translate } from "./i18n.js";
import { getFilePermissions, projectEventsForUser, projectFilesForUser } from "./permissions.js";

export function renderModel(state) {
  const user = state.users.find((item) => item.id === state.ui.currentUserId);
  if (!user) return;
  document.documentElement.lang = state.ui.language;
  document.body.classList.toggle("is-mobile-chat", state.ui.mobilePage === "chat");
  renderStaticI18n(state);
  renderCapacity(state);
  renderMembers(state, user);
  renderTabs(state, user);
  renderFileContent(state, user);
  renderChat(state, user);
  renderComposer(state, user);
  renderTasks(state);
  renderMobileFileActions(state, user);
  renderAuxiliary(state, user);
  renderScenario(state);
  renderController(state, user);
}

function renderStaticI18n(state) {
  document.querySelectorAll("[data-i18n]").forEach((node) => {
    node.textContent = translate(state.ui.language, node.dataset.i18n);
  });
  const memberCount = document.querySelector(".member-summary-value");
  if (memberCount) memberCount.textContent = state.ui.language === "en" ? String(state.room.memberCount) : `${state.room.memberCount} 人`;
}

function renderMembers(state, currentUser) {
  const panel = document.querySelector(".member-panel");
  if (!panel) return;
  const peers = state.users.filter((user) => user.id !== currentUser.id);
  panel.innerHTML = `<div class="panel-heading member-panel__heading"><div><p class="eyebrow">${t(state, "conversations")}</p><h1 id="member-panel-title">${t(state, "roomMembers")}</h1></div><span class="count-badge">${state.room.memberCount}</span></div>
    <label class="search-box">${searchIcon()}<span class="visually-hidden">${t(state, "searchMembers")}</span><input type="search" placeholder="${t(state, "searchMembers")}" /></label>
    <div class="member-list" role="list" aria-label="成员列表 / Member list">${renderMemberItems(peers, state)}</div>`;
}

function renderMemberItems(peers, state) {
  return peers.map((member) => {
    const conversation = state.messages.filter((message) => isConversation(message, state.ui.currentUserId, member.id));
    const last = conversation.at(-1);
    const unread = conversation.filter((message) => message.fromId === member.id && message.toId === state.ui.currentUserId && message.status !== "read" && message.status !== "recalled").length;
    return `<div role="listitem"><button class="member-item${member.id === state.ui.selectedChatUserId ? " is-active" : ""}" data-action="select-member" data-user-id="${member.id}" type="button">
      <span class="avatar avatar--${member.color}">${escapeHTML(member.avatar)}</span><span class="member-item__body"><span><strong>${escapeHTML(member.displayName)}</strong><time>${last ? formatTimestamp(last.createdAt, state.ui.language).split(" ").at(-1) : ""}</time></span><span>${escapeHTML(messageSummary(last, state))}</span></span>
      ${unread ? `<span class="unread-badge">${unread}</span>` : ""}<span class="presence presence--${member.presence}" title="${presenceLabel(member.presence, state.ui.language)}"></span></button></div>`;
  }).join("");
}

function renderChat(state, currentUser) {
  const panel = document.querySelector(".chat-panel");
  if (!panel) return;
  const peer = state.users.find((user) => user.id === state.ui.selectedChatUserId && user.id !== currentUser.id);
  if (!peer) {
    panel.innerHTML = `<div class="content-placeholder"><h3>${t(state, "selectMember")}</h3><p>${t(state, "noGroupChat")}</p></div>`;
    return;
  }
  const messages = state.ui.scenario === "empty-chat" ? [] : state.messages.filter((message) => isConversation(message, currentUser.id, peer.id));
  const lastOutgoingId = [...messages].reverse().find((message) => message.fromId === currentUser.id && message.status !== "recalled")?.id;
  panel.innerHTML = `<div class="chat-header"><button class="icon-button mobile-chat-back" data-action="close-mobile-chat" type="button" aria-label="${t(state, "backFiles")}">←</button><span class="avatar avatar--${peer.color}">${escapeHTML(peer.avatar)}</span><div><h2 id="chat-panel-title">${escapeHTML(peer.displayName)}</h2><p><span class="presence presence--${peer.presence}"></span>${presenceLabel(peer.presence, state.ui.language)} · ${t(state, "oneToOne")}</p></div><button class="icon-button" data-action="open-members" type="button" aria-label="${t(state, "switchMember")}">•••</button></div>
    <div class="chat-content" aria-label="${t(state, "oneToOne")}">${messages.length ? `<div class="chat-message-list">${messages.map((message) => renderMessage(message, state, currentUser, lastOutgoingId)).join("")}</div>` : `<div class="chat-empty"><strong>${t(state, "noChatTitle")}</strong><span>${t(state, "noChatText")}</span></div>`}</div>
    <div class="chat-composer" aria-label="${t(state, "typeMessage")}"><div class="chat-composer__tools"><button class="icon-button" data-action="append-emoji" type="button" aria-label="${t(state, "addEmoji")}"><svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M8 14s1.5 2 4 2 4-2 4-2M9 9h.01M15 9h.01"/></svg></button><button class="icon-button" data-action="chat-direct-file" type="button" aria-label="${t(state, "sendPrivateFile")}"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m20 11-8 8a6 6 0 0 1-8-8l9-9a4 4 0 0 1 6 6l-9 9a2 2 0 0 1-3-3l8-8"/></svg></button><button class="icon-button" data-action="open-shared-reference" type="button" aria-label="${t(state, "referenceShared")}"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M10 14 21 3M15 3h6v6M21 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h6"/></svg></button></div><textarea data-input="chat-draft" aria-label="${t(state, "typeMessage")}" placeholder="${t(state, "typeMessage")}" maxlength="500">${escapeHTML(state.ui.chatDraft)}</textarea><button class="button button--primary" data-action="send-chat" type="button" ${state.ui.chatDraft.trim() ? "" : "disabled"}>${t(state, "send")}</button></div>`;
}

function renderMessage(message, state, currentUser, lastOutgoingId) {
  const outgoing = message.fromId === currentUser.id;
  if (message.status === "recalled") return `<article class="chat-message chat-message--system">${t(state, outgoing ? "recalledSelf" : "recalledPeer")}</article>`;
  const file = message.fileId ? state.files.find((item) => item.id === message.fileId) : null;
  let content = escapeHTML(message.text ?? "");
  if (message.type === "image") content = `<div class="message-image" role="img" aria-label="${escapeHTML(message.text ?? t(state, "imagePreview"))}"><span>${t(state, "imagePreview")}</span></div>`;
  if (message.type === "shared_reference" && file) content = renderMessageFile(file, t(state, "sharedReference"), t(state, "viewShared"), false);
  if (message.type === "direct_file" && file) content = renderMessageFile(file, t(state, "privateFile", { alias: file.alias }), file.receiverStates?.[currentUser.id] === "pending" ? t(state, "acceptDownload") : t(state, "download"), true);
  const receipt = outgoing && message.id === lastOutgoingId ? `<span class="read-receipt">${t(state, message.status === "read" ? "read" : message.status === "failed" ? "sendFailed" : message.status === "sending" ? "sending" : "delivered")}</span>` : "";
  return `<article class="chat-message ${outgoing ? "chat-message--outgoing" : "chat-message--incoming"}" data-message-id="${message.id}"><div class="message-bubble ${outgoing ? "message-bubble--outgoing" : ""}">${content}</div><time>${formatTimestamp(message.createdAt, state.ui.language).split(" ").at(-1)}</time>${outgoing ? `<button class="recall-button" data-action="recall-message" data-message-id="${message.id}" type="button">${t(state, "recall")}</button>` : ""}${receipt}</article>`;
}

function renderMessageFile(file, label, action, direct) {
  return `<div class="message-file-card"><span class="file-icon file-icon--${file.kind}">${fileIcon(file.kind)}</span><div><strong>${escapeHTML(file.name)}</strong><small>${escapeHTML(label)} · ${formatBytes(file.sizeBytes)}</small></div><button class="mini-button mini-button--primary" data-action="${direct ? "download" : "show-shared-file"}" data-file-id="${file.id}" type="button">${action}</button></div>`;
}

function renderAuxiliary(state, currentUser) {
  let root = document.querySelector("#auxiliary-root");
  if (!root) { root = document.createElement("div"); root.id = "auxiliary-root"; document.body.append(root); }
  if (state.ui.rejectFileId) {
    const file = projectFilesForUser(state.files, currentUser, state.room).find((item) => item.id === state.ui.rejectFileId);
    if (file?.permissions.canDecline) {
      const sender = state.users.find((user) => user.id === file.uploaderId)?.displayName ?? "-";
      root.innerHTML = `<div class="modal-backdrop"><section class="prototype-modal reject-modal" role="alertdialog" aria-modal="true" aria-labelledby="reject-file-title" aria-describedby="reject-file-description">
        <header><div><p class="eyebrow">${t(state, "direct")}</p><h2 id="reject-file-title">${t(state, "rejectFileTitle")}</h2></div><button class="icon-button" data-action="close-file-overlays" aria-label="${t(state, "close")}" type="button">×</button></header>
        <dl><div><dt>${t(state, "fileName")}</dt><dd>${escapeHTML(file.name)}</dd></div><div><dt>${t(state, "sender")}</dt><dd>${escapeHTML(sender)}</dd></div><div><dt>${t(state, "fileSize")}</dt><dd>${formatBytes(file.sizeBytes)}</dd></div></dl>
        <p id="reject-file-description">${t(state, "rejectFileConsequence")}</p>
        <footer><button class="button" data-action="close-file-overlays" type="button">${t(state, "cancel")}</button><button class="button button--danger" data-action="confirm-decline" data-file-id="${escapeHTML(file.id)}" type="button">${t(state, "confirmReject")}</button></footer>
      </section></div>`;
      return;
    }
  }
  if (state.ui.actionSheetFileId) {
    const file = projectFilesForUser(state.files, currentUser, state.room).find((item) => item.id === state.ui.actionSheetFileId);
    const actions = deriveFileActions(file, currentUser, state.room).menuActions;
    if (file && actions.length) {
      const name = file.visibility === "anonymous" ? t(state, "privateFile", { alias: file.alias }) : file.name;
      root.innerHTML = `<div class="action-sheet-backdrop" data-action="close-file-overlays"><section class="file-action-sheet" role="dialog" aria-modal="true" aria-labelledby="file-action-sheet-title">
        <div class="action-sheet-handle" aria-hidden="true"></div>
        <header><div><h2 id="file-action-sheet-title">${escapeHTML(name)}</h2><p><span class="scope-tag scope-tag--${file.visibility === "anonymous" ? "private" : file.scope}">${t(state, file.visibility === "anonymous" ? "privateAudit" : file.scope)}</span>${formatBytes(file.sizeBytes)}</p></div><button class="icon-button" data-action="close-file-overlays" type="button" aria-label="${t(state, "close")}">×</button></header>
        <div class="action-sheet-list">${actions.map((item) => `<button class="${item.tone === "danger" ? "is-danger" : ""}" data-action="file-menu-action" data-file-operation="${item.id}" data-file-id="${escapeHTML(file.id)}" type="button">${t(state, item.labelKey)}<span aria-hidden="true">›</span></button>`).join("")}</div>
        <button class="action-sheet-cancel" data-action="close-file-overlays" type="button">${t(state, "cancel")}</button>
      </section></div>`;
      return;
    }
  }
  if (state.ui.modal?.type === "shared-reference") {
    const files = projectFilesForUser(state.files, currentUser, state.room).filter((file) => file.scope === "shared" && file.status === "available");
    root.innerHTML = `<div class="modal-backdrop"><section class="prototype-modal" role="dialog" aria-modal="true" aria-labelledby="reference-title"><header><h2 id="reference-title">${t(state, "referenceTitle")}</h2><button class="icon-button" data-action="close-modal" aria-label="${t(state, "close")}" type="button">×</button></header><div class="existing-list">${files.map((file) => `<button data-action="send-shared-reference" data-file-id="${file.id}" type="button"><strong>${escapeHTML(file.name)}</strong><span>${formatBytes(file.sizeBytes)} · ${t(state, "noCapacityIncrease")}</span></button>`).join("")}</div></section></div>`;
    return;
  }
  if (state.ui.modal?.type === "capacity") {
    const capacity = calculateCapacity({ files: state.files, roomCapacityBytes: state.room.capacityBytes, recycleConfig: state.recycleConfig });
    const owner = currentUser.id === state.room.ownerId;
    const rows = owner ? [
      ["sharedUsage", capacity.sharedBytes], ["directUsage", capacity.directBytes], ["reservedUsage", capacity.reservedBytes],
      ["recycleActual", capacity.recycledBytes], ["recycleFree", capacity.freeRecycleBytes], ["recycleCharged", capacity.recycledChargedBytes],
      ["totalUsed", capacity.occupiedBytes], ["totalCapacity", capacity.capacityBytes],
    ] : [["totalUsed", capacity.occupiedBytes], ["totalCapacity", capacity.capacityBytes]];
    root.innerHTML = `<div class="modal-backdrop"><section class="prototype-modal capacity-modal" role="dialog" aria-modal="true" aria-labelledby="capacity-title"><header><h2 id="capacity-title">${t(state, "capacityDetails")}</h2><button class="icon-button" data-action="close-modal" aria-label="${t(state, "close")}" type="button">×</button></header><dl>${rows.map(([key, bytes]) => `<div><dt>${t(state, key)}</dt><dd>${formatBytes(bytes)}</dd></div>`).join("")}</dl></section></div>`;
    return;
  }
  if (state.ui.modal?.type === "qr") {
    root.innerHTML = `<div class="modal-backdrop"><section class="prototype-modal qr-modal" role="dialog" aria-modal="true" aria-labelledby="qr-title"><header><h2 id="qr-title">${t(state, "qrCode")}</h2><button class="icon-button" data-action="close-modal" aria-label="${t(state, "close")}" type="button">×</button></header><div class="mock-qr" aria-label="${t(state, "qrCode")}">${Array.from({ length: 81 }, (_, index) => `<i class="${[0,1,2,9,11,18,19,20,6,7,8,15,17,24,25,26,54,55,56,63,65,72,73,74].includes(index) || (index * 7 + index % 5) % 4 === 0 ? "is-dark" : ""}"></i>`).join("")}</div><strong>1234</strong><p>${t(state, "scanJoin")}</p></section></div>`;
    return;
  }
  if (state.ui.modal?.type === "room-menu") {
    const owner = currentUser.id === state.room.ownerId;
    root.innerHTML = `<div class="modal-backdrop"><section class="prototype-modal room-menu-modal" role="dialog" aria-modal="true" aria-labelledby="room-menu-title"><header><h2 id="room-menu-title">${t(state, "roomActions")}</h2><button class="icon-button" data-action="close-modal" aria-label="${t(state, "close")}" type="button">×</button></header><div class="room-menu-list"><button data-action="show-capacity" type="button">${t(state, "roomDetails")}<span>1234 · ${state.room.memberCount}</span></button>${owner ? `<button data-action="extend-room" type="button">${t(state, "extendRoom")}<span>+ 2 h</span></button><button class="is-danger" data-action="dissolve-room" type="button">${t(state, "dissolveRoom")}</button>` : `<button class="is-danger" data-action="leave-room" type="button">${t(state, "leaveRoom")}</button>`}</div></section></div>`;
    return;
  }
  if (state.ui.drawer === "members" || state.ui.drawer === "requests") {
    const members = state.users.filter((user) => user.id !== currentUser.id);
    root.innerHTML = `<div class="drawer-backdrop" data-action="close-drawer"><aside class="prototype-drawer" role="dialog" aria-modal="true" aria-labelledby="drawer-title" onclick="event.stopPropagation()"><header><div><p class="eyebrow">${t(state, state.ui.drawer === "requests" ? "ownerManagement" : "oneToOne")}</p><h2 id="drawer-title">${t(state, state.ui.drawer === "requests" ? "joinRequests" : "roomMembers")}</h2></div><button class="icon-button" data-action="close-drawer" type="button" aria-label="${t(state, "close")}">×</button></header>${state.ui.drawer === "requests" ? renderRequests(state) : `<div class="drawer-member-list">${renderMemberItems(members, state)}</div>`}</aside></div>`;
    return;
  }
  root.innerHTML = "";
}

function renderRequests(state) {
  return `<div class="join-request-list"><article><span class="avatar avatar--blue">许</span><div><strong>许知夏</strong><small>01:42</small></div><button class="mini-button mini-button--primary" data-action="resolve-request" type="button">${t(state, "approve")}</button><button class="mini-button" data-action="resolve-request" type="button">${t(state, "reject")}</button></article><article><span class="avatar avatar--amber">韩</span><div><strong>韩川</strong><small>03:18</small></div><button class="mini-button mini-button--primary" data-action="resolve-request" type="button">${t(state, "approve")}</button><button class="mini-button" data-action="resolve-request" type="button">${t(state, "reject")}</button></article></div>`;
}

function renderScenario(state) {
  let root = document.querySelector("#scenario-root");
  if (!root) { root = document.createElement("div"); root.id = "scenario-root"; document.body.append(root); }
  const scenario = state.ui.scenario;
  const scenes = state.ui.language === "en" ? {
    "join-free": ["Join room 1234", "No verification is required for this room.", "Join now", "workspace"],
    pin: ["Enter room PIN", "Enter the 4-digit PIN provided by the owner.", "Verify and join", "pending"],
    pending: ["Waiting for approval", "Your request was sent. You will enter after approval.", "Back to home", "workspace"],
    dissolved: ["Room dissolved", "All files and Recycle Bin content will be removed.", "Back to home", "workspace"],
    kicked: ["Removed from room", "The owner ended your room access.", "Back to home", "workspace"],
    "not-found": ["Room not found", "The room code is invalid, expired, or dissolved.", "Back to home", "workspace"],
    "load-error": ["Load failed", "Unable to load room data. Check the LAN connection.", "Retry", "workspace"],
  } : {
    "join-free": ["加入房间 1234", "该房间无需验证，可直接加入。", "立即加入", "workspace"],
    pin: ["输入房间密码", "请输入房主提供的 4 位数字密码。", "验证并加入", "pending"],
    pending: ["等待房主审批", "申请已发送，批准后自动进入房间。", "返回首页", "workspace"],
    dissolved: ["房间已解散", "所有文件和回收站内容将被清理。", "返回首页", "workspace"],
    kicked: ["你已被移出房间", "房主结束了你的本次房间访问。", "返回首页", "workspace"],
    "not-found": ["房间不存在", "房间号无效、已过期或已被解散。", "返回首页", "workspace"],
    "load-error": ["加载失败", "无法加载房间数据，请检查局域网连接。", "重试", "workspace"],
  };
  if (!scenes[scenario]) { root.innerHTML = ""; return; }
  const [title, text, button, next] = scenes[scenario];
  root.innerHTML = `<div class="scenario-screen" role="alertdialog" aria-modal="true"><section><div class="brand-mark">FD</div><p class="eyebrow">FileDock 房间</p><h1>${title}</h1><p>${text}</p>${scenario === "pin" ? `<div class="pin-inputs" aria-label="四位房间密码"><input inputmode="numeric" maxlength="1" value="1"><input inputmode="numeric" maxlength="1" value="2"><input inputmode="numeric" maxlength="1"><input inputmode="numeric" maxlength="1"></div>` : ""}<button class="button button--primary" data-action="set-scenario" data-scenario="${next}" type="button">${button}</button></section></div>`;
}

function renderController(state, currentUser) {
  let root = document.querySelector("#demo-controller-root");
  if (!root) { root = document.createElement("div"); root.id = "demo-controller-root"; document.body.append(root); }
  if (state.ui.controllerHidden) { root.innerHTML = ""; return; }
  const owner = currentUser.id === state.room.ownerId;
  const scenarios = [
    ["workspace", "workspace"], ["join-free", "joinFree"], ["pin", "pinJoin"], ["pending", "approvalWait"],
    ["dissolved", "dissolved"], ["kicked", "kicked"], ["not-found", "notFound"], ["load-error", "loadError"],
    ["empty-files", "emptyFiles"], ["empty-chat", "emptyChat"], ["empty-recycle", "emptyRecycle"],
  ];
  root.innerHTML = `<div class="demo-controller${state.ui.controllerOpen ? " is-open" : ""}">
    <button class="demo-controller__toggle" data-action="toggle-controller" type="button" aria-expanded="${state.ui.controllerOpen}" aria-label="${t(state, "openController")}"><span>PROTOTYPE</span>${t(state, "demo")}</button>
    ${state.ui.controllerOpen ? `<section aria-label="${t(state, "demo")}"><header><div><strong>${t(state, "demo")}</strong><span>${t(state, "demoOnly")}</span></div><button class="icon-button" data-action="toggle-controller" type="button" aria-label="${t(state, "close")}">×</button></header>
      <label><span>${t(state, "role")}</span><select data-input="demo-role"><option value="user-owner"${owner ? " selected" : ""}>${t(state, "owner")}</option><option value="user-lin"${!owner ? " selected" : ""}>${t(state, "member")} · 林小满</option></select></label>
      <label><span>${t(state, "language")}</span><select data-input="demo-language"><option value="zh-CN"${state.ui.language === "zh-CN" ? " selected" : ""}>简体中文</option><option value="en"${state.ui.language === "en" ? " selected" : ""}>English</option></select></label>
      <label><span>${t(state, "scenario")}</span><select data-input="demo-scenario">${scenarios.map(([value, key]) => option(value, t(state, key), state.ui.scenario)).join("")}</select></label>
      <label class="switch-row"><span>${t(state, "recycleCounts")}</span><input data-input="recycle-count" type="checkbox" role="switch" ${state.recycleConfig.countTowardRoomCapacity ? "checked" : ""}></label>
      ${state.recycleConfig.countTowardRoomCapacity ? "" : `<label><span>${t(state, "freeRecycle")}</span><input data-input="recycle-free" type="number" min="0" step="16" value="${Math.round(state.recycleConfig.freeBytes / 1024 / 1024)}"><small>MB</small></label>`}
      <div class="demo-injections"><span>${t(state, "injectEvent")}</span><div><button data-action="inject-event" data-inject="upload" type="button">${t(state, "upload")}</button><button data-action="inject-event" data-inject="download" type="button">${t(state, "receive")}</button><button data-action="inject-event" data-inject="message" type="button">${t(state, "send")}</button><button data-action="inject-event" data-inject="read" type="button">${t(state, "read")}</button><button data-action="inject-event" data-inject="request" type="button">${t(state, "requestRestore")}</button></div></div>
      <footer><button class="button" data-action="reset-state" type="button">${t(state, "reset")}</button><button class="button button--text" data-action="hide-controller" type="button">${t(state, "hideController")}</button></footer>
    </section>` : ""}
  </div>`;
}

function renderCapacity(state) {
  const capacity = calculateCapacity({ files: state.files, roomCapacityBytes: state.room.capacityBytes, recycleConfig: state.recycleConfig });
  const label = document.querySelector(".capacity-summary__value");
  const fill = document.querySelector(".capacity-track span");
  if (label) label.textContent = `${formatBytes(capacity.occupiedBytes)} / ${formatBytes(capacity.capacityBytes)}`;
  if (fill) fill.style.width = `${Math.min(100, capacity.usageRatio * 100).toFixed(1)}%`;
}

function renderTabs(state, user) {
  const recycledCount = projectFilesForUser(state.files, user, state.room).filter((file) => file.status === "recycled").length;
  document.querySelectorAll(".file-tab").forEach((tab) => {
    const active = tab.dataset.tab === state.ui.activeFileTab;
    tab.classList.toggle("is-active", active);
    tab.setAttribute("aria-selected", String(active));
    if (tab.dataset.tab === "recycle") {
      const count = tab.querySelector(".tab-count");
      if (count) count.textContent = String(recycledCount);
    }
  });
}

function renderFileContent(state, user) {
  const container = document.querySelector(".file-content");
  if (!container) return;
  let content;
  if (state.ui.activeFileTab === "timeline") {
    content = renderTimeline(state, user);
  } else if (state.ui.activeFileTab === "recycle") {
    content = renderRecycleBin(state, user);
  } else {
    content = renderFileList(state, user);
  }
  container.innerHTML = `<div class="workspace-drop-frame">${content}<div class="workspace-drop-overlay" aria-hidden="true"><strong>${t(state, "dropTitle")}</strong><span>${t(state, "dropText")}</span></div></div>`;
}

function renderFileList(state, user) {
  const projected = state.ui.scenario === "empty-files" ? [] : projectFilesForUser(state.files, user, state.room)
    .filter((file) => file.status === "available" || file.status === "uploading");
  const groups = deriveFileGroups({
    files: projected,
    search: state.ui.fileSearch,
    scopeView: state.ui.fileScopeView,
    identityFilter: state.ui.fileIdentityFilter,
    sort: state.ui.fileSort,
    userId: user.id,
  });
  const selectedFiles = projected.filter((file) => state.ui.selectedFileIds.includes(file.id));
  const batch = deriveBatchCapabilities(selectedFiles, user, state.room);
  return `
    <div class="file-filter-bar">
      <div class="file-scope-switch" role="radiogroup" aria-label="${t(state, "fileScope")}">
        ${["all", "shared", "direct"].map((scope) => `<button class="${state.ui.fileScopeView === scope ? "is-active" : ""}" data-action="set-file-scope-view" data-scope="${scope}" type="button" role="radio" aria-checked="${state.ui.fileScopeView === scope}">${t(state, scope === "all" ? "all" : scope)}</button>`).join("")}
      </div>
      <label class="search-box file-search">
        ${searchIcon()}<span class="visually-hidden">${t(state, "searchFiles")}</span>
        <input data-input="file-search" type="search" placeholder="${t(state, "searchFiles")}" value="${escapeHTML(state.ui.fileSearch)}" />
      </label>
      <label class="select-shell identity-filter"><span class="visually-hidden">${t(state, "identityFilter")}</span>
        <select data-input="file-identity-filter">
          ${option("all", t(state, "allMembers"), state.ui.fileIdentityFilter)}${option("mine", t(state, "uploadedByMe"), state.ui.fileIdentityFilter)}
          ${option("sent-to-me", t(state, "sentToMe"), state.ui.fileIdentityFilter)}
        </select>${chevronIcon()}</label>
      <label class="select-shell"><span class="visually-hidden">${t(state, "newest")}</span>
        <select data-input="file-sort">
          ${option("newest", t(state, "newest"), state.ui.fileSort)}${option("oldest", t(state, "oldest"), state.ui.fileSort)}
          ${option("size-asc", t(state, "sizeAsc"), state.ui.fileSort)}${option("size-desc", t(state, "sizeDesc"), state.ui.fileSort)}
        </select>${chevronIcon()}</label>
      ${state.ui.batchMode ? "" : `<button class="button batch-entry" data-action="enter-batch-mode" type="button">${t(state, "batchSelect")}</button>`}
    </div>
    <button class="file-drop-strip" data-action="open-upload" type="button">${uploadIcon()}<span>${t(state, "dropStrip")}</span></button>
    ${state.ui.batchMode ? renderBatchToolbar(state, batch) : ""}
    <div class="file-groups">${renderFileGroups(groups, state, user)}</div>`;
}

function renderBatchToolbar(state, batch) {
  const reason = !batch.download.enabled ? batch.download.reason : !batch.privateSend.enabled ? batch.privateSend.reason : null;
  return `<div class="batch-toolbar" role="region" aria-label="${t(state, "batchActions")}">
    <strong>${t(state, "selectedCount", { count: state.ui.selectedFileIds.length })}</strong>
    <button class="button" data-action="batch-download" type="button" ${batch.download.enabled ? "" : "disabled"} title="${t(state, batch.download.reason ?? "batchDownload")}">${t(state, "batchDownload")}</button>
    <button class="button" data-action="batch-private-send" type="button" ${batch.privateSend.enabled ? "" : "disabled"} title="${t(state, batch.privateSend.reason ?? "batchPrivateSend")}">${t(state, "batchPrivateSend")}</button>
    ${reason ? `<span class="batch-reason">${t(state, reason)}</span>` : ""}
    <button class="button button--text" data-action="exit-batch-mode" type="button">${t(state, "exitBatch")}</button>
  </div>`;
}

function renderFileGroups(groups, state, user) {
  if (state.ui.fileScopeView !== "all") {
    const group = groups.orderedGroups[0];
    return `<section class="file-group file-group--single" aria-label="${t(state, group.scope)}">
      <div class="file-group__single-title"><strong>${t(state, group.scope)}</strong><span>${group.count}</span></div>
      ${renderFileGroupTable(group.files, state, user)}
    </section>`;
  }
  return groups.orderedGroups.map((group) => {
    const collapsed = state.ui.collapsedFileGroups[group.scope];
    return `<section class="file-group file-group--${group.scope}">
      <button class="file-group__heading" data-action="toggle-file-group" data-scope="${group.scope}" type="button" aria-expanded="${!collapsed}">
        <span class="file-group__icon" aria-hidden="true">${group.scope === "shared" ? sharedIcon() : lockIcon()}</span>
        <strong>${t(state, group.scope)}</strong><span class="file-group__count">${group.count}</span><span class="file-group__chevron" aria-hidden="true">⌄</span>
      </button>
      ${collapsed ? "" : renderFileGroupTable(group.files, state, user)}
    </section>`;
  }).join("");
}

function renderFileGroupTable(files, state, user) {
  return `<div class="file-table" role="table" aria-label="${t(state, "currentRoomFiles")}">
    ${renderTableHeader(state, files)}
    ${files.length ? files.map((file) => renderFileRow(file, state, user)).join("") : renderEmptyRow(t(state, "noMatchingFiles"))}
  </div>`;
}

function renderTimeline(state, user) {
  const events = projectEventsForUser(state.events, state.files, user, state.room)
    .sort((left, right) => Date.parse(right.occurredAt) - Date.parse(left.occurredAt));
  if (!events.length) return renderEmptyState(t(state, "fileTimeline"), t(state, "noMessages"));
  return `<div class="timeline-toolbar"><strong>${t(state, "fileTimeline")}</strong><span>${t(state, "timelineVisible", { count: events.length })}</span></div>
    <ol class="file-timeline">${events.map((event) => renderTimelineEvent(event, state)).join("")}</ol>
    <button class="button timeline-more" type="button">${t(state, "loadEarlier")}</button>`;
}

function renderTimelineEvent(event, state) {
  const actor = state.users.find((user) => user.id === event.actorId)?.displayName ?? "未知用户";
  const file = event.file;
  const name = file.visibility === "anonymous" ? t(state, "privateFile", { alias: file.alias }) : file.name;
  const labels = state.ui.language === "en" ? {
    upload_started: "started uploading", upload_completed: "uploaded", upload_progress: "is uploading", upload_failed: "upload failed",
    direct_sent: "sent privately", direct_declined: "declined", resent: "sent again from history", download_started: "started receiving", download_completed: "completed transfer",
    recycled: "moved to Recycle Bin", restore_requested: "requested restore", restored: "restored", restore_approved: "approved restore",
    permanently_deleted: "deleted permanently", published_shared: "published to shared files",
  } : {
    upload_started: "开始上传", upload_completed: "完成上传", upload_progress: "正在上传", upload_failed: "上传失败",
    direct_sent: "私密发送", direct_declined: "拒绝接收", resent: "从历史文件再次发送", download_started: "开始接收", download_completed: "传输完成",
    recycled: "移入回收站", restore_requested: "申请恢复", restored: "恢复文件", restore_approved: "批准恢复",
    permanently_deleted: "永久删除", published_shared: "发布到共享目录",
  };
  const progress = event.progress ?? (file.status === "uploading" ? file.uploadProgress : null);
  return `<li class="timeline-item${file.visibility === "anonymous" ? " timeline-item--audit" : ""}">
    <time>${formatTimestamp(event.occurredAt, state.ui.language)}</time>
    <span class="timeline-dot" aria-hidden="true"></span>
    <article class="timeline-card"><div><strong>${escapeHTML(actor)} ${labels[event.type] ?? event.type}</strong><span class="scope-tag scope-tag--${file.visibility === "anonymous" ? "private" : file.scope}">${t(state, file.visibility === "anonymous" ? "privateAudit" : file.scope)}</span></div>
      <p>${escapeHTML(name ?? "-")} · ${formatBytes(file.sizeBytes)}</p>
      ${progress !== null && progress < 100 ? `<div class="timeline-progress"><span style="width:${progress}%"></span></div><small>${progress}%</small>` : ""}
    </article></li>`;
}

function renderRecycleBin(state, user) {
  const files = state.ui.scenario === "empty-recycle" ? [] : projectFilesForUser(state.files, user, state.room).filter((file) => file.status === "recycled");
  const capacity = calculateCapacity({ files: state.files, roomCapacityBytes: state.room.capacityBytes, recycleConfig: state.recycleConfig });
  return `<div class="recycle-summary"><div><strong>${t(state, "recycleBin")}</strong><span>${t(state, "recycleRetention")}</span></div><div><span>${t(state, "actualSize", { size: formatBytes(capacity.recycledBytes) })}</span><span>${t(state, "chargedSize", { size: formatBytes(capacity.recycledChargedBytes) })}</span></div></div>
    <div class="file-table" role="table" aria-label="回收站文件 / Recycled files">
      ${renderTableHeader(state)}
      ${files.length ? files.map((file) => renderRecycleRow(file, state, user)).join("") : renderEmptyRow(t(state, "recycleEmpty"))}
    </div>`;
}

function renderFileRow(file, state, user) {
  return renderCommonRow(file, state, file.permissions, renderFileActions(file, state, user));
}

function renderRecycleRow(file, state, user) {
  const permissions = getFilePermissions(file, user, state.room);
  let action = "";
  if (permissions.canRestore) action += `<button class="mini-button mini-button--primary" data-action="restore" data-file-id="${escapeHTML(file.id)}" type="button">${t(state, "restore")}</button>`;
  if (permissions.canRequestRestore) action += `<button class="mini-button" data-action="request-restore" data-file-id="${escapeHTML(file.id)}" type="button">${t(state, file.restoreRequested ? "requested" : "requestRestore")}</button>`;
  if (file.restoreRequested && user.id === state.room.ownerId) action += `<button class="mini-button mini-button--primary" data-action="approve-restore" data-file-id="${escapeHTML(file.id)}" type="button">${t(state, "approveRestore")}</button>`;
  if (permissions.canPermanentDelete) action += `<button class="mini-button mini-button--danger" data-action="permanent-delete" data-file-id="${escapeHTML(file.id)}" type="button">${t(state, "permanentDelete")}</button>`;
  return renderCommonRow(file, state, permissions, action || "—");
}

function renderCommonRow(file, state, permissions, actions) {
  const locale = state.ui.language;
  const uploader = state.users.find((item) => item.id === file.uploaderId);
  const recipients = file.recipientIds?.map((id) => state.users.find((item) => item.id === id)?.displayName).filter(Boolean).join("、");
  const anonymous = file.visibility === "anonymous";
  const displayName = anonymous ? translate(locale, "privateFile", { alias: file.alias }) : file.name;
  const secondary = anonymous ? t(state, "privateAudit") : file.scope === "direct" ? translate(locale, "privateFile", { alias: file.alias }) : file.mimeLabel;
  const ownerLabel = anonymous ? `${uploader?.displayName ?? "-"} → ${recipients || "-"}` : uploader?.displayName ?? "-";
  const selected = state.ui.selectedFileIds.includes(file.id);
  const checkbox = state.ui.batchMode ? `<input class="file-select-checkbox" data-input="file-selection" data-file-id="${escapeHTML(file.id)}" type="checkbox" ${selected ? "checked" : ""} aria-label="${t(state, "selectFile", { name: displayName })}">` : "";
  return `<article class="file-row${anonymous ? " file-row--private-audit" : ""}${selected ? " is-selected" : ""}" role="row" data-file-id="${escapeHTML(file.id)}">
    <div class="file-cell file-cell--name" role="cell">${checkbox}<span class="file-icon file-icon--${escapeHTML(file.kind ?? "document")}">${fileIcon(file.kind)}</span><div><strong>${escapeHTML(displayName ?? "")}</strong><small>${escapeHTML(secondary ?? "")}</small><span class="scope-tag mobile-scope-tag scope-tag--${anonymous ? "private" : file.scope}">${t(state, anonymous ? "privateAudit" : file.scope)}</span></div></div>
    <div class="file-cell file-cell--owner" role="cell" data-label="${t(state, "uploader")}">${escapeHTML(ownerLabel)}</div>
    <div class="file-cell file-cell--size" role="cell">${formatBytes(file.sizeBytes)}</div>
    <div class="file-cell file-cell--time" role="cell">${formatTimestamp(file.recycledAt ?? file.createdAt, locale)}</div>
    <div class="file-cell file-cell--status" role="cell">${renderStatus(file, locale)}</div>
    <div class="file-cell file-cell--actions" role="cell">${state.ui.batchMode ? "" : actions}</div></article>`;
}

function renderFileActions(file, state, user) {
  const actions = deriveFileActions(file, user, state.room);
  const primary = actions.primaryActions.map((item) => {
    const mappedAction = item.id === "accept-download" ? "download" : item.id === "decline" ? "open-reject-confirm" : item.id === "view-task" ? "show-tasks" : item.id;
    const tone = item.tone === "primary" ? " mini-button--primary" : item.tone === "danger-secondary" ? " mini-button--danger" : "";
    return `<button class="mini-button${tone}" data-action="${mappedAction}" data-file-id="${escapeHTML(file.id)}" type="button">${t(state, item.labelKey)}</button>`;
  }).join("");
  if (!actions.menuActions.length) return primary;
  const menuOpen = state.ui.openFileMenuId === file.id;
  const menu = menuOpen ? `<div class="file-action-menu" role="menu" aria-label="${t(state, "moreFileActions")}">${actions.menuActions.map((item) => `<button class="${item.tone === "danger" ? "is-danger" : ""}" data-action="file-menu-action" data-file-operation="${item.id}" data-file-id="${escapeHTML(file.id)}" type="button" role="menuitem">${t(state, item.labelKey)}</button>`).join("")}</div>` : "";
  return `${primary}<button class="icon-button icon-button--small file-more-button" data-action="open-file-actions" data-file-id="${escapeHTML(file.id)}" type="button" aria-label="${t(state, "moreFileActions")}" aria-haspopup="menu" aria-expanded="${menuOpen}">${moreIcon()}</button>${menu}`;
}

function renderStatus(file, locale) {
  if (file.status === "uploading") {
    const progress = Math.max(0, Math.min(100, file.uploadProgress ?? 0));
    return `<div class="row-progress" role="progressbar" aria-valuenow="${progress}" aria-valuemin="0" aria-valuemax="100"><span style="width:${progress}%"></span></div><small>${progress}%</small>`;
  }
  if (file.status === "recycled") return `<span class="file-status">${translate(locale, "recycleBin")}</span>`;
  if (file.visibility === "anonymous") return `<span class="file-status">${translate(locale, "transferComplete")}</span>`;
  const accepted = Object.values(file.receiverStates ?? {}).filter((value) => value === "accepted" || value === "downloaded").length;
  if (file.scope === "direct" && accepted > 0) return `<span class="file-status file-status--accepted">${translate(locale, "receivedCount", { count: accepted })}</span>`;
  return `<span class="file-status file-status--ready">${translate(locale, "ready")}</span>`;
}

function renderComposer(state, user) {
  let root = document.querySelector("#prototype-overlay-root");
  if (!root) {
    root = document.createElement("div");
    root.id = "prototype-overlay-root";
    document.body.append(root);
  }
  const composer = state.ui.composer;
  if (!composer) { root.innerHTML = ""; return; }
  if (composer.choosingExisting) {
    const reusable = projectFilesForUser(state.files, user, state.room).filter((file) => file.scope === "direct" && file.uploaderId === user.id && file.status === "available");
    root.innerHTML = `<div class="modal-backdrop"><section class="prototype-modal existing-picker" role="dialog" aria-modal="true" aria-labelledby="existing-title"><header><div><p class="eyebrow">${t(state, "historyReuse")}</p><h2 id="existing-title">${t(state, "chooseUploaded")}</h2></div><button class="icon-button" data-action="close-composer" aria-label="${t(state, "close")}" type="button">×</button></header><div class="existing-list">${reusable.length ? reusable.map((file) => { const selected = composer.existingFileIds.includes(file.id); return `<button class="${selected ? "is-selected" : ""}" data-action="toggle-existing-file" data-file-id="${file.id}" type="button" aria-pressed="${selected}"><span class="mock-checkbox">${selected ? "✓" : ""}</span><strong>${escapeHTML(file.name)}</strong><span>${t(state, "privateFile", { alias: file.alias })} · ${formatBytes(file.sizeBytes)}</span></button>`; }).join("") : `<p>${t(state, "noReusable")}</p>`}</div><footer><span>${t(state, "selectedCount", { count: composer.existingFileIds.length })}</span><button class="button button--primary" data-action="confirm-existing-files" type="button" ${composer.existingFileIds.length ? "" : "disabled"}>${t(state, "nextSelectRecipients")}</button></footer></section></div>`;
    return;
  }
  const pending = composer.pendingFiles ?? [];
  const existingFiles = state.files.filter((file) => composer.existingFileIds?.includes(file.id));
  const total = existingFiles.length ? existingFiles.reduce((sum, file) => sum + file.sizeBytes, 0) : pending.reduce((sum, file) => sum + file.sizeBytes, 0);
  const recipients = state.users.filter((member) => member.id !== user.id);
  const submitDisabled = (!existingFiles.length && pending.length === 0) || (composer.mode === "direct" && state.ui.selectedRecipientIds.length === 0);
  root.innerHTML = `<div class="modal-backdrop"><section class="prototype-modal send-modal" role="dialog" aria-modal="true" aria-labelledby="send-title">
    <header><div><p class="eyebrow">${t(state, "pendingConfirm")}</p><h2 id="send-title">${t(state, "pendingArea")}</h2></div><button class="icon-button" data-action="close-composer" aria-label="${t(state, "close")}" type="button">×</button></header>
    <div class="send-mode" role="radiogroup" aria-label="${t(state, "pendingConfirm")}"><button class="${composer.mode === "shared" ? "is-active" : ""}" data-action="set-send-mode" data-mode="shared" type="button">${t(state, "sharedToRoom")}</button><button class="${composer.mode === "direct" ? "is-active" : ""}" data-action="set-send-mode" data-mode="direct" type="button">${t(state, "directSend")}</button></div>
    <div class="pending-list">${existingFiles.length ? existingFiles.map((file) => `<article><div><strong>${escapeHTML(file.name)}</strong><span>${t(state, "reuseNoCapacity")}</span></div><b>${formatBytes(file.sizeBytes)}</b></article>`).join("") : pending.map((file) => `<article><div><strong>${escapeHTML(file.name)}</strong><span>${escapeHTML(file.type || "-")}</span></div><b>${formatBytes(file.sizeBytes)}</b><button class="icon-button icon-button--small" data-action="remove-pending" data-pending-id="${file.id}" aria-label="${t(state, "close")} ${escapeHTML(file.name)}" type="button">×</button></article>`).join("") || `<div class="composer-drop-empty">${t(state, "dropTitle")} · ${t(state, "addFiles")}</div>`}</div>
    ${composer.mode === "direct" ? `<div class="recipient-heading"><strong>${t(state, "selectRecipients")}</strong><button class="button button--text" data-action="select-all-recipients" type="button">${t(state, "selectOnline")}</button></div><div class="recipient-grid">${recipients.map((member) => { const already = existingFiles.length > 0 && existingFiles.every((file) => file.recipientIds.includes(member.id)); const selected = state.ui.selectedRecipientIds.includes(member.id); return `<button class="recipient-chip${selected ? " is-selected" : ""}" data-action="toggle-recipient" data-user-id="${member.id}" type="button" ${already ? "disabled" : ""}><span class="presence presence--${member.presence}"></span>${escapeHTML(member.displayName)}${already ? ` · ${t(state, "alreadySent")}` : ""}</button>`; }).join("")}</div>` : `<div class="shared-warning">${t(state, "sharedWarning")}</div>`}
    <footer><div><strong>${translate(state.ui.language, "filesCount", { count: existingFiles.length || pending.length })} · ${formatBytes(total)}</strong><span>${t(state, existingFiles.length ? "serverDirect" : "confirmUpload")}</span></div>${existingFiles.length ? "" : `<button class="button" data-action="add-files" type="button">${t(state, "addFiles")}</button>`}<button class="button button--primary" data-action="confirm-send" type="button" ${submitDisabled ? "disabled" : ""}>${t(state, existingFiles.length ? "sendRecipients" : "startUpload")}</button></footer>
  </section></div>`;
}

function renderTasks(state) {
  const bar = document.querySelector(".transfer-bar");
  if (!bar) return;
  const active = state.tasks.filter((task) => task.status === "active");
  const shown = active.length ? active : state.tasks.slice(0, 2);
  const totalBytes = shown.reduce((sum, task) => sum + task.sizeBytes, 0);
  const sentBytes = shown.reduce((sum, task) => sum + task.transferredBytes, 0);
  const progress = totalBytes ? Math.round(sentBytes / totalBytes * 100) : 0;
  const title = active.length ? t(state, "tasksCount", { count: active.length }) : state.tasks.length ? t(state, "tasksDone") : t(state, "noTasks");
  bar.innerHTML = `<div class="transfer-bar__summary"><span class="transfer-indicator${active.length ? "" : " is-idle"}" aria-hidden="true"></span><strong>${title}</strong><span>${shown.map((task) => `${t(state, task.type === "upload" ? "upload" : "receive")} ${task.progress}%`).join(" · ")}</span></div><div class="transfer-progress" role="progressbar" aria-label="${t(state, "tasks")}" aria-valuenow="${progress}" aria-valuemin="0" aria-valuemax="100"><span style="width:${progress}%"></span></div><button class="button button--text" data-action="show-tasks" type="button">${t(state, "tasks")}</button>`;
}

function renderMobileFileActions(state, user) {
  const root = document.querySelector(".mobile-file-actions");
  if (!root) return;
  if (state.ui.batchMode && state.ui.activeFileTab === "files") {
    const projected = projectFilesForUser(state.files, user, state.room);
    const selected = projected.filter((file) => state.ui.selectedFileIds.includes(file.id));
    const batch = deriveBatchCapabilities(selected, user, state.room);
    root.classList.add("is-batch");
    root.innerHTML = `<span>${t(state, "selectedCount", { count: selected.length })}</span><button class="button" data-action="batch-download" type="button" ${batch.download.enabled ? "" : "disabled"}>${t(state, "download")}</button><button class="button button--primary" data-action="batch-private-send" type="button" ${batch.privateSend.enabled ? "" : "disabled"}>${t(state, "batchPrivateSend")}</button><button class="icon-button" data-action="exit-batch-mode" type="button" aria-label="${t(state, "exitBatch")}">×</button>`;
    return;
  }
  root.classList.remove("is-batch");
  root.innerHTML = `<button class="button button--primary" data-action="open-upload" type="button">${t(state, "addFile")}</button><button class="button" data-action="open-existing" type="button">${t(state, "sendExistingPrivate")}</button>`;
}

function renderTableHeader(state, files = []) {
  const labels = state.ui.language === "en" ? ["File", "Uploader", "Size", "Time", "Status", "Actions"] : ["文件名", "上传者", "大小", "时间", "状态", "操作"];
  if (state.ui.batchMode) {
    const allSelected = files.length > 0 && files.every((file) => state.ui.selectedFileIds.includes(file.id));
    labels[0] = `<label class="batch-select-all"><input data-input="batch-group-selection" data-file-ids="${files.map((file) => file.id).join(",")}" type="checkbox" ${allSelected ? "checked" : ""}><span>${labels[0]}</span></label>`;
  }
  return `<div class="file-table__header" role="row">${labels.map((label, index) => `<span role="columnheader">${index === 0 && state.ui.batchMode ? label : escapeHTML(label)}</span>`).join("")}</div>`;
}
function renderEmptyRow(text) { return `<div class="table-empty">${escapeHTML(text)}</div>`; }
function renderEmptyState(title, text) { return `<div class="content-placeholder"><h3>${escapeHTML(title)}</h3><p>${escapeHTML(text)}</p></div>`; }
function option(value, label, selected) { return `<option value="${value}"${value === selected ? " selected" : ""}>${label}</option>`; }
function searchIcon() { return `<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/></svg>`; }
function chevronIcon() { return `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m7 10 5 5 5-5"/></svg>`; }
function moreIcon() { return `<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/></svg>`; }
function uploadIcon() { return `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 16V4M7 9l5-5 5 5M5 20h14"/></svg>`; }
function sharedIcon() { return `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h6l2 2h8v10H4z"/></svg>`; }
function lockIcon() { return `<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="5" y="10" width="14" height="10" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/></svg>`; }
function isConversation(message, firstId, secondId) { return (message.fromId === firstId && message.toId === secondId) || (message.fromId === secondId && message.toId === firstId); }
function presenceLabel(value, locale = "zh-CN") { return translate(locale, value); }
function messageSummary(message, state) {
  if (!message) return t(state, "noMessages");
  if (message.status === "recalled") return t(state, "recall");
  if (message.type === "image") return state.ui.language === "en" ? "[Image]" : "[图片]";
  if (message.type === "direct_file") return t(state, "sendPrivateFile");
  if (message.type === "shared_reference") return `${t(state, "sharedReference")}: ${state.files.find((file) => file.id === message.fileId)?.name ?? t(state, "sharedFiles")}`;
  return message.text ?? "";
}

function t(state, key, params = {}) { return translate(state.ui.language, key, params); }

export function formatBytes(value) {
  const bytes = Math.max(0, Number(value) || 0);
  if (bytes >= 1024 ** 3) return `${trim(bytes / 1024 ** 3)} GB`;
  if (bytes >= 1024 ** 2) return `${trim(bytes / 1024 ** 2)} MB`;
  if (bytes >= 1024) return `${trim(bytes / 1024)} KB`;
  return `${Math.round(bytes)} B`;
}

function formatTimestamp(value, locale) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat(locale === "en" ? "en" : "zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }).format(date);
}
function trim(value) { return Number.isInteger(value) ? String(value) : value.toFixed(1).replace(/\.0$/, ""); }
function escapeHTML(value) { return String(value).replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "&#039;"); }
function fileIcon(kind) {
  const icons = {
    archive: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16v13H4zM3 3h18v4H3zM10 11h4" /></svg>',
    sheet: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 2h9l4 4v16H6zM15 2v5h4M9 12h7M9 16h7" /></svg>',
    video: '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="5" width="14" height="14" rx="2" /><path d="m17 10 4-2v8l-4-2" /></svg>',
    private: '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="4" y="10" width="16" height="11" rx="2" /><path d="M8 10V7a4 4 0 0 1 8 0v3" /></svg>',
    document: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 2h9l4 4v16H6zM15 2v5h4M9 12h7M9 16h7" /></svg>',
  };
  return icons[kind] ?? icons.document;
}
