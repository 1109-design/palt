/**
 * DeviceStatusChip.tsx — a paired device's battery and network, set in mono:
 * "82% ⚡  ·  Wi-Fi HomeNet" or "41%  ·  5G ▂▄▆_".
 */
import React from 'react';
import { Box, Tooltip, Typography } from '@mui/material';
import BatteryFullIcon from '@mui/icons-material/BatteryFull';
import Battery20Icon from '@mui/icons-material/Battery20';
import BatteryChargingFullIcon from '@mui/icons-material/BatteryChargingFull';
import WifiIcon from '@mui/icons-material/Wifi';
import SignalCellularAltIcon from '@mui/icons-material/SignalCellularAlt';
import CableIcon from '@mui/icons-material/Cable';
import { beacon, fonts } from '../theme/theme';
import type { DeviceStatus } from '../types/peer';

/** Four signal bars, lit up to `level` (0-4). */
const Bars: React.FC<{ level: number }> = ({ level }) => (
  <Box sx={{ display: 'flex', alignItems: 'flex-end', gap: '1.5px', height: 11 }}>
    {[1, 2, 3, 4].map((i) => (
      <Box
        key={i}
        sx={{
          width: 3,
          height: 2 + i * 2.25,
          borderRadius: '1px',
          backgroundColor: i <= level ? beacon.text : beacon.hairline,
        }}
      />
    ))}
  </Box>
);

export const networkLabel = (s: DeviceStatus) =>
  s.network === 'wifi'
    ? s.ssid || 'Wi-Fi'
    : s.network === 'cellular'
    ? `Mobile data${s.cellular ? ` · ${s.cellular}` : ''}`
    : s.network === 'ethernet'
    ? 'Ethernet'
    : s.network === 'usb'
    ? 'USB cable only'
    : '';

const DeviceStatusChip: React.FC<{ status?: DeviceStatus; compact?: boolean }> = ({ status, compact }) => {
  if (!status) return null;
  const hasBattery = status.battery >= 0;
  const net = networkLabel(status);
  if (!hasBattery && !net) return null;
  const low = hasBattery && status.battery <= 15 && !status.charging;
  const BatteryIcon = status.charging ? BatteryChargingFullIcon : low ? Battery20Icon : BatteryFullIcon;
  const text = { fontFamily: fonts.mono, fontSize: '0.72rem', color: beacon.textSecondary, whiteSpace: 'nowrap' } as const;
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25, minWidth: 0 }}>
      {hasBattery && (
        <Tooltip title={status.charging ? 'Charging' : 'Battery'}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25, flexShrink: 0 }}>
            <BatteryIcon sx={{ fontSize: 15, color: low ? beacon.danger : status.charging ? beacon.ok : beacon.textSecondary }} />
            <Typography sx={{ ...text, color: low ? beacon.danger : beacon.textSecondary }}>{status.battery}%</Typography>
          </Box>
        </Tooltip>
      )}
      {net && (
        <Tooltip title={status.network === 'cellular' ? 'On mobile data' : status.network === 'wifi' ? 'Wi-Fi network' : 'Wired network'}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, minWidth: 0 }}>
            {status.network === 'wifi' && <WifiIcon sx={{ fontSize: 14, color: beacon.textSecondary }} />}
            {status.network === 'ethernet' && <CableIcon sx={{ fontSize: 14, color: beacon.textSecondary }} />}
            {status.network === 'cellular' &&
              (status.signal >= 0 ? <Bars level={status.signal} /> : <SignalCellularAltIcon sx={{ fontSize: 14, color: beacon.textSecondary }} />)}
            {!compact && <Typography noWrap sx={{ ...text, minWidth: 0 }}>{net}</Typography>}
          </Box>
        </Tooltip>
      )}
    </Box>
  );
};

export default DeviceStatusChip;
