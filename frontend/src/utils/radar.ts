export const RADAR_NODE_COUNT = 5;
export const RADAR_NODE_MIN_DIAMETER = 40;
export const RADAR_NODE_MAX_DIAMETER = 88;

const MIN_LIFETIME_MS = 10_000;
const MAX_LIFETIME_MS = 30_000;
const POSITION_ATTEMPTS = 240;
const COLOR_ATTEMPTS = 48;
const MIN_HUE_DISTANCE = 38;
const EDGE_GAP = 14;
const NODE_GAP = 10;
const LABEL_WIDTH = 72;
const LABEL_HEIGHT = 25;
const LABEL_GAP = 6;
const CENTER_DIAMETER = 112;

let nodeSequence = 0;

export interface RadarBounds {
  width: number;
  height: number;
}

export interface RadarNode {
  id: string;
  slot: number;
  roomCode: string;
  color: string;
  hue: number;
  diameter: number;
  left: number;
  top: number;
  createdAt: number;
  expiresAt: number;
  phase: "entering" | "visible" | "exiting";
}

export const DEFAULT_RADAR_BOUNDS: RadarBounds = { width: 900, height: 560 };

function randomBetween(min: number, max: number, random: () => number) {
  return min + random() * (max - min);
}

function randomInteger(min: number, max: number, random: () => number) {
  return Math.floor(random() * (max - min + 1)) + min;
}

function randomCode(random: () => number, usedCodes: Set<string>) {
  let code = "";
  do code = String(randomInteger(1, 9999, random)).padStart(4, "0"); while (usedCodes.has(code));
  return code;
}

function hueDistance(left: number, right: number) {
  const direct = Math.abs(left - right);
  return Math.min(direct, 360 - direct);
}

function createRadarColor(nodes: RadarNode[], random: () => number) {
  const existingHues = nodes.map((node) => node.hue);
  let best = { hue: randomInteger(0, 359, random), score: -1 };

  for (let attempt = 0; attempt < COLOR_ATTEMPTS; attempt += 1) {
    const hue = randomInteger(0, 359, random);
    const score = existingHues.length
      ? Math.min(...existingHues.map((existing) => hueDistance(hue, existing)))
      : 180;
    if (score > best.score) best = { hue, score };
    if (score >= MIN_HUE_DISTANCE) break;
  }

  const saturation = randomInteger(55, 75, random);
  const lightness = randomInteger(38, 52, random);
  return { hue: best.hue, color: `hsl(${best.hue} ${saturation}% ${lightness}%)` };
}

function nodeRect(left: number, top: number, diameter: number) {
  const radius = diameter / 2;
  const halfWidth = Math.max(radius, LABEL_WIDTH / 2);
  return {
    left: left - halfWidth,
    right: left + halfWidth,
    top: top - radius,
    bottom: top + radius + LABEL_GAP + LABEL_HEIGHT,
  };
}

function rectangleClearance(left: ReturnType<typeof nodeRect>, right: ReturnType<typeof nodeRect>) {
  const horizontal = Math.max(right.left - left.right, left.left - right.right);
  const vertical = Math.max(right.top - left.bottom, left.top - right.bottom);
  if (horizontal >= 0 || vertical >= 0) return Math.max(horizontal, vertical);
  return Math.max(horizontal, vertical);
}

function positionScore(left: number, top: number, diameter: number, nodes: RadarNode[], bounds: RadarBounds) {
  const rect = nodeRect(left, top, diameter);
  const boundaryClearance = Math.min(
    rect.left - EDGE_GAP,
    bounds.width - EDGE_GAP - rect.right,
    rect.top - EDGE_GAP,
    bounds.height - EDGE_GAP - rect.bottom,
  );

  const radius = diameter / 2;
  const centerX = bounds.width / 2;
  const centerY = bounds.height / 2;
  const centerRadiusX = CENTER_DIAMETER / 2 + radius + 42;
  const centerRadiusY = CENTER_DIAMETER / 2 + radius + 68;
  const normalizedCenterDistance = Math.hypot(
    (left - centerX) / centerRadiusX,
    (top - centerY) / centerRadiusY,
  );
  const centerClearance = (normalizedCenterDistance - 1) * Math.min(centerRadiusX, centerRadiusY);

  const nodeClearance = nodes.length
    ? Math.min(...nodes.map((node) => {
      const circleClearance = Math.hypot(left - node.left, top - node.top)
        - (radius + node.diameter / 2 + NODE_GAP);
      const rectClearance = rectangleClearance(rect, nodeRect(node.left, node.top, node.diameter)) - NODE_GAP;
      return Math.min(circleClearance, rectClearance);
    }))
    : Number.POSITIVE_INFINITY;

  return Math.min(boundaryClearance, centerClearance, nodeClearance);
}

