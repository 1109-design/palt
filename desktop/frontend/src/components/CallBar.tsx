/**
 * CallBar.tsx — the call in progress on a paired phone (Beacon)
 *
 * A floating card at the top of the window: the caller's photo with a pulse
 * while it rings, name and number, a live timer once talking, and Answer /
 * Decline / Silence or Hang up. The audio stays on the phone.
 */
import React, { useEffect, useState } from 'react';
import { Avatar, Box, Button, IconButton, Tooltip, Typography } from '@mui/material';
import { alpha, keyframes } from '@mui/material/styles';
import CallIcon from '@mui/icons-material/CallRounded';
import CallEndIcon from '@mui/icons-material/CallEndRounded';
import NotificationsOffIcon from '@mui/icons-material/NotificationsOffRounded';
import PhoneInTalkIcon from '@mui/icons-material/PhoneInTalkRounded';
import PhoneCallbackIcon from '@mui/icons-material/PhoneCallbackRounded';
import PhoneForwardedIcon from '@mui/icons-material/PhoneForwardedRounded';
import { beacon, fonts } from '../theme/theme';
import type { ActiveCall } from '../types/peer';

const pulse = keyframes`
  0%   { box-shadow: 0 0 0 0 ${alpha('#53D993', 0.55)}; }
  70%  { box-shadow: 0 0 0 16px ${alpha('#53D993', 0)}; }
  100% { box-shadow: 0 0 0 0 ${alpha('#53D993', 0)}; }
`;

const slideIn = keyframes`
  from { transform: translate(-50%, -16px); opacity: 0; }
  to   { transform: translate(-50%, 0); opacity: 1; }
`;

const clock = (ms: number) => {
  const s = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = String(s % 60).padStart(2, '0');
  return h ? `${h}:${String(m).padStart(2, '0')}:${sec}` : `${m}:${sec}`;
};

const initials = (s: string) =>
  s.replace(/[^\p{L}\p{N} ]/gu, '').split(' ').filter(Boolean).slice(0, 2).map((w) => w[0]).join('').toUpperCase() || '#';

const CallBar: React.FC<{
  call: ActiveCall;
  onAnswer: () => void;
  onEnd: () => void;
  onSilence: () => void;
}> = ({ call, onAnswer, onEnd, onSilence }) => {
  const [, tick] = useState(0);
  useEffect(() => {
    if (call.state !== 'talking') return;
    const t = setInterval(() => tick((n) => n + 1), 1000);
    return () => clearInterval(t);
  }, [call.state]);

  const ringing = call.state === 'ringing';
  const incoming = call.direction !== 'outgoing';
  const who = call.name || call.number || 'Unknown number';
  const status = ringing
    ? `Incoming call · ${call.device}`
    : call.since
    ? clock(Date.now() - call.since)
    : 'Calling…';
  const StatusIcon = ringing ? PhoneCallbackIcon : incoming ? PhoneInTalkIcon : PhoneForwardedIcon;

  return (
    <Box
      role="status"
      aria-live="polite"
      sx={{
        position: 'fixed',
        top: 76,
        left: '50%',
        transform: 'translateX(-50%)',
        zIndex: 1400,
        width: 'min(560px, calc(100vw - 32px))',
        display: 'flex',
        alignItems: 'center',
        gap: 1.75,
        p: 1.5,
        pl: 1.75,
        borderRadius: '20px',
        background: `linear-gradient(135deg, ${alpha(beacon.surfaceRaised, 0.97)}, ${alpha(beacon.surface, 0.97)})`,
        backdropFilter: 'blur(14px)',
        border: `1px solid ${alpha(ringing ? beacon.ok : beacon.accent, 0.45)}`,
        boxShadow: `0 18px 48px ${alpha('#000', 0.55)}, 0 0 0 1px ${alpha('#fff', 0.02)} inset`,
        animation: `${slideIn} 240ms ease-out`,
      }}
    >
      <Avatar
        src={call.photo || undefined}
        sx={{
          width: 52,
          height: 52,
          fontFamily: fonts.display,
          fontWeight: 800,
          bgcolor: alpha(ringing ? beacon.ok : beacon.accent, 0.18),
          color: ringing ? beacon.ok : beacon.accent,
          animation: ringing ? `${pulse} 1.6s infinite` : 'none',
        }}
      >
        {initials(who)}
      </Avatar>

      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography noWrap sx={{ fontFamily: fonts.display, fontWeight: 800, fontSize: '1.05rem', color: beacon.text, lineHeight: 1.25 }}>
          {who}
        </Typography>
        {call.name && call.number && (
          <Typography noWrap sx={{ fontFamily: fonts.mono, fontSize: '0.72rem', color: beacon.textSecondary }}>
            {call.number}
          </Typography>
        )}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.6, mt: 0.25 }}>
          <StatusIcon sx={{ fontSize: 14, color: ringing ? beacon.ok : beacon.accent }} />
          <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.72rem', color: ringing ? beacon.ok : beacon.accent, fontVariantNumeric: 'tabular-nums' }}>
            {status}
          </Typography>
        </Box>
      </Box>

      {ringing && incoming ? (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Tooltip title="Silence the ringer">
            <IconButton onClick={onSilence} sx={{ color: beacon.textSecondary }}>
              <NotificationsOffIcon />
            </IconButton>
          </Tooltip>
          <Tooltip title="Decline">
            <IconButton onClick={onEnd} sx={{ width: 46, height: 46, bgcolor: beacon.danger, color: '#fff', '&:hover': { bgcolor: alpha(beacon.danger, 0.85) } }}>
              <CallEndIcon />
            </IconButton>
          </Tooltip>
          <Button
            onClick={onAnswer}
            startIcon={<CallIcon />}
            sx={{
              height: 46, px: 2.25, borderRadius: '23px', bgcolor: beacon.ok, color: beacon.bg, fontWeight: 800,
              '&:hover': { bgcolor: alpha(beacon.ok, 0.85) },
            }}
          >
            Answer
          </Button>
        </Box>
      ) : (
        <Tooltip title="Hang up">
          <Button
            onClick={onEnd}
            startIcon={<CallEndIcon />}
            sx={{ height: 46, px: 2.25, borderRadius: '23px', bgcolor: beacon.danger, color: '#fff', fontWeight: 800, '&:hover': { bgcolor: alpha(beacon.danger, 0.85) } }}
          >
            Hang up
          </Button>
        </Tooltip>
      )}
    </Box>
  );
};

export default CallBar;
