/**
 * RenameDialog.tsx (Beacon)
 *
 * Sets a local nickname for a peer. Replaces the old window.prompt() flow —
 * prompt() is unreliable inside the Wails webview and unstylable everywhere.
 */
import React, { useEffect, useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogActions,
  Button,
  TextField,
  Typography,
} from '@mui/material';
import type { Peer } from '../types/peer';
import { beacon, fonts } from '../theme/theme';

interface RenameDialogProps {
  peer: Peer | null;
  currentAlias: string;
  onSave: (peer: Peer, alias: string) => void;
  onClose: () => void;
}

const RenameDialog: React.FC<RenameDialogProps> = ({ peer, currentAlias, onSave, onClose }) => {
  const [value, setValue] = useState('');

  useEffect(() => {
    if (peer) setValue(currentAlias);
  }, [peer, currentAlias]);

  if (!peer) return null;

  const save = () => {
    onSave(peer, value.trim());
    onClose();
  };

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogContent sx={{ pt: 3.5, pb: 1 }}>
        <Typography variant="overline" sx={{ display: 'block', lineHeight: 1.4 }}>
          Rename device
        </Typography>
        <Typography
          noWrap
          sx={{
            fontFamily: fonts.display,
            fontWeight: 700,
            fontSize: '1.2rem',
            letterSpacing: '-0.01em',
            color: beacon.text,
            mb: 2,
          }}
        >
          {peer.deviceName}
        </Typography>
        <TextField
          autoFocus
          fullWidth
          size="small"
          placeholder="Nickname — leave empty to reset"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') save();
          }}
          inputProps={{ 'aria-label': 'device nickname' }}
        />
        <Typography sx={{ fontSize: '0.74rem', color: beacon.textFaint, mt: 1 }}>
          Only this computer sees the nickname.
        </Typography>
      </DialogContent>
      <DialogActions sx={{ p: 2.5, pt: 2, gap: 1 }}>
        <Button onClick={onClose} variant="text">
          Cancel
        </Button>
        <Button onClick={save} variant="contained">
          Save
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default RenameDialog;
