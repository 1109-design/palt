/**
 * ChangelogDialog.tsx (Beacon)
 *
 * Shown once per version on the first boot after an upgrade.
 * App.tsx compares GetAppVersion() with GetLastSeenVersion(); if they
 * differ it renders this dialog with the release notes from GitHub.
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
import { beacon, fonts } from '../theme/theme';

interface ChangelogDialogProps {
  open: boolean;
  version: string;
  releaseNotes: string;
  onDismiss: () => void;
}

const ChangelogDialog: React.FC<ChangelogDialogProps> = ({
  open,
  version,
  releaseNotes,
  onDismiss,
}) => {
  return (
    <Dialog open={open} onClose={onDismiss} maxWidth="sm" fullWidth>
      <DialogContent sx={{ pt: 3.5, pb: 1 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 2 }}>
          <Box sx={{ minWidth: 0 }}>
            <Typography variant="overline" sx={{ display: 'block', lineHeight: 1.4 }}>
              You're up to date
            </Typography>
            <Typography
              sx={{
                fontFamily: fonts.display,
                fontWeight: 700,
                fontSize: '1.3rem',
                letterSpacing: '-0.02em',
                color: beacon.text,
              }}
            >
              What's new
            </Typography>
          </Box>
          <Box sx={{ flex: 1 }} />
          <Box
            sx={{
              px: 1.25,
              py: 0.5,
              borderRadius: '6px',
              border: `1px solid ${alpha(beacon.accent, 0.5)}`,
              backgroundColor: alpha(beacon.accent, 0.1),
            }}
          >
            <Typography
              sx={{
                fontFamily: fonts.mono,
                fontSize: '0.76rem',
                fontWeight: 600,
                color: beacon.accent,
              }}
            >
              {version}
            </Typography>
          </Box>
        </Box>

        <Box
          sx={{
            maxHeight: 300,
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
            {releaseNotes || 'Bug fixes and performance improvements.'}
          </Typography>
        </Box>
      </DialogContent>

      <DialogActions sx={{ p: 2.5, pt: 2 }}>
        <Button id="btn-changelog-dismiss" onClick={onDismiss} variant="contained" fullWidth>
          Got it
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default ChangelogDialog;
