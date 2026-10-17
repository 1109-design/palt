/**
 * MessagesDialog.tsx — a paired phone's SMS on the computer (Beacon)
 *
 * Conversations on the left, the open thread on the right with a reply box.
 * New texts and sends refresh it live (sms_received / sms_changed events).
 * "New message" picks a contact or takes a number.
 */
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Autocomplete,
  Avatar,
  Badge,
  Box,
  Button,
  CircularProgress,
  Dialog,
  IconButton,
  InputBase,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import { alpha } from '@mui/material/styles';
import CloseIcon from '@mui/icons-material/Close';
import SendIcon from '@mui/icons-material/ArrowUpward';
import EditIcon from '@mui/icons-material/EditOutlined';
import SearchIcon from '@mui/icons-material/Search';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import CallIcon from '@mui/icons-material/CallRounded';
import CallMadeIcon from '@mui/icons-material/CallMadeRounded';
import CallReceivedIcon from '@mui/icons-material/CallReceivedRounded';
import CallMissedIcon from '@mui/icons-material/CallMissedRounded';
import BlockIcon from '@mui/icons-material/BlockRounded';
import SmsIcon from '@mui/icons-material/SmsOutlined';
import SimCardIcon from '@mui/icons-material/SimCardOutlined';
import { beacon, fonts } from '../theme/theme';
import type { CallLogEntry, Contact, SIM, SMSMessage } from '../types/peer';

export interface MessagesApi {
  getConversations: (device: string) => Promise<SMSMessage[]>;
  getThread: (device: string, threadId: number, before: number) => Promise<SMSMessage[]>;
  send: (device: string, address: string, body: string, sim: number) => Promise<void>;
  getContacts: (device: string) => Promise<Contact[]>;
  getCallLog: (device: string, before: number) => Promise<CallLogEntry[]>;
  getSims: (device: string) => Promise<SIM[]>;
  placeCall: (device: string, number: string, sim: number) => Promise<void>;
  copy: (text: string) => void;
}

export type PhoneTab = 'messages' | 'calls';

/** The SIM choice, remembered per phone. */
const simKey = (device: string) => `palt.sim.${device}`;
const loadSim = (device: string) => {
  try {
    return Number(localStorage.getItem(simKey(device)) || 0);
  } catch {
    return 0;
  }
};

/** "SIM 1 · Econet" chips when the phone has two SIMs; nothing with one. */
const SimChips: React.FC<{ sims: SIM[]; value: number; onChange: (id: number) => void; size?: 'small' | 'medium' }> = ({ sims, value, onChange, size = 'small' }) => {
  if (sims.length < 2) return null;
  return (
    <Box sx={{ display: 'flex', gap: 0.75 }}>
      {sims.map((s) => {
        const on = value === s.id || (!sims.some((x) => x.id === value) && s.slot === 1);
        return (
          <Box
            key={s.id}
            component="button"
            onClick={() => onChange(s.id)}
            sx={{
              display: 'inline-flex', alignItems: 'center', gap: 0.5, cursor: 'pointer',
              px: size === 'small' ? 1 : 1.4, py: size === 'small' ? 0.35 : 0.6, borderRadius: 999,
              border: `1px solid ${on ? beacon.accent : beacon.hairline}`,
              backgroundColor: on ? alpha(beacon.accent, 0.14) : 'transparent',
              color: on ? beacon.accent : beacon.textSecondary,
              fontFamily: fonts.mono, fontSize: size === 'small' ? '0.66rem' : '0.74rem', fontWeight: 600,
            }}
          >
            <SimCardIcon sx={{ fontSize: size === 'small' ? 13 : 15 }} />
            {s.slot} · {s.name}
          </Box>
        );
      })}
    </Box>
  );
};

const callIcon = (type: string) =>
  type === 'missed' || type === 'rejected'
    ? { Icon: CallMissedIcon, color: beacon.danger }
    : type === 'blocked'
    ? { Icon: BlockIcon, color: beacon.textFaint }
    : type === 'outgoing'
    ? { Icon: CallMadeIcon, color: beacon.accent }
    : { Icon: CallReceivedIcon, color: beacon.ok };

