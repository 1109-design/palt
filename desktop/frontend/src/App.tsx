/**
 * App.tsx — PALT Desktop Root Component (Beacon)
 *
 * Orchestrates:
 *   1. Polling the Go backend (or mock) for the peer list.
 *   2. Maintaining local state: loading, peers, local device, search.
 *   3. Drag-and-drop: Wails OnFileDrop → SendFilePaths to the card under
 *      the cursor.
 *   4. Rendering: TopBar → LocalDeviceCard → tabs → peer grid / history →
 *      StatusBar, plus dialogs and the transfer shelf.
 *
 * When Wails is available the real wailsjs/go/main/App bindings are used.
 * In standalone browser dev the mock in src/wailsjs/App.mock.ts is used.
 */
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Box, Container, Grid, Typography, Skeleton, Snackbar, Alert, Dialog, DialogContent, DialogActions, Button } from '@mui/material';
import { alpha } from '@mui/material/styles';

import TopBar from './components/TopBar';
import LocalDeviceCard from './components/LocalDeviceCard';
import PeerCard from './components/PeerCard';
import EmptyState from './components/EmptyState';
import StatusBar from './components/StatusBar';
import { TransferDialog } from './components/TransferDialog';
import type { OfferData } from './components/TransferDialog';
import { ProgressSnack } from './components/ProgressSnack';
import type { TransferProgress } from './components/ProgressSnack';
import UpdateDialog from './components/UpdateDialog';
import ChangelogDialog from './components/ChangelogDialog';
import HistoryView from './components/HistoryView';
import RenameDialog from './components/RenameDialog';
import DevicePage from './components/DevicePage';
import PairingDialog from './components/PairingDialog';
import type { PairingInfo, RelayStatus } from './components/PairingDialog';
import { beacon, fonts } from './theme/theme';

import type { ClipboardEntry, ClipboardHelper, DeviceStatus, Peer, PhoneNotification } from './types/peer';
import PhoneNotificationsDrawer from './components/PhoneNotificationsDrawer';
import type { DrawerTab } from './components/PhoneNotificationsDrawer';
import type { HistoryEntry, HistoryFile } from './types/history';

// ── Wails bindings ────────────────────────────────────────────────────────────
// When running under `wails dev` or as a built binary, the Wails runtime
// injects window['go'] which the generated App.js calls into.
// When opened in a plain browser (npm run dev, no Wails), window['go'] is
// absent, so we fall back to the mock to keep the UI previewable.
import * as WailsApp  from '../wailsjs/go/main/App';
import * as MockApp   from './wailsjs/App.mock';

const isWails = typeof (window as any)['go'] !== 'undefined';
const GetPeers       = isWails ? WailsApp.GetPeers       : MockApp.GetPeers;
const GetLocalDevice = isWails ? WailsApp.GetLocalDevice : MockApp.GetLocalDevice;

// Update bindings — only available under Wails; no-ops in browser preview.
const _GetAppVersion     = isWails ? (WailsApp as any).GetAppVersion     : () => Promise.resolve('dev');
const _CheckForUpdate    = isWails ? (WailsApp as any).CheckForUpdate    : () => Promise.resolve({ isNewer: false });
const _OpenURL           = isWails ? (WailsApp as any).OpenURL           : (url: string) => { window.open(url, '_blank'); };
const _GetLastSeenVersion  = isWails ? (WailsApp as any).GetLastSeenVersion  : () => Promise.resolve('');
const _SaveLastSeenVersion = isWails ? (WailsApp as any).SaveLastSeenVersion : () => Promise.resolve();
const GetHistory           = isWails ? (WailsApp as any).GetHistory           : MockApp.GetHistory;
const ClearHistory         = isWails ? (WailsApp as any).ClearHistory         : MockApp.ClearHistory;
const OpenDownloadFolder   = isWails ? (WailsApp as any).OpenDownloadFolder   : () => { console.log('Mock: Open Folder'); };
const GetAliases           = isWails ? (WailsApp as any).GetAliases           : () => Promise.resolve({});
const SendText             = isWails ? (WailsApp as any).SendText             : MockApp.SendText;
const GetAutoCopyDevices   = isWails ? (WailsApp as any).GetAutoCopyDevices   : MockApp.GetAutoCopyDevices;
const SetAutoCopy          = isWails ? (WailsApp as any).SetAutoCopy          : MockApp.SetAutoCopy;
const ReadClipboard        = isWails
  ? () => (window as any).runtime.ClipboardGetText() as Promise<string>
  : () => navigator.clipboard.readText();
