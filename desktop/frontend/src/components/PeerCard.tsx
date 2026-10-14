/**
 * PeerCard.tsx — a discovered peer (Beacon)
 *
 * Surface card with an OS tile, display-type name, mono address readout and
 * a single amber Send action. Clicking the card opens the device page
 * (thread + composer). Doubles as a drag-and-drop target: dropping
 * files anywhere on the card streams them to this peer (wired in App.tsx via
 * the Wails OnFileDrop runtime + the data-peer-* attributes below).
 */
import React, { useState } from 'react';
import { Box, Typography, IconButton, Tooltip, Button } from '@mui/material';
import EditIcon from '@mui/icons-material/EditOutlined';
import SendIcon from '@mui/icons-material/NorthEast';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import CheckIcon from '@mui/icons-material/Check';
import DownloadIcon from '@mui/icons-material/SouthWest';
import type { Peer } from '../types/peer';
import { getOsInfo } from '../utils/osHelpers';
import { alpha } from '@mui/material/styles';
import { beacon, fonts } from '../theme/theme';

interface PeerCardProps {
  peer: Peer;
  alias?: string;
  /** True while the user is dragging files over the window. */
  dragActive?: boolean;
  onSendFile?: (peer: Peer) => void;
  onRename?: (peer: Peer) => void;
  onOpen?: (peer: Peer) => void;
}

const PeerCard: React.FC<PeerCardProps> = ({
  peer,
  alias,
  dragActive = false,
  onSendFile,
  onRename,
  onOpen,
}) => {
  const osInfo = getOsInfo(peer.os);
  const OsIcon = osInfo.Icon;
  const [copied, setCopied] = useState(false);
  const [hoverDrop, setHoverDrop] = useState(false);

  const handleCopyIp = (e: React.MouseEvent) => {
    e.stopPropagation();
    navigator.clipboard.writeText(`${peer.ipAddress}:${peer.port}`);
    setCopied(true);
    setTimeout(() => setCopied(false), 1400);
  };

  const displayName = alias || peer.deviceName;
  const isAliased = !!alias;
  const highlighted = dragActive && hoverDrop;

  return (
    <Box
      id={`card-peer-${peer.id}`}
      data-peer-ip={peer.ipAddress}
      data-peer-port={peer.port}
      data-peer-name={peer.deviceName}
      role={onOpen ? 'button' : undefined}
      tabIndex={onOpen ? 0 : undefined}
      aria-label={onOpen ? `Open ${alias || peer.deviceName}` : undefined}
      onClick={() => onOpen?.(peer)}
      onKeyDown={(e) => {
        if (onOpen && e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) {
          e.preventDefault();
          onOpen(peer);
        }
      }}
      onDragOver={(e) => {
        e.preventDefault();
        setHoverDrop(true);
      }}
      onDragLeave={() => setHoverDrop(false)}
      onDrop={() => setHoverDrop(false)}
      sx={{
        // Wails: marks this element as a native file-drop target.
        '--wails-drop-target': 'drop',
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        backgroundColor: beacon.surface,
        border: `1px solid ${highlighted ? beacon.accent : beacon.hairline}`,
        borderRadius: '12px',
        p: 2.5,
        cursor: onOpen ? 'pointer' : 'default',
        outline: 'none',
        '&:focus-visible': { borderColor: beacon.accent },
        transition: 'border-color 150ms ease, transform 150ms ease, background-color 150ms ease',
        animation: 'peer-arrive 320ms ease-out backwards',
        '@keyframes peer-arrive': {
          from: { opacity: 0, transform: 'translateY(8px)' },
          to:   { opacity: 1, transform: 'translateY(0)' },
        },
        '@media (prefers-reduced-motion: reduce)': { animation: 'none' },
        ...(highlighted && { backgroundColor: alpha(beacon.accent, 0.06) }),
        '&:hover': {
          borderColor: highlighted ? beacon.accent : beacon.textFaint,
        },
        '&:hover .peer-rename': { opacity: 1 },
        '&:hover .peer-send': {
          backgroundColor: beacon.accent,
          borderColor: beacon.accent,
          color: beacon.onAccent,
        },
      }}
    >
      <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 2, flex: 1 }}>
        {/* OS tile */}
        <Box
          sx={{
            width: 48,
            height: 48,
            borderRadius: '12px',
            backgroundColor: beacon.surfaceRaised,
            border: `1px solid ${beacon.hairline}`,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: beacon.textSecondary,
            flexShrink: 0,
          }}
        >
          <OsIcon sx={{ fontSize: 24 }} />
        </Box>

        {/* Name + address */}
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
            <Tooltip title={isAliased ? `Broadcast name: ${peer.deviceName}` : ''}>
              <Typography
                noWrap
                sx={{
                  fontFamily: fonts.display,
                  fontWeight: 700,
                  fontSize: '1.05rem',
                  letterSpacing: '-0.01em',
                  color: beacon.text,
                  lineHeight: 1.3,
                }}
              >
                {displayName}
              </Typography>
            </Tooltip>
            {onRename && (
              <Tooltip title="Rename">
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
                    opacity: { xs: 1, sm: 0 },
                    transition: 'opacity 150ms',
                    '&:hover': { color: beacon.text },
                    '&:focus-visible': { opacity: 1 },
                  }}
                >
                  <EditIcon sx={{ fontSize: 15 }} />
                </IconButton>
              </Tooltip>
            )}
          </Box>

          <Typography sx={{ fontSize: '0.78rem', color: beacon.textFaint, mt: 0.25 }}>
            {osInfo.label}
          </Typography>

          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, mt: 1 }}>
            <Typography
              sx={{
                fontFamily: fonts.mono,
                fontSize: '0.74rem',
                color: beacon.textSecondary,
              }}
            >
              {peer.ipAddress}:{peer.port}
            </Typography>
            <Tooltip title={copied ? 'Copied' : 'Copy address'}>
              <IconButton
                size="small"
                aria-label="copy address"
                onClick={handleCopyIp}
                sx={{ p: 0.4, color: beacon.textFaint, '&:hover': { color: beacon.text } }}
              >
                {copied ? (
                  <CheckIcon sx={{ fontSize: 13, color: beacon.ok }} />
                ) : (
                  <ContentCopyIcon sx={{ fontSize: 13 }} />
                )}
              </IconButton>
            </Tooltip>
          </Box>
        </Box>
      </Box>

      {/* Action row */}
      <Box sx={{ mt: 2.5 }}>
        {highlighted ? (
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 1,
              height: 40,
              borderRadius: '8px',
              border: `1.5px dashed ${beacon.accent}`,
              color: beacon.accent,
            }}
          >
            <DownloadIcon sx={{ fontSize: 16 }} />
            <Typography sx={{ fontSize: '0.82rem', fontWeight: 600 }}>
              Drop to send
            </Typography>
          </Box>
        ) : (
          <Button
            id={`btn-send-${peer.id}`}
            className="peer-send"
            variant="outlined"
            fullWidth
            endIcon={<SendIcon sx={{ fontSize: 16 }} />}
            onClick={(e) => {
              e.stopPropagation();
              onSendFile?.(peer);
            }}
            sx={{
              height: 40,
              fontSize: '0.85rem',
              color: beacon.textSecondary,
              transition: 'background-color 150ms, color 150ms, border-color 150ms',
            }}
          >
            Send files
          </Button>
        )}
      </Box>
    </Box>
  );
};

export default PeerCard;
