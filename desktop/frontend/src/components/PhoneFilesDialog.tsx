/**
 * PhoneFilesDialog — browse a paired phone's storage (Go phonefiles_app.go):
 * folders, picture and video thumbnails, download, upload into the open
 * folder, new folder, delete.
 *
 * Downloads land in Downloads/PALT like any received file.
 */
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Box, Button, Checkbox, CircularProgress, Dialog, IconButton, InputBase, Tooltip, Typography,
} from '@mui/material';
import CloseIcon from '@mui/icons-material/CloseRounded';
import ArrowUpwardIcon from '@mui/icons-material/ArrowUpwardRounded';
import RefreshIcon from '@mui/icons-material/RefreshRounded';
import FolderIcon from '@mui/icons-material/FolderRounded';
import CreateNewFolderIcon from '@mui/icons-material/CreateNewFolderOutlined';
import UploadIcon from '@mui/icons-material/FileUploadOutlined';
import DownloadIcon from '@mui/icons-material/FileDownloadOutlined';
import DeleteIcon from '@mui/icons-material/DeleteOutline';
import InsertDriveFileIcon from '@mui/icons-material/InsertDriveFileOutlined';
import ChevronRightIcon from '@mui/icons-material/ChevronRightRounded';
import { beacon, fonts } from '../theme/theme';
import { formatSize } from './HistoryView';
import PhoneStoragePanel from './PhoneStoragePanel';

const go = () => (window as any).go?.main?.App;
const events = () => (window as any).runtime;

const ROOT = '/storage/emulated/0';
const THUMB_EXT = /\.(jpe?g|png|webp|gif|heic|bmp|mp4|mkv|webm|mov|3gp)$/i;

interface PhoneFile {
  name: string;
  dir: boolean;
  size: number;
  modified: number;
}

const join = (dir: string, name: string) => `${dir.replace(/\/$/, '')}/${name}`;

/** Loads thumbnails a few at a time so the phone isn't flooded. */
const thumbQueue = (() => {
  let active = 0;
  const waiting: (() => void)[] = [];
  const next = () => {
    if (active >= 3 || waiting.length === 0) return;
    active++;
    waiting.shift()!();
  };
  return <T,>(work: () => Promise<T>): Promise<T> =>
    new Promise<T>((resolve, reject) => {
      waiting.push(() =>
        work()
          .then(resolve, reject)
          .finally(() => {
            active--;
            next();
          }),
      );
      next();
    });
})();

const Thumb: React.FC<{ device: string; path: string; cache: Map<string, string> }> = ({ device, path, cache }) => {
  const [src, setSrc] = useState(cache.get(path) ?? '');
  useEffect(() => {
    if (src) return;
    let live = true;
    thumbQueue(() => go().PhoneThumbnail(device, path) as Promise<string>)
      .then((url) => {
        cache.set(path, url);
        if (live) setSrc(url);
      })
      .catch(() => {});
    return () => {
      live = false;
    };
  }, [device, path]);
  return src ? (
    <Box component="img" src={src} sx={{ width: 40, height: 40, objectFit: 'cover', borderRadius: '10px' }} />
  ) : (
    <InsertDriveFileIcon sx={{ fontSize: 24, color: beacon.textSecondary }} />
  );
};

