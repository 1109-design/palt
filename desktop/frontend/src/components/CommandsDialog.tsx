/**
 * CommandsDialog — the commands paired phones can run on this computer
 * (Go commands_app.go). Lock, Suspend and Mute are built in; the user adds
 * shell commands of their own.
 */
import React, { useEffect, useState } from 'react';
import { Box, Button, Dialog, IconButton, InputBase, Typography } from '@mui/material';
import CloseIcon from '@mui/icons-material/CloseRounded';
import DeleteIcon from '@mui/icons-material/DeleteOutline';
import TerminalIcon from '@mui/icons-material/TerminalRounded';
import { beacon, fonts } from '../theme/theme';

const go = () => (window as any).go?.main?.App;

interface Command {
  id: string;
  name: string;
  command: string;
  builtIn: boolean;
}

const field = {
  px: 1.5,
  py: 0.75,
  borderRadius: '12px',
  backgroundColor: beacon.tile,
  fontSize: '0.88rem',
};

const CommandsDialog: React.FC<{ open: boolean; onClose: () => void }> = ({ open, onClose }) => {
  const [commands, setCommands] = useState<Command[]>([]);
  const [name, setName] = useState('');
  const [command, setCommand] = useState('');
  const [error, setError] = useState('');

  const load = () => go()?.GetCommands().then((c: Command[]) => setCommands(c || [])).catch(() => {});
  useEffect(() => {
    if (open) load();
  }, [open]);

  const add = async () => {
    setError('');
    try {
      await go().SaveCommand({ id: '', name, command, builtIn: false });
      setName('');
      setCommand('');
      load();
    } catch (e) {
      setError(String(e));
    }
  };

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm" PaperProps={{ sx: { borderRadius: '24px', backgroundImage: 'none', backgroundColor: beacon.surface } }}>
      <Box sx={{ display: 'flex', alignItems: 'center', px: 2.5, pt: 2 }}>
        <Typography sx={{ fontFamily: fonts.display, fontWeight: 800, fontSize: '1.15rem', flex: 1 }}>Commands phones can run</Typography>
        <IconButton onClick={onClose} sx={{ color: beacon.textSecondary }}>
          <CloseIcon />
        </IconButton>
      </Box>
      <Typography sx={{ px: 2.5, pb: 1.5, fontSize: '0.82rem', color: beacon.textSecondary }}>
        They show on your paired phones under “Run on this computer”. Phones only see the names.
      </Typography>

      <Box sx={{ px: 1.5, pb: 1 }}>
        {commands.map((c) => (
          <Box key={c.id} sx={{ display: 'flex', alignItems: 'center', gap: 1.5, px: 1, py: 1, borderRadius: '14px', '&:hover': { backgroundColor: beacon.tile } }}>
            <TerminalIcon sx={{ fontSize: 20, color: beacon.accent }} />
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography sx={{ fontSize: '0.9rem', fontWeight: 600 }}>{c.name}</Typography>
              <Typography noWrap sx={{ fontFamily: fonts.mono, fontSize: '0.72rem', color: beacon.textFaint }}>
                {c.builtIn ? 'built in' : c.command}
              </Typography>
            </Box>
            {!c.builtIn && (
              <IconButton size="small" onClick={() => go().DeleteCommand(c.id).then(load)} sx={{ color: beacon.textSecondary }}>
                <DeleteIcon fontSize="small" />
              </IconButton>
            )}
          </Box>
        ))}
      </Box>

      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, px: 2.5, pb: 2.5 }}>
        <Typography sx={{ fontSize: '0.8rem', fontWeight: 700, color: beacon.textSecondary }}>Add a command</Typography>
        <InputBase placeholder="Name, e.g. Open Spotify" value={name} onChange={(e) => setName(e.target.value)} sx={field} />
        <InputBase
          placeholder="Shell command, e.g. spotify"
          value={command}
          onChange={(e) => setCommand(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && add()}
          sx={{ ...field, fontFamily: fonts.mono, fontSize: '0.82rem' }}
        />
        {error && <Typography sx={{ fontSize: '0.8rem', color: beacon.danger }}>{error}</Typography>}
        <Button variant="contained" disabled={!name.trim() || !command.trim()} onClick={add} sx={{ alignSelf: 'flex-end', borderRadius: 999 }}>
          Add
        </Button>
      </Box>
    </Dialog>
  );
};

export default CommandsDialog;
