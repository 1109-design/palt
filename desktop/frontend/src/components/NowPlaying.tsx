/**
 * NowPlaying.tsx — what a paired device is playing, with controls (Beacon).
 *
 * Same card as the phone's: blurred cover art under a gradient tinted with
 * the art's own colour, a slow drifting glow while playing, and a One UI–
 * style wavy seek line in that colour running into amber. The wave moves
 * while playing and settles flat when paused; play/pause morphs, the art
 * cross-fades between tracks and eases back a little when paused.
 */
import React, { useEffect, useRef, useState } from 'react';
import { Box, IconButton, Slider, Tooltip, Typography } from '@mui/material';
import { alpha } from '@mui/material/styles';
import PlayArrowIcon from '@mui/icons-material/PlayArrowRounded';
import PauseIcon from '@mui/icons-material/PauseRounded';
import SkipNextIcon from '@mui/icons-material/SkipNextRounded';
import SkipPreviousIcon from '@mui/icons-material/SkipPreviousRounded';
import VolumeUpIcon from '@mui/icons-material/VolumeUpRounded';
import VolumeOffIcon from '@mui/icons-material/VolumeOffRounded';
import MusicNoteIcon from '@mui/icons-material/MusicNoteRounded';
import { beacon, fonts } from '../theme/theme';
import type { MediaPlayer, MediaState } from '../types/peer';