const OpenHistoryFile      = isWails ? (WailsApp as any).OpenHistoryFile      : MockApp.OpenHistoryFile;
const RevealHistoryFile    = isWails ? (WailsApp as any).RevealHistoryFile    : MockApp.RevealHistoryFile;
const GetRelayStatus       = isWails ? (WailsApp as any).GetRelayStatus       : MockApp.GetRelayStatus;
const GetPairing           = isWails ? (WailsApp as any).GetPairing           : MockApp.GetPairing;
const JoinPairing          = isWails ? (WailsApp as any).JoinPairing          : MockApp.JoinPairing;
const ResetPairing         = isWails ? (WailsApp as any).ResetPairing         : MockApp.ResetPairing;
const SetRelayServer       = isWails ? (WailsApp as any).SetRelayServer       : MockApp.SetRelayServer;
const ConnectByIP          = isWails ? (WailsApp as any).ConnectByIP          : MockApp.ConnectByIP;
const GetDeviceStatus      = isWails ? (WailsApp as any).GetDeviceStatus      : MockApp.GetDeviceStatus;
const RingDevice           = isWails ? (WailsApp as any).RingDevice           : MockApp.RingDevice;
const PingDevice           = isWails ? (WailsApp as any).PingDevice           : MockApp.PingDevice;
const StopRinging          = isWails ? (WailsApp as any).StopRinging          : MockApp.StopRinging;
const GetPhoneNotifications = isWails ? (WailsApp as any).GetPhoneNotifications : MockApp.GetPhoneNotifications;
const ReplyToNotification  = isWails ? (WailsApp as any).ReplyToNotification  : MockApp.ReplyToNotification;
const DismissPhoneNotification = isWails ? (WailsApp as any).DismissPhoneNotification : MockApp.DismissPhoneNotification;
const RunPhoneNotificationAction = isWails ? (WailsApp as any).RunPhoneNotificationAction : MockApp.RunPhoneNotificationAction;
const GetDeviceSettings    = isWails ? (WailsApp as any).GetDeviceSettings    : MockApp.GetDeviceSettings;
const SetDeviceSetting     = isWails ? (WailsApp as any).SetDeviceSetting     : MockApp.SetDeviceSetting;
const GetClipboardHistory  = isWails ? (WailsApp as any).GetClipboardHistory  : MockApp.GetClipboardHistory;
const GetClipboardHelper   = isWails ? (WailsApp as any).GetClipboardHelper   : MockApp.GetClipboardHelper;
const InstallClipboardHelper = isWails ? (WailsApp as any).InstallClipboardHelper : MockApp.InstallClipboardHelper;
const CopyFromHistory      = isWails ? (WailsApp as any).CopyFromHistory      : MockApp.CopyFromHistory;
const RescanPeers          = isWails ? (WailsApp as any).RescanPeers          : () => Promise.resolve();
const GetMediaPreview      = isWails ? (WailsApp as any).GetMediaPreview      : MockApp.GetMediaPreview;
const DeleteHistoryEntry   = isWails ? (WailsApp as any).DeleteHistoryEntry   : MockApp.DeleteHistoryEntry;
const SetAlias             = isWails ? (WailsApp as any).SetAlias             : (name: string, alias: string) => { console.log(`Mock: Set ${name} alias to ${alias}`); return Promise.resolve(); };

/** How often (ms) to auto-refresh the peer list */
const POLL_INTERVAL_MS = 30_000;

/** Shape of the update info returned by the Go CheckForUpdate binding */
interface UpdateInfo {
  isNewer: boolean;
  latestVersion: string;
  downloadUrl: string;
  releaseNotes: string;
}

// ─────────────────────────────────────────────────────────────────────────────

