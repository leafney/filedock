import { calculateCapacity } from "./capacity.js";
import { translate } from "./i18n.js";
import { getFilePermissions, projectEventsForUser, projectFilesForUser } from "./permissions.js";

export function renderModel(state) {
  const user = state.users.find((item) => item.id === state.ui.currentUserId);
  if (!user) return;
  document.documentElement.lang = state.ui.language;
  document.body.classList.toggle("is-mobile-chat", state.ui.mobilePage === "chat");
  renderCapacity(state);
  renderMembers(state, user);
  renderTabs(state, user);
  renderFileContent(state, user);
  renderChat(state, user);
  renderComposer(state, user);
  renderTasks(state);
  renderAuxiliary(state, user);
  renderScenario(state);
}

function renderMembers(state, currentUser) {
  const panel = document.querySelector(".member-panel");
  if (!panel) return;
  const peers = state.users.filter((user) => user.id !== currentUser.id);
  panel.innerHTML = `<div class="panel-heading member-panel__heading"><div><p class="eyebrow">会话</p><h1 id="member-panel-title">房间成员</h1></div><span class="count-badge">${state.room.memberCount}</span></div>
    <label class="search-box">${searchIcon()}<span class="visually-hidden">搜索成员 / Search members</span><input type="search" placeholder="搜索成员" /></label>
    <div class="member-list" role="list" aria-label="成员列表 / Member list">${renderMemberItems(peers, state)}</div>`;
}

function renderMemberItems(peers, state) {
  return peers.map((member) => {
    const conversation = state.messages.filter((message) => isConversation(message, state.ui.currentUserId, member.id));
    const last = conversation.at(-1);
    const unread = conversation.filter((message) => message.fromId === member.id && message.toId === state.ui.currentUserId && message.status !== "read" && message.status !== "recalled").length;
    return `<div role="listitem"><button class="member-item${member.id === state.ui.selectedChatUserId ? " is-active" : ""}" data-action="select-member" data-user-id="${member.id}" type="button">
      <span class="avatar avatar--${member.color}">${escapeHTML(member.avatar)}</span><span class="member-item__body"><span><strong>${escapeHTML(member.displayName)}</strong><time>${last ? formatTimestamp(last.createdAt, state.ui.language).split(" ").at(-1) : ""}</time></span><span>${escapeHTML(messageSummary(last, state))}</span></span>
      ${unread ? `<span class="unread-badge">${unread}</span>` : ""}<span class="presence presence--${member.presence}" title="${presenceLabel(member.presence)}"></span></button></div>`;
  }).join("");
}

function renderChat(state, currentUser) {
  const panel = document.querySelector(".chat-panel");
  if (!panel) return;
  const peer = state.users.find((user) => user.id === state.ui.selectedChatUserId && user.id !== currentUser.id);
  if (!peer) {
    panel.innerHTML = `<div class="content-placeholder"><h3>选择一名成员</h3><p>这里只支持房间内一对一聊天，不提供群聊。</p></div>`;
    return;
  }
  const messages = state.ui.scenario === "empty-chat" ? [] : state.messages.filter((message) => isConversation(message, currentUser.id, peer.id));
  const lastOutgoingId = [...messages].reverse().find((message) => message.fromId === currentUser.id && message.status !== "recalled")?.id;
  panel.innerHTML = `<div class="chat-header"><button class="icon-button mobile-chat-back" data-action="close-mobile-chat" type="button" aria-label="返回文件">←</button><span class="avatar avatar--${peer.color}">${escapeHTML(peer.avatar)}</span><div><h2 id="chat-panel-title">${escapeHTML(peer.displayName)}</h2><p><span class="presence presence--${peer.presence}"></span>${presenceLabel(peer.presence)} · 一对一聊天</p></div><button class="icon-button" data-action="open-members" type="button" aria-label="切换成员">•••</button></div>
    <div class="chat-content" aria-label="聊天消息 / Chat messages">${messages.length ? `<div class="chat-message-list">${messages.map((message) => renderMessage(message, state, currentUser, lastOutgoingId)).join("")}</div>` : `<div class="chat-empty"><strong>暂无消息</strong><span>发送文字或引用共享文件开始对话。</span></div>`}</div>
    <div class="chat-composer" aria-label="消息输入 / Message composer"><div class="chat-composer__tools"><button class="icon-button" data-action="append-emoji" type="button" aria-label="添加表情"><svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M8 14s1.5 2 4 2 4-2 4-2M9 9h.01M15 9h.01"/></svg></button><button class="icon-button" data-action="chat-direct-file" type="button" aria-label="发送私密文件"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m20 11-8 8a6 6 0 0 1-8-8l9-9a4 4 0 0 1 6 6l-9 9a2 2 0 0 1-3-3l8-8"/></svg></button><button class="icon-button" data-action="open-shared-reference" type="button" aria-label="引用共享文件"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M10 14 21 3M15 3h6v6M21 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h6"/></svg></button></div><textarea data-input="chat-draft" aria-label="输入消息 / Enter message" placeholder="输入消息，Enter 发送" maxlength="500">${escapeHTML(state.ui.chatDraft)}</textarea><button class="button button--primary" data-action="send-chat" type="button" ${state.ui.chatDraft.trim() ? "" : "disabled"}>发送</button></div>`;
}

