/**
 * UpdateDialog.tsx (Beacon)
 *
 * Shown when the user clicks the update chip. Version diff as a mono
 * readout, release notes, and a direct .deb download.
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
import ArrowForwardIcon from '@mui/icons-material/ArrowForward';
import OpenInNewIcon from '@mui/icons-material/OpenInNew';
import { beacon, fonts } from '../theme/theme';

export interface UpdateDialogProps {
  open: boolean;
  currentVersion: string;
  latestVersion: string;
  releaseNotes: string;
  downloadUrl: string;
  onDownload: () => void;
  onClose: () => void;
}

const VersionTag: React.FC<{ version: string; highlight?: boolean }> = ({
  version,
  highlight,
}) => (
  <Box
    sx={{
      px: 1.25,
      py: 0.5,
      borderRadius: '6px',
      border: `1px solid ${highlight ? alpha(beacon.accent, 0.5) : beacon.hairline}`,
      backgroundColor: highlight ? alpha(beacon.accent, 0.1) : beacon.bg,
    }}
  >
    <Typography
      sx={{
        fontFamily: fonts.mono,
        fontSize: '0.78rem',
        fontWeight: 600,
        color: highlight ? beacon.accent : beacon.textSecondary,
      }}
    >
      {version}
    </Typography>
  </Box>
);

const UpdateDialog: React.FC<UpdateDialogProps> = ({
  open,
  currentVersion,
  latestVersion,
  releaseNotes,
  onDownload,
  onClose,
}) => {
  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogContent sx={{ pt: 3.5, pb: 1 }}>
        <Typography variant="overline" sx={{ display: 'block', lineHeight: 1.4 }}>
          Update available
        </Typography>
        <Typography
          sx={{
            fontFamily: fonts.display,
            fontWeight: 700,
            fontSize: '1.3rem',
            letterSpacing: '-0.02em',
            color: beacon.text,
            mb: 2,
          }}
        >
          A newer PALT is out
        </Typography>

        {/* Version diff */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 2.5 }}>
          <VersionTag version={currentVersion} />
          <ArrowForwardIcon sx={{ color: beacon.textFaint, fontSize: 16 }} />
          <VersionTag version={latestVersion} highlight />
        </Box>

        {/* Release notes */}
        <Typography variant="overline" sx={{ display: 'block', mb: 1 }}>
          What changed
        </Typography>
        <Box
          sx={{
            maxHeight: 240,
            overflowY: 'auto',
            backgroundColor: beacon.bg,
            border: `1px solid ${beacon.hairline}`,
            borderRadius: '10px',
            p: 2,
          }}
        >
          <Typography
            sx={{
              fontSize: '0.82rem',
              color: beacon.textSecondary,
              whiteSpace: 'pre-wrap',
              lineHeight: 1.7,
            }}
          >
            {releaseNotes || 'No release notes available.'}
          </Typography>
        </Box>
      </DialogContent>

      <DialogActions sx={{ p: 2.5, pt: 2, gap: 1 }}>
        <Button onClick={onClose} variant="text">
          Remind me later
        </Button>
        <Button
          id="btn-download-update"
          onClick={onDownload}
          variant="contained"
          startIcon={<OpenInNewIcon sx={{ fontSize: 16 }} />}
        >
          Download {latestVersion}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default UpdateDialog;
