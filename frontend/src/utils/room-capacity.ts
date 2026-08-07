import type { RoomCapacity, RoomSnapshot } from "../types/domain";

export interface RoomCapacitySummary {
  occupiedBytes: number;
  percent: number;
}

export function roomCapacitySummary(capacity: RoomCapacity, role: RoomSnapshot["role"]): RoomCapacitySummary {
  const usedBytes = Math.max(0, capacity.usedBytes);
  const reservedBytes = role === "owner" ? Math.max(0, capacity.reservedBytes ?? 0) : 0;
  const occupiedBytes = usedBytes + reservedBytes;
  const percent = capacity.capacityBytes > 0 ? Math.min(100, occupiedBytes * 100 / capacity.capacityBytes) : 0;
  return { occupiedBytes, percent };
}