const PhoneFilesDialog: React.FC<{ device: string; open: boolean; onClose: () => void }> = ({ device, open, onClose }) => {
  const [path, setPath] = useState(ROOT);
  const [entries, setEntries] = useState<PhoneFile[]>([]);
  const [access, setAccess] = useState(true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [note, setNote] = useState('');
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [newFolder, setNewFolder] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const thumbs = useRef(new Map<string, string>()).current;
  const [storageKey, setStorageKey] = useState(0);

  const load = useCallback(
    async (p: string, select?: string) => {
      setLoading(true);
      setError('');
      try {
        const f = await go().ListPhoneFiles(device, p);
        setPath(f.path || p);
        setAccess(f.access);
        setEntries(f.entries || []);
        setSelected(new Set(select ? [select] : []));
      } catch (e) {
        setError(String(e));
      }
      setLoading(false);
    },
    [device],
  );

  useEffect(() => {
    if (open) load(path);
  }, [open]);

  // Uploads finish in the background; show them when they land.
  useEffect(() => {
    if (!open) return;
    const off = events()?.EventsOn?.('transfer_complete', () => load(path));
    return () => off?.();
  }, [open, path, load]);

  // Waiting for "All files access": look again every few seconds.
  useEffect(() => {
    if (!open || access) return;
    const t = setInterval(() => load(path), 3000);
    return () => clearInterval(t);
  }, [open, access, path, load]);

  const sorted = useMemo(
    () =>
      entries
        .filter((e) => !e.name.startsWith('.'))
        .sort((a, b) => (a.dir !== b.dir ? (a.dir ? -1 : 1) : a.name.localeCompare(b.name, undefined, { numeric: true }))),
    [entries],
  );

  const crumbs = useMemo(() => {
    const rel = path.startsWith(ROOT) ? path.slice(ROOT.length) : path;
    const parts = rel.split('/').filter(Boolean);
    return [{ name: 'Phone', path: ROOT }, ...parts.map((n, i) => ({ name: n, path: ROOT + '/' + parts.slice(0, i + 1).join('/') }))];
  }, [path]);

  const run = async (what: string, work: () => Promise<unknown>, after?: () => void) => {
    setNote(what);
    setError('');
    try {
      await work();
      after?.();
    } catch (e) {
      setError(String(e));
    }
    setNote('');
  };

  const chosen = [...selected];
  const download = (paths: string[]) =>
    run('Asking the phone to send…', () => go().DownloadPhoneFiles(device, paths), () => {
      setNote('');
      setSelected(new Set());
    });

  const toggle = (p: string) =>
    setSelected((s) => {
      const n = new Set(s);
      n.has(p) ? n.delete(p) : n.add(p);
      return n;
    });

  const iconButton = { color: beacon.textSecondary, '&:hover': { color: beacon.text } };

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="md" PaperProps={{ sx: { height: '82vh', borderRadius: '24px', backgroundImage: 'none', backgroundColor: beacon.surface } }}>
      {/* Header */}
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, px: 2.5, pt: 2, pb: 1 }}>
        <Typography sx={{ fontFamily: fonts.display, fontWeight: 800, fontSize: '1.15rem', flex: 1 }}>{device} files</Typography>
        <Tooltip title="Up">
          <span>
            <IconButton disabled={path === ROOT} onClick={() => load(path.slice(0, path.lastIndexOf('/')) || ROOT)} sx={iconButton}>
              <ArrowUpwardIcon />
            </IconButton>
          </span>
        </Tooltip>
        <Tooltip title="Refresh">
          <IconButton
            onClick={() => {
              load(path);
              setStorageKey((k) => k + 1);
            }}
            sx={iconButton}
          >
            <RefreshIcon />
          </IconButton>
        </Tooltip>
        <Tooltip title="New folder">
          <span>
            <IconButton disabled={!access} onClick={() => setNewFolder('')} sx={iconButton}>
              <CreateNewFolderIcon />
            </IconButton>
          </span>
        </Tooltip>
        <Button
          variant="contained"
          disabled={!access}
          startIcon={<UploadIcon />}
          onClick={() => run('Sending…', () => go().UploadToPhone(device, path))}
          sx={{ borderRadius: 999, height: 36 }}
        >
          Upload here
        </Button>
        <IconButton onClick={onClose} sx={iconButton}>
          <CloseIcon />
        </IconButton>
      </Box>

      {open && <PhoneStoragePanel device={device} refreshKey={storageKey} onOpen={(dir, file) => load(dir, file)} />}

      {/* Breadcrumbs */}
      <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', px: 2.5, pb: 1 }}>
        {crumbs.map((c, i) => (
          <React.Fragment key={c.path}>
            {i > 0 && <ChevronRightIcon sx={{ fontSize: 18, color: beacon.textFaint }} />}
            <Box
              component="button"
              onClick={() => load(c.path)}
              sx={{
                border: 'none', background: 'none', cursor: 'pointer', font: 'inherit', px: 0.75, py: 0.25, borderRadius: 1,
                fontSize: '0.84rem', color: i === crumbs.length - 1 ? beacon.text : beacon.textSecondary,
                '&:hover': { backgroundColor: beacon.surfaceRaised },
              }}
            >
              {c.name}
            </Box>
          </React.Fragment>
        ))}
      </Box>

      {/* Selection bar / notes */}
      {(chosen.length > 0 || note || error || newFolder !== null) && (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mx: 2.5, mb: 1, px: 1.5, py: 1, borderRadius: '14px', backgroundColor: beacon.tile }}>
          {newFolder !== null ? (
            <>
              <InputBase
                autoFocus
                placeholder="Folder name"
                value={newFolder}
                onChange={(e) => setNewFolder(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Escape') setNewFolder(null);
                  if (e.key === 'Enter' && newFolder.trim())
                    run('Making folder…', () => go().NewPhoneFolder(device, join(path, newFolder.trim())), () => {
                      setNewFolder(null);
                      load(path);
                    });
                }}
                sx={{ flex: 1, fontSize: '0.9rem' }}
              />
              <Button size="small" onClick={() => setNewFolder(null)}>Cancel</Button>
            </>
          ) : confirmDelete ? (
            <>
              <Typography sx={{ flex: 1, fontSize: '0.86rem', color: beacon.danger }}>
                Delete {chosen.length} item{chosen.length > 1 ? 's' : ''} from the phone? This can't be undone.
              </Typography>
              <Button size="small" onClick={() => setConfirmDelete(false)}>Keep</Button>
              <Button
                size="small"
                color="error"
                variant="contained"
                onClick={() => {
                  setConfirmDelete(false);
                  run('Deleting…', () => go().DeletePhoneFiles(device, chosen), () => {
                    load(path);
                    setStorageKey((k) => k + 1);
                  });
                }}
              >
                Delete
              </Button>
            </>
          ) : (
            <>
              <Typography sx={{ flex: 1, fontSize: '0.86rem', color: error ? beacon.danger : beacon.textSecondary }}>
                {error || note || `${chosen.length} selected`}
              </Typography>
              {chosen.length > 0 && !note && (
                <>
                  <Button size="small" startIcon={<DownloadIcon />} onClick={() => download(chosen)}>
                    Download
                  </Button>
                  <Button size="small" color="error" startIcon={<DeleteIcon />} onClick={() => setConfirmDelete(true)}>
                    Delete
                  </Button>
                </>
              )}
            </>
          )}
        </Box>
      )}

      {/* Listing */}
      <Box sx={{ flex: 1, overflowY: 'auto', px: 1.5, pb: 2 }}>
        {!access ? (
          <Box sx={{ textAlign: 'center', py: 8, px: 4 }}>
            <Typography sx={{ fontWeight: 700, mb: 1 }}>Allow PALT to see the phone's files</Typography>
            <Typography sx={{ fontSize: '0.88rem', color: beacon.textSecondary, mb: 2.5 }}>
              On {device}, turn on “All files access” for PALT. This page updates by itself.
            </Typography>
            <Button variant="contained" onClick={() => run('Opening the setting on the phone…', () => go().AllowPhoneFiles(device))} sx={{ borderRadius: 999 }}>
              Open the setting on {device}
            </Button>
          </Box>
        ) : loading && entries.length === 0 ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
            <CircularProgress size={28} />
          </Box>
        ) : sorted.length === 0 ? (
          <Typography sx={{ textAlign: 'center', py: 8, color: beacon.textSecondary }}>This folder is empty</Typography>
        ) : (
          sorted.map((e) => {
            const p = join(path, e.name);
            const on = selected.has(p);
            return (
              <Box
                key={p}
                onClick={() => (e.dir ? load(p) : toggle(p))}
                onDoubleClick={() => !e.dir && download([p])}
                sx={{
                  display: 'flex', alignItems: 'center', gap: 1.5, px: 1, py: 0.75, borderRadius: '14px', cursor: 'pointer',
                  backgroundColor: on ? beacon.sent : 'transparent',
                  '&:hover': { backgroundColor: on ? beacon.sent : beacon.tile },
                  '&:hover .pick, & .pick.Mui-checked': { opacity: 1 },
                }}
              >
                <Checkbox
                  className="pick"
                  size="small"
                  checked={on}
                  onClick={(ev) => ev.stopPropagation()}
                  onChange={() => toggle(p)}
                  sx={{ opacity: on ? 1 : 0.35, p: 0.5 }}
                />
                <Box sx={{ width: 40, height: 40, display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                  {e.dir ? (
                    <FolderIcon sx={{ fontSize: 28, color: beacon.accent }} />
                  ) : THUMB_EXT.test(e.name) ? (
                    <Thumb device={device} path={p} cache={thumbs} />
                  ) : (
                    <InsertDriveFileIcon sx={{ fontSize: 24, color: beacon.textSecondary }} />
                  )}
                </Box>
                <Typography noWrap sx={{ flex: 1, minWidth: 0, fontSize: '0.9rem' }}>{e.name}</Typography>
                <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.72rem', color: beacon.textFaint, width: 90, textAlign: 'right' }}>
                  {e.dir ? '' : formatSize(e.size)}
                </Typography>
                <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.72rem', color: beacon.textFaint, width: 110, textAlign: 'right' }}>
                  {new Date(e.modified).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' })}
                </Typography>
              </Box>
            );
          })
        )}
      </Box>
    </Dialog>
  );
};

export default PhoneFilesDialog;
