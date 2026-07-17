/**
 * BeaconPulse.tsx
 *
 * The Beacon signature element: concentric rings emanating from a center,
 * like an mDNS beacon announcing itself. Wraps any child (usually an OS icon
 * tile). Rings animate only while `active`; with reduced motion they freeze
 * into a static halo.
 */
import React from 'react';
import { Box } from '@mui/material';
import { alpha } from '@mui/material/styles';
import { beacon } from '../theme/theme';

interface BeaconPulseProps {
  /** Rings animate while true (discovery broadcasting). */
  active: boolean;
  /** Diameter of the wrapped content in px. */
  size: number;
  /** Ring color; defaults to signal amber. */
  color?: string;
  children: React.ReactNode;
}

const RING_COUNT = 3;

const BeaconPulse: React.FC<BeaconPulseProps> = ({
  active,
  size,
  color = beacon.accent,
  children,
}) => (
  <Box
    sx={{
      position: 'relative',
      width: size,
      height: size,
      flexShrink: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      '@keyframes beacon-ring': {
        '0%':   { transform: 'scale(1)',   opacity: 0.45 },
        '100%': { transform: 'scale(2.1)', opacity: 0 },
      },
    }}
  >
    {active &&
      Array.from({ length: RING_COUNT }).map((_, i) => (
        <Box
          key={i}
          sx={{
            position: 'absolute',
            inset: 0,
            borderRadius: '50%',
            border: `1.5px solid ${color}`,
            animation: `beacon-ring 2.8s ease-out ${i * 0.9}s infinite`,
            opacity: 0,
            pointerEvents: 'none',
            '@media (prefers-reduced-motion: reduce)': {
              animation: 'none',
              opacity: i === 0 ? 0.25 : 0,
              transform: 'scale(1.25)',
            },
          }}
        />
      ))}
    {/* Static halo grounds the rings even when idle */}
    <Box
      sx={{
        position: 'absolute',
        inset: -6,
        borderRadius: '50%',
        border: `1px solid ${alpha(color, active ? 0.3 : 0.12)}`,
        transition: 'border-color 400ms',
        pointerEvents: 'none',
      }}
    />
    {children}
  </Box>
);

export default BeaconPulse;
