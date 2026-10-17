/**
 * RouteBadge.tsx — how a peer or transfer travels (Beacon)
 *
 * USB: over the cable (USB tethering). WI-FI / NETWORK: same local network,
 * point-to-point. RELAY: through the palt-relay server (end-to-end
 * encrypted). DIRECT: same network, path unknown (transfers).
 *
 * `route` takes a transfer route ("direct" | "relay") or a peer's path
 * ("usb" | "wifi" | "lan" | "relay").
 */
import React from 'react';
import { Box, Tooltip } from '@mui/material';
import { alpha } from '@mui/material/styles';
import { beacon, fonts } from '../theme/theme';

const PATHS: Record<string, { short: string; label: string; tip: string }> = {
  usb: { short: 'USB', label: 'USB cable', tip: 'Over the USB cable — works without Wi-Fi or data' },
  wifi: { short: 'Wi-Fi', label: 'same Wi-Fi', tip: 'Same Wi-Fi — sent device to device' },
  lan: { short: 'Network', label: 'same network', tip: 'Same network — sent device to device' },
  relay: { short: 'Relay', label: 'via relay', tip: 'Not on this network — goes through your relay server, end-to-end encrypted' },
  direct: { short: 'Direct', label: 'direct', tip: 'Same network — sent device to device' },
};

const pathInfo = (route?: string) => PATHS[route || 'direct'] ?? PATHS.direct;

export const routeColor = (route?: string) =>
  route === 'relay' ? beacon.relay : route === 'usb' ? beacon.ok : beacon.textSecondary;

export const routeLabel = (route?: string) => pathInfo(route).label;

const RouteBadge: React.FC<{ route?: string; size?: 'small' | 'medium' }> = ({
  route = 'direct',
  size = 'small',
}) => {
  const color = routeColor(route);
  return (
    <Tooltip title={pathInfo(route).tip}>
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
        {pathInfo(route).short}
      </Box>
    </Tooltip>
  );
};

export default RouteBadge;
