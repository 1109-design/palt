/**
 * PeerMediaCard.tsx — what a paired phone is playing, with controls (Beacon)
 *
 * Shows the phone's main player (playing first) with art, a seek bar that
 * moves on its own, transport buttons, and the phone's media volume.
 */
import React, { useEffect, useState } from 'react';
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

const time = (ms: number) => {
  const s = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = String(s % 60).padStart(2, '0');
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${sec}` : `${m}:${sec}`;
};

const PeerMediaCard: React.FC<{
  device: string;
  state?: MediaState;
  receivedAt?: number;
  onAction: (player: string, action: string, value?: number) => void;
}> = ({ device, state, receivedAt, onAction }) => {
  const [, tick] = useState(0);
  const [seek, setSeek] = useState<number | null>(null);
  const [vol, setVol] = useState<number | null>(null);
  useEffect(() => {
    const t = setInterval(() => tick((n) => n + 1), 1000);
    return () => clearInterval(t);
  }, []);

  const players = state?.players ?? [];
  const p: MediaPlayer | undefined = players.find((x) => x.playing) ?? players.find((x) => x.title);
  if (!state || (!p && state.volume.level < 0)) return null;

  const position = p
    ? Math.min(p.position + (p.playing && receivedAt ? Date.now() - receivedAt : 0), p.length || Infinity)
    : 0;

  return (
    <Box
      sx={{
        mt: 1.25,
        p: 1.5,
        borderRadius: 2,
        border: `1px solid ${p?.playing ? alpha(beacon.accent, 0.45) : beacon.hairline}`,
        backgroundColor: beacon.surface,
        display: 'flex',
        gap: 1.75,
        alignItems: 'center',
        flexWrap: 'wrap',
      }}
    >
      {p && (
        <>
          <Box
            sx={{
              width: 64,
              height: 64,
              borderRadius: 1.5,
              overflow: 'hidden',
              flexShrink: 0,
              backgroundColor: beacon.surfaceRaised,
              display: 'grid',
              placeItems: 'center',
            }}
          >
            {p.art ? (
              <img src={p.art} alt="" style={{ width: '100%', height: '100%', objectFit: 'cover' }} />
            ) : (
              <MusicNoteIcon sx={{ color: beacon.textFaint }} />
            )}
          </Box>
          <Box sx={{ flex: '1 1 220px', minWidth: 0 }}>
            <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.62rem', letterSpacing: '0.08em', color: p.playing ? beacon.accent : beacon.textFaint, textTransform: 'uppercase' }}>
              {p.name} · on {device}
            </Typography>
            <Typography noWrap sx={{ fontFamily: fonts.display, fontWeight: 700, fontSize: '0.98rem', color: beacon.text }}>
              {p.title || 'Unknown'}
            </Typography>
            {p.artist && (
              <Typography noWrap sx={{ fontSize: '0.8rem', color: beacon.textSecondary }}>
                {p.artist}
              </Typography>
            )}
            {p.canSeek && p.length > 0 && (
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.66rem', color: beacon.textFaint }}>{time(seek ?? position)}</Typography>
                <Slider
                  size="small"
                  value={seek ?? position}
                  max={p.length}
                  onChange={(_, v) => setSeek(v as number)}
                  onChangeCommitted={(_, v) => {
                    onAction(p.id, 'seek', v as number);
                    setTimeout(() => setSeek(null), 1000);
                  }}
                  sx={{ py: 0.75 }}
                />
                <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.66rem', color: beacon.textFaint }}>{time(p.length)}</Typography>
              </Box>
            )}
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center' }}>
            <IconButton disabled={!p.canPrev} onClick={() => onAction(p.id, 'previous')} aria-label="previous">
              <SkipPreviousIcon />
            </IconButton>
            <IconButton
              onClick={() => onAction(p.id, 'playpause')}
              aria-label={p.playing ? 'pause' : 'play'}
              sx={{ backgroundColor: alpha(beacon.accent, 0.15), color: beacon.accent, mx: 0.5, '&:hover': { backgroundColor: alpha(beacon.accent, 0.25) } }}
            >
              {p.playing ? <PauseIcon /> : <PlayArrowIcon />}
            </IconButton>
            <IconButton disabled={!p.canNext} onClick={() => onAction(p.id, 'next')} aria-label="next">
              <SkipNextIcon />
            </IconButton>
          </Box>
        </>
      )}
      {state.volume.level >= 0 && (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flex: p ? '0 1 200px' : '1 1 auto', minWidth: 160 }}>
          <Tooltip title={state.volume.muted ? `Unmute ${device}` : `Mute ${device}`}>
            <IconButton size="small" onClick={() => onAction('', 'mute')} sx={{ color: state.volume.muted ? beacon.danger : beacon.textSecondary }}>
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
          />
          <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.7rem', color: beacon.textSecondary, minWidth: 30 }}>
            {Math.round(vol ?? state.volume.level)}%
          </Typography>
        </Box>
      )}
    </Box>
  );
};

export default PeerMediaCard;
