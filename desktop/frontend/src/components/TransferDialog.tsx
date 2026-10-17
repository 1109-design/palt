/**
 * TransferDialog.tsx — incoming offer (Beacon)
 *
 * The safety gate: a sender wants to transmit. Sender identity up top,
 * file manifest as a mono readout, explicit Decline / Accept.
 */
import React from 'react';
import {
  Dialog,
  DialogContent,
  DialogActions,
  Button,
  Typography,
  Box,
} from '@mui/material';
import { alpha } from '@mui/material/styles';
import SouthWestIcon from '@mui/icons-material/SouthWest';
import { beacon, fonts } from '../theme/theme';
import type { Route } from '../types/peer';
import RouteBadge from './RouteBadge';

export interface FileMeta {
  name: string;
  size: number;
}

export interface OfferData {
  transferId: string;
  files: FileMeta[];
  totalSize: number;
  senderName: string;
  route?: Route;
}

interface TransferDialogProps {
  open: boolean;
  offer: OfferData | null;
  onAccept: (transferId: string) => void;
  onReject: (transferId: string) => void;
}

const formatBytes = (bytes: number) => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
};

export const TransferDialog: React.FC<TransferDialogProps> = ({
  open,
  offer,
  onAccept,
  onReject,
}) => {
  if (!offer) return null;

  const fileCount = offer.files.length;

  return (
    <Dialog
      open={open}
      onClose={() => onReject(offer.transferId)}
      maxWidth="xs"
      fullWidth
    >
      <DialogContent sx={{ pt: 3.5, pb: 1 }}>
        {/* Sender identity */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2.5 }}>
          <Box
            sx={{
              width: 44,
              height: 44,
              borderRadius: '50%',
              backgroundColor: alpha(beacon.ok, 0.1),
              color: beacon.ok,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flexShrink: 0,
            }}
          >
            <SouthWestIcon sx={{ fontSize: 20 }} />
          </Box>
          <Box sx={{ minWidth: 0 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <Typography variant="overline" sx={{ display: 'block', lineHeight: 1.4 }}>
                Incoming transfer
              </Typography>
              <RouteBadge route={offer.route} />
            </Box>
            <Typography
              noWrap
              sx={{
                fontFamily: fonts.display,
                fontWeight: 700,
                fontSize: '1.15rem',
                letterSpacing: '-0.01em',
                color: beacon.text,
              }}
            >
              {offer.senderName}
            </Typography>
          </Box>
        </Box>

        {/* File manifest */}
        <Box
          sx={{
            backgroundColor: beacon.bg,
            border: `1px solid ${beacon.hairline}`,
            borderRadius: '10px',
            px: 2,
            py: 1.5,
          }}
        >
          <Box sx={{ maxHeight: 168, overflowY: 'auto' }}>
            {offer.files.map((file, idx) => (
              <Box
                key={idx}
                sx={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'baseline',
                  gap: 2,
                  py: 0.5,
                }}
              >
                <Typography
                  noWrap
                  sx={{ fontSize: '0.82rem', color: beacon.text, minWidth: 0 }}
                >
                  {file.name}
                </Typography>
                <Typography
                  sx={{
                    fontFamily: fonts.mono,
                    fontSize: '0.72rem',
                    color: beacon.textFaint,
                    flexShrink: 0,
                  }}
                >
                  {formatBytes(file.size)}
                </Typography>
              </Box>
            ))}
          </Box>
          <Box
            sx={{
              display: 'flex',
              justifyContent: 'space-between',
              borderTop: `1px solid ${beacon.hairline}`,
              mt: 1,
              pt: 1,
            }}
          >
            <Typography sx={{ fontSize: '0.76rem', color: beacon.textSecondary }}>
              {fileCount} file{fileCount !== 1 ? 's' : ''}
            </Typography>
            <Typography
              sx={{ fontFamily: fonts.mono, fontSize: '0.76rem', fontWeight: 600, color: beacon.text }}
            >
              {formatBytes(offer.totalSize)}
            </Typography>
          </Box>
        </Box>

        <Typography sx={{ fontSize: '0.76rem', color: beacon.textFaint, mt: 1.5 }}>
          {offer.senderName} isn't paired. Pair it (the link icon at the top) and its files come in
          without asking.
        </Typography>
      </DialogContent>

      <DialogActions sx={{ p: 2.5, pt: 1.5, gap: 1 }}>
        <Button onClick={() => onReject(offer.transferId)} variant="outlined" sx={{ flex: 1 }}>
          Decline
        </Button>
        <Button
          onClick={() => onAccept(offer.transferId)}
          variant="contained"
          sx={{ flex: 1 }}
        >
          Accept
        </Button>
      </DialogActions>
    </Dialog>
  );
};
