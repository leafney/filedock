import { Avatar } from "antd";
import { useEffect, useRef, useState, type CSSProperties } from "react";

import { getAvatarInitial, getStableAvatarColor } from "../utils/avatar";
import { useTheme } from "./ThemeProvider";
import {
  DEFAULT_RADAR_BOUNDS,
  RADAR_NODE_COUNT,
  createRadarNodes,
  relayoutRadarNodes,
  replaceRadarNode,
  type RadarBounds,
  type RadarNode,
} from "../utils/radar";
import { createRadarWaveRenderer, type RadarWaveRenderer } from "../utils/radarCanvas";

const REPLACEMENT_POLL_MS = 750;
const EXIT_ANIMATION_MS = 350;
const LABEL_WIDTH = 72;
const BREATH_DURATIONS = [2.8, 3.15, 3.5, 3.85, 4.2] as const;
const BREATH_DELAYS = [-0.4, -1.7, -2.9, -0.9, -3.6] as const;

function useMediaQuery(query: string) {
  const [matches, setMatches] = useState(() => window.matchMedia(query).matches);
  useEffect(() => {
    const media = window.matchMedia(query);
    const update = () => setMatches(media.matches);
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, [query]);
  return matches;
}

export function RadarCanvas({ displayName }: { displayName: string }) {
  const { resolvedTheme } = useTheme();
  const reducedMotion = useMediaQuery("(prefers-reduced-motion: reduce)");
  const mobile = useMediaQuery("(max-width: 768px)");
  const containerRef = useRef<HTMLDivElement>(null);
  const waveCanvasRef = useRef<HTMLCanvasElement>(null);
  const waveRendererRef = useRef<RadarWaveRenderer | null>(null);
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
    const waveCanvas = waveCanvasRef.current;
    if (!waveCanvas) return;
    const renderer = createRadarWaveRenderer(waveCanvas, { mobile, reducedMotion, theme: resolvedTheme });
    waveRendererRef.current = renderer;
    return () => {
      renderer?.destroy();
      waveRendererRef.current = null;
    };
  }, []);

  useEffect(() => {
    waveRendererRef.current?.setReducedMotion(reducedMotion);
  }, [reducedMotion]);

  useEffect(() => {
    waveRendererRef.current?.setMobile(mobile);
  }, [mobile]);

  useEffect(() => {
    waveRendererRef.current?.setTheme(resolvedTheme);
  }, [resolvedTheme]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const waveCanvas = waveCanvasRef.current;
    if (!waveCanvas) return;

    const resizeWave = () => {
      const rect = waveCanvas.getBoundingClientRect();
      const width = Math.round(rect.width);
      const height = Math.round(rect.height);
      if (width <= 0 || height <= 0) return;
      waveRendererRef.current?.resize({ width, height });
    };

    const observer = new ResizeObserver((entries) => {
      const containerEntry = entries.find((entry) => entry.target === container);
      if (!containerEntry) {
        resizeWave();
        return;
      }

      const width = Math.round(containerEntry.contentRect.width);
      const height = Math.round(containerEntry.contentRect.height);
      if (width <= 0 || height <= 0) return;
      const bounds = { width, height };
      resizeWave();
      const current = boundsRef.current;
      if (Math.abs(current.width - width) < 2 && Math.abs(current.height - height) < 2) return;
      boundsRef.current = bounds;
      commitNodes((value) => relayoutRadarNodes(value, bounds));
    });
    observer.observe(container);
    observer.observe(waveCanvas);
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
      ref={containerRef}
      className={`radar-canvas${reducedMotion ? " radar-canvas-reduced" : ""}`}
      aria-label=""
    >
      <canvas ref={waveCanvasRef} className="radar-wave-canvas" aria-hidden="true" />
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
