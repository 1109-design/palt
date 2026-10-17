/**
 * zoom.ts — whole-window zoom, like a browser's.
 *
 * Ctrl + / Ctrl − / Ctrl 0 and Ctrl + wheel change it; the top bar shows it.
 * Go saves the level (~/.config/palt/zoom) and, on Linux, applies it as
 * WebKit's own page zoom. Elsewhere (and in browser preview) the page falls
 * back to CSS zoom on <html>, which MUI menus position less well.
 */
import { useCallback, useEffect, useState } from 'react';

const go = () => (window as any).go?.main?.App;

export const ZOOM_STEPS = [0.67, 0.75, 0.8, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2];

/** CSS zoom in use; stays 1 while the platform zooms natively. */
let cssZoom = 1;

/** Saves and applies a level. */
const apply = async (z: number) => {
  let native = false;
  if (go()) native = !!(await go().SetZoom(z).catch(() => false));
  else {
    try {
      localStorage.setItem('palt.zoom', String(z));
    } catch {}
  }
  cssZoom = native ? 1 : z;
  (document.documentElement.style as any).zoom = native ? '' : String(z);
  // Native zoom lands on GTK's next idle; let layout settle before measuring.
  setTimeout(() => window.dispatchEvent(new Event('palt-zoom')), native ? 120 : 0);
};

const currentZoom = () => cssZoom;

/**
 * Window width in the page's own (zoomed) CSS pixels. Media queries ignore
 * CSS zoom, so layouts that switch on width use this instead.
 */
export function useLayoutWidth() {
  const [width, setWidth] = useState(() => window.innerWidth / currentZoom());
  useEffect(() => {
    const update = () => setWidth(window.innerWidth / currentZoom());
    window.addEventListener('resize', update);
    window.addEventListener('palt-zoom', update);
    return () => {
      window.removeEventListener('resize', update);
      window.removeEventListener('palt-zoom', update);
    };
  }, []);
  return width;
}

const load = async (): Promise<number> => {
  if (go()) return (await go().GetZoom().catch(() => 1)) || 1;
  try {
    return Number(localStorage.getItem('palt.zoom')) || 1;
  } catch {
    return 1;
  }
};

export function useZoom() {
  const [zoom, setZoomState] = useState(1);

  const setZoom = useCallback((z: number) => {
    const clamped = Math.min(2, Math.max(0.67, Math.round(z * 100) / 100));
    apply(clamped);
    setZoomState(clamped);
  }, []);

  const step = useCallback(
    (dir: 1 | -1) =>
      setZoomState((cur) => {
        const next =
          dir > 0 ? ZOOM_STEPS.find((s) => s > cur + 0.001) ?? cur : [...ZOOM_STEPS].reverse().find((s) => s < cur - 0.001) ?? cur;
        if (next !== cur) apply(next);
        return next;
      }),
    [],
  );

  useEffect(() => {
    load().then((z) => {
      apply(z);
      setZoomState(z);
    });
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey) || e.altKey) return;
      if (e.key === '+' || e.key === '=' || e.code === 'NumpadAdd') {
        e.preventDefault();
        step(1);
      } else if (e.key === '-' || e.key === '_' || e.code === 'NumpadSubtract') {
        e.preventDefault();
        step(-1);
      } else if (e.key === '0' || e.code === 'Numpad0') {
        e.preventDefault();
        setZoom(1);
      }
    };
    const onWheel = (e: WheelEvent) => {
      if (!e.ctrlKey) return;
      e.preventDefault();
      step(e.deltaY < 0 ? 1 : -1);
    };
    window.addEventListener('keydown', onKey);
    window.addEventListener('wheel', onWheel, { passive: false });
    return () => {
      window.removeEventListener('keydown', onKey);
      window.removeEventListener('wheel', onWheel);
    };
  }, [step, setZoom]);

  return { zoom, zoomIn: () => step(1), zoomOut: () => step(-1), resetZoom: () => setZoom(1) };
}
