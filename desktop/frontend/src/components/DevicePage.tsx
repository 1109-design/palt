/**
 * DevicePage.tsx — one peer, full screen (Beacon)
 *
 * Opened by clicking a PeerCard. Shows the shared history with that device
 * as a thread (outgoing on the right in amber, incoming on the left in
 * green) and a composer for text messages and files. The whole page is a
 * file drop target for this peer via the same data-peer-* attributes the
 * cards use. Auto-copy (incoming texts → clipboard) is opt-in per device and
 * only offered for paired devices; "Send clipboard" is always a manual tap. If the peer drops off the network the thread stays readable
 * but sending is disabled until it comes back.
 */
import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Box, Typography, IconButton, Tooltip, Button, InputBase, CircularProgress, Switch,
  Dialog, DialogTitle, DialogContent, DialogActions, Menu, MenuItem, ListItemIcon, ListItemText,
  Skeleton,
} from '@mui/material';
import { alpha } from '@mui/material/styles';
import ArrowBackIcon from '@mui/icons-material/ArrowBack';
import AttachFileIcon from '@mui/icons-material/AttachFile';
import SendIcon from '@mui/icons-material/ArrowUpward';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import CheckIcon from '@mui/icons-material/Check';
import InsertDriveFileIcon from '@mui/icons-material/InsertDriveFileOutlined';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline';
import ForumOutlinedIcon from '@mui/icons-material/ForumOutlined';
import ContentPasteIcon from '@mui/icons-material/ContentPaste';
import ContentPasteGoIcon from '@mui/icons-material/ContentPasteGo';
import OpenInNewIcon from '@mui/icons-material/OpenInNew';
import FolderOpenIcon from '@mui/icons-material/FolderOpenOutlined';
import DeleteIcon from '@mui/icons-material/DeleteOutline';
import PlayArrowIcon from '@mui/icons-material/PlayArrowRounded';
import type { DeviceStatus, MediaState, Peer } from '../types/peer';
import type { HistoryEntry, HistoryFile } from '../types/history';
import { getOsInfo } from '../utils/osHelpers';
import NotificationsActiveIcon from '@mui/icons-material/NotificationsActive';
import NotificationsIcon from '@mui/icons-material/Notifications';
import TuneIcon from '@mui/icons-material/Tune';
import { Badge } from '@mui/material';
import WavingHandIcon from '@mui/icons-material/WavingHand';
import WifiIcon from '@mui/icons-material/Wifi';
import RouterIcon from '@mui/icons-material/Router';
import SmsIcon from '@mui/icons-material/SmsOutlined';
import DeviceStatusPanel from './DeviceStatusPanel';
import PeerMediaCard from './PeerMediaCard';
import { beacon, fonts } from '../theme/theme';
import { formatSize, formatTime, dayLabel } from './HistoryView';
import RouteBadge, { routeLabel } from './RouteBadge';

/** Must match MaxTextBytes in internal/transfer/protocol.go */
const MAX_TEXT_BYTES = 16 * 1024;

interface DevicePageProps {
  /** The device as last seen; kept so the page survives the peer going offline. */
  peer: Peer;
  /** True while the device is currently discovered on the network. */
  online: boolean;
  alias?: string;
  history: HistoryEntry[];
  dragActive?: boolean;
  onBack: () => void;
  onSendFiles: (peer: Peer) => void;
  onSendText: (peer: Peer, text: string) => Promise<void>;
  /** Device is paired (authenticated link up); required for auto-copy. */
  paired: boolean;
  /** Incoming texts from this device go straight to the clipboard. */
  autoCopy: boolean;
  onToggleAutoCopy: (peer: Peer, enabled: boolean) => void;
  readClipboard: () => Promise<string>;
  onFileAction: (file: HistoryFile, action: 'open' | 'reveal') => void;
  /** Resolves a picture to a small preview (data: URL); rejects when unavailable. */
  loadPreview: (path: string, name: string) => Promise<MediaPreview>;
  /** Removes one message/transfer from local history (files stay on disk). */
  onDeleteEntry: (entry: HistoryEntry) => void;
  /** Battery and signal reported by a paired device. */
  status?: DeviceStatus;
  onRing: (peer: Peer) => void;
  onPing: (peer: Peer) => void;
  onShareWifi: (peer: Peer) => void;
  onShareInternet: (peer: Peer) => void;
  onShowMessages: () => void;
  hotspotOn: boolean;
  media?: MediaState;
  mediaReceivedAt?: number;
  onMediaAction: (player: string, action: string, value?: number) => void;
  /** Number of the phone's current notifications; opens the drawer. */
  notificationCount: number;
  onShowNotifications: () => void;
  /** Per-device feature switches (Go deviceSettingDefaults). */
  settings: Record<string, boolean>;
  onSetSetting: (key: string, on: boolean) => void;
}

