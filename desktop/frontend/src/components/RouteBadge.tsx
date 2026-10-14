/**
 * RouteBadge.tsx — how a peer or transfer travels (Beacon)
 *
 * DIRECT: same local network, point-to-point. RELAY: through the palt-relay
 * server (end-to-end encrypted). Direct is always preferred.
 */
import React from 'react';
import { Box, Tooltip } from '@mui/material';
import { alpha } from '@mui/material/styles';
import { beacon, fonts } from '../theme/theme';

export const routeColor = (route?: string) => (route === 'relay' ? beacon.relay : beacon.textSecondary);

export const routeLabel = (route?: string) => (route === 'relay' ? 'via relay' : 'direct');

const RouteBadge: React.FC<{ route?: string; size?: 'small' | 'medium' }> = ({
  route = 'direct',
  size = 'small',
}) => {
  const color = routeColor(route);
  return (
    <Tooltip
      title={
        route === 'relay'
          ? 'Not on this network — goes through your relay server, end-to-end encrypted'
          : 'Same network — sent device to device'
      }
    >
      <Box
        component="span"
        sx={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 0.6,
          px: size === 'small' ? 0.8 : 1,
          height: size === 'small' ? 18 : 22,
          borderRadius: 999,
          border: `1px solid ${alpha(color, 0.45)}`,
          backgroundColor: alpha(color, 0.08),
          color,
          fontFamily: fonts.mono,
          fontSize: size === 'small' ? '0.62rem' : '0.68rem',
          fontWeight: 600,
          letterSpacing: '0.06em',
          textTransform: 'uppercase',
          lineHeight: 1,
          flexShrink: 0,
          whiteSpace: 'nowrap',
        }}
      >
        <Box component="span" sx={{ width: 5, height: 5, borderRadius: '50%', backgroundColor: color }} />
        {route === 'relay' ? 'Relay' : 'Direct'}
      </Box>
    </Tooltip>
  );
};

export default RouteBadge;