const clock = (ms: number) => {
  const s = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = String(s % 60).padStart(2, '0');
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${sec}` : `${m}:${sec}`;
};

// ── The art's colour ────────────────────────────────────────────────────────

const tints = new Map<string, Promise<string | null>>();

/** The art's most vivid colour, tamed for dark surfaces; null when unreadable. */
const artTint = (url: string) => {
  if (!tints.has(url)) {
    tints.set(
      url,
      new Promise((resolve) => {
        if (!url) return resolve(null);
        const img = new Image();
        img.crossOrigin = 'anonymous';
        img.onload = () => {
          try {
            const c = document.createElement('canvas');
            c.width = c.height = 32;
            const g = c.getContext('2d')!;
            g.drawImage(img, 0, 0, 32, 32);
            const d = g.getImageData(0, 0, 32, 32).data; // throws on a tainted canvas
            let r = 0, gg = 0, b = 0, total = 0;
            for (let i = 0; i < d.length; i += 4) {
              const max = Math.max(d[i], d[i + 1], d[i + 2]) / 255;
              const min = Math.min(d[i], d[i + 1], d[i + 2]) / 255;
              const sat = max === 0 ? 0 : (max - min) / max;
              // Vivid, not too dark or washed out, counts most.
              const w = sat * sat * max * (max > 0.15 ? 1 : 0) + 0.002;
              r += d[i] * w;
              gg += d[i + 1] * w;
              b += d[i + 2] * w;
              total += w;
            }
            resolve(tame(r / total, gg / total, b / total));
          } catch {
            resolve(null);
          }
        };
        img.onerror = () => resolve(null);
        img.src = url;
      }),
    );
  }
  return tints.get(url)!;
};

/** Saturation 35–80%, brightness 85%, as an rgb() string. */
const tame = (r: number, g: number, b: number) => {
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  let h = 0;
  if (max !== min) {
    const d = max - min;
    h = max === r ? ((g - b) / d) % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4;
    h *= 60;
    if (h < 0) h += 360;
  }
  const s = Math.min(0.8, Math.max(0.35, max === 0 ? 0 : (max - min) / max));
  const v = 0.85;
  const c = v * s;
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1));
  const m = v - c;
  const [rr, gg, bb] =
    h < 60 ? [c, x, 0] : h < 120 ? [x, c, 0] : h < 180 ? [0, c, x] : h < 240 ? [0, x, c] : h < 300 ? [x, 0, c] : [c, 0, x];
  return `rgb(${Math.round((rr + m) * 255)}, ${Math.round((gg + m) * 255)}, ${Math.round((bb + m) * 255)})`;
};

// ── Wavy seek line ──────────────────────────────────────────────────────────

const WavySeek: React.FC<{
  /** ms now, computed per frame */
  position: () => number;
  length: number;
  playing: boolean;
  color: string;
  canSeek: boolean;
  onSeek: (ms: number) => void;
}> = ({ position, length, playing, color, canSeek, onSeek }) => {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [drag, setDrag] = useState<number | null>(null);
  const [shown, setShown] = useState(0);
  const live = useRef({ position, playing, color, drag });
  live.current = { position, playing, color, drag };

  useEffect(() => {
    let raf = 0;
    let amp = playing ? 1 : 0;
    let phase = 0;
    let last = performance.now();
    let lastShown = -1;
    const draw = (now: number) => {
      const dt = Math.min(64, now - last) / 1000;
      last = now;
      const { position, playing, color, drag } = live.current;
      amp += ((playing ? 1 : 0) - amp) * Math.min(1, dt * 6);
      phase += dt * 2 * Math.PI * 1.1;

      const cv = canvasRef.current;
      if (cv) {
        const dpr = window.devicePixelRatio || 1;
        const w = cv.clientWidth;
        const h = cv.clientHeight;
        if (cv.width !== Math.round(w * dpr)) cv.width = Math.round(w * dpr);
        if (cv.height !== Math.round(h * dpr)) cv.height = Math.round(h * dpr);
        const g = cv.getContext('2d')!;
        g.setTransform(dpr, 0, 0, dpr, 0, 0);
        g.clearRect(0, 0, w, h);

        const pos = drag ?? position();
        const frac = length > 0 ? Math.min(1, Math.max(0, pos / length)) : 0;
        const x = w * frac;
        const mid = h / 2;

        if (x + 8 < w) {
          g.strokeStyle = 'rgba(255,255,255,0.18)';
          g.lineWidth = 3;
          g.lineCap = 'round';
          g.beginPath();
          g.moveTo(x + 8, mid);
          g.lineTo(w, mid);
          g.stroke();
        }
        if (x > 1) {
          const grad = g.createLinearGradient(0, 0, Math.max(x, 1), 0);
          grad.addColorStop(0, color);
          grad.addColorStop(1, beacon.accent);
          g.strokeStyle = grad;
          g.lineWidth = 4;
          g.lineJoin = 'round';
          g.beginPath();
          for (let px = 0; px <= x; px += 1.5) {
            const taper = Math.min(1, Math.min(px, x - px) / 14);
            const y = mid + Math.sin((px / 26) * 2 * Math.PI - phase) * 4 * amp * taper;
            px === 0 ? g.moveTo(px, y) : g.lineTo(px, y);
          }
          g.stroke();
        }
        // Thumb: a short upright pill with a glow.
        g.fillStyle = alpha(beacon.accent, 0.22);
        g.beginPath();
        g.roundRect(x - 7, mid - 14, 14, 28, 7);
        g.fill();
        g.fillStyle = beacon.accent;
        g.beginPath();
        g.roundRect(x - 3, mid - 10, 6, 20, 3);
        g.fill();

        const sec = Math.floor(pos / 1000);
        if (sec !== lastShown) {
          lastShown = sec;
          setShown(pos);
        }
      }
      raf = requestAnimationFrame(draw);
    };
    raf = requestAnimationFrame(draw);
    return () => cancelAnimationFrame(raf);
  }, [length]);

  const at = (e: React.PointerEvent<HTMLCanvasElement>) => {
    const r = e.currentTarget.getBoundingClientRect();
    return Math.min(1, Math.max(0, (e.clientX - r.left) / r.width)) * length;
  };

  return (
    <Box>
      <Box
        component="canvas"
        ref={canvasRef}
        onPointerDown={(e: React.PointerEvent<HTMLCanvasElement>) => {
          if (!canSeek) return;
          e.currentTarget.setPointerCapture(e.pointerId);
          setDrag(at(e));
        }}
        onPointerMove={(e: React.PointerEvent<HTMLCanvasElement>) => {
          if (drag !== null) setDrag(at(e));
        }}
        onPointerUp={(e: React.PointerEvent<HTMLCanvasElement>) => {
          if (drag === null) return;
          const ms = at(e);
          onSeek(ms);
          setDrag(ms);
          setTimeout(() => setDrag(null), 1000);
        }}
        sx={{ display: 'block', width: '100%', height: 30, cursor: canSeek ? 'pointer' : 'default', touchAction: 'none' }}
      />
      <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
        <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.66rem', color: alpha(beacon.text, 0.6) }}>{clock(drag ?? shown)}</Typography>
        <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.66rem', color: alpha(beacon.text, 0.6) }}>{clock(length)}</Typography>
      </Box>
    </Box>
  );
};

// ── The card ────────────────────────────────────────────────────────────────

const RoundButton: React.FC<{ label: string; disabled?: boolean; onClick: () => void; children: React.ReactNode }> = ({
  label,
  disabled,
  onClick,
  children,
}) => (
  <IconButton
    aria-label={label}
    disabled={disabled}
    onClick={onClick}
    sx={{
      width: 46,
      height: 46,
      backgroundColor: alpha('#fff', 0.08),
      color: beacon.text,
      '&:hover': { backgroundColor: alpha('#fff', 0.14) },
      '&.Mui-disabled': { backgroundColor: alpha('#fff', 0.03), color: beacon.textFaint },
      '& svg': { fontSize: 26 },
    }}
  >
    {children}
  </IconButton>
);

const NowPlaying: React.FC<{
  device: string;
  state?: MediaState;
  receivedAt?: number;
  onAction: (player: string, action: string, value?: number) => void;
}> = ({ device, state, receivedAt, onAction }) => {
  const [vol, setVol] = useState<number | null>(null);
  const [tint, setTint] = useState<string>(beacon.accent);

  const players = state?.players ?? [];
  const p: MediaPlayer | undefined = players.find((x) => x.playing) ?? players.find((x) => x.title);

  useEffect(() => {
    let alive = true;
    artTint(p?.art ?? '').then((c) => alive && setTint(c ?? beacon.accent));
    return () => {
      alive = false;
    };
  }, [p?.art]);

  if (!state || (!p && state.volume.level < 0)) return null;

  const position = () =>
    p ? Math.min(p.position + (p.playing && receivedAt ? Date.now() - receivedAt : 0), p.length || Infinity) : 0;

  const volume = state.volume.level >= 0 && (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25, mt: p ? 1 : 0 }}>
      <Tooltip title={state.volume.muted ? `Unmute ${device}` : `Mute ${device}`}>
        <IconButton size="small" onClick={() => onAction('', 'mute')} sx={{ color: state.volume.muted ? beacon.danger : alpha(beacon.text, 0.75) }}>
          {state.volume.muted ? <VolumeOffIcon fontSize="small" /> : <VolumeUpIcon fontSize="small" />}
        </IconButton>
      </Tooltip>
      <Slider
        size="small"
        value={vol ?? state.volume.level}
        max={100}
        onChange={(_, v) => setVol(v as number)}
        onChangeCommitted={(_, v) => {
          onAction('', 'volume', v as number);
          setTimeout(() => setVol(null), 1000);
        }}
        aria-label={`${device} volume`}
        sx={{
          color: alpha(beacon.text, 0.85),
          '& .MuiSlider-rail': { backgroundColor: alpha('#fff', 0.3) },
          '& .MuiSlider-thumb': { width: 12, height: 12, backgroundColor: beacon.text },
        }}
      />
      <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.7rem', color: alpha(beacon.text, 0.7), minWidth: 32, textAlign: 'right' }}>
        {Math.round(vol ?? state.volume.level)}%
      </Typography>
    </Box>
  );

  if (!p) {
    return <Box sx={{ px: 2, py: 1, borderRadius: '20px', backgroundColor: beacon.tile }}>{volume}</Box>;
  }

  return (
    <Box
      sx={{
        position: 'relative',
        overflow: 'hidden',
        borderRadius: '26px',
        border: `1px solid ${alpha('#fff', 0.07)}`,
        isolation: 'isolate',
      }}
    >
      {/* Blurred art, the tint gradient, and a drifting glow. */}
      <Box
        key={p.art}
        sx={{
          position: 'absolute',
          inset: -40,
          zIndex: -3,
          backgroundImage: p.art ? `url("${p.art}")` : 'none',
          backgroundSize: 'cover',
          backgroundPosition: 'center',
          filter: 'blur(40px)',
          opacity: 0.55,
          animation: 'np-fade 700ms ease',
          '@keyframes np-fade': { from: { opacity: 0 }, to: { opacity: 0.55 } },
        }}
      />
      <Box
        sx={{
          position: 'absolute',
          inset: 0,
          zIndex: -2,
          transition: 'background 800ms',
          background: `linear-gradient(135deg, color-mix(in srgb, ${tint} 42%, ${beacon.bg}) 0%, color-mix(in srgb, ${tint} 12%, ${beacon.bg}) 55%, ${beacon.bg} 100%)`,
          opacity: 0.88,
        }}
      />
      <Box
        sx={{
          position: 'absolute',
          inset: '-30%',
          zIndex: -1,
          background: `radial-gradient(40% 40% at 50% 50%, color-mix(in srgb, ${tint} 30%, transparent), transparent 70%)`,
          opacity: p.playing ? 1 : 0.35,
          transition: 'opacity 600ms',
          animation: 'np-drift 8s linear infinite',
          '@keyframes np-drift': {
            '0%': { transform: 'translate(25%, -10%)' },
            '25%': { transform: 'translate(0%, 10%)' },
            '50%': { transform: 'translate(-25%, -10%)' },
            '75%': { transform: 'translate(0%, -25%)' },
            '100%': { transform: 'translate(25%, -10%)' },
          },
          '@media (prefers-reduced-motion: reduce)': { animation: 'none' },
        }}
      />

      <Box sx={{ p: 2.25, pb: 1.5 }}>
        <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
          <Box
            key={p.art}
            sx={{
              width: 80,
              height: 80,
              borderRadius: '16px',
              overflow: 'hidden',
              flexShrink: 0,
              backgroundColor: beacon.surfaceRaised,
              display: 'grid',
              placeItems: 'center',
              boxShadow: `0 8px 18px color-mix(in srgb, ${tint} 35%, transparent)`,
              transform: p.playing ? 'scale(1)' : 'scale(0.92)',
              transition: 'transform 450ms cubic-bezier(0.34, 1.56, 0.64, 1)',
              animation: 'np-fade-in 500ms ease',
              '@keyframes np-fade-in': { from: { opacity: 0 }, to: { opacity: 1 } },
            }}
          >
            {p.art ? (
              <img src={p.art} alt="" style={{ width: '100%', height: '100%', objectFit: 'cover' }} />
            ) : (
              <MusicNoteIcon sx={{ color: beacon.textFaint }} />
            )}
          </Box>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              {/* Equalizer bars */}
              <Box sx={{ display: 'flex', alignItems: 'flex-end', gap: '2px', height: 12, width: 14, flexShrink: 0 }}>
                {[0, 1, 2].map((i) => (
                  <Box
                    key={i}
                    sx={{
                      width: 3,
                      height: 4,
                      borderRadius: '2px',
                      backgroundColor: `color-mix(in srgb, ${tint} 60%, ${beacon.accent})`,
                      animation: p.playing ? `np-bar 900ms ease-in-out ${i * 150}ms infinite alternate` : 'none',
                      '@keyframes np-bar': { from: { height: 4 }, to: { height: 12 } },
                    }}
                  />
                ))}
              </Box>
              <Typography noWrap sx={{ fontFamily: fonts.mono, fontSize: '0.62rem', letterSpacing: '0.08em', color: alpha(beacon.text, 0.7), textTransform: 'uppercase' }}>
                {p.name} · {device}
              </Typography>
            </Box>
            <Typography
              key={p.title}
              noWrap
              sx={{
                fontFamily: fonts.display,
                fontWeight: 800,
                fontSize: '1.2rem',
                color: beacon.text,
                mt: 0.5,
                animation: 'np-fade-in 350ms ease',
              }}
            >
              {p.title || 'Unknown'}
            </Typography>
            {p.artist && (
              <Typography noWrap sx={{ fontSize: '0.84rem', color: alpha(beacon.text, 0.72) }}>
                {p.artist}
              </Typography>
            )}
          </Box>
        </Box>

        <Box sx={{ mt: 1.75 }}>
          {p.length > 0 ? (
            <WavySeek
              position={position}
              length={p.length}
              playing={p.playing}
              color={tint}
              canSeek={p.canSeek}
              onSeek={(ms) => onAction(p.id, 'seek', Math.round(ms))}
            />
          ) : (
            <Box sx={{ height: 8 }} />
          )}
        </Box>

        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 2, mt: 0.75 }}>
          <RoundButton label="previous" disabled={!p.canPrev} onClick={() => onAction(p.id, 'previous')}>
            <SkipPreviousIcon />
          </RoundButton>
          <Box
            component="button"
            aria-label={p.playing ? 'pause' : 'play'}
            onClick={() => onAction(p.id, 'playpause')}
            sx={{
              position: 'relative',
              width: 64,
              height: 64,
              border: 'none',
              borderRadius: '50%',
              cursor: 'pointer',
              color: beacon.onAccent,
              background: `linear-gradient(135deg, color-mix(in srgb, ${tint} 35%, ${beacon.accent}), ${beacon.accent})`,
              boxShadow: p.playing ? `0 0 26px 1px ${alpha(beacon.accent, 0.45)}` : `0 0 10px ${alpha(beacon.accent, 0.15)}`,
              transition: 'box-shadow 400ms, transform 150ms',
              '&:hover': { transform: 'scale(1.04)' },
              '&:active': { transform: 'scale(0.96)' },
              // Play and pause cross-morph: one scales and turns away as the other arrives.
              '& svg': {
                position: 'absolute',
                top: '50%',
                left: '50%',
                fontSize: 34,
                transition: 'opacity 300ms, transform 380ms cubic-bezier(0.4, 0, 0.2, 1)',
              },
              '& .np-play': {
                opacity: p.playing ? 0 : 1,
                transform: `translate(-50%, -50%) rotate(${p.playing ? -90 : 0}deg) scale(${p.playing ? 0.4 : 1})`,
              },
              '& .np-pause': {
                opacity: p.playing ? 1 : 0,
                transform: `translate(-50%, -50%) rotate(${p.playing ? 0 : 90}deg) scale(${p.playing ? 1 : 0.4})`,
              },
            }}
          >
            <PlayArrowIcon className="np-play" />
            <PauseIcon className="np-pause" />
          </Box>
          <RoundButton label="next" disabled={!p.canNext} onClick={() => onAction(p.id, 'next')}>
            <SkipNextIcon />
          </RoundButton>
        </Box>

        {volume}
      </Box>
    </Box>
  );
};

export default NowPlaying;