const App: React.FC = () => {
  const [peers, setPeers] = useState<Peer[]>([]);
  const [localDevice, setLocalDevice] = useState<Peer | null>(null);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const [offer, setOffer] = useState<OfferData | null>(null);
  const [progress, setProgress] = useState<TransferProgress | null>(null);
  const [transferError, setTransferError] = useState<string | null>(null);
  const [history, setHistory] = useState<HistoryEntry[]>([]);
  const [aliases, setAliases] = useState<Record<string, string>>({});
  const [currentTab, setCurrentTab] = useState(0); // 0 = Devices, 1 = History
  const [renamingPeer, setRenamingPeer] = useState<Peer | null>(null);
  const [dragActive, setDragActive] = useState(false);
  /** Device whose page is open; a snapshot so the page survives the peer going offline. */
  const [openPeer, setOpenPeer] = useState<Peer | null>(null);
  const [autoCopy, setAutoCopy] = useState<string[]>([]);
  const [relayStatus, setRelayStatus] = useState<RelayStatus | null>(null);
  const [relayOpen, setRelayOpen] = useState(false);
  const [pairing, setPairing] = useState<PairingInfo | null>(null);
  const [deviceStatus, setDeviceStatus] = useState<Record<string, DeviceStatus>>({});
  /** Set while this computer rings because a paired device is looking for it. */
  const [ringingFrom, setRingingFrom] = useState<string | null>(null);
  /** The open device's mirrored notifications and switches. */
  const [phoneNotes, setPhoneNotes] = useState<PhoneNotification[]>([]);
  const [notesOpen, setNotesOpen] = useState(false);
  const [drawerTab, setDrawerTab] = useState<DrawerTab>('notifications');
  const [clipHistory, setClipHistory] = useState<ClipboardEntry[]>([]);
  const [clipHelper, setClipHelper] = useState<ClipboardHelper | null>(null);
  const [deviceSettings, setDeviceSettings] = useState<Record<string, boolean>>({});

  // ── Update state ───────────────────────────────────────────────────────────
  const [appVersion, setAppVersion] = useState('');
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null);
  const [updateDialogOpen, setUpdateDialogOpen] = useState(false);
  const [changelogOpen, setChangelogOpen] = useState(false);
  const [changelogNotes, setChangelogNotes] = useState('');

  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const openPeerNameRef = useRef<string | null>(null);
  openPeerNameRef.current = openPeer?.deviceName ?? null;
  const peersRef = useRef<Peer[]>([]);
  peersRef.current = peers;
  const dragTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // ── Update + Changelog on mount ────────────────────────────────────────────
  useEffect(() => {
    (async () => {
      try {
        // 1. Fetch and store the current build version.
        const version = await _GetAppVersion();
        setAppVersion(version);

        // 2. Check if the user has seen this version before.
        const lastSeen = await _GetLastSeenVersion();
        if (lastSeen !== version && version !== 'dev') {
          setChangelogNotes(''); // will be filled by CheckForUpdate below
          setTimeout(() => setChangelogOpen(true), 600);
        }

        // 3. Check for a newer version in the background.
        const info: UpdateInfo = await _CheckForUpdate();
        if (info.isNewer) {
          setUpdateInfo(info);
        }
        // Reuse release notes for the changelog if available.
        if (info.releaseNotes) {
          setChangelogNotes(info.releaseNotes);
        }
      } catch (err) {
        console.warn('[App] Update check failed:', err);
      }
    })();
  }, []);

  const handleChangelogDismiss = useCallback(async () => {
    setChangelogOpen(false);
    try {
      await _SaveLastSeenVersion(appVersion);
    } catch (err) {
      console.warn('[App] SaveLastSeenVersion failed:', err);
    }
  }, [appVersion]);

  const handleUpdateDownload = useCallback(() => {
    if (updateInfo?.downloadUrl) {
      _OpenURL(updateInfo.downloadUrl);
    }
  }, [updateInfo]);

  // ── Data fetching ──────────────────────────────────────────────────────────

  const fetchPeers = useCallback(async () => {
    try {
      const results = await GetPeers();
      setPeers(results);
      setLastUpdated(new Date());
    } catch (err) {
      console.error('[App] GetPeers failed:', err);
      setErrorMsg('Could not reach the discovery service. Is the app running?');
    } finally {
      setLoading(false);
    }
  }, []);

  const fetchLocalDevice = useCallback(async () => {
    try {
      const device = await GetLocalDevice();
      setLocalDevice(device);
    } catch (err) {
      console.error('[App] GetLocalDevice failed:', err);
    }
  }, []);

  const fetchHistory = useCallback(async () => {
    try {
      const results = await GetHistory();
      setHistory(results);
    } catch (err) {
      console.error('[App] GetHistory failed:', err);
    }
  }, []);

  const fetchAliases = useCallback(async () => {
    try {
      const results = await GetAliases();
      setAliases(results);
    } catch (err) {
      console.error('[App] GetAliases failed:', err);
    }
  }, []);

  const fetchDeviceSettings = useCallback(async () => {
    try {
      setAutoCopy((await GetAutoCopyDevices()) || []);
    } catch (err) {
      console.error('[App] Device settings fetch failed:', err);
    }
  }, []);

  useEffect(() => {
    fetchDeviceSettings();
  }, [fetchDeviceSettings]);

  const fetchRelayStatus = useCallback(async () => {
    try {
      const [st, p] = await Promise.all([GetRelayStatus(), GetPairing()]);
      setRelayStatus(st);
      setPairing(p);
    } catch (err) {
      console.error('[App] GetRelayStatus failed:', err);
    }
  }, []);

  useEffect(() => {
    fetchRelayStatus();
  }, [fetchRelayStatus]);

  const handleJoinPairing = useCallback(async (code: string) => {
    await JoinPairing(code);
    await fetchRelayStatus();
  }, [fetchRelayStatus]);

  const handleResetPairing = useCallback(async () => {
    await ResetPairing();
    await fetchRelayStatus();
  }, [fetchRelayStatus]);

  const handleSetRelayServer = useCallback(async (addr: string) => {
    await SetRelayServer(addr);
    await fetchRelayStatus();
  }, [fetchRelayStatus]);

  useEffect(() => {
    // Through a promise chain so even a missing binding can't throw here.
    Promise.resolve()
      .then(() => GetDeviceStatus())
      .then((s: Record<string, DeviceStatus>) => setDeviceStatus(s || {}))
      .catch(() => {});
  }, []);

  // Notifications and switches follow the open device.
  const openName = openPeer?.deviceName;
  useEffect(() => {
    if (!openName) return;
    Promise.resolve().then(() => GetPhoneNotifications(openName)).then((n: PhoneNotification[]) => setPhoneNotes(n || [])).catch(() => {});
    Promise.resolve().then(() => GetDeviceSettings(openName)).then((s: Record<string, boolean>) => setDeviceSettings(s || {})).catch(() => {});
  }, [openName]);

  const handleSetSetting = useCallback(async (key: string, on: boolean) => {
    if (!openName) return;
    setDeviceSettings((s) => ({ ...s, [key]: on }));
    try {
      await SetDeviceSetting(openName, key, on);
    } catch (e: any) {
      setErrorMsg(String(e?.message || e));
    }
  }, [openName]);

  const noteError = (e: any) => setErrorMsg(String(e?.message || e));

  useEffect(() => {
    if (!notesOpen) return;
    Promise.resolve().then(() => GetClipboardHistory()).then((h: ClipboardEntry[]) => setClipHistory(h || [])).catch(() => {});
    Promise.resolve().then(() => GetClipboardHelper()).then(setClipHelper).catch(() => {});
  }, [notesOpen]);

  const handleRing = useCallback(async (peer: Peer) => {
    try {
      await RingDevice(peer.deviceName);
      setErrorMsg(`Ringing ${peer.deviceName}…`);
    } catch (e: any) {
      setErrorMsg(String(e?.message || e));
    }
  }, []);

  const handlePing = useCallback(async (peer: Peer) => {
    try {
      await PingDevice(peer.deviceName);
    } catch (e: any) {
      setErrorMsg(String(e?.message || e));
    }
  }, []);

  // Initial load
  useEffect(() => {
    fetchLocalDevice();
    fetchPeers();
    fetchHistory();
    fetchAliases();
  }, [fetchLocalDevice, fetchPeers, fetchHistory, fetchAliases]);

  // Polling
  useEffect(() => {
    pollRef.current = setInterval(fetchPeers, POLL_INTERVAL_MS);
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [fetchPeers]);

  // ── Wails transfer events ──────────────────────────────────────────────────
  useEffect(() => {
    if (!isWails) return;

    // We cast window as any because __wails is not in standard TS lib
    const win = window as any;
    if (!win.runtime) return;

    win.runtime.EventsOn('peers_changed', (newPeers: Peer[]) => {
      setPeers(newPeers);
      setLastUpdated(new Date());
    });

    win.runtime.EventsOn('transfer_offer', (data: OfferData) => {
      setOffer(data);
    });

    win.runtime.EventsOn('transfer_started', (data: any) => {
      setTransferError(null);
      setProgress({ transferId: data.transferId, written: 0, total: 100, route: data.route || undefined });
    });

    win.runtime.EventsOn('transfer_progress', (data: TransferProgress) => {
      setProgress(prev => ({ ...data, route: data.route || (prev?.transferId === data.transferId ? prev.route : undefined) }));
    });

    win.runtime.EventsOn('transfer_complete', () => {
      setProgress(prev => {
        if (!prev) return null;
        return { ...prev, status: 'completed' };
      });
    });

    win.runtime.EventsOn('transfer_error', (data: any) => {
      setProgress(null);
      setTransferError(data.error);
    });

    win.runtime.EventsOn('relay_status', (data: RelayStatus) => {
      setRelayStatus(data);
    });

    win.runtime.EventsOn('device_status', (data: Record<string, DeviceStatus>) => {
      setDeviceStatus(data || {});
    });

    win.runtime.EventsOn('ringing', (data: { from?: string; on: boolean }) => {
      setRingingFrom(data.on ? data.from || 'A device' : null);
    });

    win.runtime.EventsOn('clipboard_history', (data: ClipboardEntry[]) => setClipHistory(data || []));

    win.runtime.EventsOn('phone_notifications', (data: { device: string; items: PhoneNotification[] }) => {
      if (openPeerNameRef.current === data.device) setPhoneNotes(data.items || []);
    });

    // "Reply" on a desktop notification: open that device with the drawer.
    win.runtime.EventsOn('notification_reply', (n: PhoneNotification) => {
      const peer = peersRef.current.find((p) => p.deviceName === n.device);
      if (peer) setOpenPeer(peer);
      setDrawerTab('notifications');
      setNotesOpen(true);
    });

    win.runtime.EventsOn('history_updated', (data: HistoryEntry[]) => {
      setHistory(data);
    });

    win.runtime.EventsOn('text_received', (data: { senderName: string; text: string; copied?: boolean }) => {
      // Already visible in the thread when that device's page is open.
      if (openPeerNameRef.current === data.senderName) return;
      const preview = data.text.length > 80 ? `${data.text.slice(0, 80)}…` : data.text;
      setErrorMsg(`${data.senderName}: ${preview}${data.copied ? ' — copied to clipboard' : ''}`);
    });

    // A device picked from the tray menu opens its chat.
    win.runtime.EventsOn('tray_open_device', (deviceName: string) => {
      const peer = peersRef.current.find(p => p.deviceName === deviceName);
      if (peer) setOpenPeer(peer);
    });

    return () => {
      win.runtime.EventsOff('peers_changed');
      win.runtime.EventsOff('history_updated');
      win.runtime.EventsOff('transfer_offer');
      win.runtime.EventsOff('transfer_progress');
      win.runtime.EventsOff('transfer_started');
      win.runtime.EventsOff('transfer_complete');
      win.runtime.EventsOff('transfer_error');
      win.runtime.EventsOff('text_received');
      win.runtime.EventsOff('relay_status');
      win.runtime.EventsOff('tray_open_device');
      win.runtime.EventsOff('device_status');
      win.runtime.EventsOff('ringing');
      win.runtime.EventsOff('phone_notifications');
      win.runtime.EventsOff('notification_reply');
      win.runtime.EventsOff('clipboard_history');
    };
  }, []);

  // ── Drag & drop → send to the peer card under the cursor ───────────────────
  useEffect(() => {
    // Track "files are being dragged over the window" for card highlighting.
    // The native drop is swallowed by Wails, so a short timeout (reset on
    // every dragover) is the reliable way to clear the state.
    const onDragOver = (e: DragEvent) => {
      e.preventDefault();
      setDragActive(true);
      if (dragTimerRef.current) clearTimeout(dragTimerRef.current);
      dragTimerRef.current = setTimeout(() => setDragActive(false), 400);
    };
    const onDrop = () => setDragActive(false);
    window.addEventListener('dragover', onDragOver);
    window.addEventListener('drop', onDrop);

    if (isWails) {
      const win = window as any;
      win.runtime?.OnFileDrop?.((x: number, y: number, paths: string[]) => {
        setDragActive(false);
        if (!paths || paths.length === 0) return;
        const el = document.elementFromPoint(x, y)?.closest('[data-peer-ip]') as HTMLElement | null;
        if (!el) return;
        const ip = el.getAttribute('data-peer-ip')!;
        const port = parseInt(el.getAttribute('data-peer-port') || '0', 10);
        const name = el.getAttribute('data-peer-name') || ip;
        const sendPaths = (WailsApp as any).SendFilePaths;
        if (typeof sendPaths === 'function') {
          sendPaths(ip, port, name, paths).catch((e: any) =>
            setErrorMsg(`Send failed: ${e?.message || e}`),
          );
        }
      }, true);
    }

    return () => {
      window.removeEventListener('dragover', onDragOver);
      window.removeEventListener('drop', onDrop);
      if (isWails) (window as any).runtime?.OnFileDropOff?.();
      if (dragTimerRef.current) clearTimeout(dragTimerRef.current);
    };
  }, []);

  // ── Manual refresh ─────────────────────────────────────────────────────────

  const handleRefresh = useCallback(() => {
    setLoading(true);
    RescanPeers();
    fetchPeers();
    // Restart poll timer
    if (pollRef.current) clearInterval(pollRef.current);
    pollRef.current = setInterval(fetchPeers, POLL_INTERVAL_MS);
  }, [fetchPeers]);

  // ── Send file ──────────────────────────────────────────────────────────────

  const handleSendFile = useCallback(async (peer: Peer) => {
    try {
      if (isWails) {
        if (typeof (WailsApp as any).SendFile !== 'function') {
          setErrorMsg('SendFile function not found on backend. Please restart `wails dev` to load the new bindings.');
          return;
        }
        // @ts-ignore
        await WailsApp.SendFile(peer.ipAddress, peer.port, peer.deviceName);
      } else {
        setErrorMsg(`Mock transfer to ${peer.deviceName} triggered`);
      }
    } catch (e: any) {
      setErrorMsg(`SendFile error: ${e?.message || e}`);
    }
  }, []);

  const handleSendText = useCallback(async (peer: Peer, text: string) => {
    await SendText(peer.ipAddress, peer.port, peer.deviceName, text);
    if (!isWails) {
      setHistory((prev) => [
        {
          id: `${Date.now()}`,
          partnerName: peer.deviceName,
          files: [],
          totalSize: text.length,
          direction: 'outgoing',
          timestamp: new Date().toISOString(),
          status: 'completed',
          durationMillis: 0,
          kind: 'text',
          text,
        },
        ...prev,
      ]);
    }
  }, []);

  const handleToggleAutoCopy = useCallback(async (peer: Peer, enabled: boolean) => {
    try {
      await SetAutoCopy(peer.deviceName, enabled);
    } catch (e: any) {
      setErrorMsg(String(e?.message || e));
    }
    fetchDeviceSettings();
  }, [fetchDeviceSettings]);

  const handleDeleteEntry = useCallback(async (entry: HistoryEntry) => {
    // Optimistic: drop it now; history_updated confirms.
    setHistory((h) => h.filter((e) => !(e.id === entry.id && e.direction === entry.direction)));
    try {
      await DeleteHistoryEntry(entry.id, entry.direction);
    } catch (e: any) {
      setErrorMsg(`Could not delete: ${e?.message || e}`);
      fetchHistory();
    }
  }, [fetchHistory]);

  const handleFileAction = useCallback(async (file: HistoryFile, action: 'open' | 'reveal') => {
    try {
      await (action === 'open' ? OpenHistoryFile : RevealHistoryFile)(file.path || '', file.name);
    } catch (e: any) {
      setErrorMsg(String(e?.message || e));
    }
  }, []);

  const acceptOffer = useCallback(async (transferId: string) => {
    setOffer(null);
    if (isWails) {
      // @ts-ignore
      await WailsApp.AcceptOffer(transferId);
    }
  }, []);

  const rejectOffer = useCallback(async (transferId: string) => {
    setOffer(null);
    if (isWails) {
      // @ts-ignore
      await WailsApp.RejectOffer(transferId);
    }
  }, []);

  const handleClearHistory = useCallback(async () => {
    if (isWails) {
      // @ts-ignore
      await WailsApp.ClearHistory();
    } else {
      setHistory([]);
    }
  }, []);

  const handleOpenFolder = useCallback(async () => {
    if (isWails) {
      await OpenDownloadFolder();
    }
  }, []);

  const handleSaveAlias = useCallback(async (peer: Peer, alias: string) => {
    await SetAlias(peer.deviceName, alias);
    const updated = await GetAliases();
    setAliases(updated);
  }, []);

  // Live record for the open device (fresh IP/port), matched by broadcast name.
  const livePeer = useMemo(
    () => (openPeer ? peers.find((p) => p.deviceName === openPeer.deviceName) ?? null : null),
    [peers, openPeer],
  );

  // ── Filtered peers ─────────────────────────────────────────────────────────

  const filteredPeers = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return peers;
    return peers.filter(
      (p) =>
        p.deviceName.toLowerCase().includes(q) ||
        p.ipAddress.includes(q) ||
        p.os.toLowerCase().includes(q),
    );
  }, [peers, searchQuery]);

  // ─────────────────────────────────────────────────────────────────────────────

  /** Skeleton grid while the first load is in progress */
  const renderSkeletons = () =>
    Array.from({ length: 3 }).map((_, i) => (
      <Grid item xs={12} sm={6} md={4} key={i}>
        <Skeleton
          variant="rounded"
          height={172}
          sx={{ borderRadius: '12px', backgroundColor: beacon.surface, transform: 'none' }}
          animation="wave"
        />
      </Grid>
    ));

  const tabs = ['Devices', 'History'];

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        minHeight: '100vh',
        backgroundColor: 'background.default',
      }}
    >
      <TopBar
        peerCount={peers.length}
        loading={loading}
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        onRefresh={handleRefresh}
        updateAvailable={!!updateInfo?.isNewer}
        latestVersion={updateInfo?.latestVersion}
        onUpdateClick={() => setUpdateDialogOpen(true)}
        relayStatus={relayStatus}
        onRelayClick={() => setRelayOpen(true)}
      />

      <PairingDialog
        open={relayOpen}
        pairing={pairing}
        status={relayStatus}
        pairedCount={peers.filter((p) => p.paired).length}
        onClose={() => setRelayOpen(false)}
        onJoin={handleJoinPairing}
        onReset={handleResetPairing}
        onSetRelay={handleSetRelayServer}
        onConnectIP={ConnectByIP}
      />

      <PhoneNotificationsDrawer
        open={notesOpen && !!openPeer}
        device={openPeer?.deviceName || ''}
        items={phoneNotes}
        tab={drawerTab}
        onTab={setDrawerTab}
        clipboard={clipHistory.filter((c) => !c.from || c.from === openPeer?.deviceName)}
        helper={clipHelper}
        onInstallHelper={async () => {
          try {
            await InstallClipboardHelper();
          } catch (e: any) {
            noteError(e);
          }
        }}
        onCopy={(text) => CopyFromHistory(text).then(() => setErrorMsg('Copied')).catch(noteError)}
        onClose={() => setNotesOpen(false)}
        onDismiss={(n) => DismissPhoneNotification(n.device, n.id).catch(noteError)}
        onAction={(n, i) => RunPhoneNotificationAction(n.device, n.id, i).catch(noteError)}
        onReply={(n, text) => ReplyToNotification(n.device, n.id, text).catch(noteError)}
      />

      <Dialog open={!!ringingFrom} onClose={() => StopRinging()} maxWidth="xs" fullWidth>
        <DialogContent sx={{ pt: 3.5, textAlign: 'center' }}>
          <Typography sx={{ fontSize: '2.2rem', lineHeight: 1 }}>🔔</Typography>
          <Typography sx={{ fontWeight: 700, fontSize: '1.15rem', mt: 1.5 }}>This computer is ringing</Typography>
          <Typography sx={{ fontSize: '0.86rem', opacity: 0.7, mt: 0.5 }}>{ringingFrom} is looking for it.</Typography>
        </DialogContent>
        <DialogActions sx={{ p: 2.5 }}>
          <Button variant="contained" fullWidth onClick={() => StopRinging()}>
            Found it
          </Button>
        </DialogActions>
      </Dialog>

      {openPeer ? (
        <DevicePage
          peer={livePeer ?? openPeer}
          online={!!livePeer}
          alias={aliases[openPeer.deviceName]}
          history={history}
          dragActive={dragActive}
          onBack={() => setOpenPeer(null)}
          onSendFiles={handleSendFile}
          onSendText={handleSendText}
          paired={!!(livePeer ?? openPeer).paired}
          autoCopy={autoCopy.includes(openPeer.deviceName)}
          onToggleAutoCopy={handleToggleAutoCopy}
          readClipboard={ReadClipboard}
          onFileAction={handleFileAction}
          loadPreview={GetMediaPreview}
          onDeleteEntry={handleDeleteEntry}
          status={deviceStatus[openPeer.deviceName]}
          onRing={handleRing}
          onPing={handlePing}
          notificationCount={phoneNotes.length}
          onShowNotifications={() => {
            setDrawerTab('notifications');
            setNotesOpen(true);
          }}
          settings={deviceSettings}
          onSetSetting={handleSetSetting}
        />
      ) : (
      /* ── Main content (offset for AppBar + StatusBar) ───────────────────── */
      <Container maxWidth="lg" component="main" sx={{ pt: 12, pb: 8, flex: 1 }}>
        <LocalDeviceCard device={localDevice} broadcasting />

        {/* ── Segmented tabs ─────────────────────────────────────────────── */}
        <Box
          sx={{
            display: 'inline-flex',
            gap: 0.5,
            p: 0.5,
            mb: 3,
            borderRadius: 999,
            backgroundColor: beacon.surface,
            border: `1px solid ${beacon.hairline}`,
          }}
          role="tablist"
        >
          {tabs.map((label, i) => (
            <Box
              key={label}
              component="button"
              role="tab"
              aria-selected={currentTab === i}
              onClick={() => setCurrentTab(i)}
              sx={{
                border: 'none',
                cursor: 'pointer',
                px: 2.5,
                py: 0.9,
                borderRadius: 999,
                fontFamily: fonts.body,
                fontSize: '0.82rem',
                fontWeight: 600,
                lineHeight: 1,
                transition: 'background-color 150ms, color 150ms',
                backgroundColor: currentTab === i ? beacon.surfaceRaised : 'transparent',
                color: currentTab === i ? beacon.text : beacon.textSecondary,
                boxShadow: currentTab === i ? `inset 0 0 0 1px ${beacon.hairline}` : 'none',
                '&:hover': { color: beacon.text },
              }}
            >
              {label}
              {i === 0 && peers.length > 0 && (
                <Box
                  component="span"
                  sx={{
                    ml: 1,
                    fontFamily: fonts.mono,
                    fontSize: '0.7rem',
                    color: currentTab === i ? beacon.accent : beacon.textFaint,
                  }}
                >
                  {peers.length}
                </Box>
              )}
            </Box>
          ))}
        </Box>

        {currentTab === 0 ? (
          <>
            {/* Drop hint appears while dragging files over the window */}
            {dragActive && filteredPeers.length > 0 && (
              <Box
                sx={{
                  mb: 2,
                  px: 2,
                  py: 1,
                  borderRadius: '8px',
                  border: `1px dashed ${alpha(beacon.accent, 0.5)}`,
                  backgroundColor: alpha(beacon.accent, 0.05),
                }}
              >
                <Typography sx={{ fontSize: '0.8rem', color: beacon.accent, fontWeight: 500 }}>
                  Drop files on a device to send them
                </Typography>
              </Box>
            )}

            {loading && peers.length === 0 ? (
              <Grid container spacing={2}>
                {renderSkeletons()}
              </Grid>
            ) : filteredPeers.length > 0 ? (
              <Grid container spacing={2}>
                {filteredPeers.map((peer) => (
                  <Grid item xs={12} sm={6} md={4} key={peer.id}>
                    <PeerCard
                      peer={peer}
                      alias={aliases[peer.deviceName]}
                      dragActive={dragActive}
                      onSendFile={handleSendFile}
                      onRename={setRenamingPeer}
                      onOpen={setOpenPeer}
                      status={deviceStatus[peer.deviceName]}
                    />
                  </Grid>
                ))}
              </Grid>
            ) : (
              <EmptyState
                loading={loading}
                filtered={searchQuery.trim().length > 0}
                onRefresh={handleRefresh}
              />
            )}
          </>
        ) : (
          <HistoryView history={history} onClear={handleClearHistory} />
        )}
      </Container>
      )}

      <StatusBar
        loading={loading}
        peerCount={peers.length}
        lastUpdated={lastUpdated}
        appVersion={appVersion}
      />

      {/* ── Toast notifications ────────────────────────────────────────────── */}
      <Snackbar
        open={errorMsg !== null}
        autoHideDuration={4000}
        onClose={() => setErrorMsg(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
        sx={{ mb: 4 }}
      >
        <Alert
          onClose={() => setErrorMsg(null)}
          severity="info"
          variant="outlined"
          sx={{
            borderRadius: '10px',
            backgroundColor: beacon.surfaceRaised,
            borderColor: beacon.hairline,
            color: beacon.text,
          }}
        >
          {errorMsg}
        </Alert>
      </Snackbar>

      <TransferDialog
        open={offer !== null}
        offer={offer}
        onAccept={acceptOffer}
        onReject={rejectOffer}
      />

      <ProgressSnack
        progress={progress}
        error={transferError}
        onOpenFolder={handleOpenFolder}
        onClose={() => {
          setProgress(null);
          setTransferError(null);
        }}
      />

      <RenameDialog
        peer={renamingPeer}
        currentAlias={renamingPeer ? aliases[renamingPeer.deviceName] || '' : ''}
        onSave={handleSaveAlias}
        onClose={() => setRenamingPeer(null)}
      />

      {updateInfo && (
        <UpdateDialog
          open={updateDialogOpen}
          currentVersion={appVersion}
          latestVersion={updateInfo.latestVersion}
          releaseNotes={updateInfo.releaseNotes}
          downloadUrl={updateInfo.downloadUrl}
          onDownload={handleUpdateDownload}
          onClose={() => setUpdateDialogOpen(false)}
        />
      )}

      <ChangelogDialog
        open={changelogOpen}
        version={appVersion}
        releaseNotes={changelogNotes}
        onDismiss={handleChangelogDismiss}
      />
    </Box>
  );
};

export default App;
