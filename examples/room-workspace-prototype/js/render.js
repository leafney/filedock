import { calculateCapacity } from "./capacity.js";
import { translate } from "./i18n.js";
import { getFilePermissions, projectEventsForUser, projectFilesForUser } from "./permissions.js";

export function renderModel(state) {
  const user = state.users.find((item) => item.id === state.ui.currentUserId);
  if (!user) return;
  document.documentElement.lang = state.ui.language;
  renderCapacity(state);
  renderTabs(state, user);
  renderFileContent(state, user);
  renderComposer(state, user);
  renderTasks(state);
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
  const visibleFiles = filterAndSort(projectFilesForUser(state.files, user, state.room)
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
  const files = projectFilesForUser(state.files, user, state.room).filter((file) => file.status === "recycled");
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