export function randomRadarPosition(
  diameter: number,
  nodes: RadarNode[],
  bounds: RadarBounds,
  random = Math.random,
) {
  const radius = diameter / 2;
  const halfWidth = Math.max(radius, LABEL_WIDTH / 2);
  const minLeft = EDGE_GAP + halfWidth;
  const maxLeft = Math.max(minLeft, bounds.width - EDGE_GAP - halfWidth);
  const minTop = EDGE_GAP + radius;
  const maxTop = Math.max(minTop, bounds.height - EDGE_GAP - radius - LABEL_GAP - LABEL_HEIGHT);
  let best = { left: bounds.width / 2, top: bounds.height / 2, score: Number.NEGATIVE_INFINITY };

  for (let attempt = 0; attempt < POSITION_ATTEMPTS; attempt += 1) {
    const left = randomBetween(minLeft, maxLeft, random);
    const top = randomBetween(minTop, maxTop, random);
    const score = positionScore(left, top, diameter, nodes, bounds);
    if (score > best.score) best = { left, top, score };
    if (score >= 0) return { left, top };
  }

  return { left: best.left, top: best.top };
}

export function createRadarNode(
  now = Date.now(),
  nodes: RadarNode[] = [],
  bounds: RadarBounds = DEFAULT_RADAR_BOUNDS,
  random = Math.random,
  slot = nodes.length,
): RadarNode {
  const diameter = randomInteger(RADAR_NODE_MIN_DIAMETER, RADAR_NODE_MAX_DIAMETER, random);
  const position = randomRadarPosition(diameter, nodes, bounds, random);
  const usedCodes = new Set(nodes.map((node) => node.roomCode));
  const lifetime = randomBetween(MIN_LIFETIME_MS, MAX_LIFETIME_MS, random);
  const { hue, color } = createRadarColor(nodes, random);
  nodeSequence += 1;
  return {
    id: `radar-${nodeSequence}-${now}`,
    slot,
    roomCode: randomCode(random, usedCodes),
    color,
    hue,
    diameter,
    left: position.left,
    top: position.top,
    createdAt: now,
    expiresAt: now + lifetime,
    phase: "entering",
  };
}

export function createRadarNodes(
  bounds: RadarBounds = DEFAULT_RADAR_BOUNDS,
  now = Date.now(),
  random = Math.random,
) {
  const nodes: RadarNode[] = [];
  for (let slot = 0; slot < RADAR_NODE_COUNT; slot += 1) {
    nodes.push(createRadarNode(now + slot, nodes, bounds, random, slot));
  }
  return nodes;
}

export function replaceRadarNode(
  nodes: RadarNode[],
  index: number,
  expectedId: string,
  bounds: RadarBounds,
  now = Date.now(),
  random = Math.random,
) {
  if (nodes.length !== RADAR_NODE_COUNT || nodes[index]?.id !== expectedId) return nodes;
  const neighbors = nodes.filter((_, nodeIndex) => nodeIndex !== index);
  const replacement = createRadarNode(now, neighbors, bounds, random, nodes[index].slot);
  return nodes.map((node, nodeIndex) => nodeIndex === index ? replacement : node);
}

export function relayoutRadarNodes(nodes: RadarNode[], bounds: RadarBounds, random = Math.random) {
  if (nodes.length !== RADAR_NODE_COUNT) return nodes;
  const positioned: RadarNode[] = [];
  for (const node of nodes) {
    const position = randomRadarPosition(node.diameter, positioned, bounds, random);
    positioned.push({ ...node, ...position });
  }
  return positioned;
}
