/**
 * BluetoothPanel — a device's Bluetooth on its Control pane: on or off with
 * a switch, and what's connected with battery (Go bluetooth_app.go).
 *
 * Phones can't switch Bluetooth by themselves: turning one on shows
 * Android's prompt there, turning it off opens its Bluetooth settings.
 */
import React, { useEffect, useState } from 'react';
import { Box, Switch, Typography } from '@mui/material';
import BluetoothIcon from '@mui/icons-material/BluetoothRounded';
import BluetoothDisabledIcon from '@mui/icons-material/BluetoothDisabledRounded';
import HeadphonesIcon from '@mui/icons-material/HeadphonesRounded';
import MouseIcon from '@mui/icons-material/MouseRounded';
import WatchIcon from '@mui/icons-material/WatchRounded';
import BluetoothConnectedIcon from '@mui/icons-material/BluetoothConnectedRounded';
import { beacon, fonts } from '../theme/theme';

const go = () => (window as any).go?.main?.App;
const runtime = () => (window as any).runtime;

interface BluetoothState {
  available: boolean;
  on: boolean;
  devices: { name: string; kind: string; battery: number }[];
}

const kindIcon = (kind: string) => {
  const sx = { fontSize: 17, color: beacon.textSecondary };
  if (['headset', 'headphones', 'speaker'].includes(kind)) return <HeadphonesIcon sx={sx} />;
  if (['mouse', 'keyboard', 'input', 'gaming'].includes(kind)) return <MouseIcon sx={sx} />;
  if (kind === 'watch') return <WatchIcon sx={sx} />;
  return <BluetoothConnectedIcon sx={sx} />;
};

const BluetoothPanel: React.FC<{ device: string }> = ({ device }) => {
  const [st, setSt] = useState<BluetoothState | null>(null);
  const [note, setNote] = useState('');

  useEffect(() => {
    const load = () => go()?.GetPeerBluetooth(device).then(setSt).catch(() => {});
    load();
    const off = runtime()?.EventsOn?.('bluetooth_status', (name: string) => name === device && load());
    return () => off?.();
  }, [device]);

  if (!st?.available) return null;

  const toggle = async () => {
    setNote(st.on ? 'Turning off…' : 'Turning on…');
    try {
      const r: string = await go().SetPeerBluetooth(device, !st.on);
      setNote(
        r === 'asked' ? `Tap Allow on ${device}` : r === 'settings' ? `Bluetooth settings opened on ${device}` : '',
      );
    } catch (e) {
      setNote(String(e));
    }
    setTimeout(() => setNote(''), 8000);
  };

  return (
    <Box sx={{ p: 1.75, borderRadius: '20px', backgroundColor: beacon.tile }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25 }}>
        {st.on ? <BluetoothIcon sx={{ color: beacon.relay }} /> : <BluetoothDisabledIcon sx={{ color: beacon.textFaint }} />}
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography sx={{ fontSize: '0.9rem', fontWeight: 700 }}>Bluetooth</Typography>
          <Typography sx={{ fontSize: '0.76rem', color: beacon.textSecondary }}>
            {note || (!st.on ? 'Off' : st.devices.length === 0 ? 'On · nothing connected' : `On · ${st.devices.length} connected`)}
          </Typography>
        </Box>
        <Switch checked={st.on} onChange={toggle} />
      </Box>
      {st.on &&
        st.devices.map((d) => (
          <Box key={d.name} sx={{ display: 'flex', alignItems: 'center', gap: 1.25, mt: 1, pl: 0.25 }}>
            {kindIcon(d.kind)}
            <Typography noWrap sx={{ flex: 1, fontSize: '0.84rem' }}>{d.name}</Typography>
            {d.battery >= 0 && (
              <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.74rem', color: d.battery <= 15 ? beacon.danger : beacon.textSecondary }}>
                {d.battery}%
              </Typography>
            )}
          </Box>
        ))}
    </Box>
  );
};

export default BluetoothPanel;
