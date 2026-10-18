/**
 * UnlockMenuItem — "Unlock this computer with its fingerprint", in a
 * phone's device settings menu.
 *
 * Turning it on asks the phone for a key and one fingerprint, then the
 * system asks for the password once to trust the key (Go unlock_app.go).
 * Hidden in browser preview and where palt-unlock isn't installed.
 */
import React, { useEffect, useState } from 'react';
import { Box, CircularProgress, MenuItem, Switch, Typography } from '@mui/material';
import { beacon } from '../theme/theme';

const go = () => (window as any).go?.main?.App;

interface UnlockStatus {
  supported: boolean;
  enabled: boolean;
  device: string;
  deviceId: string;
}

const UnlockMenuItem: React.FC<{ device: string }> = ({ device }) => {
  const [status, setStatus] = useState<UnlockStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState('');

  const refresh = () => go()?.GetUnlockStatus().then(setStatus).catch(() => {});
  useEffect(() => {
    refresh();
  }, []);

  if (!status?.supported) return null;
  const mine = status.enabled && status.device === device;
  const other = status.enabled && !mine;

  const toggle = async () => {
    if (busy) return;
    setBusy(true);
    setNote(mine ? 'Confirm with your password' : 'Touch the fingerprint sensor on your phone');
    try {
      if (mine) await go().DisableUnlock();
      else await go().SetupUnlock(device);
      setNote('');
    } catch (e) {
      setNote(String(e) === 'cancelled' ? '' : String(e));
    }
    setBusy(false);
    refresh();
  };

  return (
    <MenuItem onClick={toggle} disabled={other} sx={{ gap: 1.5, alignItems: 'center' }}>
      {busy ? (
        <Box sx={{ width: 40, display: 'flex', justifyContent: 'center' }}>
          <CircularProgress size={18} />
        </Box>
      ) : (
        <Switch size="small" checked={mine} />
      )}
      <Box sx={{ minWidth: 0, maxWidth: 280 }}>
        <Typography sx={{ fontSize: '0.86rem' }}>Unlock this computer with its fingerprint</Typography>
        {(note || other) && (
          <Typography sx={{ fontSize: '0.74rem', color: beacon.textSecondary, whiteSpace: 'normal' }}>
            {note || `${status.device} unlocks it now`}
          </Typography>
        )}
      </Box>
    </MenuItem>
  );
};

export default UnlockMenuItem;
