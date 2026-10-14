/**
 * RelayDialog.tsx — remote transfers through your relay server (Beacon)
 *
 * One pairing code links all your devices to a palt-relay server. Create it
 * here (server address → Generate), then paste the same code on every other
 * device. Devices not on the same network then show up with a RELAY tag.
 */
import React, { useEffect, useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogActions,
  Button,
  TextField,
  Typography,
  Box,
  IconButton,
  Tooltip,
} from '@mui/material';
import { alpha } from '@mui/material/styles';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import CheckIcon from '@mui/icons-material/Check';
import VisibilityIcon from '@mui/icons-material/VisibilityOutlined';
import VisibilityOffIcon from '@mui/icons-material/VisibilityOffOutlined';
import { beacon, fonts } from '../theme/theme';

export interface RelayStatus {
  configured: boolean;
  status: 'off' | 'connecting' | 'online' | 'error';
  detail: string;
  server: string;
  code: string;
  peers: number;
}

interface RelayDialogProps {
  open: boolean;
  status: RelayStatus | null;
  onClose: () => void;
  onGenerate: (addr: string) => Promise<string>;
  onSetCode: (code: string) => Promise<void>;
}

export const relayStatusColor = (s?: RelayStatus | null) =>
  !s || s.status === 'off'
    ? beacon.textFaint
    : s.status === 'online'
    ? beacon.relay
    : s.status === 'error'
    ? beacon.danger
    : beacon.textSecondary;

const statusText = (s: RelayStatus | null) => {
  if (!s || !s.configured) return 'Not set up';
  switch (s.status) {
    case 'online':
      return `Online · ${s.peers} other device${s.peers !== 1 ? 's' : ''} connected`;
    case 'connecting':
      return `Connecting to ${s.server}…`;
    case 'error':
      return s.detail || 'Cannot reach relay';
    default:
      return 'Off';
  }
};

