/**
 * DeviceStatusPanel.tsx — a paired device's vitals as three instrument
 * tiles under its title: battery, network, and how PALT reaches it.
 */
import React from 'react';
import { Box, Typography } from '@mui/material';
import WifiIcon from '@mui/icons-material/Wifi';
import CableIcon from '@mui/icons-material/Cable';
import SignalCellularAltIcon from '@mui/icons-material/SignalCellularAlt';
import SignalWifiOffIcon from '@mui/icons-material/SignalWifiOff';
import BoltIcon from '@mui/icons-material/Bolt';
import LockIcon from '@mui/icons-material/Lock';
import { beacon, fonts } from '../theme/theme';
import type { DeviceStatus, Peer } from '../types/peer';
import { networkLabel } from './DeviceStatusChip';

const Tile: React.FC<{ label: string; footer: React.ReactNode; children: React.ReactNode }> = ({ label, footer, children }) => (
  <Box
    sx={{
      flex: 1,
      minWidth: 0,
      height: 88,
      px: 1.75,
      py: 1.25,
      borderRadius: '12px',
      border: `1px solid ${beacon.hairline}`,
      backgroundColor: beacon.surface,
      display: 'flex',
      flexDirection: 'column',
      justifyContent: 'space-between',
    }}
  >
    <Typography variant="overline" sx={{ lineHeight: 1.2 }}>
      {label}
    </Typography>
    {children}
    <Typography noWrap sx={{ fontFamily: fonts.mono, fontSize: '0.7rem', color: beacon.textFaint }}>
      {footer}
    </Typography>
  </Box>
);

/** Horizontal battery glyph filled to the level. */
const Battery: React.FC<{ level: number; color: string }> = ({ level, color }) => (
  <Box sx={{ display: 'flex', alignItems: 'center' }}>
    <Box sx={{ width: 34, height: 16, p: '2px', borderRadius: '4px', border: `1.5px solid ${color}aa` }}>
      <Box sx={{ width: `${Math.max(0, Math.min(100, level))}%`, height: '100%', borderRadius: '2px', backgroundColor: color }} />
    </Box>
    <Box sx={{ width: 2.5, height: 7, borderRadius: '0 1px 1px 0', backgroundColor: `${color}aa` }} />
  </Box>
);

const Bars: React.FC<{ level: number }> = ({ level }) => (
  <Box sx={{ display: 'flex', alignItems: 'flex-end', gap: '2px', height: 16 }}>
    {[1, 2, 3, 4].map((i) => (
      <Box key={i} sx={{ width: 4, height: 3 + i * 3, borderRadius: '1px', backgroundColor: i <= level ? beacon.text : beacon.hairline }} />
    ))}
  </Box>
);

const DeviceStatusPanel: React.FC<{ status?: DeviceStatus; peer: Peer; online: boolean }> = ({ status, peer, online }) => {
  const s: DeviceStatus = status ?? { battery: -1, charging: false, network: '', ssid: '', cellular: '', signal: -1 };
  const low = s.battery >= 0 && s.battery <= 15 && !s.charging;
  const batColor = low ? beacon.danger : s.charging ? beacon.ok : beacon.text;
  const net = networkLabel(s);
  const NetIcon = s.network === 'wifi' ? WifiIcon : s.network === 'ethernet' ? CableIcon : s.network === 'cellular' ? SignalCellularAltIcon : SignalWifiOffIcon;
  const relay = peer.via === 'relay';

  return (
    <Box sx={{ display: 'flex', gap: 1.25, mt: 2 }}>
      <Tile
        label="Battery"
        footer={
          s.battery < 0 ? (status ? 'no battery' : 'not reported yet') : s.charging ? (
            <Box component="span" sx={{ color: beacon.ok, display: 'inline-flex', alignItems: 'center' }}>
              <BoltIcon sx={{ fontSize: 12 }} /> charging
            </Box>
          ) : low ? (
            <Box component="span" sx={{ color: beacon.danger }}>low</Box>
          ) : (
            'on battery'
          )
        }
      >
        {s.battery < 0 ? (
          <Typography sx={{ fontFamily: fonts.mono, fontSize: '1.1rem', color: beacon.textFaint }}>—</Typography>
        ) : (
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <Typography sx={{ fontFamily: fonts.display, fontWeight: 800, fontSize: '1.6rem', lineHeight: 1, color: batColor }}>
              {s.battery}
              <Box component="span" sx={{ fontFamily: fonts.mono, fontSize: '0.8rem', fontWeight: 500, color: beacon.textSecondary, ml: 0.25 }}>
                %
              </Box>
            </Typography>
            <Battery level={s.battery} color={batColor} />
          </Box>
        )}
      </Tile>

      <Tile
        label="Network"
        footer={s.network === 'cellular' ? 'on mobile data' : s.network === 'wifi' ? 'Wi-Fi' : s.network === 'ethernet' ? 'wired' : status ? 'unknown' : 'not reported yet'}
      >
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, minWidth: 0 }}>
          {s.network === 'cellular' && s.signal >= 0 ? <Bars level={s.signal} /> : <NetIcon sx={{ fontSize: 20, color: net ? beacon.text : beacon.textFaint }} />}
          <Typography noWrap sx={{ fontFamily: fonts.display, fontWeight: 700, fontSize: '1.02rem', color: net ? beacon.text : beacon.textFaint, minWidth: 0 }}>
            {net || '—'}
          </Typography>
        </Box>
      </Tile>

      <Tile label="Connection" footer={relay ? 'end-to-end encrypted' : online ? `${peer.ipAddress}:${peer.port}` : 'offline'}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <LockIcon sx={{ fontSize: 17, color: relay ? beacon.relay : beacon.ok }} />
          <Typography sx={{ fontFamily: fonts.display, fontWeight: 700, fontSize: '1.02rem', color: beacon.text }}>
            {!online ? 'Offline' : relay ? 'Via relay' : 'Direct'}
          </Typography>
        </Box>
      </Tile>
    </Box>
  );
};

export default DeviceStatusPanel;
