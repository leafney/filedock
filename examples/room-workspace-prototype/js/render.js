import { calculateCapacity } from "./capacity.js";
import { translate } from "./i18n.js";
import { getFilePermissions, projectFilesForUser } from "./permissions.js";

export function renderModel(state) {
  const user = state.users.find((item) => item.id === state.ui.currentUserId);
  if (!user) return;
  renderCapacity(state);
  renderFiles(state, user);
}

function renderCapacity(state) {
  const capacity = calculateCapacity({
    files: state.files,
    roomCapacityBytes: state.room.capacityBytes,
    recycleConfig: state.recycleConfig,
  });
  const label = document.querySelector(".capacity-summary dd");
  const fill = document.querySelector(".capacity-track span");
  if (label) label.textContent = `${formatBytes(capacity.occupiedBytes)} / ${formatBytes(capacity.capacityBytes)}`;
  if (fill) fill.style.width = `${Math.min(100, capacity.usageRatio * 100).toFixed(1)}%`;
}

function renderFiles(state, user) {
  const table = document.querySelector(".file-table");
  if (!table) return;
  table.querySelectorAll(".file-row").forEach((row) => row.remove());

  const visibleFiles = projectFilesForUser(state.files, user, state.room)
    .filter((file) => file.status === "available" || file.status === "uploading")
    .filter((file) => matchesSearch(file, state.ui.fileSearch))
    .filter((file) => matchesScope(file, state.ui.fileScopeFilter))
    .sort((left, right) => sortFiles(left, right, state.ui.fileSort));

  for (const file of visibleFiles) {
    table.insertAdjacentHTML("beforeend", renderFileRow(file, state, user));
  }

  const count = document.querySelector(".file-result-count");
  if (count) count.textContent = translate(state.ui.language, "filesCount", { count: visibleFiles.length });
}

function renderFileRow(file, state, user) {
  const locale = state.ui.language;
  const uploader = state.users.find((item) => item.id === file.uploaderId);
  const recipients = file.recipientIds
    ?.map((id) => state.users.find((item) => item.id === id)?.displayName)
    .filter(Boolean)
    .join("、");
  const anonymous = file.visibility === "anonymous";
  const displayName = anonymous
    ? translate(locale, "privateFile", { alias: file.alias })
    : file.name;
  const secondary = anonymous
    ? locale === "en" ? "Owner audit projection" : "房主匿名审计视图"
    : file.scope === "direct"
      ? translate(locale, "privateFile", { alias: file.alias })
      : file.mimeLabel;
  const scopeClass = anonymous ? "private" : file.scope;
  const scopeLabel = anonymous
    ? translate(locale, "privateAudit")
    : translate(locale, file.scope);
  const ownerLabel = anonymous ? `${uploader?.displayName ?? "-"} → ${recipients || "-"}` : uploader?.displayName ?? "-";
  const permissions = getFilePermissions(file, user, state.room);
  const status = renderStatus(file, locale);
  const action = renderPrimaryAction(file, permissions, locale);

  return `
    <article class="file-row${anonymous ? " file-row--private-audit" : ""}" role="row" data-file-id="${escapeHTML(file.id)}">
      <div class="file-cell file-cell--name" role="cell">
        <span class="file-icon file-icon--${escapeHTML(file.kind ?? "document")}">${fileIcon(file.kind)}</span>
        <div><strong>${escapeHTML(displayName ?? "")}</strong><small>${escapeHTML(secondary ?? "")}</small></div>
      </div>
      <div class="file-cell file-cell--scope" role="cell"><span class="scope-tag scope-tag--${scopeClass}">${escapeHTML(scopeLabel)}</span></div>
      <div class="file-cell file-cell--owner" role="cell">${escapeHTML(ownerLabel)}</div>
      <div class="file-cell file-cell--size" role="cell">${formatBytes(file.sizeBytes)}</div>
      <div class="file-cell file-cell--time" role="cell">${formatTimestamp(file.createdAt, locale)}</div>
      <div class="file-cell file-cell--status" role="cell">${status}</div>
      <div class="file-cell file-cell--actions" role="cell">${action}</div>
    </article>`;
}

function renderStatus(file, locale) {
  if (file.status === "uploading") {
    const progress = Math.max(0, Math.min(100, file.uploadProgress ?? 0));
    return `<div class="row-progress"><span style="width:${progress}%"></span></div><small>${progress}%</small>`;
  }
  if (file.visibility === "anonymous") return `<span class="file-status">${translate(locale, "transferComplete")}</span>`;
  const accepted = Object.values(file.receiverStates ?? {}).filter((value) => value === "accepted" || value === "downloaded").length;
  if (file.scope === "direct" && accepted > 0) {
    return `<span class="file-status file-status--accepted">${locale === "en" ? `${accepted} received` : `${accepted} 人已接收`}</span>`;
  }
  return `<span class="file-status file-status--ready">${translate(locale, "ready")}</span>`;
}

function renderPrimaryAction(file, permissions, locale) {
  if (file.status === "uploading") return `<button class="mini-button" type="button" disabled>${translate(locale, "viewTask")}</button>`;
  if (file.visibility === "anonymous") return `<button class="mini-button mini-button--danger" type="button" disabled>${translate(locale, "recycle")}</button>`;
  if (permissions.canAccept) return `<button class="mini-button mini-button--primary" type="button" disabled>${translate(locale, "acceptDownload")}</button>`;
  if (permissions.canResend) return `<button class="mini-button" type="button" disabled>${translate(locale, "resend")}</button>`;
  if (permissions.canDownload) return `<button class="mini-button mini-button--primary" type="button" disabled>${translate(locale, "download")}</button>`;
  return "";
}

function matchesSearch(file, search) {
  const query = search.trim().toLocaleLowerCase();
  if (!query) return true;
  const searchable = file.visibility === "anonymous" ? file.alias : `${file.name ?? ""} ${file.alias ?? ""}`;
  return searchable.toLocaleLowerCase().includes(query);
}

function matchesScope(file, scope) {
  if (scope === "all") return true;
  return file.scope === scope;
}

function sortFiles(left, right, sort) {
  if (sort === "oldest") return Date.parse(left.createdAt) - Date.parse(right.createdAt);
  if (sort === "size-asc") return left.sizeBytes - right.sizeBytes;
  if (sort === "size-desc") return right.sizeBytes - left.sizeBytes;
  return Date.parse(right.createdAt) - Date.parse(left.createdAt);
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
  return new Intl.DateTimeFormat(locale === "en" ? "en" : "zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function trim(value) {
  return Number.isInteger(value) ? String(value) : value.toFixed(1).replace(/\.0$/, "");
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

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
