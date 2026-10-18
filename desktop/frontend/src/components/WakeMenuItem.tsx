/**
 * WakeMenuItem — "Let phones wake this computer from sleep" in a phone's
 * device settings menu. Turns on wake-on-magic-packet for the current
 * network (Go computer_status_linux.go); phones get the address with the
 * computer's status. Hidden in browser preview.
 */
import React, { useEffect, useState } from 'react';
import { Box, MenuItem, Switch, Typography } from '@mui/material';
import { beacon } from '../theme/theme';

const go = () => (window as any).go?.main?.App;

interface WakeStatus {
  ready: boolean;
  wifi: boolean;
  iface: string;
  error: string;
}

const WakeMenuItem: React.FC = () => {
  const [status, setStatus] = useState<WakeStatus | null>(null);
  const [note, setNote] = useState('');

  const refresh = () => go()?.GetWakeStatus().then(setStatus).catch(() => {});
  useEffect(() => {
    refresh();
  }, []);

  if (!status) return null;
  const enable = async () => {
    if (status.ready || status.error) return;
    try {
      await go().EnableWake();
      setNote('');
    } catch (e) {
      setNote(String(e));
    }
    refresh();
  };

  return (
    <MenuItem onClick={enable} disabled={!!status.error} sx={{ gap: 1.5 }}>
      <Switch size="small" checked={status.ready} />
      <Box sx={{ minWidth: 0, maxWidth: 280 }}>
        <Typography sx={{ fontSize: '0.86rem' }}>Let it wake this computer from sleep</Typography>
        <Typography sx={{ fontSize: '0.74rem', color: beacon.textSecondary, whiteSpace: 'normal' }}>
          {note ||
            status.error ||
            (status.ready
              ? `On for ${status.iface}. Works from sleep on the same network, not when shut down.`
              : `Turns on wake for ${status.iface}`)}
        </Typography>
      </Box>
    </MenuItem>
  );
};

export default WakeMenuItem;
