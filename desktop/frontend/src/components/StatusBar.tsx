/**
 * StatusBar.tsx — slim instrument footer (Beacon)
 *
 * Surface strip with hairline top border. Everything here is a readout, so
 * everything is mono: discovery state, service name, last scan, version.
 */
import React from 'react';
import { Box, Typography } from '@mui/material';
import { alpha } from '@mui/material/styles';
import { beacon, fonts } from '../theme/theme';

interface StatusBarProps {
  loading: boolean;
  peerCount: number;
  lastUpdated: Date | null;
  appVersion?: string;
}

const Readout: React.FC<{ children: React.ReactNode; dim?: boolean }> = ({ children, dim }) => (
  <Typography
    sx={{
      fontFamily: fonts.mono,
      fontSize: '0.68rem',
      color: dim ? beacon.textFaint : beacon.textSecondary,
      lineHeight: 1,
      whiteSpace: 'nowrap',
    }}
  >
    {children}
  </Typography>
);

const Dot = () => (
  <Box sx={{ width: 3, height: 3, borderRadius: '50%', backgroundColor: beacon.hairline }} />
);

const StatusBar: React.FC<StatusBarProps> = ({ loading, peerCount, lastUpdated, appVersion }) => {
  const formattedTime = lastUpdated
    ? lastUpdated.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    : '—';

  return (
    <Box
      component="footer"
      id="status-bar"
      sx={{
        position: 'fixed',
        bottom: 0,
        left: 0,
        right: 0,
        height: 30,
        backgroundColor: beacon.surface,
        borderTop: `1px solid ${beacon.hairline}`,
        display: 'flex',
        alignItems: 'center',
        px: 2,
        gap: 1.5,
        zIndex: (t) => t.zIndex.appBar,
      }}
    >
      {/* Live lamp */}
      <Box
        sx={{
          width: 7,
          height: 7,
          borderRadius: '50%',
          backgroundColor: loading ? beacon.accent : beacon.ok,
          boxShadow: `0 0 6px 1px ${alpha(loading ? beacon.accent : beacon.ok, 0.4)}`,
          animation: loading ? 'status-blink 1s ease-in-out infinite' : 'none',
          '@keyframes status-blink': {
            '0%, 100%': { opacity: 1 },
            '50%': { opacity: 0.35 },
          },
          '@media (prefers-reduced-motion: reduce)': { animation: 'none' },
        }}
      />

      <Readout>
        {loading ? 'scanning…' : `${peerCount} device${peerCount !== 1 ? 's' : ''} online`}
      </Readout>
      <Dot />
      <Readout dim>_palt._tcp.local</Readout>
      <Dot />
      <Readout dim>last scan {formattedTime}</Readout>

      <Box sx={{ flex: 1 }} />

      <Typography sx={{ fontSize: '0.66rem', color: beacon.textFaint, mr: 1 }}>
        made with ❤️ by uriel
      </Typography>
      <Readout dim>{appVersion || 'dev'}</Readout>
    </Box>
  );
};

export default StatusBar;