function renderMessage(message, state, currentUser, lastOutgoingId) {
  const outgoing = message.fromId === currentUser.id;
  if (message.status === "recalled") return `<article class="chat-message chat-message--system">${outgoing ? "你" : "对方"}撤回了一条消息</article>`;
  const file = message.fileId ? state.files.find((item) => item.id === message.fileId) : null;
  let content = escapeHTML(message.text ?? "");
  if (message.type === "image") content = `<div class="message-image" role="img" aria-label="${escapeHTML(message.text ?? "聊天图片")}"><span>图片预览</span></div>`;
  if (message.type === "shared_reference" && file) content = renderMessageFile(file, "共享文件引用", "查看共享文件", false);
  if (message.type === "direct_file" && file) content = renderMessageFile(file, `私密文件#${file.alias}`, file.receiverStates?.[currentUser.id] === "pending" ? "接受并下载" : "下载", true);
  const receipt = outgoing && message.id === lastOutgoingId ? `<span class="read-receipt">${message.status === "read" ? "已读" : message.status === "failed" ? "发送失败" : message.status === "sending" ? "发送中" : "已送达"}</span>` : "";
  return `<article class="chat-message ${outgoing ? "chat-message--outgoing" : "chat-message--incoming"}" data-message-id="${message.id}"><div class="message-bubble ${outgoing ? "message-bubble--outgoing" : ""}">${content}</div><time>${formatTimestamp(message.createdAt, state.ui.language).split(" ").at(-1)}</time>${outgoing ? `<button class="recall-button" data-action="recall-message" data-message-id="${message.id}" type="button">撤回</button>` : ""}${receipt}</article>`;
}

function renderMessageFile(file, label, action, direct) {
  return `<div class="message-file-card"><span class="file-icon file-icon--${file.kind}">${fileIcon(file.kind)}</span><div><strong>${escapeHTML(file.name)}</strong><small>${escapeHTML(label)} · ${formatBytes(file.sizeBytes)}</small></div><button class="mini-button mini-button--primary" data-action="${direct ? "download" : "show-shared-file"}" data-file-id="${file.id}" type="button">${action}</button></div>`;
}

