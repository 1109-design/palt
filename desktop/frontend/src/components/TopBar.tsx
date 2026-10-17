/**
 * TopBar.tsx — PALT App Bar (Beacon)
 *
 * Ink surface with a hairline underline:
 *   - Beacon mark + PALT wordmark on the left.
 *   - Centered search field for filtering peers.
 *   - Update chip, live device count, refresh on the right.
 */
import React from 'react';
import {
  AppBar,
  Toolbar,
  Typography,
  IconButton,
  InputBase,
  Box,
  Tooltip,
  CircularProgress,
} from '@mui/material';
import RefreshIcon from '@mui/icons-material/Refresh';
import SearchIcon from '@mui/icons-material/Search';
import { alpha } from '@mui/material/styles';
import LinkIcon from '@mui/icons-material/Link';
import UpdateBanner from './UpdateBanner';
import { relayStatusColor } from './PairingDialog';
import type { RelayStatus } from './PairingDialog';
import { beacon, fonts } from '../theme/theme';

interface TopBarProps {
  peerCount: number;
  loading: boolean;
  searchQuery: string;
  onSearchChange: (v: string) => void;
  onRefresh: () => void;
  updateAvailable?: boolean;
  latestVersion?: string;
  onUpdateClick?: () => void;
  relayStatus?: RelayStatus | null;
  onRelayClick?: () => void;
}

const TopBar: React.FC<TopBarProps> = ({
  peerCount,
  loading,
  searchQuery,
  onSearchChange,
  onRefresh,
  updateAvailable = false,
  latestVersion = '',
  onUpdateClick,
  relayStatus,
  onRelayClick,
}) => {
  return (
    <AppBar
      position="fixed"
      elevation={0}
      sx={{
        backgroundColor: alpha(beacon.bg, 0.88),
        backdropFilter: 'blur(12px)',
        borderBottom: `1px solid ${beacon.hairline}`,
        color: beacon.text,
      }}
    >
      <Toolbar sx={{ gap: 1.5, minHeight: { xs: 60 } }}>
        {/* ── Beacon mark + wordmark ─────────────────────────────────────── */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25, minWidth: 140 }}>
          <Box
            sx={{
              width: 34,
              height: 34,
              borderRadius: '10px',
              border: `1px solid ${beacon.hairline}`,
              backgroundColor: beacon.surface,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            {/* The transmit lamp */}
            <Box
              sx={{
                width: 10,
                height: 10,
                borderRadius: '50%',
                backgroundColor: beacon.accent,
                boxShadow: `0 0 10px 2px ${alpha(beacon.accent, 0.5)}`,
                animation: 'lamp-breathe 2.8s ease-in-out infinite',
                '@keyframes lamp-breathe': {
                  '0%, 100%': { opacity: 1 },
                  '50%': { opacity: 0.55 },
                },
                '@media (prefers-reduced-motion: reduce)': { animation: 'none' },
              }}
            />
          </Box>
          <Typography
            sx={{
              fontFamily: fonts.display,
              fontWeight: 800,
              fontSize: '1.2rem',
              letterSpacing: '0.02em',
              color: beacon.text,
              userSelect: 'none',
            }}
          >
            PALT
          </Typography>
        </Box>

        {/* ── Search ─────────────────────────────────────────────────────── */}
        <Box
          sx={{
            flex: 1,
            maxWidth: 480,
            mx: 'auto',
            display: 'flex',
            alignItems: 'center',
            backgroundColor: beacon.surface,
            borderRadius: '8px',
            px: 1.5,
            height: 38,
            border: `1px solid ${beacon.hairline}`,
            transition: 'border-color 150ms',
            '&:focus-within': { borderColor: beacon.accent },
          }}
        >
          <SearchIcon sx={{ color: beacon.textFaint, mr: 1, fontSize: 19 }} />
          <InputBase
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder="Filter devices…"
            inputProps={{ 'aria-label': 'filter devices' }}
            sx={{ flex: 1, fontSize: '0.86rem', color: beacon.text }}
          />
        </Box>

        {/* ── Right actions ──────────────────────────────────────────────── */}
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 1,
            minWidth: 140,
            justifyContent: 'flex-end',
          }}
        >
          {updateAvailable && latestVersion && onUpdateClick && (
            <UpdateBanner latestVersion={latestVersion} onClick={onUpdateClick} />
          )}

          {/* Live device count readout */}
          <Tooltip title={`${peerCount} device${peerCount !== 1 ? 's' : ''} reachable`}>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 0.75,
                px: 1.25,
                height: 28,
                borderRadius: 999,
                border: `1px solid ${beacon.hairline}`,
              }}
            >
              <Box
                sx={{
                  width: 6,
                  height: 6,
                  borderRadius: '50%',
                  backgroundColor: peerCount > 0 ? beacon.ok : beacon.textFaint,
                  transition: 'background-color 300ms',
                }}
              />
              <Typography
                sx={{
                  fontFamily: fonts.mono,
                  fontSize: '0.72rem',
                  fontWeight: 600,
                  color: beacon.textSecondary,
                  lineHeight: 1,
                }}
              >
                {peerCount}
              </Typography>
            </Box>
          </Tooltip>

          {onRelayClick && (
            <Tooltip title={`Pair devices · relay ${relayStatus?.configured ? relayStatus.status : 'off'}`}>
              <IconButton
                id="btn-relay"
                onClick={onRelayClick}
                aria-label="pair devices and relay settings"
                sx={{
                  position: 'relative',
                  color: relayStatus?.status === 'online' ? beacon.relay : beacon.textSecondary,
                  '&:hover': { color: beacon.text, backgroundColor: alpha(beacon.text, 0.06) },
                }}
              >
                <LinkIcon sx={{ fontSize: 20 }} />
                <Box
                  sx={{
                    position: 'absolute',
                    top: 9,
                    right: 9,
                    width: 7,
                    height: 7,
                    borderRadius: '50%',
                    border: `1.5px solid ${beacon.bg}`,
                    backgroundColor: relayStatusColor(relayStatus),
                  }}
                />
              </IconButton>
            </Tooltip>
          )}

          <Tooltip title="Scan again">
            <span>
              <IconButton
                id="btn-refresh"
                onClick={onRefresh}
                disabled={loading}
                size="medium"
                aria-label="scan for devices"
                sx={{
                  color: beacon.textSecondary,
                  '&:hover': { color: beacon.text, backgroundColor: alpha(beacon.text, 0.06) },
                }}
              >
                {loading ? (
                  <CircularProgress size={19} thickness={4} sx={{ color: beacon.accent }} />
                ) : (
                  <RefreshIcon sx={{ fontSize: 21 }} />
                )}
              </IconButton>
            </span>
          </Tooltip>
        </Box>
      </Toolbar>
    </AppBar>
  );
};

export default TopBar;
