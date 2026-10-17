/**
 * LocalDeviceCard.tsx — "this computer" strip at the top of home (Beacon)
 *
 * One slim line: the name other devices see, that it's visible nearby,
 * and the address they reach it at.
 */
import React from 'react';
import { Box, Typography, Tooltip } from '@mui/material';
import { alpha } from '@mui/material/styles';
import type { Peer } from '../types/peer';
import { getOsInfo } from '../utils/osHelpers';
import { beacon, fonts } from '../theme/theme';

interface LocalDeviceCardProps {
  device: Peer | null;
  /** True while discovery is actively broadcasting/scanning. */
  broadcasting?: boolean;
}

const LocalDeviceCard: React.FC<LocalDeviceCardProps> = ({ device, broadcasting = true }) => {
  if (!device) return null;

  const OsIcon = getOsInfo(device.os).Icon;

  return (
    <Box
      sx={{
        backgroundColor: beacon.surface,
        borderRadius: '16px',
        px: 2,
        py: 1.5,
        display: 'flex',
        alignItems: 'center',
        gap: 1.75,
      }}
    >
      <OsIcon sx={{ fontSize: 22, color: beacon.accent }} />
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography noWrap sx={{ fontSize: '0.9rem', fontWeight: 600, color: beacon.text }}>
          This computer · {device.deviceName}
        </Typography>
        <Typography noWrap sx={{ fontFamily: fonts.mono, fontSize: '0.72rem', color: beacon.textSecondary, mt: 0.25 }}>
          {broadcasting ? 'Visible to nearby' : 'Hidden'} · {device.ipAddress}
        </Typography>
      </Box>
      <Tooltip title="Announcing on _palt._tcp.local">
        <Box
          id="chip-local-status"
          sx={{
            width: 9,
            height: 9,
            mr: 0.5,
            borderRadius: '50%',
            backgroundColor: broadcasting ? beacon.accent : beacon.textFaint,
            boxShadow: broadcasting ? `0 0 10px 2px ${alpha(beacon.accent, 0.45)}` : 'none',
          }}
        />
      </Tooltip>
    </Box>
  );
};

export default LocalDeviceCard;
