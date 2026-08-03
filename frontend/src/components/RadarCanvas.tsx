import { Avatar } from "antd";
import { useEffect, useRef, useState, type CSSProperties } from "react";

import { getAvatarInitial, getStableAvatarColor } from "../utils/avatar";
import {
  DEFAULT_RADAR_BOUNDS,
  RADAR_NODE_COUNT,
  createRadarNodes,
  relayoutRadarNodes,
  replaceRadarNode,
  type RadarBounds,
  type RadarNode,
} from "../utils/radar";

const REPLACEMENT_POLL_MS = 750;
const EXIT_ANIMATION_MS = 350;
const LABEL_WIDTH = 72;
const WAVE_BASE_DIAMETER = 132;
const WAVE_DIAGONAL_RATIO = 1.05;
const STATIC_WAVE_PROGRESS = [0.24, 0.58, 0.9] as const;
const BREATH_DURATIONS = [2.8, 3.15, 3.5, 3.85, 4.2] as const;
const BREATH_DELAYS = [-0.4, -1.7, -2.9, -0.9, -3.6] as const;

function getWaveScales(bounds: RadarBounds) {
  const endScale = Math.max(1, Math.hypot(bounds.width, bounds.height) * WAVE_DIAGONAL_RATIO / WAVE_BASE_DIAMETER);
  const staticScales = STATIC_WAVE_PROGRESS.map((progress) => 1 + (endScale - 1) * progress);
  return { endScale, staticScales };
}

function applyWaveScales(canvas: HTMLDivElement, bounds: RadarBounds) {
  const { endScale, staticScales } = getWaveScales(bounds);
  canvas.style.setProperty("--radar-wave-end-scale", endScale.toFixed(4));
  staticScales.forEach((scale, index) => {
    canvas.style.setProperty(`--radar-wave-static-${index + 1}-scale`, scale.toFixed(4));
  });
}

const DEFAULT_WAVE_STYLE = (() => {
  const { endScale, staticScales } = getWaveScales(DEFAULT_RADAR_BOUNDS);
  return {
    "--radar-wave-end-scale": endScale,
    "--radar-wave-static-1-scale": staticScales[0],
    "--radar-wave-static-2-scale": staticScales[1],
    "--radar-wave-static-3-scale": staticScales[2],
  } as CSSProperties;
})();

function useReducedMotion() {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    const media = window.matchMedia("(prefers-reduced-motion: reduce)");
    const update = () => setReduced(media.matches);
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  return reduced;
}

export function RadarCanvas({ displayName }: { displayName: string }) {
  const reducedMotion = useReducedMotion();
  const canvasRef = useRef<HTMLDivElement>(null);
  const boundsRef = useRef<RadarBounds>(DEFAULT_RADAR_BOUNDS);
  const [nodes, setNodes] = useState<RadarNode[]>(() => createRadarNodes(DEFAULT_RADAR_BOUNDS));
  const nodesRef = useRef(nodes);
  const replacementRef = useRef<{ index: number; id: string }>();
  const replacementTimerRef = useRef<number>();
  const generationRef = useRef(0);

  const commitNodes = (updater: (current: RadarNode[]) => RadarNode[]) => {
    setNodes((current) => {
      const next = updater(current);
      nodesRef.current = next;
      return next;
    });
  };

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const observer = new ResizeObserver(([entry]) => {
      const width = Math.round(entry.contentRect.width);
      const height = Math.round(entry.contentRect.height);
      if (width <= 0 || height <= 0) return;
      const bounds = { width, height };
      applyWaveScales(canvas, bounds);
      const current = boundsRef.current;
      if (Math.abs(current.width - width) < 2 && Math.abs(current.height - height) < 2) return;
      boundsRef.current = bounds;
      commitNodes((value) => relayoutRadarNodes(value, bounds));
    });
    observer.observe(canvas);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    const generation = generationRef.current + 1;
    generationRef.current = generation;

    const pollTimer = window.setInterval(() => {
      if (replacementRef.current) return;
      const current = nodesRef.current;
      if (current.length !== RADAR_NODE_COUNT) {
        console.error(`Radar invariant violated: expected ${RADAR_NODE_COUNT} nodes, received ${current.length}.`);
        return;
      }
      const expiredIndex = current.findIndex((node) => node.expiresAt <= Date.now());
      if (expiredIndex < 0) return;

      const target = { index: expiredIndex, id: current[expiredIndex].id };
      replacementRef.current = target;
      commitNodes((value) => {
        if (generationRef.current !== generation
          || value.length !== RADAR_NODE_COUNT
          || value[target.index]?.id !== target.id) return value;
        return value.map((node, index) => index === target.index ? { ...node, phase: "exiting" } : node);
      });

      replacementTimerRef.current = window.setTimeout(() => {
        replacementTimerRef.current = undefined;
        if (generationRef.current !== generation
          || replacementRef.current?.id !== target.id
          || replacementRef.current.index !== target.index) return;
        commitNodes((value) => replaceRadarNode(
          value,
          target.index,
          target.id,
          boundsRef.current,
        ));
        replacementRef.current = undefined;
      }, reducedMotion ? 0 : EXIT_ANIMATION_MS);
    }, REPLACEMENT_POLL_MS);

    return () => {
      generationRef.current += 1;
      window.clearInterval(pollTimer);
      if (replacementTimerRef.current !== undefined) window.clearTimeout(replacementTimerRef.current);
      replacementTimerRef.current = undefined;
      replacementRef.current = undefined;
    };
  }, [reducedMotion]);

  const markVisible = (id: string) => {
    commitNodes((value) => value.map((node) => (
      node.id === id && node.phase === "entering" ? { ...node, phase: "visible" } : node
    )));
  };

  return (
    <div
      ref={canvasRef}
      className={`radar-canvas${reducedMotion ? " radar-canvas-reduced" : ""}`}
      style={DEFAULT_WAVE_STYLE}
      aria-label=""
    >
      <div className="radar-waves" aria-hidden="true">
        <span />
        <span />
        <span />
        <span />
      </div>
      {nodes.map((node) => {
        const nodeWidth = Math.max(node.diameter, LABEL_WIDTH);
        const fontSize = Math.round(10 + ((node.diameter - 40) / 48) * 5);
        const motionIndex = node.slot % BREATH_DURATIONS.length;
        return (
          <div
            key={node.id}
            className={`radar-node radar-node-${node.phase}`}
            style={{
              left: node.left - nodeWidth / 2,
              top: node.top - node.diameter / 2,
              width: nodeWidth,
            }}
            aria-hidden="true"
            onAnimationEnd={(event) => {
              if (event.currentTarget === event.target) markVisible(node.id);
            }}
          >
            <span
              className="radar-node-avatar"
              style={{
                backgroundColor: node.color,
                borderWidth: node.diameter >= 64 ? 3 : 2,
                fontSize,
                height: node.diameter,
                width: node.diameter,
                "--radar-breathe-duration": `${BREATH_DURATIONS[motionIndex]}s`,
                "--radar-breathe-delay": `${BREATH_DELAYS[motionIndex]}s`,
              } as CSSProperties}
            >
              {node.roomCode.slice(-2)}
            </span>
            <span className="radar-node-label">{node.roomCode}</span>
          </div>
        );
      })}
      <div className="radar-center">
        <Avatar size={112} style={{ backgroundColor: getStableAvatarColor(displayName), fontSize: 44, fontWeight: 700 }}>{getAvatarInitial(displayName)}</Avatar>
        <div className="radar-center-name">{displayName || "FileDock"}</div>
      </div>
    </div>
  );
}
