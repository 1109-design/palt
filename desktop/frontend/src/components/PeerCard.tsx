/**
 * PeerCard.tsx — a device on the home screen (Beacon)
 *
 * Paired devices get a hero card: identity and route, battery, network and
 * what's playing, and the four actions used most (Send in amber). Devices
 * that aren't paired are a slim row with a Send button — files and text
 * only. Clicking either opens the device page. Both are drop targets:
 * dropping files on a card streams them to that peer (wired in App.tsx via
 * the Wails OnFileDrop runtime + the data-peer-* attributes below).
 */
import React, { useState } from 'react';
import { Box, Typography, IconButton, Tooltip, Button } from '@mui/material';
import EditIcon from '@mui/icons-material/EditOutlined';
import SendIcon from '@mui/icons-material/ArrowUpwardRounded';
import ChevronRightIcon from '@mui/icons-material/ChevronRightRounded';
import DownloadIcon from '@mui/icons-material/SouthWest';
import ContentPasteGoIcon from '@mui/icons-material/ContentPasteGo';
import NotificationsActiveIcon from '@mui/icons-material/NotificationsActive';
import WavingHandIcon from '@mui/icons-material/WavingHand';
import MusicNoteIcon from '@mui/icons-material/MusicNoteRounded';
import { alpha } from '@mui/material/styles';
import type { DeviceStatus, Peer } from '../types/peer';
import { getOsInfo } from '../utils/osHelpers';
import { beacon, fonts } from '../theme/theme';
import RouteBadge from './RouteBadge';
import DeviceStatusChip from './DeviceStatusChip';

interface PeerCardProps {
  peer: Peer;
  alias?: string;
  /** True while the user is dragging files over the window. */
  dragActive?: boolean;
  onSendFile?: (peer: Peer) => void;
  onRename?: (peer: Peer) => void;
  onOpen?: (peer: Peer) => void;
  /** Battery and signal reported by a paired device. */
  status?: DeviceStatus;
  /** True while the device is playing media. */
  playing?: boolean;
  /** Quick actions, paired devices only. */
  onRing?: (peer: Peer) => void;
  onPing?: (peer: Peer) => void;
  onClipboard?: (peer: Peer) => void;
}

const CardAction: React.FC<{ icon: React.ReactNode; label: string; primary?: boolean; onClick: () => void }> = ({
  icon,
  label,
  primary,
  onClick,
}) => (
  <Box
    component="button"
    onClick={(e: React.MouseEvent) => {
      e.stopPropagation();
      onClick();
    }}
    sx={{
      flex: 1,
      minWidth: 0,
      height: 64,
      border: 'none',
      borderRadius: '16px',
      cursor: 'pointer',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 0.5,
      font: 'inherit',
      fontSize: '0.78rem',
      fontWeight: primary ? 600 : 500,
      backgroundColor: primary ? beacon.accent : '#0F1626',
      color: primary ? beacon.onAccent : '#C9D3E6',
      transition: 'background-color 150ms, color 150ms',
      '&:hover': { backgroundColor: primary ? beacon.accentHover : beacon.surfaceRaised, color: primary ? beacon.onAccent : beacon.text },
      '& svg': { fontSize: 21 },
    }}
  >
    {icon}
    {label}
  </Box>
);

