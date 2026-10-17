/**
 * DeviceStatusChip.tsx — a paired phone's battery and signal, set in mono.
 */
import React from 'react';
import { Box, Typography } from '@mui/material';
import BatteryFullIcon from '@mui/icons-material/BatteryFull';
import Battery20Icon from '@mui/icons-material/Battery20';
import BatteryChargingFullIcon from '@mui/icons-material/BatteryChargingFull';
import SignalCellularAltIcon from '@mui/icons-material/SignalCellularAlt';
import { beacon, fonts } from '../theme/theme';
import type { DeviceStatus } from '../types/peer';

const DeviceStatusChip: React.FC<{ status?: DeviceStatus }> = ({ status }) => {
  if (!status || (status.battery < 0 && !status.network)) return null;
  const low = status.battery >= 0 && status.battery <= 15 && !status.charging;
  const BatteryIcon = status.charging ? BatteryChargingFullIcon : low ? Battery20Icon : BatteryFullIcon;
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexShrink: 0 }}>
      {status.battery >= 0 && (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25 }}>
          <BatteryIcon sx={{ fontSize: 15, color: low ? beacon.danger : status.charging ? beacon.ok : beacon.textSecondary }} />
          <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.72rem', color: low ? beacon.danger : beacon.textSecondary }}>
            {status.battery}%
          </Typography>
        </Box>
      )}
      {status.network && (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25 }}>
          <SignalCellularAltIcon
            sx={{ fontSize: 14, color: beacon.textSecondary, opacity: status.signal < 0 ? 0.5 : 0.35 + status.signal * 0.16 }}
          />
          <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.72rem', color: beacon.textSecondary }}>
            {status.network}
          </Typography>
        </Box>
      )}
    </Box>
  );
};

export default DeviceStatusChip;