const RelayDialog: React.FC<RelayDialogProps> = ({ open, status, onClose, onGenerate, onSetCode }) => {
  const [addr, setAddr] = useState('');
  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [reveal, setReveal] = useState(false);
  const [copied, setCopied] = useState(false);
  const [changing, setChanging] = useState(false);

  useEffect(() => {
    if (open) {
      setError(null);
      setCode('');
      setReveal(false);
      setChanging(false);
    }
  }, [open]);

  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
      setChanging(false);
      setCode('');
    } catch (e: any) {
      setError(String(e?.message || e));
    } finally {
      setBusy(false);
    }
  };

  const copyCode = () => {
    if (!status?.code) return;
    navigator.clipboard.writeText(status.code);
    setCopied(true);
    setTimeout(() => setCopied(false), 1400);
  };

  const configured = !!status?.configured && !changing;
  const dot = relayStatusColor(status);

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogContent sx={{ pt: 3.5, pb: 1 }}>
        <Typography variant="overline" sx={{ display: 'block', lineHeight: 1.4 }}>
          Remote transfers
        </Typography>
        <Typography
          sx={{ fontFamily: fonts.display, fontWeight: 700, fontSize: '1.2rem', letterSpacing: '-0.01em', color: beacon.text }}
        >
          Relay server
        </Typography>
        <Typography sx={{ fontSize: '0.8rem', color: beacon.textSecondary, mt: 0.5, mb: 2 }}>
          Reach your devices when they aren't on the same network. Same-network transfers still go
          direct. Relayed transfers are end-to-end encrypted and never stored on the server.
        </Typography>

        {/* Status */}
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 1,
            px: 1.5,
            py: 1.1,
            mb: 2.5,
            borderRadius: '10px',
            border: `1px solid ${alpha(dot, 0.4)}`,
            backgroundColor: alpha(dot, 0.06),
          }}
        >
          <Box sx={{ width: 8, height: 8, borderRadius: '50%', backgroundColor: dot, flexShrink: 0 }} />
          <Typography sx={{ fontSize: '0.82rem', color: beacon.text, minWidth: 0 }} noWrap>
            {statusText(status)}
          </Typography>
          {status?.configured && (
            <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.72rem', color: beacon.textFaint, ml: 'auto' }} noWrap>
              {status.server}
            </Typography>
          )}
        </Box>

        {configured ? (
          <>
            <Typography sx={{ fontSize: '0.78rem', fontWeight: 600, color: beacon.textSecondary, mb: 0.75 }}>
              Pairing code — enter it on your other devices
            </Typography>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 0.5,
                pl: 1.5,
                pr: 0.5,
                height: 40,
                borderRadius: '8px',
                backgroundColor: beacon.bg,
                border: `1px solid ${beacon.hairline}`,
              }}
            >
              <Typography
                noWrap
                sx={{ fontFamily: fonts.mono, fontSize: '0.74rem', color: beacon.text, flex: 1, userSelect: 'all' }}
              >
                {reveal ? status!.code : status!.code.replace(/:[^:]+$/, ':' + '•'.repeat(24))}
              </Typography>
              <Tooltip title={reveal ? 'Hide' : 'Show'}>
                <IconButton size="small" onClick={() => setReveal((r) => !r)} sx={{ color: beacon.textFaint }}>
                  {reveal ? <VisibilityOffIcon sx={{ fontSize: 16 }} /> : <VisibilityIcon sx={{ fontSize: 16 }} />}
                </IconButton>
              </Tooltip>
              <Tooltip title={copied ? 'Copied' : 'Copy code'}>
                <IconButton size="small" onClick={copyCode} sx={{ color: beacon.textFaint }}>
                  {copied ? <CheckIcon sx={{ fontSize: 16, color: beacon.ok }} /> : <ContentCopyIcon sx={{ fontSize: 16 }} />}
                </IconButton>
              </Tooltip>
            </Box>
            <Typography sx={{ fontSize: '0.74rem', color: beacon.textFaint, mt: 1 }}>
              Treat it like a password: anyone with it can send files to your devices. Tip: while on the
              same Wi-Fi, send it to your phone as a PALT message and paste it there.
            </Typography>
          </>
        ) : (
          <>
            <Typography sx={{ fontSize: '0.78rem', fontWeight: 600, color: beacon.textSecondary, mb: 0.75 }}>
              Have a code from another device? Paste it
            </Typography>
            <Box sx={{ display: 'flex', gap: 1, mb: 2.5 }}>
              <TextField
                fullWidth
                size="small"
                placeholder="palt1:…"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                inputProps={{ 'aria-label': 'pairing code', style: { fontFamily: fonts.mono, fontSize: '0.8rem' } }}
              />
              <Button
                variant="contained"
                disabled={busy || !code.trim()}
                onClick={() => run(() => onSetCode(code.trim()))}
                sx={{ flexShrink: 0 }}
              >
                Connect
              </Button>
            </Box>

            <Typography sx={{ fontSize: '0.78rem', fontWeight: 600, color: beacon.textSecondary, mb: 0.75 }}>
              First device? Create a code for your relay server
            </Typography>
            <Box sx={{ display: 'flex', gap: 1 }}>
              <TextField
                fullWidth
                size="small"
                placeholder="relay address, e.g. 203.0.113.7 or relay.example.com:9877"
                value={addr}
                onChange={(e) => setAddr(e.target.value)}
                inputProps={{ 'aria-label': 'relay server address', style: { fontFamily: fonts.mono, fontSize: '0.8rem' } }}
              />
              <Button
                variant="outlined"
                disabled={busy || !addr.trim()}
                onClick={() => run(() => onGenerate(addr.trim()))}
                sx={{ flexShrink: 0 }}
              >
                Generate
              </Button>
            </Box>
            <Typography sx={{ fontSize: '0.74rem', color: beacon.textFaint, mt: 1 }}>
              Generating a new code unpairs devices that use the old one.
            </Typography>
          </>
        )}

        {error && (
          <Typography sx={{ fontSize: '0.78rem', color: beacon.danger, mt: 1.5 }}>{error}</Typography>
        )}
      </DialogContent>
      <DialogActions sx={{ p: 2.5, pt: 2, gap: 1 }}>
        {status?.configured && !changing && (
          <>
            <Button
              variant="text"
              disabled={busy}
              onClick={() => run(() => onSetCode(''))}
              sx={{ color: beacon.danger, mr: 'auto' }}
            >
              Turn off
            </Button>
            <Button variant="text" disabled={busy} onClick={() => setChanging(true)}>
              Use a different code
            </Button>
          </>
        )}
        {changing && (
          <Button variant="text" onClick={() => setChanging(false)} sx={{ mr: 'auto' }}>
            Back
          </Button>
        )}
        <Button onClick={onClose} variant={configured ? 'contained' : 'text'}>
          Done
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default RelayDialog;