const PeerCard: React.FC<PeerCardProps> = ({
  peer,
  alias,
  dragActive = false,
  onSendFile,
  onRename,
  onOpen,
  status,
  playing = false,
  onRing,
  onPing,
  onClipboard,
}) => {
  const osInfo = getOsInfo(peer.os);
  const OsIcon = osInfo.Icon;
  const [hoverDrop, setHoverDrop] = useState(false);

  const displayName = alias || peer.deviceName;
  const highlighted = dragActive && hoverDrop;

  const renameButton = onRename && (
    <Tooltip title={alias ? `Rename · broadcast name ${peer.deviceName}` : 'Rename'}>
      <IconButton
        size="small"
        className="peer-rename"
        aria-label={`rename ${displayName}`}
        onClick={(e) => {
          e.stopPropagation();
          onRename(peer);
        }}
        sx={{
          p: 0.5,
          color: beacon.textFaint,
          opacity: 0,
          transition: 'opacity 150ms',
          '&:hover': { color: beacon.text },
          '&:focus-visible': { opacity: 1 },
        }}
      >
        <EditIcon sx={{ fontSize: 15 }} />
      </IconButton>
    </Tooltip>
  );

  const dropHint = (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 1,
        height: 64,
        flex: 1,
        borderRadius: '16px',
        border: `1.5px dashed ${beacon.accent}`,
        color: beacon.accent,
      }}
    >
      <DownloadIcon sx={{ fontSize: 18 }} />
      <Typography sx={{ fontSize: '0.88rem', fontWeight: 600 }}>Drop to send to {displayName}</Typography>
    </Box>
  );

  const shell = {
    id: `card-peer-${peer.id}`,
    'data-peer-ip': peer.ipAddress,
    'data-peer-port': peer.port,
    'data-peer-name': peer.deviceName,
    role: onOpen ? 'button' : undefined,
    tabIndex: onOpen ? 0 : undefined,
    'aria-label': onOpen ? `Open ${displayName}` : undefined,
    onClick: () => onOpen?.(peer),
    onKeyDown: (e: React.KeyboardEvent) => {
      if (onOpen && e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) {
        e.preventDefault();
        onOpen(peer);
      }
    },
    onDragOver: (e: React.DragEvent) => {
      e.preventDefault();
      setHoverDrop(true);
    },
    onDragLeave: () => setHoverDrop(false),
    onDrop: () => setHoverDrop(false),
  };

  const common = {
    // Wails: marks this element as a native file-drop target.
    '--wails-drop-target': 'drop',
    cursor: onOpen ? 'pointer' : 'default',
    outline: 'none',
    transition: 'border-color 150ms ease, background-color 150ms ease',
    animation: 'peer-arrive 320ms ease-out backwards',
    '@keyframes peer-arrive': {
      from: { opacity: 0, transform: 'translateY(8px)' },
      to: { opacity: 1, transform: 'translateY(0)' },
    },
    '@media (prefers-reduced-motion: reduce)': { animation: 'none' },
    '&:focus-visible': { borderColor: beacon.accent },
    '&:hover .peer-rename': { opacity: 1 },
  };

  // ── Not paired: a slim row ──────────────────────────────────────────────
  if (!peer.paired) {
    return (
      <Box
        {...shell}
        sx={{
          ...common,
          display: 'flex',
          alignItems: 'center',
          gap: 1.5,
          px: 1.75,
          py: 1.5,
          borderRadius: '18px',
          backgroundColor: highlighted ? alpha(beacon.accent, 0.06) : beacon.surface,
          border: `1px solid ${highlighted ? beacon.accent : 'transparent'}`,
          '&:hover': { borderColor: highlighted ? beacon.accent : beacon.hairline },
        }}
      >
        <Box
          sx={{
            width: 40,
            height: 40,
            borderRadius: '13px',
            backgroundColor: beacon.surfaceRaised,
            display: 'grid',
            placeItems: 'center',
            color: beacon.textSecondary,
            flexShrink: 0,
          }}
        >
          <OsIcon sx={{ fontSize: 20 }} />
        </Box>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25 }}>
            <Typography noWrap sx={{ fontSize: '0.92rem', fontWeight: 600, color: beacon.text }}>
              {displayName}
            </Typography>
            {renameButton}
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mt: 0.25 }}>
            <RouteBadge route={peer.path || peer.via} />
            <Typography noWrap sx={{ fontSize: '0.78rem', color: beacon.textSecondary }}>
              {highlighted ? 'Drop to send' : 'Files and text only'}
            </Typography>
          </Box>
        </Box>
        <Button
          id={`btn-send-${peer.id}`}
          variant="outlined"
          onClick={(e) => {
            e.stopPropagation();
            onSendFile?.(peer);
          }}
          sx={{ height: 36, px: 2, borderRadius: '12px', fontSize: '0.82rem', flexShrink: 0 }}
        >
          Send
        </Button>
      </Box>
    );
  }

  // ── Paired: the hero card ───────────────────────────────────────────────
  return (
    <Box
      {...shell}
      sx={{
        ...common,
        height: '100%',
        p: 2.25,
        borderRadius: '24px',
        background: highlighted ? alpha(beacon.accent, 0.06) : beacon.cardGradient,
        border: `1px solid ${highlighted ? beacon.accent : beacon.hairline}`,
        '&:hover': { borderColor: highlighted ? beacon.accent : beacon.textFaint },
        '&:hover .peer-chevron': { color: beacon.text, transform: 'translateX(2px)' },
      }}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.75 }}>
        <Box
          sx={{
            width: 56,
            height: 56,
            borderRadius: '18px',
            backgroundColor: alpha(beacon.accent, 0.12),
            display: 'grid',
            placeItems: 'center',
            color: beacon.accent,
            flexShrink: 0,
          }}
        >
          <OsIcon sx={{ fontSize: 28 }} />
        </Box>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25 }}>
            <Typography
              noWrap
              sx={{ fontFamily: fonts.display, fontWeight: 800, fontSize: '1.35rem', letterSpacing: '-0.02em', color: beacon.text, lineHeight: 1.2 }}
            >
              {displayName}
            </Typography>
            {renameButton}
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mt: 0.5 }}>
            <Box sx={{ width: 7, height: 7, borderRadius: '50%', backgroundColor: beacon.ok, flexShrink: 0 }} />
            <Typography sx={{ fontSize: '0.84rem', color: beacon.textSecondary }}>Connected</Typography>
            <RouteBadge route={peer.path || peer.via} />
          </Box>
        </Box>
        <ChevronRightIcon className="peer-chevron" sx={{ color: beacon.textFaint, transition: 'transform 150ms, color 150ms' }} />
      </Box>

      {(status || playing) && (
        <Box
          sx={{
            mt: 2,
            pt: 1.75,
            borderTop: `1px solid ${beacon.hairline}`,
            display: 'flex',
            alignItems: 'center',
            flexWrap: 'wrap',
            columnGap: 2.25,
            rowGap: 1,
          }}
        >
          {status && <DeviceStatusChip status={status} />}
          {playing && (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, color: beacon.textSecondary }}>
              <MusicNoteIcon sx={{ fontSize: 15 }} />
              <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.74rem' }}>Playing</Typography>
            </Box>
          )}
        </Box>
      )}

      <Box sx={{ mt: 2.25, display: 'flex', gap: 1 }}>
        {highlighted ? (
          dropHint
        ) : (
          <>
            <CardAction primary icon={<SendIcon />} label="Send" onClick={() => onSendFile?.(peer)} />
            <CardAction icon={<ContentPasteGoIcon />} label="Clipboard" onClick={() => onClipboard?.(peer)} />
            <CardAction icon={<NotificationsActiveIcon />} label="Ring" onClick={() => onRing?.(peer)} />
            <CardAction icon={<WavingHandIcon />} label="Ping" onClick={() => onPing?.(peer)} />
          </>
        )}
      </Box>
    </Box>
  );
};

export default PeerCard;
