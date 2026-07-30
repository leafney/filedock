const FILE_GROUP_ORDER = ["shared", "direct"];

export function deriveFileGroups({
  files,
  search = "",
  scopeView = "all",
  identityFilter = "all",
  sort = "newest",
  userId,
}) {
  const query = search.trim().toLocaleLowerCase();
  const filtered = files.filter((file) => {
    if (scopeView !== "all" && file.scope !== scopeView) return false;
    if (identityFilter === "mine" && file.uploaderId !== userId) return false;
    if (identityFilter === "sent-to-me" && !(file.scope === "direct" && file.recipientIds?.includes(userId))) return false;
    return matchesSearch(file, query);
  });

  const groups = {
    shared: filtered.filter((file) => file.scope === "shared").sort((left, right) => compareFiles(left, right, sort)),
    direct: filtered.filter((file) => file.scope === "direct").sort((left, right) => compareFiles(left, right, sort)),
  };

  return {
    shared: groups.shared,
    direct: groups.direct,
    orderedGroups: FILE_GROUP_ORDER
      .filter((scope) => scopeView === "all" || scope === scopeView)
      .map((scope) => ({ scope, files: groups[scope], count: groups[scope].length })),
    totalCount: groups.shared.length + groups.direct.length,
  };
}

export function getDefaultComposerMode(scopeView) {
  return scopeView === "direct" ? "direct" : "shared";
}

export function deriveFileActions(file, user, room) {
  if (!file || !user || !room || !file.permissions?.visible) {
    return { primaryActions: [], menuActions: [] };
  }

  const permissions = file.permissions;
  if (permissions.canAccept || permissions.canDecline) {
    return {
      primaryActions: [
        ...(permissions.canAccept ? [action("accept-download", "acceptDownload", "primary")] : []),
        ...(permissions.canDecline ? [action("decline", "decline", "danger-secondary")] : []),
      ],
      menuActions: [],
    };
  }

  const primaryActions = file.status === "uploading"
    ? [action("view-task", "viewTask", "primary")]
    : permissions.canDownload
      ? [action("download", "download", "primary")]
      : [];

  const menuActions = [];
  if (file.visibility === "anonymous") {
    menuActions.push(action("view-anonymous-details", "viewAnonymousDetails"));
  } else {
    menuActions.push(action("view-details", "viewFileDetails"));
    if (permissions.canResend) menuActions.push(action("resend", "resend"));
    if (permissions.canPublishShared) menuActions.push(action("publish-shared", "publishShared"));
    if (file.scope === "direct" && file.uploaderId === user.id) {
      menuActions.push(action("receiver-status", "receiverStatus"));
    }
  }
  if (permissions.canRecycle) menuActions.push(action("recycle", "recycle", "danger"));
  if (permissions.canPermanentDelete && !permissions.canRecycle) {
    menuActions.push(action("permanent-delete", "permanentDelete", "danger"));
  }

  return { primaryActions, menuActions };
}

export function deriveBatchCapabilities(files, user, room) {
  if (!files.length) {
    return {
      download: capability(false, "no_selection"),
      privateSend: capability(false, "no_selection"),
    };
  }

  const allDownloadable = files.every((file) => file.permissions?.canDownload);
  const allReusablePrivate = files.every((file) => (
    file.scope === "direct"
    && file.uploaderId === user?.id
    && file.permissions?.canResend
  ));

  return {
    download: capability(allDownloadable, allDownloadable ? null : "contains_unavailable_file"),
    privateSend: capability(allReusablePrivate, allReusablePrivate ? null : "contains_non_reusable_private_file"),
  };
}

function matchesSearch(file, query) {
  if (!query) return true;
  if (file.visibility === "anonymous") {
    return String(file.alias ?? "").toLocaleLowerCase().includes(query);
  }
  return [file.name, file.alias, file.mimeLabel]
    .filter(Boolean)
    .some((value) => String(value).toLocaleLowerCase().includes(query));
}

function compareFiles(left, right, sort) {
  if (sort === "oldest") return Date.parse(left.createdAt) - Date.parse(right.createdAt);
  if (sort === "size-asc") return left.sizeBytes - right.sizeBytes;
  if (sort === "size-desc") return right.sizeBytes - left.sizeBytes;
  if (sort === "name") return safeName(left).localeCompare(safeName(right));
  return Date.parse(right.createdAt) - Date.parse(left.createdAt);
}

function safeName(file) {
  return file.visibility === "anonymous" ? file.alias ?? "" : file.name ?? file.alias ?? "";
}

function action(id, labelKey, tone = "default") {
  return { id, labelKey, tone };
}

function capability(enabled, reason) {
  return { enabled, reason };
}
