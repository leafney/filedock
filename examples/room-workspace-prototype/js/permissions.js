export function isRoomOwner(user, room) {
  return Boolean(user && room && user.id === room.ownerId);
}

export function isFileParticipant(file, userId) {
  return Boolean(
    file &&
      userId &&
      (file.uploaderId === userId || file.recipientIds?.includes(userId)),
  );
}

export function getFilePermissions(file, user, room) {
  if (!file || !user || !room) {
    return emptyPermissions();
  }

  const owner = isRoomOwner(user, room);
  const uploader = file.uploaderId === user.id;
  const recipient = file.recipientIds?.includes(user.id) ?? false;
  const participant = uploader || recipient;
  const fullVisibility = file.scope === "shared" || participant;
  const anonymousVisibility = file.scope === "direct" && owner && !participant;
  const active = file.status === "available";
  const recipientState = file.receiverStates?.[user.id];
  const recycled = file.status === "recycled";
  const forcedByOwner = file.removalKind === "owner_forced";

  return {
    fullVisibility,
    anonymousVisibility,
    visible: fullVisibility || anonymousVisibility,
    canDownload: active && (file.scope === "shared" || (participant && (!recipient || recipientState !== "declined"))),
    canAccept: active && file.scope === "direct" && recipient && file.receiverStates?.[user.id] === "pending",
    canDecline: active && file.scope === "direct" && recipient && file.receiverStates?.[user.id] === "pending",
    canResend: active && file.scope === "direct" && uploader,
    canPublishShared: active && file.scope === "direct" && uploader,
    canReference: active && file.scope === "shared",
    canRecycle: active && (uploader || owner),
    canPermanentDelete: (active || recycled) && (uploader || owner),
    canRestore: recycled && (owner || (uploader && !forcedByOwner)),
    canRequestRestore: recycled && uploader && forcedByOwner && !owner,
  };
}

export function projectFileForUser(file, user, room) {
  const permissions = getFilePermissions(file, user, room);
  if (!permissions.visible) {
    return null;
  }

  if (permissions.fullVisibility) {
    return { ...file, visibility: "full", permissions };
  }

  return {
    id: file.id,
    scope: file.scope,
    alias: file.alias,
    name: null,
    mimeLabel: null,
    kind: "private",
    sizeBytes: file.sizeBytes,
    uploaderId: file.uploaderId,
    recipientIds: [...(file.recipientIds ?? [])],
    receiverStates: null,
    status: file.status,
    uploadProgress: file.uploadProgress,
    createdAt: file.createdAt,
    recycledAt: file.recycledAt,
    removedById: file.removedById,
    removalKind: file.removalKind,
    visibility: "anonymous",
    permissions,
  };
}

export function projectFilesForUser(files, user, room) {
  return files
    .map((file) => projectFileForUser(file, user, room))
    .filter(Boolean);
}

export function projectEventsForUser(events, files, user, room) {
  const filesById = new Map(files.map((file) => [file.id, file]));
  return events.flatMap((event) => {
    const sourceFile = filesById.get(event.fileId);
    const file = projectFileForUser(sourceFile, user, room);
    return file ? [{ ...event, file }] : [];
  });
}

function emptyPermissions() {
  return {
    fullVisibility: false,
    anonymousVisibility: false,
    visible: false,
    canDownload: false,
    canAccept: false,
    canDecline: false,
    canResend: false,
    canPublishShared: false,
    canReference: false,
    canRecycle: false,
    canPermanentDelete: false,
    canRestore: false,
    canRequestRestore: false,
  };
}
