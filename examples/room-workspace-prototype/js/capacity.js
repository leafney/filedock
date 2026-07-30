export function calculateCapacity({ files, roomCapacityBytes, recycleConfig }) {
  let sharedBytes = 0;
  let directBytes = 0;
  let reservedBytes = 0;
  let recycledBytes = 0;

  for (const file of files) {
    const size = Math.max(0, Number(file.sizeBytes) || 0);
    if (file.status === "uploading") {
      reservedBytes += size;
      continue;
    }
    if (file.status === "recycled") {
      recycledBytes += size;
      continue;
    }
    if (file.status !== "available") {
      continue;
    }
    if (file.scope === "shared") {
      sharedBytes += size;
    } else if (file.scope === "direct") {
      directBytes += size;
    }
  }

  const freeRecycleBytes = recycleConfig.countTowardRoomCapacity
    ? 0
    : Math.max(0, Number(recycleConfig.freeBytes) || 0);
  const recycledChargedBytes = recycleConfig.countTowardRoomCapacity
    ? recycledBytes
    : Math.max(0, recycledBytes - freeRecycleBytes);
  const occupiedBytes = sharedBytes + directBytes + reservedBytes + recycledChargedBytes;
  const normalizedCapacity = Math.max(0, Number(roomCapacityBytes) || 0);

  return {
    sharedBytes,
    directBytes,
    reservedBytes,
    recycledBytes,
    freeRecycleBytes,
    recycledChargedBytes,
    occupiedBytes,
    availableBytes: Math.max(0, normalizedCapacity - occupiedBytes),
    capacityBytes: normalizedCapacity,
    usageRatio: normalizedCapacity === 0 ? 0 : occupiedBytes / normalizedCapacity,
    overCapacity: occupiedBytes > normalizedCapacity,
  };
}

export function canRestoreFile({ fileId, files, roomCapacityBytes, recycleConfig }) {
  const target = files.find((file) => file.id === fileId);
  if (!target || target.status !== "recycled") {
    return { allowed: false, reason: "not_recycled", capacity: calculateCapacity({ files, roomCapacityBytes, recycleConfig }) };
  }

  const projectedFiles = files.map((file) =>
    file.id === fileId ? { ...file, status: "available" } : file,
  );
  const capacity = calculateCapacity({ files: projectedFiles, roomCapacityBytes, recycleConfig });
  return {
    allowed: !capacity.overCapacity,
    reason: capacity.overCapacity ? "capacity_exceeded" : null,
    capacity,
  };
}
