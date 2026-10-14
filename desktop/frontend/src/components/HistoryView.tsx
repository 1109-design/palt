/**
 * HistoryView.tsx — transfer log (Beacon)
 *
 * Entries grouped by day. Direction is encoded consistently with the rest of
 * the app: outgoing ↑ amber, incoming ↓ green. All wire-derived values
 * (sizes, durations, times) are set in mono.
 */
import React, { useMemo } from 'react';
import { Box, Typography, Tooltip, Button } from '@mui/material';
import { alpha } from '@mui/material/styles';
import NorthEastIcon from '@mui/icons-material/NorthEast';
import SouthWestIcon from '@mui/icons-material/SouthWest';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline';
import HistoryIcon from '@mui/icons-material/History';
import ChatBubbleOutlineIcon from '@mui/icons-material/ChatBubbleOutline';
import { HistoryEntry } from '../types/history';
import { beacon, fonts } from '../theme/theme';

interface HistoryViewProps {
  history: HistoryEntry[];
  onClear: () => void;
}

export const formatSize = (bytes: number) => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
};

const formatDuration = (ms: number) => {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
};

export const formatTime = (timestamp: string) =>
  new Date(timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

/** "Today" / "Yesterday" / "Mon, 14 Jul" */
export const dayLabel = (timestamp: string) => {
  const d = new Date(timestamp);
  const today = new Date();
  const yesterday = new Date(today);
  yesterday.setDate(today.getDate() - 1);
  const sameDay = (a: Date, b: Date) =>
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate();
  if (sameDay(d, today)) return 'Today';
  if (sameDay(d, yesterday)) return 'Yesterday';
  return d.toLocaleDateString([], { weekday: 'short', day: 'numeric', month: 'short' });
};

const HistoryView: React.FC<HistoryViewProps> = ({ history, onClear }) => {
  const groups = useMemo(() => {
    const map = new Map<string, HistoryEntry[]>();
    for (const entry of history) {
      const label = dayLabel(entry.timestamp);
      if (!map.has(label)) map.set(label, []);
      map.get(label)!.push(entry);
    }
    return Array.from(map.entries());
  }, [history]);

  if (history.length === 0) {
    return (
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          py: 12,
          gap: 1,
        }}
      >
        <HistoryIcon sx={{ fontSize: 56, color: beacon.textFaint, mb: 1 }} />
        <Typography
          sx={{
            fontFamily: fonts.display,
            fontWeight: 700,
            fontSize: '1.2rem',
            color: beacon.text,
          }}
        >
          No transfers yet
        </Typography>
        <Typography sx={{ fontSize: '0.85rem', color: beacon.textSecondary }}>
          Every file and message you send or receive is logged here.
        </Typography>
      </Box>
    );
  }

  return (
    <Box>
      <Box sx={{ display: 'flex', justifyContent: 'flex-end', mb: 1 }}>
        <Button
          startIcon={<DeleteOutlineIcon sx={{ fontSize: 17 }} />}
          onClick={onClear}
          sx={{
            fontSize: '0.78rem',
            color: beacon.textFaint,
            '&:hover': { color: beacon.danger, backgroundColor: alpha(beacon.danger, 0.06) },
          }}
        >
          Clear history
        </Button>
      </Box>

      {groups.map(([label, entries]) => (
        <Box key={label} sx={{ mb: 3 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 1.25 }}>
            <Typography variant="overline">{label}</Typography>
            <Box sx={{ flex: 1, height: '1px', backgroundColor: beacon.hairline }} />
          </Box>

          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            {entries.map((entry) => {
              const outgoing = entry.direction === 'outgoing';
              const isText = entry.kind === 'text';
              const failed = entry.status !== 'completed';
              const dirColor = outgoing ? beacon.accent : beacon.ok;
              return (
                <Box
                  key={entry.id}
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 2,
                    px: 2,
                    py: 1.5,
                    borderRadius: '10px',
                    backgroundColor: beacon.surface,
                    border: `1px solid ${beacon.hairline}`,
                    transition: 'border-color 150ms',
                    '&:hover': { borderColor: beacon.textFaint },
                  }}
                >
                  {/* Direction glyph */}
                  <Box
                    sx={{
                      width: 34,
                      height: 34,
                      borderRadius: '50%',
                      backgroundColor: alpha(failed ? beacon.danger : dirColor, 0.1),
                      color: failed ? beacon.danger : dirColor,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      flexShrink: 0,
                    }}
                  >
                    {failed ? (
                      <ErrorOutlineIcon sx={{ fontSize: 17 }} />
                    ) : isText ? (
                      <ChatBubbleOutlineIcon sx={{ fontSize: 15 }} />
                    ) : outgoing ? (
                      <NorthEastIcon sx={{ fontSize: 16 }} />
                    ) : (
                      <SouthWestIcon sx={{ fontSize: 16 }} />
                    )}
                  </Box>

                  {/* File + partner */}
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Tooltip
                      title={
                        isText
                          ? entry.text || ''
                          : entry.files.length > 1
                          ? entry.files.map((f) => f.name).join(', ')
                          : failed
                          ? entry.errorMessage || 'Transfer failed'
                          : ''
                      }
                    >
                      <Typography
                        noWrap
                        sx={{ fontSize: '0.88rem', fontWeight: 600, color: beacon.text }}
                      >
                        {isText
                          ? entry.text
                          : entry.files.length === 1
                          ? entry.files[0].name
                          : `${entry.files.length} files`}
                      </Typography>
                    </Tooltip>
                    <Typography sx={{ fontSize: '0.76rem', color: beacon.textSecondary, mt: 0.25 }}>
                      {outgoing ? 'to' : 'from'}{' '}
                      <Box component="span" sx={{ color: beacon.text, fontWeight: 500 }}>
                        {entry.partnerName}
                      </Box>
                      {failed && (
                        <Box component="span" sx={{ color: beacon.danger }}>
                          {' '}
                          — failed
                        </Box>
                      )}
                    </Typography>
                  </Box>

                  {/* Mono readout */}
                  <Box sx={{ textAlign: 'right', flexShrink: 0 }}>
                    <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.76rem', color: beacon.text }}>
                      {isText ? 'message' : formatSize(entry.totalSize)}
                    </Typography>
                    <Typography
                      sx={{ fontFamily: fonts.mono, fontSize: '0.7rem', color: beacon.textFaint, mt: 0.25 }}
                    >
                      {!isText && `${formatDuration(entry.durationMillis)} · `}
                      {formatTime(entry.timestamp)}
                    </Typography>
                  </Box>
                </Box>
              );
            })}
          </Box>
        </Box>
      ))}
    </Box>
  );
};

export default HistoryView;
