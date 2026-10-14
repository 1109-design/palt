/**
 * ProgressSnack.tsx — transfer shelf (Beacon)
 *
 * A card docked bottom-right while a transfer runs: current file, amber
 * progress bar, and a mono readout of speed / bytes / percent. Turns green
 * on completion with an "Open folder" action.
 */
import React from 'react';
import { Snackbar, Box, Typography, Button, IconButton, LinearProgress } from '@mui/material';
import { alpha } from '@mui/material/styles';
import FolderOpenIcon from '@mui/icons-material/FolderOpen';
import CloseIcon from '@mui/icons-material/Close';
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline';
import SwapVertIcon from '@mui/icons-material/SwapVert';
import { beacon, fonts } from '../theme/theme';
import type { Route } from '../types/peer';
import RouteBadge from './RouteBadge';

export interface TransferProgress {
  transferId: string;
  written: number;
  total: number;
  sentItems?: number;
  totalItems?: number;
  currentFile?: string;
  speed?: number;
  status?: 'transferring' | 'completed' | 'error';
  /** How the bytes travel; unknown until the connection is up. */
  route?: Route;
}

interface ProgressSnackProps {
  progress: TransferProgress | null;
  error: string | null;
  onOpenFolder: () => void;
  onClose: () => void;
}

const formatBytes = (bytes: number) => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
};

export const ProgressSnack: React.FC<ProgressSnackProps> = ({
  progress,
  error,
  onOpenFolder,
  onClose,
}) => {
  if (!progress && !error) return null;

  const isError = !!error;
  const isCompleted = !isError && progress?.status === 'completed';
  const percent =
    progress && progress.total > 0 ? (progress.written / progress.total) * 100 : 0;
  const speedStr = progress?.speed ? `${formatBytes(Math.round(progress.speed))}/s` : '—';
  const stateColor = isError ? beacon.danger : isCompleted ? beacon.ok : beacon.accent;

  return (
    <Snackbar
      open
      autoHideDuration={isCompleted ? 8000 : null}
      onClose={(_, reason) => {
        if (reason !== 'clickaway') onClose();
      }}
      anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      sx={{ bottom: { xs: 46 }, right: { xs: 16 } }}
    >
      <Box
        sx={{
          width: 400,
          borderRadius: '14px',
          overflow: 'hidden',
          backgroundColor: beacon.surfaceRaised,
          border: `1px solid ${alpha(stateColor, 0.4)}`,
        }}
      >
        <Box sx={{ p: 2 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
            {/* State glyph */}
            <Box
              sx={{
                width: 36,
                height: 36,
                borderRadius: '10px',
                backgroundColor: alpha(stateColor, 0.1),
                color: stateColor,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                flexShrink: 0,
              }}
            >
              {isError ? (
                <ErrorOutlineIcon sx={{ fontSize: 19 }} />
              ) : isCompleted ? (
                <CheckCircleOutlineIcon sx={{ fontSize: 19 }} />
              ) : (
                <SwapVertIcon
                  sx={{
                    fontSize: 19,
                    animation: 'shelf-pulse 1.6s ease-in-out infinite',
                    '@keyframes shelf-pulse': {
                      '0%, 100%': { opacity: 1 },
                      '50%': { opacity: 0.45 },
                    },
                    '@media (prefers-reduced-motion: reduce)': { animation: 'none' },
                  }}
                />
              )}
            </Box>

            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography
                noWrap
                sx={{ fontSize: '0.86rem', fontWeight: 600, color: beacon.text }}
              >
                {isError
                  ? 'Transfer failed'
                  : isCompleted
                  ? 'Transfer complete'
                  : progress?.currentFile || 'Transferring…'}
              </Typography>
              <Typography
                noWrap
                sx={{ fontSize: '0.74rem', color: beacon.textSecondary, mt: 0.25 }}
              >
                {isError
                  ? error
                  : isCompleted
                  ? `${progress?.totalItems ?? ''} file${(progress?.totalItems ?? 0) !== 1 ? 's' : ''} transferred`
                  : `file ${progress?.sentItems ?? 1} of ${progress?.totalItems ?? 1}`}
              </Typography>
            </Box>

            {progress?.route && !isError && <RouteBadge route={progress.route} />}
            {isCompleted && (
              <Button
                size="small"
                variant="outlined"
                startIcon={<FolderOpenIcon sx={{ fontSize: 15 }} />}
                onClick={onOpenFolder}
                sx={{ flexShrink: 0, fontSize: '0.74rem', py: 0.5 }}
              >
                Open folder
              </Button>
            )}
            <IconButton
              size="small"
              aria-label="dismiss"
              onClick={onClose}
              sx={{ color: beacon.textFaint, '&:hover': { color: beacon.text }, flexShrink: 0 }}
            >
              <CloseIcon sx={{ fontSize: 16 }} />
            </IconButton>
          </Box>

          {!isError && (
            <>
              <LinearProgress
                variant="determinate"
                value={isCompleted ? 100 : percent}
                sx={{
                  mt: 1.75,
                  height: 5,
                  '& .MuiLinearProgress-bar': { backgroundColor: stateColor },
                }}
              />
              <Box sx={{ display: 'flex', justifyContent: 'space-between', mt: 0.75 }}>
                <Typography
                  sx={{ fontFamily: fonts.mono, fontSize: '0.68rem', color: beacon.textFaint }}
                >
                  {formatBytes(progress?.written ?? 0)} / {formatBytes(progress?.total ?? 0)}
                </Typography>
                <Typography
                  sx={{ fontFamily: fonts.mono, fontSize: '0.68rem', color: beacon.textSecondary }}
                >
                  {isCompleted ? 'done' : `${speedStr} · ${Math.round(percent)}%`}
                </Typography>
              </Box>
            </>
          )}
        </Box>
      </Box>
    </Snackbar>
  );
};
