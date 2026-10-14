import { createTheme, alpha } from '@mui/material/styles';

/**
 * Beacon design tokens — see /design.md.
 * Deep-ink night surfaces, signal-amber accent, mono data readouts.
 */
export const beacon = {
  bg:            '#0C1220',
  surface:       '#131B2C',
  surfaceRaised: '#1A2438',
  hairline:      '#263450',
  text:          '#EAF0FB',
  textSecondary: '#8FA0BC',
  textFaint:     '#55637F',
  accent:        '#FFB454',
  accentHover:   '#FFC474',
  accentPressed: '#E69A38',
  onAccent:      '#201503',
  ok:            '#53D993',
  danger:        '#FF7B72',
  /** Route tag for transfers going through the relay server. */
  relay:         '#7AA2FF',
} as const;

export const fonts = {
  display: '"Bricolage Grotesque", "Inter", sans-serif',
  body:    '"Inter", sans-serif',
  mono:    '"JetBrains Mono", monospace',
} as const;

/** @deprecated Concrete Lemon relic — use beacon.accent. */
export const THEME_TERTIARY = beacon.accent;

const theme = createTheme({
  palette: {
    mode: 'dark',

    primary: {
      main: beacon.accent,
      dark: beacon.accentPressed,
      light: beacon.accentHover,
      contrastText: beacon.onAccent,
    },

    secondary: {
      main: beacon.textSecondary,
      contrastText: beacon.bg,
    },

    background: {
      default: beacon.bg,
      paper: beacon.surface,
    },

    text: {
      primary: beacon.text,
      secondary: beacon.textSecondary,
      disabled: beacon.textFaint,
    },

    divider: beacon.hairline,

    error:   { main: beacon.danger },
    success: { main: beacon.ok },
    warning: { main: beacon.accent },
    info:    { main: beacon.textSecondary },
  },

  typography: {
    fontFamily: fonts.body,

    h1: { fontFamily: fonts.display, fontSize: '2.5rem',   fontWeight: 800, letterSpacing: '-0.03em' },
    h2: { fontFamily: fonts.display, fontSize: '1.9rem',   fontWeight: 800, letterSpacing: '-0.03em' },
    h3: { fontFamily: fonts.display, fontSize: '1.5rem',   fontWeight: 700, letterSpacing: '-0.02em' },
    h4: { fontFamily: fonts.display, fontSize: '1.25rem',  fontWeight: 700, letterSpacing: '-0.02em' },
    h5: { fontFamily: fonts.display, fontSize: '1.1rem',   fontWeight: 700, letterSpacing: '-0.01em' },
    h6: { fontFamily: fonts.display, fontSize: '1rem',     fontWeight: 700, letterSpacing: '-0.01em' },

    body1: { fontSize: '0.9rem',   lineHeight: 1.55 },
    body2: { fontSize: '0.83rem',  lineHeight: 1.55 },

    button: { fontWeight: 600, textTransform: 'none', letterSpacing: 0 },

    caption:  { fontSize: '0.74rem', color: beacon.textSecondary },
    overline: {
      fontSize: '0.68rem',
      fontWeight: 600,
      letterSpacing: '0.14em',
      textTransform: 'uppercase',
      color: beacon.textFaint,
    },
  },

  shape: {
    borderRadius: 12,
  },

  components: {
    MuiCssBaseline: {
      styleOverrides: {
        body: {
          backgroundColor: beacon.bg,
          backgroundImage: 'none',
        },
        '*:focus-visible': {
          outline: `2px solid ${beacon.accent}`,
          outlineOffset: 2,
        },
      },
    },

    MuiButton: {
      defaultProps: { disableElevation: true },
      styleOverrides: {
        root: {
          borderRadius: 8,
          padding: '9px 18px',
          fontWeight: 600,
        },
        containedPrimary: {
          backgroundColor: beacon.accent,
          color: beacon.onAccent,
          '&:hover':  { backgroundColor: beacon.accentHover },
          '&:active': { backgroundColor: beacon.accentPressed },
        },
        outlined: {
          borderColor: beacon.hairline,
          color: beacon.text,
          '&:hover': {
            borderColor: beacon.textSecondary,
            backgroundColor: alpha(beacon.text, 0.04),
          },
        },
        text: {
          color: beacon.textSecondary,
          '&:hover': { color: beacon.text, backgroundColor: alpha(beacon.text, 0.05) },
        },
      },
    },

    MuiCard: {
      defaultProps: { elevation: 0 },
      styleOverrides: {
        root: {
          backgroundColor: beacon.surface,
          backgroundImage: 'none',
          border: `1px solid ${beacon.hairline}`,
          borderRadius: 12,
        },
      },
    },

    MuiPaper: {
      styleOverrides: {
        root: {
          backgroundColor: beacon.surface,
          backgroundImage: 'none',
        },
        rounded: { borderRadius: 12 },
      },
    },

    MuiDialog: {
      styleOverrides: {
        paper: {
          backgroundColor: beacon.surfaceRaised,
          border: `1px solid ${beacon.hairline}`,
          borderRadius: 16,
        },
      },
    },

    MuiTextField: {
      styleOverrides: {
        root: {
          '& .MuiOutlinedInput-root': {
            borderRadius: 8,
            backgroundColor: beacon.bg,
            '& fieldset': { borderColor: beacon.hairline },
            '&:hover fieldset': { borderColor: beacon.textFaint },
            '&.Mui-focused fieldset': { borderColor: beacon.accent, borderWidth: 1 },
          },
        },
      },
    },

    MuiChip: {
      styleOverrides: {
        root: {
          borderRadius: 999,
          fontWeight: 500,
        },
      },
    },

    MuiTooltip: {
      styleOverrides: {
        tooltip: {
          backgroundColor: beacon.surfaceRaised,
          border: `1px solid ${beacon.hairline}`,
          color: beacon.text,
          fontSize: '0.72rem',
          fontWeight: 500,
        },
      },
    },

    MuiLinearProgress: {
      styleOverrides: {
        root: {
          borderRadius: 999,
          backgroundColor: beacon.hairline,
        },
        bar: { borderRadius: 999 },
      },
    },

    MuiCheckbox: {
      styleOverrides: {
        root: {
          color: beacon.textFaint,
          '&.Mui-checked': { color: beacon.accent },
        },
      },
    },

    MuiSnackbar: {
      styleOverrides: {
        root: { zIndex: 1500 },
      },
    },
  },
});

export default theme;
