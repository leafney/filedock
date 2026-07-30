export const RADAR_NODE_COUNT = 5;
const MIN_LIFETIME_MS = 10_000;
const MAX_LIFETIME_MS = 30_000;
const MIN_DISTANCE = 14;

export interface RadarNode {
  id: string;
  roomCode: string;
  color: string;
  left: number;
  top: number;
  createdAt: number;
  expiresAt: number;
  phase: "entering" | "visible" | "exiting";
}

const colors = ["#0f766e", "#2563eb", "#7c3aed", "#c2410c", "#be185d", "#4f46e5"] as const;

function randomCode(random: () => number, usedCodes: Set<string>) {
  let code = "";
  do code = String(Math.floor(random() * 9999) + 1).padStart(4, "0"); while (usedCodes.has(code));
  return code;
}

function distance(left: number, top: number, other: Pick<RadarNode, "left" | "top">) {
  return Math.hypot(left - other.left, top - other.top);
}

function isSafePosition(left: number, top: number, nodes: RadarNode[]) {
  const centerDistance = Math.hypot((left - 50) / 1.1, (top - 50) / 0.95);
  if (centerDistance < 20) return false;
  if (left < 8 || left > 92 || top < 11 || top > 88) return false;
  return nodes.every((node) => distance(left, top, node) >= MIN_DISTANCE);
}

export function randomRadarPosition(nodes: RadarNode[], random = Math.random) {
  let best = { left: 10 + random() * 80, top: 14 + random() * 70, score: -Infinity };
  for (let attempt = 0; attempt < 48; attempt += 1) {
    const left = 8 + random() * 84;
    const top = 11 + random() * 77;
    const nearest = nodes.length ? Math.min(...nodes.map((node) => distance(left, top, node))) : 100;
    const centerDistance = Math.hypot((left - 50) / 1.1, (top - 50) / 0.95);
    const score = Math.min(nearest, centerDistance);
    if (score > best.score) best = { left, top, score };
    if (isSafePosition(left, top, nodes)) return { left, top };
  }
  return { left: best.left, top: best.top };
}

export function createRadarNode(now = Date.now(), nodes: RadarNode[] = [], random = Math.random): RadarNode {
  const position = randomRadarPosition(nodes, random);
  const usedCodes = new Set(nodes.map((node) => node.roomCode));
  const lifetime = MIN_LIFETIME_MS + random() * (MAX_LIFETIME_MS - MIN_LIFETIME_MS);
  return {
    id: `${now}-${Math.floor(random() * 1_000_000)}`,
    roomCode: randomCode(random, usedCodes),
    color: colors[Math.floor(random() * colors.length)],
    left: position.left,
    top: position.top,
    createdAt: now,
    expiresAt: now + lifetime,
    phase: "entering",
  };
}

export function createRadarNodes(now = Date.now(), random = Math.random) {
  const nodes: RadarNode[] = [];
  for (let index = 0; index < RADAR_NODE_COUNT; index += 1) nodes.push(createRadarNode(now + index, nodes, random));
  return nodes;
}
