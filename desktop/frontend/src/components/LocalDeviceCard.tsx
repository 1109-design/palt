/**
 * LocalDeviceCard.tsx — "This device" identity strip (Beacon)
 *
 * Your machine drawn as a beacon: OS tile wrapped in pulsing rings while
 * discovery broadcasts, name in display type, address as a mono readout.
 */
import React from 'react';
import { Box, Typography, Tooltip } from '@mui/material';
import { alpha } from '@mui/material/styles';
import type { Peer } from '../types/peer';
import { getOsInfo } from '../utils/osHelpers';
import BeaconPulse from './BeaconPulse';
import { beacon, fonts } from '../theme/theme';

interface LocalDeviceCardProps {
  device: Peer | null;
  /** True while discovery is actively broadcasting/scanning. */
  broadcasting?: boolean;
}

const LocalDeviceCard: React.FC<LocalDeviceCardProps> = ({ device, broadcasting = true }) => {
  if (!device) return null;

  const osInfo = getOsInfo(device.os);
  const OsIcon = osInfo.Icon;

  return (
    <Box
      sx={{
        backgroundColor: beacon.surface,
        border: `1px solid ${beacon.hairline}`,
        borderRadius: '16px',
        px: 4,
        py: 3.5,
        mb: 4,
        display: 'flex',
        alignItems: 'center',
        gap: 3.5,
        overflow: 'hidden',
        position: 'relative',
      }}
    >
      {/* OS tile inside the beacon rings */}
      <BeaconPulse active={broadcasting} size={64}>
        <Box
          sx={{
            width: 64,
            height: 64,
            borderRadius: '50%',
            backgroundColor: beacon.surfaceRaised,
            border: `1px solid ${beacon.hairline}`,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: beacon.accent,
          }}
        >
          <OsIcon sx={{ fontSize: 30 }} />
        </Box>
      </BeaconPulse>

      {/* Identity */}
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography variant="overline" sx={{ display: 'block', lineHeight: 1.4 }}>
          This device
        </Typography>
        <Typography
          noWrap
          sx={{
            fontFamily: fonts.display,
            fontWeight: 800,
            fontSize: '1.75rem',
            letterSpacing: '-0.02em',
            color: beacon.text,
            lineHeight: 1.15,
            mt: 0.25,
          }}
        >
          {device.deviceName}
        </Typography>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mt: 1.25 }}>
          <Box
            sx={{
              px: 1.25,
              py: 0.5,
              borderRadius: '6px',
              backgroundColor: beacon.bg,
              border: `1px solid ${beacon.hairline}`,
            }}
          >
            <Typography
              sx={{
                fontFamily: fonts.mono,
                fontSize: '0.78rem',
                color: beacon.textSecondary,
                lineHeight: 1.4,
              }}
            >
              {device.ipAddress}:{device.port}
            </Typography>
          </Box>
          <Typography sx={{ fontSize: '0.82rem', fontWeight: 500, color: beacon.textFaint }}>
            {osInfo.label}
          </Typography>
        </Box>
      </Box>

      {/* Broadcast status */}
      <Tooltip title="Announcing on _palt._tcp.local">
        <Box
          id="chip-local-status"
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 1,
            px: 1.75,
            height: 34,
            borderRadius: 999,
            border: `1px solid ${alpha(beacon.accent, 0.35)}`,
            backgroundColor: alpha(beacon.accent, 0.08),
            flexShrink: 0,
          }}
        >
          <Box
            sx={{
              width: 7,
              height: 7,
              borderRadius: '50%',
              backgroundColor: beacon.accent,
              boxShadow: `0 0 8px 1px ${alpha(beacon.accent, 0.5)}`,
            }}
          />
          <Typography
            sx={{
              fontSize: '0.76rem',
              fontWeight: 600,
              color: beacon.accent,
              letterSpacing: '0.04em',
            }}
          >
            Broadcasting
          </Typography>
        </Box>
      </Tooltip>
    </Box>
  );
};

export default LocalDeviceCard;
