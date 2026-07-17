/**
 * UpdateBanner.tsx — "update available" chip in the top bar (Beacon)
 */
import React from 'react';
import { Box, Typography } from '@mui/material';
import { alpha } from '@mui/material/styles';
import ArrowCircleUpIcon from '@mui/icons-material/ArrowCircleUp';
import { beacon, fonts } from '../theme/theme';

interface UpdateBannerProps {
  latestVersion: string;
  onClick: () => void;
}

const UpdateBanner: React.FC<UpdateBannerProps> = ({ latestVersion, onClick }) => (
  <Box
    id="btn-update-available"
    component="button"
    onClick={onClick}
    aria-label={`update to ${latestVersion}`}
    sx={{
      display: 'flex',
      alignItems: 'center',
      gap: 0.75,
      px: 1.25,
      height: 28,
      borderRadius: 999,
      border: `1px solid ${alpha(beacon.accent, 0.4)}`,
      backgroundColor: alpha(beacon.accent, 0.1),
      cursor: 'pointer',
      transition: 'background-color 150ms',
      '&:hover': { backgroundColor: alpha(beacon.accent, 0.18) },
    }}
  >
    <ArrowCircleUpIcon sx={{ fontSize: 15, color: beacon.accent }} />
    <Typography
      sx={{
        fontFamily: fonts.mono,
        fontSize: '0.7rem',
        fontWeight: 600,
        color: beacon.accent,
        lineHeight: 1,
      }}
    >
      {latestVersion}
    </Typography>
  </Box>
);

export default UpdateBanner;
