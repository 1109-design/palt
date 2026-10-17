/**
 * PhoneNotificationsDrawer.tsx — a paired phone's current notifications,
 * with dismiss, their action buttons and reply.
 */
import React, { useState } from 'react';
import { Box, Drawer, IconButton, Typography, Button, InputBase, Tooltip } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import SendIcon from '@mui/icons-material/Send';
import { beacon, fonts } from '../theme/theme';
import type { PhoneNotification } from '../types/peer';

interface Props {
  open: boolean;
  device: string;
  items: PhoneNotification[];
  onClose: () => void;
  onDismiss: (n: PhoneNotification) => void;
  onAction: (n: PhoneNotification, index: number) => void;
  onReply: (n: PhoneNotification, text: string) => Promise<void>;
}

const timeAgo = (ms: number) => {
  const s = Math.max(0, (Date.now() - ms) / 1000);
  if (s < 60) return 'now';
  if (s < 3600) return `${Math.floor(s / 60)}m`;
  if (s < 86400) return `${Math.floor(s / 3600)}h`;
  return `${Math.floor(s / 86400)}d`;
};

const Item: React.FC<{ n: PhoneNotification } & Pick<Props, 'onDismiss' | 'onAction' | 'onReply'>> = ({
  n,
  onDismiss,
  onAction,
  onReply,
}) => {
  const [replying, setReplying] = useState(false);
  const [draft, setDraft] = useState('');
  const [busy, setBusy] = useState(false);
  const send = async () => {
    if (!draft.trim()) return;
    setBusy(true);
    try {
      await onReply(n, draft.trim());
      setDraft('');
      setReplying(false);
    } finally {
      setBusy(false);
    }
  };
  return (
    <Box sx={{ p: 1.5, borderRadius: '10px', border: `1px solid ${beacon.hairline}`, backgroundColor: beacon.surface }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography noWrap sx={{ fontSize: '0.72rem', color: beacon.textSecondary, flex: 1 }}>
          {n.app} · {timeAgo(n.time)}
        </Typography>
        <Tooltip title="Dismiss on the phone">
          <IconButton size="small" onClick={() => onDismiss(n)} sx={{ color: beacon.textFaint, p: 0.25 }}>
            <CloseIcon sx={{ fontSize: 15 }} />
          </IconButton>
        </Tooltip>
      </Box>
      {n.title && <Typography sx={{ fontSize: '0.86rem', fontWeight: 600, color: beacon.text, mt: 0.25 }}>{n.title}</Typography>}
      {n.text && (
        <Typography sx={{ fontSize: '0.82rem', color: beacon.textSecondary, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
          {n.text}
        </Typography>
      )}
      {(n.actions.length > 0 || n.replyable) && !replying && (
        <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5, mt: 1 }}>
          {n.replyable && (
            <Button size="small" variant="outlined" onClick={() => setReplying(true)}>
              Reply
            </Button>
          )}
          {n.actions.map((label, i) =>
            label ? (
              <Button key={i} size="small" variant="text" onClick={() => onAction(n, i)}>
                {label}
              </Button>
            ) : null,
          )}
        </Box>
      )}
      {replying && (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, mt: 1, pl: 1.25, borderRadius: '8px', border: `1px solid ${beacon.hairline}`, backgroundColor: beacon.bg }}>
          <InputBase
            autoFocus
            fullWidth
            placeholder="Reply…"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') send();
              if (e.key === 'Escape') setReplying(false);
            }}
            sx={{ fontSize: '0.84rem', color: beacon.text }}
          />
          <IconButton size="small" disabled={busy || !draft.trim()} onClick={send} sx={{ color: beacon.accent }}>
            <SendIcon sx={{ fontSize: 16 }} />
          </IconButton>
        </Box>
      )}
    </Box>
  );
};

const PhoneNotificationsDrawer: React.FC<Props> = ({ open, device, items, onClose, ...actions }) => (
  <Drawer anchor="right" open={open} onClose={onClose} PaperProps={{ sx: { width: 380, maxWidth: '90vw', backgroundColor: beacon.bg } }}>
    <Box sx={{ p: 2.5, pb: 1.5, display: 'flex', alignItems: 'center' }}>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography variant="overline" sx={{ display: 'block', lineHeight: 1.4 }}>
          {device}
        </Typography>
        <Typography sx={{ fontFamily: fonts.display, fontWeight: 700, fontSize: '1.15rem', color: beacon.text }}>
          Notifications
        </Typography>
      </Box>
      <IconButton onClick={onClose} sx={{ color: beacon.textSecondary }}>
        <CloseIcon />
      </IconButton>
    </Box>
    <Box sx={{ px: 2, pb: 2, display: 'flex', flexDirection: 'column', gap: 1, overflowY: 'auto' }}>
      {items.length === 0 ? (
        <Typography sx={{ fontSize: '0.84rem', color: beacon.textFaint, px: 0.5 }}>
          Nothing new. Notifications from the phone show up here and as desktop notifications.
        </Typography>
      ) : (
        items.map((n) => <Item key={n.id} n={n} {...actions} />)
      )}
    </Box>
  </Drawer>
);

export default PhoneNotificationsDrawer;
