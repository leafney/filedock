import type { RadarBounds } from "./radar";

const RING_COUNT = 8;
const LINE_WIDTH = 1;
const RING_SPACING_DIVISOR = 9;
const START_RADIUS = 33;
const PHASE_DURATION_MS = 3_000;
const FRAME_INTERVAL_MS = 1_000 / 60;
const DESKTOP_BASE_OPACITY = 0.18;
const MOBILE_BASE_OPACITY = 0.12;
const MAX_DEVICE_PIXEL_RATIO = 2;
const RING_COLOR = "13 148 136";

export interface RadarWaveRenderer {
  resize: (bounds: RadarBounds) => void;
  setMobile: (mobile: boolean) => void;
  setReducedMotion: (reduced: boolean) => void;
  start: () => void;
  pause: () => void;
  destroy: () => void;
}

export function createRadarWaveRenderer(
  canvas: HTMLCanvasElement,
  options: { mobile: boolean; reducedMotion: boolean },
): RadarWaveRenderer | null {
  const context = canvas.getContext("2d");
  if (!context || typeof window.requestAnimationFrame !== "function"
    || typeof window.cancelAnimationFrame !== "function") {
    canvas.hidden = true;
    return null;
  }

  let bounds: RadarBounds = { width: 0, height: 0 };
  let mobile = options.mobile;
  let reducedMotion = options.reducedMotion;
  let accumulatedTime = 0;
  let animationFrame: number | undefined;
  let lastAnimationTime: number | undefined;
  let lastDrawTime: number | undefined;
  let destroyed = false;

  const getPhase = () => (accumulatedTime % PHASE_DURATION_MS) / PHASE_DURATION_MS;

  const draw = (phase: number) => {
    const { width, height } = bounds;
    if (width <= 0 || height <= 0) return;

    context.clearRect(0, 0, width, height);
    context.lineWidth = LINE_WIDTH;

    const spacing = Math.max(1, Math.round(Math.max(width * 0.6, height) / RING_SPACING_DIVISOR));
    const phaseOffset = spacing * phase;
    const maxDimension = Math.max(1, width, height);
    const baseOpacity = mobile ? MOBILE_BASE_OPACITY : DESKTOP_BASE_OPACITY;

    for (let index = RING_COUNT - 1; index >= 0; index -= 1) {
      const radius = spacing * index + phaseOffset + START_RADIUS;
      let opacity = Math.max(0, baseOpacity * (1 - 1.2 * radius / maxDimension));
      if (radius > spacing * 7) {
        const outerFade = Math.min(1, Math.max(0, (spacing * 8 - radius) / spacing));
        opacity *= outerFade;
      }
      opacity = Math.min(baseOpacity, Math.max(0, opacity));
      if (opacity <= 0) continue;

      context.beginPath();
      context.arc(width / 2, height / 2, radius, 0, Math.PI * 2);
      context.strokeStyle = `rgb(${RING_COLOR} / ${opacity})`;
      context.stroke();
    }
  };

  const pause = () => {
    if (animationFrame !== undefined) {
      window.cancelAnimationFrame(animationFrame);
      animationFrame = undefined;
    }
    lastAnimationTime = undefined;
    lastDrawTime = undefined;
  };

  const tick = (time: number) => {
    if (destroyed || reducedMotion || document.hidden) {
      pause();
      return;
    }

    if (lastAnimationTime === undefined) lastAnimationTime = time;
    accumulatedTime += Math.max(0, time - lastAnimationTime);
    lastAnimationTime = time;

    if (lastDrawTime === undefined || time - lastDrawTime >= FRAME_INTERVAL_MS) {
      draw(getPhase());
      lastDrawTime = time;
    }
    animationFrame = window.requestAnimationFrame(tick);
  };

  const start = () => {
    if (destroyed || reducedMotion || document.hidden || animationFrame !== undefined) return;
    lastAnimationTime = undefined;
    animationFrame = window.requestAnimationFrame(tick);
  };

  const handleVisibilityChange = () => {
    if (document.hidden) {
      pause();
      return;
    }
    if (reducedMotion) draw(0);
    else start();
  };

  document.addEventListener("visibilitychange", handleVisibilityChange);
  if (reducedMotion) draw(0);
  else start();

  return {
    resize(nextBounds) {
      if (destroyed || nextBounds.width <= 0 || nextBounds.height <= 0) return;
      bounds = nextBounds;
      const devicePixelRatio = Math.min(window.devicePixelRatio || 1, MAX_DEVICE_PIXEL_RATIO);
      const bitmapWidth = Math.round(bounds.width * devicePixelRatio);
      const bitmapHeight = Math.round(bounds.height * devicePixelRatio);
      if (canvas.width !== bitmapWidth || canvas.height !== bitmapHeight) {
        canvas.width = bitmapWidth;
        canvas.height = bitmapHeight;
        context.setTransform(devicePixelRatio, 0, 0, devicePixelRatio, 0, 0);
      }
      draw(reducedMotion ? 0 : getPhase());
    },
    setMobile(nextMobile) {
      if (destroyed || mobile === nextMobile) return;
      mobile = nextMobile;
      draw(reducedMotion ? 0 : getPhase());
    },
    setReducedMotion(nextReducedMotion) {
      if (destroyed || reducedMotion === nextReducedMotion) return;
      reducedMotion = nextReducedMotion;
      if (reducedMotion) {
        pause();
        draw(0);
      } else {
        draw(getPhase());
        start();
      }
    },
    start,
    pause,
    destroy() {
      if (destroyed) return;
      destroyed = true;
      pause();
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      context.clearRect(0, 0, bounds.width, bounds.height);
    },
  };
}