const duration = (s: number) => (s >= 3600 ? `${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m` : s >= 60 ? `${Math.floor(s / 60)}m ${s % 60}s` : `${s}s`);

/** Last 9 digits: matches local and international forms of a number. */
const numberKey = (n: string) => n.replace(/\D/g, '').slice(-9);

const when = (ms: number) => {
  const d = new Date(ms);
  const now = new Date();
  if (d.toDateString() === now.toDateString()) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  const days = (now.getTime() - ms) / 86400000;
  if (days < 6) return d.toLocaleDateString([], { weekday: 'short' });
  return d.toLocaleDateString([], { day: 'numeric', month: 'short', year: d.getFullYear() === now.getFullYear() ? undefined : '2-digit' });
};

const initials = (s: string) =>
  s.replace(/[^\p{L}\p{N} ]/gu, '').split(' ').filter(Boolean).slice(0, 2).map((w) => w[0]).join('').toUpperCase() || '#';

const MessagesDialog: React.FC<{
  open: boolean;
  device: string;
  /** Open this conversation (from a notification), focusing the reply. */
  focus?: { threadId: number; address: string; reply: boolean } | null;
  /** Bumped on sms_received / sms_changed for this device. */
  refreshKey: number;
  api: MessagesApi;
  /** Tab to start on. */
  tab?: PhoneTab;
  onClose: () => void;
}> = ({ open, device, focus, refreshKey, api, tab: startTab = 'messages', onClose }) => {
  const [tab, setTab] = useState<PhoneTab>(startTab);
  const [sims, setSims] = useState<SIM[]>([]);
  const [sim, setSimState] = useState(() => loadSim(device));
  const setSim = (id: number) => {
    setSimState(id);
    try {
      localStorage.setItem(simKey(device), String(id));
    } catch {
      /* per-viewer convenience only */
    }
  };
  const [calls, setCalls] = useState<CallLogEntry[] | null>(null);
  const [callsMore, setCallsMore] = useState(false);
  const [picked, setPicked] = useState<{ number: string; name: string } | null>(null);
  const [dial, setDial] = useState('');
  const [threads, setThreads] = useState<SMSMessage[] | null>(null);
  const [error, setError] = useState('');
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [current, setCurrent] = useState<{ threadId: number; address: string; name: string } | null>(null);
  const [messages, setMessages] = useState<SMSMessage[]>([]);
  const [loadingThread, setLoadingThread] = useState(false);
  const [more, setMore] = useState(false);
  const [draft, setDraft] = useState('');
  const [sending, setSending] = useState(false);
  const [filter, setFilter] = useState('');
  const [composing, setComposing] = useState(false);
  const replyRef = useRef<HTMLInputElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  const photos = useMemo(() => {
    const m: Record<string, string> = {};
    for (const c of contacts) if (c.photo) for (const n of c.numbers) m[numberKey(n)] = c.photo;
    return m;
  }, [contacts]);

  const loadThreads = useCallback(() => {
    api
      .getConversations(device)
      .then((t) => {
        setThreads(t || []);
        setError('');
      })
      .catch((e: any) => setError(String(e?.message || e)));
  }, [api, device]);

  const loadThread = useCallback(
    (threadId: number, older = false) => {
      if (!threadId) return;
      setLoadingThread(true);
      const before = older && messages.length ? messages[messages.length - 1].date : 0;
      api
        .getThread(device, threadId, before)
        .then((m) => {
          setMessages((prev) => (older ? [...prev, ...(m || [])] : m || []));
          setMore((m || []).length >= 60);
        })
        .catch((e: any) => setError(String(e?.message || e)))
        .finally(() => setLoadingThread(false));
    },
    [api, device, messages],
  );

  useEffect(() => {
    if (!open) return;
    loadThreads();
    api.getContacts(device).then((c) => setContacts(c || [])).catch(() => {});
    api.getSims(device).then((s) => setSims(s || [])).catch(() => {});
  }, [open, device, refreshKey]); // eslint-disable-line react-hooks/exhaustive-deps

  const loadCalls = useCallback(
    (older = false) => {
      const before = older && calls?.length ? calls[calls.length - 1].date : 0;
      api
        .getCallLog(device, before)
        .then((c) => {
          setCalls((prev) => (older ? [...(prev || []), ...(c || [])] : c || []));
          setCallsMore((c || []).length >= 80);
          setError('');
        })
        .catch((e: any) => setError(String(e?.message || e)));
    },
    [api, device, calls],
  );
  useEffect(() => {
    if (open && tab === 'calls') loadCalls();
  }, [open, tab, refreshKey]); // eslint-disable-line react-hooks/exhaustive-deps

  const call = (number: string) => {
    if (!number.trim()) return;
    api.placeCall(device, number.trim(), sims.length > 1 ? sim : 0).catch((e: any) => setError(String(e?.message || e)));
  };
  const nameFor = (number: string) => {
    const k = numberKey(number);
    return contacts.find((c) => c.numbers.some((n) => numberKey(n) === k))?.name || '';
  };
  const messageNumber = (number: string, name: string) => {
    setTab('messages');
    const existing = threads?.find((t) => numberKey(t.address) === numberKey(number));
    if (existing) openThread(existing.threadId, existing.address, existing.name);
    else {
      setComposing(false);
      setCurrent({ threadId: 0, address: number, name });
      setMessages([]);
      setTimeout(() => replyRef.current?.focus(), 100);
    }
  };

  // Keep the open thread live.
  useEffect(() => {
    if (open && current?.threadId) loadThread(current.threadId);
  }, [refreshKey]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!open || !focus) return;
    const t = threads?.find((x) => x.threadId === focus.threadId);
    openThread(focus.threadId, focus.address, t?.name || '');
    if (focus.reply) setTimeout(() => replyRef.current?.focus(), 300);
  }, [open, focus]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ block: 'end' });
  }, [current?.threadId, messages.length > 0 && messages[0].id]); // eslint-disable-line react-hooks/exhaustive-deps

  const openThread = (threadId: number, address: string, name: string) => {
    setComposing(false);
    setCurrent({ threadId, address, name });
    setMessages([]);
    setDraft('');
    if (threadId) loadThread(threadId);
  };

  const send = async () => {
    const body = draft.trim();
    if (!current || !body) return;
    setSending(true);
    try {
      await api.send(device, current.address, body, sims.length > 1 ? sim : 0);
      setDraft('');
      // The phone reports the stored copy via sms_changed; show it meanwhile.
      setMessages((m) => [
        { id: -Date.now(), threadId: current.threadId, address: current.address, body, date: Date.now(), type: 4, read: true, name: '' },
        ...m,
      ]);
      setTimeout(loadThreads, 1500);
    } catch (e: any) {
      setError(String(e?.message || e));
    } finally {
      setSending(false);
    }
  };

  const shown = (threads || []).filter((t) => {
    const q = filter.trim().toLowerCase();
    return !q || t.name.toLowerCase().includes(q) || t.address.includes(q) || t.body.toLowerCase().includes(q);
  });

  const avatar = (name: string, address: string, size = 40) => (
    <Avatar
      src={photos[numberKey(address)]}
      sx={{ width: size, height: size, fontSize: size * 0.38, fontFamily: fonts.display, fontWeight: 700, bgcolor: alpha(beacon.accent, 0.18), color: beacon.accent }}
    >
      {initials(name || address)}
    </Avatar>
  );

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth={false}
      PaperProps={{ sx: { width: 'min(1100px, 94vw)', height: '86vh', backgroundColor: beacon.bg, backgroundImage: 'none', border: `1px solid ${beacon.hairline}`, display: 'flex' } }}
    >
      <Box sx={{ display: 'flex', flex: 1, minHeight: 0 }}>
        {/* ── Conversations ─────────────────────────────────────────── */}
        <Box sx={{ width: { xs: 260, md: 330 }, borderRight: `1px solid ${beacon.hairline}`, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
          <Box sx={{ p: 2, pb: 1, display: 'flex', alignItems: 'center', gap: 1 }}>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography sx={{ fontFamily: fonts.display, fontWeight: 800, fontSize: '1.15rem', color: beacon.text }}>
                {tab === 'calls' ? 'Calls' : 'Messages'}
              </Typography>
              <Typography noWrap sx={{ fontFamily: fonts.mono, fontSize: '0.66rem', color: beacon.textFaint, textTransform: 'uppercase', letterSpacing: '0.08em' }}>
                on {device}
              </Typography>
            </Box>
            {tab === 'messages' && (
              <Tooltip title="New message">
                <IconButton onClick={() => { setComposing(true); setCurrent(null); }} sx={{ color: beacon.accent }}>
                  <EditIcon />
                </IconButton>
              </Tooltip>
            )}
          </Box>
          <Box sx={{ mx: 2, mb: 1.25, p: 0.4, display: 'flex', borderRadius: 999, backgroundColor: beacon.surface, border: `1px solid ${beacon.hairline}` }}>
            {(['messages', 'calls'] as PhoneTab[]).map((t) => (
              <Box
                key={t}
                component="button"
                onClick={() => setTab(t)}
                sx={{
                  flex: 1, display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: 0.75, py: 0.7,
                  border: 0, borderRadius: 999, cursor: 'pointer', fontWeight: 700, fontSize: '0.8rem',
                  backgroundColor: tab === t ? beacon.surfaceRaised : 'transparent',
                  color: tab === t ? beacon.text : beacon.textFaint,
                  boxShadow: tab === t ? `0 1px 0 ${alpha('#fff', 0.04)} inset` : 'none',
                }}
              >
                {t === 'messages' ? <SmsIcon sx={{ fontSize: 16 }} /> : <CallIcon sx={{ fontSize: 16 }} />}
                {t === 'messages' ? 'Messages' : 'Calls'}
              </Box>
            ))}
          </Box>
          {tab === 'messages' ? (
            <>
          <Box sx={{ mx: 2, mb: 1, px: 1.25, display: 'flex', alignItems: 'center', gap: 1, borderRadius: 2, border: `1px solid ${beacon.hairline}`, backgroundColor: beacon.surface }}>
            <SearchIcon sx={{ fontSize: 17, color: beacon.textFaint }} />
            <InputBase value={filter} onChange={(e) => setFilter(e.target.value)} placeholder="Search" sx={{ flex: 1, fontSize: '0.85rem', py: 0.6 }} />
          </Box>
          <Box sx={{ flex: 1, overflowY: 'auto' }}>
            {threads === null && !error && (
              <Box sx={{ display: 'grid', placeItems: 'center', py: 6 }}>
                <CircularProgress size={22} />
              </Box>
            )}
            {threads !== null && shown.length === 0 && (
              <Typography sx={{ p: 3, color: beacon.textFaint, fontSize: '0.85rem', textAlign: 'center' }}>
                {filter ? 'No matches' : 'No messages — or PALT on the phone needs the SMS permission.'}
              </Typography>
            )}
            {shown.map((t) => {
              const active = current?.threadId === t.threadId;
              return (
                <Box
                  key={t.threadId}
                  onClick={() => openThread(t.threadId, t.address, t.name)}
                  sx={{
                    display: 'flex', gap: 1.25, px: 2, py: 1.25, cursor: 'pointer', alignItems: 'center',
                    backgroundColor: active ? alpha(beacon.accent, 0.08) : 'transparent',
                    borderLeft: `3px solid ${active ? beacon.accent : 'transparent'}`,
                    '&:hover': { backgroundColor: active ? alpha(beacon.accent, 0.1) : beacon.surface },
                  }}
                >
                  <Badge color="primary" badgeContent={t.unread || 0} max={99}>
                    {avatar(t.name, t.address)}
                  </Badge>
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 1 }}>
                      <Typography noWrap sx={{ flex: 1, fontWeight: t.unread ? 800 : 600, fontSize: '0.9rem', color: beacon.text }}>
                        {t.name || t.address}
                      </Typography>
                      <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.66rem', color: t.unread ? beacon.accent : beacon.textFaint, flexShrink: 0 }}>
                        {when(t.date)}
                      </Typography>
                    </Box>
                    <Typography noWrap sx={{ fontSize: '0.8rem', color: t.unread ? beacon.textSecondary : beacon.textFaint }}>
                      {t.type !== 1 ? 'You: ' : ''}
                      {t.body}
                    </Typography>
                  </Box>
                </Box>
              );
            })}
          </Box>
            </>
          ) : (
            <>
          <Box sx={{ mx: 2, mb: 1, display: 'flex', gap: 1, alignItems: 'center' }}>
            <Autocomplete
              freeSolo
              sx={{ flex: 1 }}
              size="small"
              inputValue={dial}
              onInputChange={(_, v) => setDial(v)}
              options={contacts.flatMap((c) => c.numbers.map((n) => ({ label: `${c.name} · ${n}`, name: c.name, number: n })))}
              filterOptions={(opts, s) => {
                const q = s.inputValue.toLowerCase();
                if (!q) return [];
                return opts.filter((o) => o.label.toLowerCase().includes(q) || numberKey(o.number).includes(q.replace(/\D/g, '') || '§')).slice(0, 30);
              }}
              onChange={(_, v) => {
                if (v && typeof v !== 'string') setPicked({ number: v.number, name: v.name });
              }}
              renderInput={(params) => <TextField {...params} placeholder="Name or number" />}
            />
            <Tooltip title="Call">
              <span>
                <IconButton
                  disabled={!dial.trim()}
                  onClick={() => call(dial.includes('·') ? dial.split('·').pop()!.trim() : dial)}
                  sx={{ bgcolor: beacon.ok, color: beacon.bg, '&:hover': { bgcolor: alpha(beacon.ok, 0.85) }, '&.Mui-disabled': { bgcolor: beacon.surfaceRaised } }}
                >
                  <CallIcon />
                </IconButton>
              </span>
            </Tooltip>
          </Box>
          <Box sx={{ flex: 1, overflowY: 'auto' }}>
            {calls === null && !error && (
              <Box sx={{ display: 'grid', placeItems: 'center', py: 6 }}>
                <CircularProgress size={22} />
              </Box>
            )}
            {calls !== null && calls.length === 0 && (
              <Typography sx={{ p: 3, color: beacon.textFaint, fontSize: '0.85rem', textAlign: 'center' }}>No calls yet</Typography>
            )}
            {(calls || []).map((c, i) => {
              const { Icon, color } = callIcon(c.type);
              const name = c.name || nameFor(c.number);
              const active = picked?.number === c.number;
              const prev = calls![i - 1];
              const newDay = !prev || new Date(prev.date).toDateString() !== new Date(c.date).toDateString();
              return (
                <React.Fragment key={`${c.date}-${i}`}>
                  {newDay && (
                    <Typography sx={{ px: 2, pt: 1.5, pb: 0.5, fontFamily: fonts.mono, fontSize: '0.62rem', color: beacon.textFaint, textTransform: 'uppercase', letterSpacing: '0.08em' }}>
                      {new Date(c.date).toDateString() === new Date().toDateString()
                        ? 'Today'
                        : new Date(c.date).toLocaleDateString([], { weekday: 'long', day: 'numeric', month: 'short' })}
                    </Typography>
                  )}
                  <Box
                    onClick={() => setPicked({ number: c.number, name })}
                    sx={{
                      display: 'flex', gap: 1.25, px: 2, py: 1, cursor: 'pointer', alignItems: 'center',
                      backgroundColor: active ? alpha(beacon.accent, 0.08) : 'transparent',
                      borderLeft: `3px solid ${active ? beacon.accent : 'transparent'}`,
                      '&:hover': { backgroundColor: active ? alpha(beacon.accent, 0.1) : beacon.surface },
                      '&:hover .quickcall': { opacity: 1 },
                    }}
                  >
                    {avatar(name, c.number, 38)}
                    <Box sx={{ flex: 1, minWidth: 0 }}>
                      <Typography noWrap sx={{ fontWeight: 600, fontSize: '0.9rem', color: c.type === 'missed' ? beacon.danger : beacon.text }}>
                        {name || c.number || 'Unknown'}
                      </Typography>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                        <Icon sx={{ fontSize: 14, color }} />
                        <Typography noWrap sx={{ fontFamily: fonts.mono, fontSize: '0.68rem', color: beacon.textFaint }}>
                          {new Date(c.date).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                          {c.duration > 0 ? ` · ${duration(c.duration)}` : ''}
                          {sims.length > 1 && c.sim ? ` · SIM ${c.sim}` : ''}
                        </Typography>
                      </Box>
                    </Box>
                    <IconButton
                      className="quickcall"
                      size="small"
                      onClick={(e) => {
                        e.stopPropagation();
                        call(c.number);
                      }}
                      sx={{ opacity: 0, color: beacon.ok }}
                    >
                      <CallIcon fontSize="small" />
                    </IconButton>
                  </Box>
                </React.Fragment>
              );
            })}
            {callsMore && (
              <Box sx={{ display: 'grid', placeItems: 'center', py: 1 }}>
                <Button size="small" onClick={() => loadCalls(true)}>Older calls</Button>
              </Box>
            )}
          </Box>
            </>
          )}
        </Box>

        {/* ── Thread ────────────────────────────────────────────────── */}
        {tab === 'messages' ? (
        <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
          <Box sx={{ px: 2.5, py: 1.5, display: 'flex', alignItems: 'center', gap: 1.5, borderBottom: `1px solid ${beacon.hairline}`, minHeight: 64 }}>
            {current && avatar(current.name, current.address, 36)}
            {composing ? (
              <Autocomplete
                freeSolo
                autoFocus
                sx={{ flex: 1 }}
                options={contacts.flatMap((c) => c.numbers.map((n) => ({ label: `${c.name} · ${n}`, name: c.name, number: n })))}
                filterOptions={(opts, s) => {
                  const q = s.inputValue.toLowerCase();
                  return opts.filter((o) => o.label.toLowerCase().includes(q) || numberKey(o.number).includes(q.replace(/\D/g, '') || '§')).slice(0, 30);
                }}
                onChange={(_, v) => {
                  if (!v) return;
                  const number = typeof v === 'string' ? v : v.number;
                  const name = typeof v === 'string' ? '' : v.name;
                  const existing = threads?.find((t) => numberKey(t.address) === numberKey(number));
                  if (existing) openThread(existing.threadId, existing.address, existing.name);
                  else {
                    setComposing(false);
                    setCurrent({ threadId: 0, address: number, name });
                    setMessages([]);
                    setTimeout(() => replyRef.current?.focus(), 100);
                  }
                }}
                renderInput={(params) => <TextField {...params} autoFocus size="small" placeholder="To: name or number" />}
              />
            ) : current ? (
              <Box sx={{ minWidth: 0 }}>
                <Typography noWrap sx={{ fontWeight: 700, color: beacon.text }}>{current.name || current.address}</Typography>
                {current.name && <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.7rem', color: beacon.textFaint }}>{current.address}</Typography>}
              </Box>
            ) : null}
            {current && !composing ? (
              <Tooltip title={`Call ${current.name || current.address}`}>
                <IconButton onClick={() => call(current.address)} sx={{ ml: 0.5, color: beacon.ok }}>
                  <CallIcon />
                </IconButton>
              </Tooltip>
            ) : !current && !composing ? (
              <Typography sx={{ color: beacon.textFaint }}>Pick a conversation</Typography>
            ) : null}
            <Box sx={{ flex: 1 }} />
            <IconButton onClick={onClose} aria-label="close">
              <CloseIcon />
            </IconButton>
          </Box>

          {error && (
            <Typography sx={{ px: 2.5, py: 1, fontSize: '0.8rem', color: beacon.danger, borderBottom: `1px solid ${beacon.hairline}` }}>{error}</Typography>
          )}

          <Box sx={{ flex: 1, overflowY: 'auto', px: 2.5, py: 2, display: 'flex', flexDirection: 'column-reverse', gap: 0.75 }}>
            <div ref={bottomRef} />
            {messages.map((m, i) => {
              const mine = m.type !== 1;
              const older = messages[i + 1];
              const newDay = !older || new Date(older.date).toDateString() !== new Date(m.date).toDateString();
              return (
                <React.Fragment key={m.id}>
                  <Box sx={{ alignSelf: mine ? 'flex-end' : 'flex-start', maxWidth: '72%', display: 'flex', alignItems: 'flex-end', gap: 0.5, flexDirection: mine ? 'row-reverse' : 'row', '&:hover .copy': { opacity: 1 } }}>
                    <Box
                      sx={{
                        px: 1.5, py: 1, borderRadius: '14px', whiteSpace: 'pre-wrap', wordBreak: 'break-word',
                        fontSize: '0.88rem', lineHeight: 1.45,
                        backgroundColor: mine ? alpha(beacon.accent, 0.16) : beacon.surfaceRaised,
                        color: beacon.text,
                        border: `1px solid ${mine ? alpha(beacon.accent, 0.3) : beacon.hairline}`,
                        opacity: m.type >= 4 ? 0.6 : 1,
                      }}
                    >
                      {m.body}
                      <Typography component="span" sx={{ display: 'block', textAlign: 'right', fontFamily: fonts.mono, fontSize: '0.6rem', color: beacon.textFaint, mt: 0.25 }}>
                        {new Date(m.date).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                        {sims.length > 1 && m.subscription != null && m.subscription >= 0
                          ? ` · SIM ${sims.find((s) => s.id === m.subscription)?.slot ?? '?'}`
                          : ''}
                        {m.type === 5 ? ' · failed' : m.type >= 4 ? ' · sending' : ''}
                      </Typography>
                    </Box>
                    <IconButton className="copy" size="small" onClick={() => api.copy(m.body)} sx={{ opacity: 0, color: beacon.textFaint }}>
                      <ContentCopyIcon sx={{ fontSize: 14 }} />
                    </IconButton>
                  </Box>
                  {newDay && (
                    <Typography sx={{ alignSelf: 'center', fontFamily: fonts.mono, fontSize: '0.64rem', color: beacon.textFaint, textTransform: 'uppercase', letterSpacing: '0.08em', my: 1 }}>
                      {new Date(m.date).toLocaleDateString([], { weekday: 'long', day: 'numeric', month: 'long' })}
                    </Typography>
                  )}
                </React.Fragment>
              );
            })}
            {current && more && (
              <Button size="small" disabled={loadingThread} onClick={() => loadThread(current.threadId, true)} sx={{ alignSelf: 'center' }}>
                Older messages
              </Button>
            )}
            {loadingThread && messages.length === 0 && <CircularProgress size={22} sx={{ alignSelf: 'center', my: 4 }} />}
          </Box>

          {current && sims.length > 1 && (
            <Box sx={{ px: 1.5, pt: 1, borderTop: `1px solid ${beacon.hairline}`, display: 'flex', alignItems: 'center', gap: 1 }}>
              <Typography sx={{ fontSize: '0.72rem', color: beacon.textFaint }}>Send from</Typography>
              <SimChips sims={sims} value={sim} onChange={setSim} />
            </Box>
          )}
          {current && (
            <Box sx={{ p: 1.5, borderTop: sims.length > 1 ? 'none' : `1px solid ${beacon.hairline}`, display: 'flex', gap: 1, alignItems: 'flex-end' }}>
              <InputBase
                inputRef={replyRef}
                multiline
                maxRows={6}
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault();
                    send();
                  }
                }}
                placeholder={`Text ${current.name || current.address}`}
                sx={{ flex: 1, px: 1.75, py: 1, borderRadius: 3, border: `1px solid ${beacon.hairline}`, backgroundColor: beacon.surface, fontSize: '0.9rem' }}
              />
              <IconButton
                onClick={send}
                disabled={!draft.trim() || sending}
                sx={{ backgroundColor: beacon.accent, color: beacon.bg, '&:hover': { backgroundColor: beacon.accentHover }, '&.Mui-disabled': { backgroundColor: beacon.surfaceRaised } }}
              >
                {sending ? <CircularProgress size={18} /> : <SendIcon />}
              </IconButton>
            </Box>
          )}
        </Box>
        ) : (
        <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
          <Box sx={{ px: 2.5, py: 1.5, display: 'flex', alignItems: 'center', borderBottom: `1px solid ${beacon.hairline}`, minHeight: 64 }}>
            <SimChips sims={sims} value={sim} onChange={setSim} size="medium" />
            <Box sx={{ flex: 1 }} />
            <IconButton onClick={onClose} aria-label="close">
              <CloseIcon />
            </IconButton>
          </Box>
          {error && (
            <Typography sx={{ px: 2.5, py: 1, fontSize: '0.8rem', color: beacon.danger, borderBottom: `1px solid ${beacon.hairline}` }}>{error}</Typography>
          )}
          {picked ? (
            <Box sx={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column', alignItems: 'center', px: 3, py: 5 }}>
              {avatar(picked.name, picked.number, 112)}
              <Typography sx={{ mt: 2, fontFamily: fonts.display, fontWeight: 800, fontSize: '1.5rem', color: beacon.text, textAlign: 'center' }}>
                {picked.name || picked.number}
              </Typography>
              {picked.name && <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.85rem', color: beacon.textSecondary }}>{picked.number}</Typography>}
              <Box sx={{ display: 'flex', gap: 1.5, mt: 3 }}>
                <Button
                  onClick={() => call(picked.number)}
                  startIcon={<CallIcon />}
                  sx={{ height: 48, px: 3, borderRadius: '24px', bgcolor: beacon.ok, color: beacon.bg, fontWeight: 800, '&:hover': { bgcolor: alpha(beacon.ok, 0.85) } }}
                >
                  Call{sims.length > 1 ? ` · SIM ${(sims.find((s) => s.id === sim) || sims[0]).slot}` : ''}
                </Button>
                <Button
                  onClick={() => messageNumber(picked.number, picked.name)}
                  startIcon={<SmsIcon />}
                  variant="outlined"
                  sx={{ height: 48, px: 3, borderRadius: '24px', fontWeight: 700 }}
                >
                  Message
                </Button>
              </Box>
              <Box sx={{ width: '100%', maxWidth: 420, mt: 4 }}>
                <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.64rem', color: beacon.textFaint, textTransform: 'uppercase', letterSpacing: '0.08em', mb: 1 }}>
                  Recent calls
                </Typography>
                {(calls || [])
                  .filter((c) => numberKey(c.number) === numberKey(picked.number))
                  .slice(0, 12)
                  .map((c, i) => {
                    const { Icon, color } = callIcon(c.type);
                    return (
                      <Box key={i} sx={{ display: 'flex', alignItems: 'center', gap: 1.25, py: 0.9, borderBottom: `1px solid ${beacon.hairline}` }}>
                        <Icon sx={{ fontSize: 17, color }} />
                        <Typography sx={{ flex: 1, fontSize: '0.85rem', color: beacon.textSecondary, textTransform: 'capitalize' }}>{c.type}</Typography>
                        <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.72rem', color: beacon.textFaint }}>
                          {when(c.date)}
                          {c.duration > 0 ? ` · ${duration(c.duration)}` : ''}
                          {sims.length > 1 && c.sim ? ` · SIM ${c.sim}` : ''}
                        </Typography>
                      </Box>
                    );
                  })}
              </Box>
            </Box>
          ) : (
            <Box sx={{ flex: 1, display: 'grid', placeItems: 'center', color: beacon.textFaint, textAlign: 'center', px: 4 }}>
              <Box>
                <CallIcon sx={{ fontSize: 42, opacity: 0.4 }} />
                <Typography sx={{ mt: 1 }}>Pick a call, or type a name or number to call from your phone.</Typography>
                <Typography sx={{ mt: 0.5, fontSize: '0.8rem' }}>The call runs on the phone — talk there or on a Bluetooth headset.</Typography>
              </Box>
            </Box>
          )}
        </Box>
        )}
      </Box>
    </Dialog>
  );
};

export default MessagesDialog;
