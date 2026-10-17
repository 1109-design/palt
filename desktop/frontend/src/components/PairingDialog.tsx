/**
 * PairingDialog.tsx — pair devices and set up the relay (Beacon)
 *
 * All your devices share one pairing code. Paired devices encrypt every
 * connection, accept each other's files without asking and get the live
 * features. Scan the QR with PALT on the phone (or paste the code) to pair.
 * The optional relay server reaches paired devices on other networks.
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
import QRCode from 'qrcode';
import { beacon, fonts } from '../theme/theme';

export interface RelayStatus {
  configured: boolean;
  status: 'off' | 'connecting' | 'online' | 'error';
  detail: string;
  server: string;
  code: string;
  peers: number;
}

export interface PairingInfo {
  deviceId: string;
  code: string;
  relay: string;
}

interface PairingDialogProps {
  open: boolean;
  pairing: PairingInfo | null;
  status: RelayStatus | null;
  pairedCount: number;
  onClose: () => void;
  onJoin: (code: string) => Promise<void>;
  onReset: () => Promise<void>;
  onSetRelay: (addr: string) => Promise<void>;
  onConnectIP: (addr: string) => Promise<void>;
}

export const relayStatusColor = (s?: RelayStatus | null) =>
  !s || s.status === 'off'
    ? beacon.textFaint
    : s.status === 'online'
    ? beacon.relay
    : s.status === 'error'
    ? beacon.danger
    : beacon.textSecondary;

const relayText = (s: RelayStatus | null) => {
  if (!s || !s.configured) return 'Off — paired devices are reached on the same network only';
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

const Label: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <Typography sx={{ fontSize: '0.78rem', fontWeight: 600, color: beacon.textSecondary, mb: 0.75 }}>{children}</Typography>
);

const Hint: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <Typography sx={{ fontSize: '0.74rem', color: beacon.textFaint, mt: 1 }}>{children}</Typography>
);

const PairingDialog: React.FC<PairingDialogProps> = ({
  open,
  pairing,
  status,
  pairedCount,
  onClose,
  onJoin,
  onReset,
  onSetRelay,
  onConnectIP,
}) => {
  const [qr, setQr] = useState('');
  const [joinCode, setJoinCode] = useState('');
  const [relayAddr, setRelayAddr] = useState('');
  const [ipAddr, setIpAddr] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [confirmReset, setConfirmReset] = useState(false);

  useEffect(() => {
    if (open) {
      setError(null);
      setJoinCode('');
      setConfirmReset(false);
      setRelayAddr(pairing?.relay ?? '');
    }
  }, [open, pairing?.relay]);

  useEffect(() => {
    if (!pairing?.code) return;
    QRCode.toDataURL(pairing.code, {
      margin: 1,
      width: 360,
      errorCorrectionLevel: 'M',
      color: { dark: beacon.bg, light: '#FFFFFF' },
    })
      .then(setQr)
      .catch(() => setQr(''));
  }, [pairing?.code]);

  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
      setJoinCode('');
      setConfirmReset(false);
    } catch (e: any) {
      setError(String(e?.message || e));
    } finally {
      setBusy(false);
    }
  };

  const copyCode = () => {
    if (!pairing?.code) return;
    navigator.clipboard.writeText(pairing.code);
    setCopied(true);
    setTimeout(() => setCopied(false), 1400);
  };

  const dot = relayStatusColor(status);
  const relayChanged = relayAddr.trim() !== (pairing?.relay ?? '');

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogContent sx={{ pt: 3.5, pb: 1 }}>
        <Typography variant="overline" sx={{ display: 'block', lineHeight: 1.4 }}>
          {pairedCount > 0 ? `${pairedCount} paired device${pairedCount !== 1 ? 's' : ''} connected` : 'No paired devices connected'}
        </Typography>
        <Typography
          sx={{ fontFamily: fonts.display, fontWeight: 700, fontSize: '1.2rem', letterSpacing: '-0.01em', color: beacon.text }}
        >
          Pair devices
        </Typography>
        <Typography sx={{ fontSize: '0.8rem', color: beacon.textSecondary, mt: 0.5, mb: 2 }}>
          Paired devices encrypt everything they send each other, accept each other's files without
          asking, and stay connected in the background.
        </Typography>

        {/* QR + code */}
        <Box sx={{ display: 'flex', gap: 2.5, alignItems: 'center', mb: 2.5 }}>
          <Box
            sx={{
              width: 152,
              height: 152,
              flexShrink: 0,
              borderRadius: '12px',
              overflow: 'hidden',
              backgroundColor: '#FFFFFF',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            {qr && <img src={qr} alt="Pairing QR code" width={152} height={152} />}
          </Box>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Typography sx={{ fontSize: '0.86rem', color: beacon.text, fontWeight: 600 }}>
              Scan with PALT on your phone
            </Typography>
            <Typography sx={{ fontSize: '0.78rem', color: beacon.textSecondary, mt: 0.5, mb: 1.25 }}>
              Open PALT → Pair → Scan code. Or copy the code and paste it on the other device.
            </Typography>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 0.5,
                pl: 1.5,
                pr: 0.5,
                height: 36,
                borderRadius: '8px',
                backgroundColor: beacon.bg,
                border: `1px solid ${beacon.hairline}`,
              }}
            >
              <Typography noWrap sx={{ fontFamily: fonts.mono, fontSize: '0.72rem', color: beacon.textFaint, flex: 1 }}>
                {pairing ? pairing.code.slice(0, 10) + '•'.repeat(18) : ''}
              </Typography>
              <Tooltip title={copied ? 'Copied' : 'Copy code'}>
                <IconButton size="small" onClick={copyCode} sx={{ color: beacon.textFaint }}>
                  {copied ? <CheckIcon sx={{ fontSize: 16, color: beacon.ok }} /> : <ContentCopyIcon sx={{ fontSize: 16 }} />}
                </IconButton>
              </Tooltip>
            </Box>
          </Box>
        </Box>
        <Hint>Treat the code like a password: anyone with it is one of your devices.</Hint>

        {/* Join */}
        <Box sx={{ mt: 2.5 }}>
          <Label>Or join another device's pairing</Label>
          <Box sx={{ display: 'flex', gap: 1 }}>
            <TextField
              fullWidth
              size="small"
              placeholder="palt2:…"
              value={joinCode}
              onChange={(e) => setJoinCode(e.target.value)}
              inputProps={{ 'aria-label': 'pairing code', style: { fontFamily: fonts.mono, fontSize: '0.8rem' } }}
            />
            <Button
              variant="outlined"
              disabled={busy || !joinCode.trim()}
              onClick={() => run(() => onJoin(joinCode.trim()))}
              sx={{ flexShrink: 0 }}
            >
              Join
            </Button>
          </Box>
        </Box>

        {/* By address */}
        <Box sx={{ mt: 2.5 }}>
          <Label>Paired device not showing up? Connect by its IP address</Label>
          <Box sx={{ display: 'flex', gap: 1 }}>
            <TextField
              fullWidth
              size="small"
              placeholder="e.g. 192.168.1.38"
              value={ipAddr}
              onChange={(e) => setIpAddr(e.target.value)}
              inputProps={{ 'aria-label': 'device IP address', style: { fontFamily: fonts.mono, fontSize: '0.8rem' } }}
            />
            <Button
              variant="outlined"
              disabled={busy || !ipAddr.trim()}
              onClick={() => run(async () => { await onConnectIP(ipAddr.trim()); setIpAddr(''); })}
              sx={{ flexShrink: 0 }}
            >
              Connect
            </Button>
          </Box>
        </Box>

        {/* Relay */}
        <Box sx={{ mt: 2.5 }}>
          <Label>Relay server — reach paired devices on other networks (optional)</Label>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              gap: 1,
              px: 1.5,
              py: 1,
              mb: 1,
              borderRadius: '10px',
              border: `1px solid ${alpha(dot, 0.4)}`,
              backgroundColor: alpha(dot, 0.06),
            }}
          >
            <Box sx={{ width: 8, height: 8, borderRadius: '50%', backgroundColor: dot, flexShrink: 0 }} />
            <Typography sx={{ fontSize: '0.8rem', color: beacon.text, minWidth: 0 }} noWrap>
              {relayText(status)}
            </Typography>
          </Box>
          <Box sx={{ display: 'flex', gap: 1 }}>
            <TextField
              fullWidth
              size="small"
              placeholder="e.g. 203.0.113.7 or relay.example.com:9877"
              value={relayAddr}
              onChange={(e) => setRelayAddr(e.target.value)}
              inputProps={{ 'aria-label': 'relay server address', style: { fontFamily: fonts.mono, fontSize: '0.8rem' } }}
            />
            <Button
              variant="outlined"
              disabled={busy || !relayChanged}
              onClick={() => run(() => onSetRelay(relayAddr.trim()))}
              sx={{ flexShrink: 0 }}
            >
              {relayAddr.trim() || !pairing?.relay ? 'Save' : 'Turn off'}
            </Button>
          </Box>
          <Hint>The relay is part of the code: set it before pairing so other devices get it too.</Hint>
        </Box>

        {error && <Typography sx={{ fontSize: '0.78rem', color: beacon.danger, mt: 1.5 }}>{error}</Typography>}
      </DialogContent>
      <DialogActions sx={{ p: 2.5, pt: 2, gap: 1 }}>
        {confirmReset ? (
          <>
            <Typography sx={{ fontSize: '0.78rem', color: beacon.danger, mr: 'auto' }}>
              Every other device gets unpaired until it scans the new code.
            </Typography>
            <Button variant="text" disabled={busy} onClick={() => setConfirmReset(false)}>
              Cancel
            </Button>
            <Button variant="text" disabled={busy} onClick={() => run(onReset)} sx={{ color: beacon.danger }}>
              Unpair all
            </Button>
          </>
        ) : (
          <>
            <Button variant="text" disabled={busy} onClick={() => setConfirmReset(true)} sx={{ color: beacon.danger, mr: 'auto' }}>
              New code
            </Button>
            <Button onClick={onClose} variant="contained">
              Done
            </Button>
          </>
        )}
      </DialogActions>
    </Dialog>
  );
};

export default PairingDialog;