/** Labels for the per-device switches, in menu order. */
const SETTING_LABELS: [string, string][] = [
  ['notifications', 'Show its notifications here'],
  ['clipboardSend', 'Send my clipboard to it'],
  ['clipboardReceive', 'Take its clipboard'],
  ['pauseOnCall', 'Pause media during calls'],
];

/** Received files fall back to the download folder by name; sent files need a recorded path. */
const canLocate = (entry: HistoryEntry, file: HistoryFile) =>
  !!file.path || entry.direction === 'incoming';

const MEDIA_EXT = /\.(jpe?g|png|gif|webp|mp4|mov|m4v|webm|mkv|avi|3gp)$/i;

export interface MediaPreview {
  url: string;
  video: boolean;
  /** Seconds; 0 when unknown. */
  duration: number;
}

/** Previews already loaded (or known unavailable) this session, keyed by path|name. */
const previewCache = new Map<string, MediaPreview | null>();

/** Pictures/videos that can be shown inline: completed, locatable, previewable type. */
const isInlineImage = (entry: HistoryEntry, file: HistoryFile) =>
  entry.status === 'completed' && canLocate(entry, file) && MEDIA_EXT.test(file.name);

const formatDuration = (secs: number) => {
  const s = Math.round(secs);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const ss = String(s % 60).padStart(2, '0');
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`;
};

/**
 * ImageTile — a picture shown in the thread, WhatsApp-style. Falls back to the
 * plain file row when the preview can't be made (file moved, unsupported, …).
 */
const ImageTile: React.FC<{
  entry: HistoryEntry;
  file: HistoryFile;
  tint: string;
  onFileAction: (file: HistoryFile, action: 'open' | 'reveal') => void;
  loadPreview: (path: string, name: string) => Promise<MediaPreview>;
}> = ({ entry, file, tint, onFileAction, loadPreview }) => {
  const key = `${file.path || ''}|${file.name}`;
  const [preview, setPreview] = useState<MediaPreview | null | undefined>(previewCache.get(key));

  useEffect(() => {
    if (previewCache.has(key)) return;
    let alive = true;
    loadPreview(file.path || '', file.name)
      .then((p) => {
        previewCache.set(key, p);
        if (alive) setPreview(p);
      })
      .catch(() => {
        previewCache.set(key, null);
        if (alive) setPreview(null);
      });
    return () => {
      alive = false;
    };
  }, [key, file.path, file.name, loadPreview]);

  if (preview === null) {
    return <FileRow entry={entry} file={file} tint={tint} onFileAction={onFileAction} />;
  }
  const src = preview?.url;

  return (
    <Box sx={{ position: 'relative', '&:hover .img-reveal': { opacity: 1 } }}>
      {src ? (
        <Box
          component="img"
          src={src}
          alt={file.name}
          title={`${file.name} · ${formatSize(file.size)}`}
          onClick={() => onFileAction(file, 'open')}
          sx={{
            display: 'block',
            maxWidth: '100%',
            width: 'auto',
            maxHeight: 320,
            minWidth: 120,
            borderRadius: '8px',
            cursor: 'pointer',
            objectFit: 'cover',
            backgroundColor: beacon.bg,
          }}
        />
      ) : (
        <Skeleton
          variant="rounded"
          width={240}
          height={160}
          animation="wave"
          sx={{ borderRadius: '8px', backgroundColor: alpha(tint, 0.08), transform: 'none' }}
        />
      )}
      {preview?.video && (
        <>
          <Box
            onClick={() => onFileAction(file, 'open')}
            sx={{
              position: 'absolute',
              top: '50%',
              left: '50%',
              transform: 'translate(-50%, -50%)',
              width: 48,
              height: 48,
              borderRadius: '50%',
              backgroundColor: alpha(beacon.bg, 0.65),
              border: `1px solid ${alpha(beacon.text, 0.25)}`,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              cursor: 'pointer',
            }}
          >
            <PlayArrowIcon sx={{ fontSize: 28, color: beacon.text }} />
          </Box>
          {preview.duration > 0 && (
            <Typography
              sx={{
                position: 'absolute',
                left: 8,
                bottom: 8,
                px: 0.75,
                borderRadius: '4px',
                backgroundColor: alpha(beacon.bg, 0.7),
                fontFamily: fonts.mono,
                fontSize: '0.7rem',
                color: beacon.text,
                pointerEvents: 'none',
              }}
            >
              {formatDuration(preview.duration)}
            </Typography>
          )}
        </>
      )}
      {src && (
        <Tooltip title="Show in folder">
          <IconButton
            className="img-reveal"
            size="small"
            aria-label={`show ${file.name} in folder`}
            onClick={() => onFileAction(file, 'reveal')}
            sx={{
              position: 'absolute',
              top: 6,
              right: 6,
              opacity: 0,
              transition: 'opacity 150ms',
              color: beacon.text,
              backgroundColor: alpha(beacon.bg, 0.7),
              '&:hover': { backgroundColor: alpha(beacon.bg, 0.9) },
              '&:focus-visible': { opacity: 1 },
            }}
          >
            <FolderOpenIcon sx={{ fontSize: 16 }} />
          </IconButton>
        </Tooltip>
      )}
    </Box>
  );
};

const FileRow: React.FC<{
  entry: HistoryEntry;
  file: HistoryFile;
  tint: string;
  onFileAction: (file: HistoryFile, action: 'open' | 'reveal') => void;
}> = ({ entry, file, tint, onFileAction }) => {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const actionable = canLocate(entry, file) && entry.status === 'completed';

  const row = (
    <Box
      component={actionable ? 'button' : 'div'}
      onClick={actionable ? (e: React.MouseEvent<HTMLElement>) => setAnchor(e.currentTarget) : undefined}
      aria-haspopup={actionable ? 'menu' : undefined}
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 1.25,
        minWidth: 0,
        // The negative margin gives the hover highlight room; widen by the same
        // amount so the row's own size isn't 8px short (which truncated names).
        width: 'calc(100% + 8px)',
        boxSizing: 'border-box',
        border: 'none',
        background: 'none',
        font: 'inherit',
        textAlign: 'left',
        p: 0.5,
        m: -0.5,
        borderRadius: '6px',
        cursor: actionable ? 'pointer' : 'default',
        ...(actionable && {
          '&:hover': { backgroundColor: alpha(tint, 0.1) },
          '&:hover .file-name': { textDecoration: 'underline' },
          '&:focus-visible': { outline: `1px solid ${tint}` },
        }),
      }}
    >
      <InsertDriveFileIcon sx={{ fontSize: 18, color: tint, flexShrink: 0 }} />
      <Typography className="file-name" noWrap sx={{ fontSize: '0.86rem', fontWeight: 600, color: beacon.text, minWidth: 0 }}>
        {file.name}
      </Typography>
      <Typography
        sx={{ fontFamily: fonts.mono, fontSize: '0.72rem', color: beacon.textSecondary, flexShrink: 0, ml: 'auto', pl: 1 }}
      >
        {formatSize(file.size)}
      </Typography>
    </Box>
  );

  return (
    <>
      <Tooltip title={actionable ? '' : 'Location not recorded for this file'} placement="top">
        {row}
      </Tooltip>
      <Menu
        anchorEl={anchor}
        open={!!anchor}
        onClose={() => setAnchor(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        slotProps={{ paper: { sx: { backgroundColor: beacon.surfaceRaised, border: `1px solid ${beacon.hairline}` } } }}
      >
        <MenuItem
          onClick={() => {
            setAnchor(null);
            onFileAction(file, 'open');
          }}
        >
          <ListItemIcon><OpenInNewIcon sx={{ fontSize: 18 }} /></ListItemIcon>
          <ListItemText primaryTypographyProps={{ fontSize: '0.86rem' }}>Open</ListItemText>
        </MenuItem>
        <MenuItem
          onClick={() => {
            setAnchor(null);
            onFileAction(file, 'reveal');
          }}
        >
          <ListItemIcon><FolderOpenIcon sx={{ fontSize: 18 }} /></ListItemIcon>
          <ListItemText primaryTypographyProps={{ fontSize: '0.86rem' }}>Show in folder</ListItemText>
        </MenuItem>
      </Menu>
    </>
  );
};

const Bubble: React.FC<{
  entry: HistoryEntry;
  onFileAction: (file: HistoryFile, action: 'open' | 'reveal') => void;
  loadPreview: (path: string, name: string) => Promise<MediaPreview>;
  onDelete: (entry: HistoryEntry) => void;
}> = ({ entry, onFileAction, loadPreview, onDelete }) => {
  const outgoing = entry.direction === 'outgoing';
  const failed = entry.status !== 'completed';
  const isText = entry.kind === 'text';
  const tint = failed ? beacon.danger : outgoing ? beacon.accent : beacon.ok;
  // Picture-only bubbles hug the image, like a chat app.
  const picturesOnly = !isText && entry.files.length > 0 && entry.files.every((f) => isInlineImage(entry, f));
  const [copied, setCopied] = useState(false);
  // Right-click (or long-press on a touchscreen) opens the message menu.
  const [menuAt, setMenuAt] = useState<{ top: number; left: number } | null>(null);
  const singleFile = !isText && entry.files.length === 1 && entry.status === 'completed' && canLocate(entry, entry.files[0]);

  const copy = () => {
    navigator.clipboard.writeText(entry.text || '');
    setCopied(true);
    setTimeout(() => setCopied(false), 1400);
  };

  const menuItem = (label: string, icon: React.ReactNode, action: () => void, danger = false) => (
    <MenuItem
      onClick={() => {
        setMenuAt(null);
        action();
      }}
      sx={danger ? { color: beacon.danger } : undefined}
    >
      <ListItemIcon sx={danger ? { color: beacon.danger } : undefined}>{icon}</ListItemIcon>
      <ListItemText primaryTypographyProps={{ fontSize: '0.86rem' }}>{label}</ListItemText>
    </MenuItem>
  );

  return (
    <Box
      onContextMenu={(e: React.MouseEvent) => {
        e.preventDefault();
        setMenuAt({ top: e.clientY, left: e.clientX });
      }}
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: outgoing ? 'flex-end' : 'flex-start',
        '&:hover .bubble-copy': { opacity: 1 },
      }}
    >
      <Box
        sx={{
          maxWidth: '72%',
          px: picturesOnly ? 0.5 : 1.75,
          py: picturesOnly ? 0.5 : 1.25,
          borderRadius: '12px',
          borderTopRightRadius: outgoing ? '4px' : '12px',
          borderTopLeftRadius: outgoing ? '12px' : '4px',
          backgroundColor: alpha(tint, 0.08),
          border: `1px solid ${alpha(tint, 0.28)}`,
        }}
      >
        {isText ? (
          <Typography
            sx={{
              fontSize: '0.9rem',
              color: beacon.text,
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
              userSelect: 'text',
            }}
          >
            {entry.text}
          </Typography>
        ) : (
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
            {entry.files.map((f, i) => (
              isInlineImage(entry, f) ? (
                <ImageTile key={i} entry={entry} file={f} tint={tint} onFileAction={onFileAction} loadPreview={loadPreview} />
              ) : (
                <FileRow key={i} entry={entry} file={f} tint={tint} onFileAction={onFileAction} />
              )
            ))}
          </Box>
        )}
      </Box>

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, mt: 0.5, px: 0.5 }}>
        {failed && (
          <Tooltip title={entry.errorMessage || ''}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: beacon.danger }}>
              <ErrorOutlineIcon sx={{ fontSize: 13 }} />
              <Typography sx={{ fontSize: '0.7rem' }}>Not delivered</Typography>
            </Box>
          </Tooltip>
        )}
        {entry.copied && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.4, color: beacon.textSecondary }}>
            <ContentPasteIcon sx={{ fontSize: 12 }} />
            <Typography sx={{ fontSize: '0.7rem' }}>Copied to clipboard</Typography>
          </Box>
        )}
        <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.68rem', color: beacon.textFaint }}>
          {formatTime(entry.timestamp)}
        </Typography>
        {entry.route && (
          <Typography
            sx={{ fontFamily: fonts.mono, fontSize: '0.68rem', color: entry.route === 'relay' ? beacon.relay : beacon.textFaint }}
          >
            · {routeLabel(entry.route)}
          </Typography>
        )}
        {isText && (
          <Tooltip title={copied ? 'Copied' : 'Copy text'}>
            <IconButton
              size="small"
              className="bubble-copy"
              aria-label="copy message"
              onClick={copy}
              sx={{
                p: 0.25,
                color: beacon.textFaint,
                opacity: copied ? 1 : 0,
                transition: 'opacity 150ms',
                '&:hover': { color: beacon.text },
                '&:focus-visible': { opacity: 1 },
              }}
            >
              {copied ? <CheckIcon sx={{ fontSize: 13, color: beacon.ok }} /> : <ContentCopyIcon sx={{ fontSize: 13 }} />}
            </IconButton>
          </Tooltip>
        )}
      </Box>

      <Menu
        open={!!menuAt}
        onClose={() => setMenuAt(null)}
        anchorReference="anchorPosition"
        anchorPosition={menuAt ?? undefined}
        slotProps={{ paper: { sx: { backgroundColor: beacon.surfaceRaised, border: `1px solid ${beacon.hairline}` } } }}
      >
        {isText && menuItem('Copy', <ContentCopyIcon sx={{ fontSize: 18 }} />, copy)}
        {singleFile && menuItem('Open', <OpenInNewIcon sx={{ fontSize: 18 }} />, () => onFileAction(entry.files[0], 'open'))}
        {singleFile && menuItem('Show in folder', <FolderOpenIcon sx={{ fontSize: 18 }} />, () => onFileAction(entry.files[0], 'reveal'))}
        {menuItem('Delete', <DeleteIcon sx={{ fontSize: 18 }} />, () => onDelete(entry), true)}
      </Menu>
    </Box>
  );
};

const DevicePage: React.FC<DevicePageProps> = ({
  peer,
  online,
  alias,
  history,
  dragActive = false,
  onBack,
  onSendFiles,
  onSendText,
  paired,
  autoCopy,
  onToggleAutoCopy,
  readClipboard,
  onFileAction,
  loadPreview,
  onDeleteEntry,
  status,
  onRing,
  onPing,
  onShareWifi,
  onShareInternet,
  onShowMessages,
  hotspotOn,
  media,
  mediaReceivedAt,
  onMediaAction,
  notificationCount,
  onShowNotifications,
  settings,
  onSetSetting,
}) => {
  const [settingsAnchor, setSettingsAnchor] = useState<HTMLElement | null>(null);
  const osInfo = getOsInfo(peer.os);
  const OsIcon = osInfo.Icon;
  const displayName = alias || peer.deviceName;

  const [draft, setDraft] = useState('');
  const [sending, setSending] = useState(false);
  const [sendError, setSendError] = useState<string | null>(null);
  const threadRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLTextAreaElement | null>(null);

  // History arrives newest-first; a thread reads oldest-first.
  const thread = useMemo(
    () => history.filter((e) => e.partnerName === peer.deviceName).reverse(),
    [history, peer.deviceName],
  );

  const groups = useMemo(() => {
    const out: { label: string; entries: HistoryEntry[] }[] = [];
    for (const entry of thread) {
      const label = dayLabel(entry.timestamp);
      if (out.length === 0 || out[out.length - 1].label !== label) out.push({ label, entries: [] });
      out[out.length - 1].entries.push(entry);
    }
    return out;
  }, [thread]);

  // Keep the newest item in view.
  useEffect(() => {
    const el = threadRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [thread.length]);

  useEffect(() => {
    if (online) inputRef.current?.focus();
  }, [online]);

  const draftBytes = useMemo(() => new TextEncoder().encode(draft).length, [draft]);
  const tooLong = draftBytes > MAX_TEXT_BYTES;
  const canSend = online && !sending && draft.trim().length > 0 && !tooLong;

  const send = async (text: string, clearDraft: boolean) => {
    setSending(true);
    setSendError(null);
    try {
      await onSendText(peer, text);
      if (clearDraft) setDraft('');
    } catch (e: any) {
      setSendError(String(e?.message || e));
    } finally {
      setSending(false);
      inputRef.current?.focus();
    }
  };

  const submit = () => {
    if (canSend) send(draft, true);
  };

  const sendClipboard = async () => {
    if (!online || sending) return;
    let text = '';
    try {
      text = (await readClipboard()) || '';
    } catch {
      setSendError('Could not read the clipboard.');
      return;
    }
    if (!text.trim()) {
      setSendError('Clipboard is empty.');
      return;
    }
    if (new TextEncoder().encode(text).length > MAX_TEXT_BYTES) {
      setSendError(`Clipboard is over ${MAX_TEXT_BYTES / 1024} KB. Send it as a file instead.`);
      return;
    }
    send(text, false);
  };

  return (
    <Box
      data-peer-ip={online ? peer.ipAddress : undefined}
      data-peer-port={online ? peer.port : undefined}
      data-peer-name={online ? peer.deviceName : undefined}
      sx={{
        '--wails-drop-target': online ? 'drop' : undefined,
        position: 'fixed',
        top: 60,
        bottom: 30,
        left: 0,
        right: 0,
        display: 'flex',
        flexDirection: 'column',
        backgroundColor: 'background.default',
      }}
    >
      {/* ── Header ───────────────────────────────────────────────────────── */}
      <Box sx={{ borderBottom: `1px solid ${beacon.hairline}` }}>
        <Box
          sx={{
            maxWidth: 900,
            mx: 'auto',
            px: { xs: 2, sm: 3 },
            py: 2,
            display: 'flex',
            alignItems: 'center',
            gap: 2,
          }}
        >
          <Tooltip title="Back to devices">
            <IconButton aria-label="back" onClick={onBack} sx={{ color: beacon.textSecondary, '&:hover': { color: beacon.text } }}>
              <ArrowBackIcon sx={{ fontSize: 20 }} />
            </IconButton>
          </Tooltip>

          <Box
            sx={{
              width: 44,
              height: 44,
              borderRadius: '12px',
              backgroundColor: beacon.surfaceRaised,
              border: `1px solid ${beacon.hairline}`,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: beacon.textSecondary,
              flexShrink: 0,
            }}
          >
            <OsIcon sx={{ fontSize: 22 }} />
          </Box>

          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Typography
              noWrap
              sx={{ fontFamily: fonts.display, fontWeight: 700, fontSize: '1.15rem', letterSpacing: '-0.01em', color: beacon.text }}
            >
              {displayName}
            </Typography>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mt: 0.25 }}>
              <Box
                sx={{
                  width: 7,
                  height: 7,
                  borderRadius: '50%',
                  backgroundColor: online ? beacon.ok : beacon.textFaint,
                  flexShrink: 0,
                }}
              />
              <Typography sx={{ fontSize: '0.8rem', color: online ? beacon.textSecondary : beacon.textFaint }} noWrap>
                {!online ? 'Offline' : paired ? 'Paired' : 'Not paired'} · {osInfo.label}
              </Typography>
              {online && <RouteBadge route={peer.path || peer.via} />}
              {online && !paired && (
                <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.72rem', color: beacon.textSecondary }} noWrap>
                  {peer.via === 'relay' ? 'through relay server' : `${peer.ipAddress}:${peer.port}`}
                </Typography>
              )}
            </Box>
          </Box>

          {online && paired && (
            <>
              <Tooltip title="Ring — plays a loud sound, even on silent">
                <IconButton onClick={() => onRing(peer)} aria-label="ring device" sx={{ color: beacon.textSecondary }}>
                  <NotificationsActiveIcon sx={{ fontSize: 19 }} />
                </IconButton>
              </Tooltip>
              <Tooltip title="Ping">
                <IconButton onClick={() => onPing(peer)} aria-label="ping device" sx={{ color: beacon.textSecondary }}>
                  <WavingHandIcon sx={{ fontSize: 18 }} />
                </IconButton>
              </Tooltip>
              <Tooltip title="Share this computer's Wi-Fi">
                <IconButton onClick={() => onShareWifi(peer)} aria-label="share wi-fi" sx={{ color: beacon.textSecondary }}>
                  <WifiIcon sx={{ fontSize: 19 }} />
                </IconButton>
              </Tooltip>
              <Tooltip
                title={
                  hotspotOn
                    ? 'Hotspot on — click to stop'
                    : status && !status.internet
                    ? `${peer.deviceName} has no internet — share this computer's`
                    : "Share this computer's internet"
                }
              >
                <IconButton
                  onClick={() => onShareInternet(peer)}
                  aria-label="share internet"
                  sx={{ color: hotspotOn ? beacon.ok : status && !status.internet ? beacon.relay : beacon.textSecondary }}
                >
                  <RouterIcon sx={{ fontSize: 19 }} />
                </IconButton>
              </Tooltip>
              {peer.os === 'android' && (
                <Tooltip title="Messages (SMS)">
                  <IconButton onClick={onShowMessages} aria-label="messages" sx={{ color: beacon.textSecondary }}>
                    <SmsIcon sx={{ fontSize: 19 }} />
                  </IconButton>
                </Tooltip>
              )}
              <Tooltip title="Phone notifications">
                <IconButton onClick={onShowNotifications} aria-label="phone notifications" sx={{ color: beacon.textSecondary }}>
                  <Badge badgeContent={notificationCount} color="primary" max={99}>
                    <NotificationsIcon sx={{ fontSize: 19 }} />
                  </Badge>
                </IconButton>
              </Tooltip>
              <Tooltip title="Device settings">
                <IconButton onClick={(e) => setSettingsAnchor(e.currentTarget)} aria-label="device settings" sx={{ color: beacon.textSecondary }}>
                  <TuneIcon sx={{ fontSize: 19 }} />
                </IconButton>
              </Tooltip>
              <Menu anchorEl={settingsAnchor} open={!!settingsAnchor} onClose={() => setSettingsAnchor(null)}>
                <MenuItem onClick={() => onToggleAutoCopy(peer, !autoCopy)} sx={{ gap: 1.5 }}>
                  <Switch size="small" checked={autoCopy} />
                  <Typography sx={{ fontSize: '0.86rem' }}>Copy its messages to my clipboard</Typography>
                </MenuItem>
                {SETTING_LABELS.map(([key, label]) => (
                  <MenuItem key={key} onClick={() => onSetSetting(key, !settings[key])} sx={{ gap: 1.5 }}>
                    <Switch size="small" checked={!!settings[key]} />
                    <Typography sx={{ fontSize: '0.86rem' }}>{label}</Typography>
                  </MenuItem>
                ))}
              </Menu>
            </>
          )}
          <Button
            variant="outlined"
            startIcon={<AttachFileIcon sx={{ fontSize: 17 }} />}
            disabled={!online}
            onClick={() => onSendFiles(peer)}
            sx={{ height: 38, fontSize: '0.84rem', flexShrink: 0 }}
          >
            Send files
          </Button>
        </Box>
        {paired && (
          <Box sx={{ maxWidth: 900, mx: 'auto', px: { xs: 2, sm: 3 }, pb: 2, mt: -1 }}>
            <DeviceStatusPanel status={status} peer={peer} online={online} />
            {online && <PeerMediaCard device={peer.deviceName} state={media} receivedAt={mediaReceivedAt} onAction={onMediaAction} />}
          </Box>
        )}
      </Box>

      {/* ── Thread ───────────────────────────────────────────────────────── */}
      <Box ref={threadRef} sx={{ flex: 1, overflowY: 'auto', position: 'relative' }}>
        <Box sx={{ maxWidth: 900, mx: 'auto', px: { xs: 2, sm: 3 }, py: 3, minHeight: '100%', display: 'flex', flexDirection: 'column' }}>
          {thread.length === 0 ? (
            <Box sx={{ m: 'auto', textAlign: 'center', py: 6 }}>
              <ForumOutlinedIcon sx={{ fontSize: 48, color: beacon.textFaint, mb: 1.5 }} />
              <Typography sx={{ fontFamily: fonts.display, fontWeight: 700, fontSize: '1.1rem', color: beacon.text }}>
                Nothing shared yet
              </Typography>
              <Typography sx={{ fontSize: '0.84rem', color: beacon.textSecondary, mt: 0.5 }}>
                Type a message below, or drop files anywhere on this page.
              </Typography>
            </Box>
          ) : (
            <Box sx={{ mt: 'auto', display: 'flex', flexDirection: 'column', gap: 1.5 }}>
              {groups.map(({ label, entries }, gi) => (
                <React.Fragment key={`${label}-${gi}`}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, my: 1 }}>
                    <Box sx={{ flex: 1, height: '1px', backgroundColor: beacon.hairline }} />
                    <Typography variant="overline">{label}</Typography>
                    <Box sx={{ flex: 1, height: '1px', backgroundColor: beacon.hairline }} />
                  </Box>
                  {entries.map((entry) => (
                    <Bubble key={entry.id} entry={entry} onFileAction={onFileAction} loadPreview={loadPreview} onDelete={onDeleteEntry} />
                  ))}
                </React.Fragment>
              ))}
            </Box>
          )}
        </Box>

        {dragActive && online && (
          <Box
            sx={{
              position: 'absolute',
              inset: 12,
              borderRadius: '12px',
              border: `1.5px dashed ${beacon.accent}`,
              backgroundColor: alpha(beacon.bg, 0.85),
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              pointerEvents: 'none',
            }}
          >
            <Typography sx={{ color: beacon.accent, fontWeight: 600, fontSize: '0.95rem' }}>
              Drop to send to {displayName}
            </Typography>
          </Box>
        )}
      </Box>

      {/* ── Composer ─────────────────────────────────────────────────────── */}
      <Box sx={{ borderTop: `1px solid ${beacon.hairline}` }}>
        <Box sx={{ maxWidth: 900, mx: 'auto', px: { xs: 2, sm: 3 }, py: 1.5 }}>
          {(sendError || tooLong || !online) && (
            <Typography sx={{ fontSize: '0.76rem', color: online ? beacon.danger : beacon.textSecondary, mb: 1 }}>
              {!online
                ? `${displayName} is offline. Sending resumes when it reappears on the network.`
                : tooLong
                ? `Message is too long (${(draftBytes / 1024).toFixed(1)} KB of ${MAX_TEXT_BYTES / 1024} KB). Send it as a file instead.`
                : sendError}
            </Typography>
          )}
          <Box
            sx={{
              display: 'flex',
              alignItems: 'flex-end',
              gap: 1,
              p: 0.75,
              pl: 0.5,
              borderRadius: '12px',
              backgroundColor: beacon.surface,
              border: `1px solid ${beacon.hairline}`,
              '&:focus-within': { borderColor: beacon.textFaint },
            }}
          >
            <Tooltip title="Send files">
              <span>
                <IconButton
                  aria-label="attach files"
                  disabled={!online}
                  onClick={() => onSendFiles(peer)}
                  sx={{ color: beacon.textSecondary, '&:hover': { color: beacon.text } }}
                >
                  <AttachFileIcon sx={{ fontSize: 20 }} />
                </IconButton>
              </span>
            </Tooltip>
            <Tooltip title="Send clipboard">
              <span>
                <IconButton
                  aria-label="send clipboard"
                  disabled={!online || sending}
                  onClick={sendClipboard}
                  sx={{ color: beacon.textSecondary, '&:hover': { color: beacon.text } }}
                >
                  <ContentPasteGoIcon sx={{ fontSize: 20 }} />
                </IconButton>
              </span>
            </Tooltip>
            <InputBase
              inputRef={inputRef}
              multiline
              maxRows={8}
              fullWidth
              disabled={!online}
              placeholder={online ? `Message ${displayName}` : 'Device offline'}
              value={draft}
              onChange={(e) => {
                setDraft(e.target.value);
                if (sendError) setSendError(null);
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
                  e.preventDefault();
                  submit();
                }
              }}
              sx={{ fontSize: '0.9rem', color: beacon.text, py: 1, '& textarea::placeholder': { color: beacon.textFaint, opacity: 1 } }}
            />
            <Tooltip title="Send (Enter) · newline (Shift+Enter)">
              <span>
                <IconButton
                  aria-label="send message"
                  disabled={!canSend}
                  onClick={submit}
                  sx={{
                    width: 38,
                    height: 38,
                    borderRadius: '9px',
                    backgroundColor: beacon.accent,
                    color: beacon.onAccent,
                    '&:hover': { backgroundColor: beacon.accentHover },
                    '&.Mui-disabled': { backgroundColor: beacon.surfaceRaised, color: beacon.textFaint },
                  }}
                >
                  {sending ? <CircularProgress size={16} sx={{ color: beacon.textFaint }} /> : <SendIcon sx={{ fontSize: 18 }} />}
                </IconButton>
              </span>
            </Tooltip>
          </Box>
        </Box>
      </Box>
    </Box>
  );
};

export default DevicePage;