function renderAuxiliary(state, currentUser) {
  let root = document.querySelector("#auxiliary-root");
  if (!root) { root = document.createElement("div"); root.id = "auxiliary-root"; document.body.append(root); }
  if (state.ui.modal?.type === "shared-reference") {
    const files = projectFilesForUser(state.files, currentUser, state.room).filter((file) => file.scope === "shared" && file.status === "available");
    root.innerHTML = `<div class="modal-backdrop"><section class="prototype-modal" role="dialog" aria-modal="true" aria-labelledby="reference-title"><header><h2 id="reference-title">引用共享文件</h2><button class="icon-button" data-action="close-modal" aria-label="关闭" type="button">×</button></header><div class="existing-list">${files.map((file) => `<button data-action="send-shared-reference" data-file-id="${file.id}" type="button"><strong>${escapeHTML(file.name)}</strong><span>${formatBytes(file.sizeBytes)} · 只发送引用，不增加容量</span></button>`).join("")}</div></section></div>`;
    return;
  }
  if (state.ui.drawer === "members" || state.ui.drawer === "requests") {
    const members = state.users.filter((user) => user.id !== currentUser.id);
    root.innerHTML = `<div class="drawer-backdrop" data-action="close-drawer"><aside class="prototype-drawer" role="dialog" aria-modal="true" aria-labelledby="drawer-title" onclick="event.stopPropagation()"><header><div><p class="eyebrow">${state.ui.drawer === "requests" ? "房主管理" : "一对一会话"}</p><h2 id="drawer-title">${state.ui.drawer === "requests" ? "加入申请" : "房间成员"}</h2></div><button class="icon-button" data-action="close-drawer" type="button" aria-label="关闭">×</button></header>${state.ui.drawer === "requests" ? renderRequests() : `<div class="drawer-member-list">${renderMemberItems(members, state)}</div>`}</aside></div>`;
    return;
  }
  root.innerHTML = "";
}

function renderRequests() {
  return `<div class="join-request-list"><article><span class="avatar avatar--blue">许</span><div><strong>许知夏</strong><small>申请剩余 01:42</small></div><button class="mini-button mini-button--primary" data-action="resolve-request" type="button">同意</button><button class="mini-button" data-action="resolve-request" type="button">拒绝</button></article><article><span class="avatar avatar--amber">韩</span><div><strong>韩川</strong><small>申请剩余 03:18</small></div><button class="mini-button mini-button--primary" data-action="resolve-request" type="button">同意</button><button class="mini-button" data-action="resolve-request" type="button">拒绝</button></article></div>`;
}

