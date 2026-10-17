/**
 * PhoneNotificationsDrawer.tsx — side panel for a paired device: its current
 * notifications (dismiss, action buttons, reply) and the clipboard history.
 */
import React, { useState } from 'react';
import { Box, Drawer, IconButton, Typography, Button, InputBase, Tooltip, Tabs, Tab } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import SendIcon from '@mui/icons-material/Send';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import { beacon, fonts } from '../theme/theme';
import type { ClipboardEntry, ClipboardHelper, PhoneNotification } from '../types/peer';

export type DrawerTab = 'notifications' | 'clipboard';

interface Props {
  open: boolean;
  tab: DrawerTab;
  onTab: (t: DrawerTab) => void;
  device: string;
  items: PhoneNotification[];
  clipboard: ClipboardEntry[];
  helper: ClipboardHelper | null;
  onInstallHelper: () => Promise<void>;
  onCopy: (text: string) => void;
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

const ClipboardPanel: React.FC<Pick<Props, 'clipboard' | 'helper' | 'onInstallHelper' | 'onCopy' | 'device'>> = ({
  clipboard,
  helper,
  onInstallHelper,
  onCopy,
  device,
}) => {
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  return (
    <>
      {helper && !helper.running && (
        <Box sx={{ p: 1.5, borderRadius: '10px', border: `1px solid ${beacon.accent}55`, backgroundColor: `${beacon.accent}10` }}>
          <Typography sx={{ fontSize: '0.84rem', fontWeight: 600, color: beacon.text }}>Turn on clipboard sync</Typography>
          <Typography sx={{ fontSize: '0.78rem', color: beacon.textSecondary, mt: 0.5 }}>
            GNOME only lets the app you're using touch the clipboard. A tiny GNOME helper lets PALT sync it
            in the background.{' '}
            {helper.installed || done ? 'It is installed — log out and back in once to start it.' : ''}
          </Typography>
          {!helper.installed && !done && (
            <Button
              size="small"
              variant="contained"
              disabled={busy}
              sx={{ mt: 1 }}
              onClick={async () => {
                setBusy(true);
                try {
                  await onInstallHelper();
                  setDone(true);
                } finally {
                  setBusy(false);
                }
              }}
            >
              Install helper
            </Button>
          )}
        </Box>
      )}
      {clipboard.length === 0 ? (
        <Typography sx={{ fontSize: '0.84rem', color: beacon.textFaint, px: 0.5 }}>
          Copy something here or on {device} and it shows up on the other side, and in this list.
        </Typography>
      ) : (
        clipboard.map((c, i) => (
          <Box key={i} sx={{ p: 1.5, borderRadius: '10px', border: `1px solid ${beacon.hairline}`, backgroundColor: beacon.surface, display: 'flex', gap: 1 }}>
            <Box sx={{ minWidth: 0, flex: 1 }}>
              <Typography sx={{ fontSize: '0.72rem', color: beacon.textSecondary }}>
                {c.from ? `from ${c.from}` : 'copied here'} · {timeAgo(c.time)}
              </Typography>
              <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.78rem', color: beacon.text, whiteSpace: 'pre-wrap', wordBreak: 'break-word', maxHeight: 96, overflow: 'hidden' }}>
                {c.text}
              </Typography>
            </Box>
            <Tooltip title="Copy again">
              <IconButton size="small" onClick={() => onCopy(c.text)} sx={{ color: beacon.textFaint, alignSelf: 'flex-start' }}>
                <ContentCopyIcon sx={{ fontSize: 15 }} />
              </IconButton>
            </Tooltip>
          </Box>
        ))
      )}
    </>
  );
};

const PhoneNotificationsDrawer: React.FC<Props> = ({
  open,
  tab,
  onTab,
  device,
  items,
  clipboard,
  helper,
  onInstallHelper,
  onCopy,
  onClose,
  ...actions
}) => (
  <Drawer anchor="right" open={open} onClose={onClose} PaperProps={{ sx: { width: 380, maxWidth: '90vw', backgroundColor: beacon.bg } }}>
    <Box sx={{ p: 2.5, pb: 1.5, display: 'flex', alignItems: 'center' }}>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography variant="overline" sx={{ display: 'block', lineHeight: 1.4 }}>
          {device}
        </Typography>
        <Typography sx={{ fontFamily: fonts.display, fontWeight: 700, fontSize: '1.15rem', color: beacon.text }}>
          Activity
        </Typography>
      </Box>
      <IconButton onClick={onClose} sx={{ color: beacon.textSecondary }}>
        <CloseIcon />
      </IconButton>
    </Box>
    <Tabs value={tab} onChange={(_, v) => onTab(v)} sx={{ px: 2, mb: 1.5, minHeight: 36 }}>
      <Tab value="notifications" label={`Notifications${items.length ? ` · ${items.length}` : ''}`} sx={{ minHeight: 36, textTransform: 'none' }} />
      <Tab value="clipboard" label="Clipboard" sx={{ minHeight: 36, textTransform: 'none' }} />
    </Tabs>
    <Box sx={{ px: 2, pb: 2, display: 'flex', flexDirection: 'column', gap: 1, overflowY: 'auto' }}>
      {tab === 'clipboard' ? (
        <ClipboardPanel clipboard={clipboard} helper={helper} onInstallHelper={onInstallHelper} onCopy={onCopy} device={device} />
      ) : items.length === 0 ? (
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
