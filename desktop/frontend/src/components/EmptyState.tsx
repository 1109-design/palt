/**
 * EmptyState.tsx (Beacon)
 *
 * Shown when no peers are discovered. The beacon keeps pulsing — the app is
 * always broadcasting — and the copy tells the user exactly how to make a
 * device appear.
 */
import React from 'react';
import { Box, Typography, Button } from '@mui/material';
import WifiTetheringIcon from '@mui/icons-material/WifiTethering';
import SearchOffIcon from '@mui/icons-material/SearchOff';
import BeaconPulse from './BeaconPulse';
import { beacon, fonts } from '../theme/theme';

interface EmptyStateProps {
  loading: boolean;
  filtered: boolean; // true when a search filter is active
  onRefresh: () => void;
}

const EmptyState: React.FC<EmptyStateProps> = ({ loading, filtered, onRefresh }) => {
  return (
    <Box
      id="empty-state"
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        py: 10,
        px: 3,
        textAlign: 'center',
        gap: 2,
      }}
    >
      <Box sx={{ mb: 2 }}>
        <BeaconPulse active={!filtered} size={88}>
          <Box
            sx={{
              width: 88,
              height: 88,
              borderRadius: '50%',
              backgroundColor: beacon.surface,
              border: `1px solid ${beacon.hairline}`,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: filtered ? beacon.textFaint : beacon.accent,
            }}
          >
            {filtered ? (
              <SearchOffIcon sx={{ fontSize: 36 }} />
            ) : (
              <WifiTetheringIcon sx={{ fontSize: 36 }} />
            )}
          </Box>
        </BeaconPulse>
      </Box>

      <Typography
        sx={{
          fontFamily: fonts.display,
          fontWeight: 700,
          fontSize: '1.35rem',
          letterSpacing: '-0.02em',
          color: beacon.text,
        }}
      >
        {filtered ? 'No matching devices' : 'Listening for devices'}
      </Typography>

      <Typography sx={{ fontSize: '0.88rem', color: beacon.textSecondary, maxWidth: 380 }}>
        {filtered
          ? 'No device matches your filter. Clear it to see everything on the network.'
          : 'Open PALT on another device connected to the same Wi‑Fi or LAN — it will appear here the moment it announces itself.'}
      </Typography>

      <Typography
        sx={{
          fontFamily: fonts.mono,
          fontSize: '0.72rem',
          color: beacon.textFaint,
          mt: 0.5,
        }}
      >
        broadcasting on _palt._tcp.local
      </Typography>

      {!filtered && (
        <Button
          id="btn-empty-refresh"
          variant="outlined"
          onClick={onRefresh}
          disabled={loading}
          sx={{ mt: 1.5 }}
        >
          {loading ? 'Scanning…' : 'Scan again'}
        </Button>
      )}
    </Box>
  );
};

export default EmptyState;