function renderScenario(state) {
  let root = document.querySelector("#scenario-root");
  if (!root) { root = document.createElement("div"); root.id = "scenario-root"; document.body.append(root); }
  const scenario = state.ui.scenario;
  const scenes = {
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

function renderCapacity(state) {
  const capacity = calculateCapacity({ files: state.files, roomCapacityBytes: state.room.capacityBytes, recycleConfig: state.recycleConfig });
  const label = document.querySelector(".capacity-summary dd");
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
  if (state.ui.activeFileTab === "timeline") {
    container.innerHTML = renderTimeline(state, user);
    return;
  }
  if (state.ui.activeFileTab === "recycle") {
    container.innerHTML = renderRecycleBin(state, user);
    return;
  }
  container.innerHTML = renderFileList(state, user);
}

function renderFileList(state, user) {
  const visibleFiles = state.ui.scenario === "empty-files" ? [] : filterAndSort(projectFilesForUser(state.files, user, state.room)
    .filter((file) => file.status === "available" || file.status === "uploading"), state, user);
  return `
    <div class="file-filter-bar">
      <label class="search-box file-search">
        ${searchIcon()}<span class="visually-hidden">搜索文件 / Search files</span>
        <input data-input="file-search" type="search" placeholder="搜索文件名" value="${escapeHTML(state.ui.fileSearch)}" />
      </label>
      <label class="select-shell"><span class="visually-hidden">文件范围 / File scope</span>
        <select data-input="file-scope">
          ${option("all", "全部文件", state.ui.fileScopeFilter)}${option("shared", "公共文件", state.ui.fileScopeFilter)}
          ${option("direct", "定向文件", state.ui.fileScopeFilter)}${option("mine", "我上传的", state.ui.fileScopeFilter)}
          ${option("sent-to-me", "发给我的", state.ui.fileScopeFilter)}
        </select>${chevronIcon()}</label>
      <label class="select-shell"><span class="visually-hidden">文件排序 / File sorting</span>
        <select data-input="file-sort">
          ${option("newest", "最新上传", state.ui.fileSort)}${option("oldest", "最早上传", state.ui.fileSort)}
          ${option("size-asc", "大小升序", state.ui.fileSort)}${option("size-desc", "大小降序", state.ui.fileSort)}
        </select>${chevronIcon()}</label>
      <span class="file-result-count">${translate(state.ui.language, "filesCount", { count: visibleFiles.length })}</span>
    </div>
    <div class="drop-zone" data-drop-zone>
      <div class="file-table" role="table" aria-label="当前房间文件 / Current room files">
        ${renderTableHeader()}
        ${visibleFiles.length ? visibleFiles.map((file) => renderFileRow(file, state, user)).join("") : renderEmptyRow("没有符合条件的文件")}
      </div>
      <div class="drop-overlay" aria-hidden="true"><strong>释放文件到待发送区</strong><span>松手后仍需确认范围和接收者</span></div>
    </div>
    <div class="file-list-footer"><span>已显示 ${visibleFiles.length} 个文件</span><span>拖放文件不会立即上传</span></div>`;
}

function renderTimeline(state, user) {
  const events = projectEventsForUser(state.events, state.files, user, state.room)
    .sort((left, right) => Date.parse(right.occurredAt) - Date.parse(left.occurredAt));
  if (!events.length) return renderEmptyState("暂无文件动态", "上传、下载和回收操作会出现在这里。");
  return `<div class="timeline-toolbar"><strong>文件动态</strong><span>${events.length} 条可见记录 · 最新优先</span></div>
    <ol class="file-timeline">${events.map((event) => renderTimelineEvent(event, state)).join("")}</ol>
    <button class="button timeline-more" type="button">加载更早记录</button>`;
}

function renderTimelineEvent(event, state) {
  const actor = state.users.find((user) => user.id === event.actorId)?.displayName ?? "未知用户";
  const file = event.file;
  const name = file.visibility === "anonymous" ? `私密文件#${file.alias}` : file.name;
  const labels = {
    upload_started: "开始上传", upload_completed: "完成上传", upload_progress: "正在上传", upload_failed: "上传失败",
    direct_sent: "定向发送", resent: "从历史文件再次发送", download_started: "开始接收", download_completed: "传输完成",
    recycled: "移入回收站", restore_requested: "申请恢复", restored: "恢复文件", restore_approved: "批准恢复",
    permanently_deleted: "永久删除", published_shared: "发布到共享目录",
  };
  const progress = event.progress ?? (file.status === "uploading" ? file.uploadProgress : null);
  return `<li class="timeline-item${file.visibility === "anonymous" ? " timeline-item--audit" : ""}">
    <time>${formatTimestamp(event.occurredAt, state.ui.language)}</time>
    <span class="timeline-dot" aria-hidden="true"></span>
    <article class="timeline-card"><div><strong>${escapeHTML(actor)} ${labels[event.type] ?? event.type}</strong><span class="scope-tag scope-tag--${file.visibility === "anonymous" ? "private" : file.scope}">${file.visibility === "anonymous" ? "匿名审计" : file.scope === "shared" ? "公共" : "定向"}</span></div>
      <p>${escapeHTML(name ?? "-")} · ${formatBytes(file.sizeBytes)}</p>
      ${progress !== null && progress < 100 ? `<div class="timeline-progress"><span style="width:${progress}%"></span></div><small>${progress}%</small>` : ""}
    </article></li>`;
}

function renderRecycleBin(state, user) {
  const files = state.ui.scenario === "empty-recycle" ? [] : projectFilesForUser(state.files, user, state.room).filter((file) => file.status === "recycled");
  const capacity = calculateCapacity({ files: state.files, roomCapacityBytes: state.room.capacityBytes, recycleConfig: state.recycleConfig });
  return `<div class="recycle-summary"><div><strong>回收站</strong><span>文件保留至房间销毁</span></div><div><span>实际大小 ${formatBytes(capacity.recycledBytes)}</span><span>计费 ${formatBytes(capacity.recycledChargedBytes)}</span></div></div>
    <div class="file-table" role="table" aria-label="回收站文件 / Recycled files">
      ${renderTableHeader()}
      ${files.length ? files.map((file) => renderRecycleRow(file, state, user)).join("") : renderEmptyRow("回收站为空")}
    </div>`;
}

function renderFileRow(file, state, user) {
  const permissions = getFilePermissions(file, user, state.room);
  return renderCommonRow(file, state, permissions, renderFileActions(file, permissions));
}

function renderRecycleRow(file, state, user) {
  const permissions = getFilePermissions(file, user, state.room);
  let action = "";
  if (permissions.canRestore) action += `<button class="mini-button mini-button--primary" data-action="restore" data-file-id="${escapeHTML(file.id)}" type="button">恢复</button>`;
  if (permissions.canRequestRestore) action += `<button class="mini-button" data-action="request-restore" data-file-id="${escapeHTML(file.id)}" type="button">${file.restoreRequested ? "已申请" : "申请恢复"}</button>`;
  if (file.restoreRequested && user.id === state.room.ownerId) action += `<button class="mini-button mini-button--primary" data-action="approve-restore" data-file-id="${escapeHTML(file.id)}" type="button">批准恢复</button>`;
  if (permissions.canPermanentDelete) action += `<button class="mini-button mini-button--danger" data-action="permanent-delete" data-file-id="${escapeHTML(file.id)}" type="button">永久删除</button>`;
  return renderCommonRow(file, state, permissions, action || "—");
}

function renderCommonRow(file, state, permissions, actions) {
  const locale = state.ui.language;
  const uploader = state.users.find((item) => item.id === file.uploaderId);
  const recipients = file.recipientIds?.map((id) => state.users.find((item) => item.id === id)?.displayName).filter(Boolean).join("、");
  const anonymous = file.visibility === "anonymous";
  const displayName = anonymous ? translate(locale, "privateFile", { alias: file.alias }) : file.name;
  const secondary = anonymous ? "房主匿名审计视图" : file.scope === "direct" ? translate(locale, "privateFile", { alias: file.alias }) : file.mimeLabel;
  const ownerLabel = anonymous ? `${uploader?.displayName ?? "-"} → ${recipients || "-"}` : uploader?.displayName ?? "-";
  return `<article class="file-row${anonymous ? " file-row--private-audit" : ""}" role="row" data-file-id="${escapeHTML(file.id)}">
    <div class="file-cell file-cell--name" role="cell"><span class="file-icon file-icon--${escapeHTML(file.kind ?? "document")}">${fileIcon(file.kind)}</span><div><strong>${escapeHTML(displayName ?? "")}</strong><small>${escapeHTML(secondary ?? "")}</small></div></div>
    <div class="file-cell file-cell--scope" role="cell"><span class="scope-tag scope-tag--${anonymous ? "private" : file.scope}">${anonymous ? "匿名审计" : file.scope === "shared" ? "公共" : "定向"}</span></div>
    <div class="file-cell file-cell--owner" role="cell">${escapeHTML(ownerLabel)}</div>
    <div class="file-cell file-cell--size" role="cell">${formatBytes(file.sizeBytes)}</div>
    <div class="file-cell file-cell--time" role="cell">${formatTimestamp(file.recycledAt ?? file.createdAt, locale)}</div>
    <div class="file-cell file-cell--status" role="cell">${renderStatus(file, locale)}</div>
    <div class="file-cell file-cell--actions" role="cell">${actions}</div></article>`;
}

function renderFileActions(file, permissions) {
  if (file.status === "uploading") return `<button class="mini-button" type="button" data-action="show-tasks">查看任务</button>`;
  if (file.visibility === "anonymous") return permissions.canRecycle ? `<button class="mini-button mini-button--danger" data-action="recycle" data-file-id="${file.id}" type="button">移入回收站</button>` : "";
  let actions = "";
  if (permissions.canAccept || permissions.canDownload) actions += `<button class="mini-button mini-button--primary" data-action="download" data-file-id="${file.id}" type="button">${permissions.canAccept ? "接受并下载" : "下载"}</button>`;
  if (permissions.canResend) actions += `<button class="mini-button" data-action="reuse-file" data-file-id="${file.id}" type="button">再次发送</button>`;
  if (permissions.canPublishShared) actions += `<button class="mini-button" data-action="publish-shared" data-file-id="${file.id}" type="button">公开</button>`;
  if (permissions.canRecycle) actions += `<button class="mini-button mini-button--danger" data-action="recycle" data-file-id="${file.id}" type="button">回收</button>`;
  return actions;
}

function renderStatus(file, locale) {
  if (file.status === "uploading") {
    const progress = Math.max(0, Math.min(100, file.uploadProgress ?? 0));
    return `<div class="row-progress" role="progressbar" aria-valuenow="${progress}" aria-valuemin="0" aria-valuemax="100"><span style="width:${progress}%"></span></div><small>${progress}%</small>`;
  }
  if (file.status === "recycled") return `<span class="file-status">回收站</span>`;
  if (file.visibility === "anonymous") return `<span class="file-status">${translate(locale, "transferComplete")}</span>`;
  const accepted = Object.values(file.receiverStates ?? {}).filter((value) => value === "accepted" || value === "downloaded").length;
  if (file.scope === "direct" && accepted > 0) return `<span class="file-status file-status--accepted">${accepted} 人已接收</span>`;
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
  if (composer.existingFileId === "choose") {
    const reusable = projectFilesForUser(state.files, user, state.room).filter((file) => file.scope === "direct" && file.uploaderId === user.id && file.status === "available");
    root.innerHTML = `<div class="modal-backdrop"><section class="prototype-modal" role="dialog" aria-modal="true" aria-labelledby="existing-title"><header><div><p class="eyebrow">历史复用</p><h2 id="existing-title">选择已上传文件</h2></div><button class="icon-button" data-action="close-composer" aria-label="关闭" type="button">×</button></header><div class="existing-list">${reusable.length ? reusable.map((file) => `<button data-action="choose-existing" data-file-id="${file.id}" type="button"><strong>${escapeHTML(file.name)}</strong><span>私密文件#${file.alias} · ${formatBytes(file.sizeBytes)}</span></button>`).join("") : "<p>没有可复用的定向文件</p>"}</div></section></div>`;
    return;
  }
  const pending = composer.pendingFiles ?? [];
  const existing = composer.existingFileId ? state.files.find((file) => file.id === composer.existingFileId) : null;
  const total = existing?.sizeBytes ?? pending.reduce((sum, file) => sum + file.sizeBytes, 0);
  const recipients = state.users.filter((member) => member.id !== user.id);
  const submitDisabled = (!existing && pending.length === 0) || (composer.mode === "direct" && state.ui.selectedRecipientIds.length === 0);
  root.innerHTML = `<div class="modal-backdrop"><section class="prototype-modal send-modal" role="dialog" aria-modal="true" aria-labelledby="send-title">
    <header><div><p class="eyebrow">发送前确认</p><h2 id="send-title">待发送区</h2></div><button class="icon-button" data-action="close-composer" aria-label="关闭" type="button">×</button></header>
    <div class="send-mode" role="radiogroup" aria-label="发送范围"><button class="${composer.mode === "shared" ? "is-active" : ""}" data-action="set-send-mode" data-mode="shared" type="button">共享到房间</button><button class="${composer.mode === "direct" ? "is-active" : ""}" data-action="set-send-mode" data-mode="direct" type="button">定向发送</button></div>
    <div class="pending-list">${existing ? `<article><div><strong>${escapeHTML(existing.name)}</strong><span>复用历史文件，不重复占用容量</span></div><b>${formatBytes(existing.sizeBytes)}</b></article>` : pending.map((file) => `<article><div><strong>${escapeHTML(file.name)}</strong><span>${escapeHTML(file.type || "未知类型")}</span></div><b>${formatBytes(file.sizeBytes)}</b><button class="icon-button icon-button--small" data-action="remove-pending" data-pending-id="${file.id}" aria-label="移除 ${escapeHTML(file.name)}" type="button">×</button></article>`).join("") || `<div class="composer-drop-empty">拖放文件到页面，或点击“添加文件”</div>`}</div>
    ${composer.mode === "direct" ? `<div class="recipient-heading"><strong>选择接收者</strong><button class="button button--text" data-action="select-all-recipients" type="button">全选在线成员</button></div><div class="recipient-grid">${recipients.map((member) => { const already = existing?.recipientIds.includes(member.id); const selected = state.ui.selectedRecipientIds.includes(member.id); return `<button class="recipient-chip${selected ? " is-selected" : ""}" data-action="toggle-recipient" data-user-id="${member.id}" type="button" ${already ? "disabled" : ""}><span class="presence presence--${member.presence}"></span>${escapeHTML(member.displayName)}${already ? " · 已发送" : ""}</button>`; }).join("")}</div>` : `<div class="shared-warning">房间当前及后加入成员都能查看和下载这些文件。</div>`}
    <footer><div><strong>${existing ? 1 : pending.length} 个文件 · ${formatBytes(total)}</strong><span>${existing ? "服务端直接发送" : "确认后开始模拟上传"}</span></div>${existing ? "" : `<button class="button" data-action="add-files" type="button">添加文件</button>`}<button class="button button--primary" data-action="confirm-send" type="button" ${submitDisabled ? "disabled" : ""}>${existing ? "发送给接收者" : "开始上传并发送"}</button></footer>
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
  const title = active.length ? `${active.length} 个传输任务` : state.tasks.length ? "传输已完成" : "暂无传输任务";
  bar.innerHTML = `<div class="transfer-bar__summary"><span class="transfer-indicator${active.length ? "" : " is-idle"}" aria-hidden="true"></span><strong>${title}</strong><span>${shown.map((task) => `${task.type === "upload" ? "上传" : "接收"} ${task.progress}%`).join(" · ")}</span></div><div class="transfer-progress" role="progressbar" aria-valuenow="${progress}" aria-valuemin="0" aria-valuemax="100"><span style="width:${progress}%"></span></div><button class="button button--text" data-action="show-tasks" type="button">查看任务</button>`;
}

function filterAndSort(files, state, user) {
  const query = state.ui.fileSearch.trim().toLocaleLowerCase();
  return files.filter((file) => {
    const searchable = file.visibility === "anonymous" ? file.alias : `${file.name ?? ""} ${file.alias ?? ""}`;
    if (query && !searchable.toLocaleLowerCase().includes(query)) return false;
    const scope = state.ui.fileScopeFilter;
    if (scope === "mine") return file.uploaderId === user.id;
    if (scope === "sent-to-me") return file.recipientIds?.includes(user.id);
    return scope === "all" || file.scope === scope;
  }).sort((left, right) => sortFiles(left, right, state.ui.fileSort));
}

function sortFiles(left, right, sort) {
  if (sort === "oldest") return Date.parse(left.createdAt) - Date.parse(right.createdAt);
  if (sort === "size-asc") return left.sizeBytes - right.sizeBytes;
  if (sort === "size-desc") return right.sizeBytes - left.sizeBytes;
  return Date.parse(right.createdAt) - Date.parse(left.createdAt);
}

function renderTableHeader() { return `<div class="file-table__header" role="row"><span role="columnheader">文件名</span><span role="columnheader">范围</span><span role="columnheader">上传者</span><span role="columnheader">大小</span><span role="columnheader">时间</span><span role="columnheader">状态</span><span role="columnheader">操作</span></div>`; }
function renderEmptyRow(text) { return `<div class="table-empty">${escapeHTML(text)}</div>`; }
function renderEmptyState(title, text) { return `<div class="content-placeholder"><h3>${escapeHTML(title)}</h3><p>${escapeHTML(text)}</p></div>`; }
function option(value, label, selected) { return `<option value="${value}"${value === selected ? " selected" : ""}>${label}</option>`; }
function searchIcon() { return `<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/></svg>`; }
function chevronIcon() { return `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m7 10 5 5 5-5"/></svg>`; }
function isConversation(message, firstId, secondId) { return (message.fromId === firstId && message.toId === secondId) || (message.fromId === secondId && message.toId === firstId); }
function presenceLabel(value) { return ({ online: "在线", away: "暂离", offline: "离线" })[value] ?? value; }
function messageSummary(message, state) {
  if (!message) return "暂无消息";
  if (message.status === "recalled") return "消息已撤回";
  if (message.type === "image") return "[图片]";
  if (message.type === "direct_file") return "发来一个私密文件";
  if (message.type === "shared_reference") return `引用：${state.files.find((file) => file.id === message.fileId)?.name ?? "共享文件"}`;
  return message.text ?? "";
}

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
