import { Avatar } from "antd";
import { useEffect, useRef, useState } from "react";

import { getAvatarInitial, getStableAvatarColor } from "../utils/avatar";
import { createRadarNode, createRadarNodes, type RadarNode } from "../utils/radar";

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
  const [nodes, setNodes] = useState<RadarNode[]>(() => createRadarNodes());
  const nodesRef = useRef(nodes);
  const replacing = useRef(false);

  useEffect(() => { nodesRef.current = nodes; }, [nodes]);
  useEffect(() => {
    const timer = window.setInterval(() => {
      if (replacing.current) return;
      const current = nodesRef.current;
      const expiredIndex = current.findIndex((node) => node.expiresAt <= Date.now());
      if (expiredIndex < 0) return;
      replacing.current = true;
      const expired = current[expiredIndex];
      setNodes((value) => value.map((node) => node.id === expired.id ? { ...node, phase: "exiting" } : node));
      window.setTimeout(() => {
        setNodes((value) => {
          const remaining = value.filter((node) => node.id !== expired.id);
          const next = createRadarNode(Date.now(), remaining);
          return [...remaining, { ...next, phase: reducedMotion ? "visible" : "entering" }];
        });
        replacing.current = false;
      }, reducedMotion ? 0 : 350);
    }, 750);
    return () => window.clearInterval(timer);
  }, [reducedMotion]);

  return (
    <div className={`radar-canvas${reducedMotion ? " radar-canvas-reduced" : ""}`} aria-label="">
      <div className="radar-waves" aria-hidden="true"><span /><span /><span /><span /></div>
      {nodes.map((node) => (
        <div key={node.id} className={`radar-node radar-node-${node.phase}`} style={{ left: `${node.left}%`, top: `${node.top}%` }} aria-hidden="true">
          <span className="radar-node-avatar" style={{ backgroundColor: node.color }}>{node.roomCode.slice(-2)}</span>
          <span className="radar-node-label">{node.roomCode}</span>
        </div>
      ))}
      <div className="radar-center">
        <Avatar size={112} style={{ backgroundColor: getStableAvatarColor(displayName), fontSize: 44, fontWeight: 700 }}>{getAvatarInitial(displayName)}</Avatar>
        <div className="radar-center-name">{displayName || "FileDock"}</div>
      </div>
    </div>
  );
}
