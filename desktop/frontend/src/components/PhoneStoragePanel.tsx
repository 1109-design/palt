/**
 * PhoneStoragePanel — a phone's space and memory, at the top of its file
 * browser (Go GetPhoneStorage): used vs free split by kind of file, the
 * biggest folders and files (click to go there), SD card, RAM and swap.
 */
import React, { useEffect, useState } from 'react';
import { Box, ButtonBase, CircularProgress, Collapse, Typography } from '@mui/material';
import ExpandMoreIcon from '@mui/icons-material/ExpandMoreRounded';
import StorageIcon from '@mui/icons-material/StorageRounded';
import { beacon, fonts } from '../theme/theme';
import { formatSize } from './HistoryView';

const go = () => (window as any).go?.main?.App;

interface PhoneStorage {
  total: number;
  free: number;
  categories: Record<'images' | 'videos' | 'audio' | 'documents' | 'other' | 'system', number>;
  folders: { name: string; path: string; size: number }[] | null;
  largest: { path: string; size: number }[] | null;
  volumes: { name: string; path: string; removable: boolean; total: number; free: number }[] | null;
  ram: { total: number; available: number; low: boolean; threshold: number; cached: number; swapTotal: number; swapFree: number };
}

const KINDS: { key: keyof PhoneStorage['categories']; label: string; color: string }[] = [
  { key: 'images', label: 'Photos', color: '#FFB454' },
  { key: 'videos', label: 'Videos', color: '#FF7B72' },
  { key: 'audio', label: 'Audio', color: '#C792EA' },
  { key: 'documents', label: 'Documents', color: '#7AA2FF' },
  { key: 'other', label: 'Other files', color: '#53D993' },
  { key: 'system', label: 'Apps & system', color: '#8FA0BC' },
];

const ROOT = '/storage/emulated/0';

/** A rounded bar made of coloured parts; the rest of total is empty. */
const Bar: React.FC<{ parts: { size: number; color: string }[]; total: number; height?: number }> = ({ parts, total, height = 12 }) => (
  <Box sx={{ display: 'flex', height, borderRadius: 999, overflow: 'hidden', backgroundColor: 'rgba(255,255,255,0.07)' }}>
    {parts.map((p, i) => (
      <Box key={i} sx={{ width: `${total > 0 ? (p.size / total) * 100 : 0}%`, backgroundColor: p.color, transition: 'width 600ms ease' }} />
    ))}
  </Box>
);

const Row: React.FC<{ label: string; size: number; of: number; onClick?: () => void }> = ({ label, size, of, onClick }) => (
  <ButtonBase
    onClick={onClick}
    disabled={!onClick}
    sx={{ display: 'flex', alignItems: 'center', gap: 1, width: '100%', px: 1, py: 0.6, borderRadius: '10px', justifyContent: 'flex-start', '&:hover': { backgroundColor: beacon.tile } }}
  >
    <Typography noWrap title={label} sx={{ flex: 1, minWidth: 0, textAlign: 'left', fontSize: '0.82rem' }}>
      {label}
    </Typography>
    <Box sx={{ width: 70 }}>
      <Bar parts={[{ size, color: beacon.accent }]} total={of} height={5} />
    </Box>
    <Typography sx={{ fontFamily: fonts.mono, fontSize: '0.72rem', color: beacon.textSecondary, width: 72, textAlign: 'right' }}>{formatSize(size)}</Typography>
  </ButtonBase>
);

const heading = { fontSize: '0.74rem', fontWeight: 700, letterSpacing: 0.4, textTransform: 'uppercase' as const, color: beacon.textFaint, mb: 0.75, px: 1 };

const PhoneStoragePanel: React.FC<{ device: string; refreshKey: number; onOpen: (folder: string, file?: string) => void }> = ({ device, refreshKey, onOpen }) => {
  const [info, setInfo] = useState<PhoneStorage | null>(null);
  const [error, setError] = useState('');
  const [open, setOpen] = useState(false);

  useEffect(() => {
    let live = true;
    setError('');
    go()
      ?.GetPhoneStorage(device)
      .then((s: PhoneStorage) => live && setInfo(s))
      .catch((e: unknown) => live && setError(String(e)));
    return () => {
      live = false;
    };
  }, [device, refreshKey]);

  if (error) return <Typography sx={{ mx: 2.5, mb: 1, fontSize: '0.8rem', color: beacon.textFaint }}>{error}</Typography>;
  if (!info)
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mx: 2.5, mb: 1.5 }}>
        <CircularProgress size={14} />
        <Typography sx={{ fontSize: '0.8rem', color: beacon.textSecondary }}>Reading storage…</Typography>
      </Box>
    );

  const used = info.total - info.free;
  const ram = info.ram;
  const ramUsed = ram.total - ram.available;
  const swapUsed = ram.swapTotal - ram.swapFree;
  const maxFolder = info.folders?.[0]?.size ?? 1;
  const maxFile = info.largest?.[0]?.size ?? 1;

  return (
    <Box sx={{ mx: 2.5, mb: 1.5, p: 1.75, borderRadius: '18px', backgroundColor: beacon.tile }}>
      <ButtonBase onClick={() => setOpen((o) => !o)} sx={{ display: 'flex', width: '100%', alignItems: 'center', gap: 1.25, borderRadius: '10px', mb: 1.25 }}>
        <StorageIcon sx={{ fontSize: 20, color: beacon.accent }} />
        <Typography sx={{ flex: 1, textAlign: 'left', fontSize: '0.9rem', fontWeight: 700 }}>
          {formatSize(used)} used of {formatSize(info.total)}
          <Box component="span" sx={{ fontWeight: 400, color: beacon.textSecondary }}>
            {'  ·  '}
            {formatSize(info.free)} free · RAM {formatSize(ram.available)} free of {formatSize(ram.total)}
          </Box>
        </Typography>
        <ExpandMoreIcon sx={{ color: beacon.textSecondary, transform: open ? 'rotate(180deg)' : 'none', transition: 'transform 200ms' }} />
      </ButtonBase>

      <Bar parts={KINDS.map((k) => ({ size: info.categories[k.key], color: k.color }))} total={info.total} />
      <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1.75, mt: 1 }}>
        {KINDS.map((k) => (
          <Box key={k.key} sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
            <Box sx={{ width: 8, height: 8, borderRadius: 999, backgroundColor: k.color }} />
            <Typography sx={{ fontSize: '0.76rem', color: beacon.textSecondary }}>
              {k.label} <b style={{ color: beacon.text }}>{formatSize(info.categories[k.key])}</b>
            </Typography>
          </Box>
        ))}
      </Box>

      <Collapse in={open}>
        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: 2, mt: 2 }}>
          <Box>
            <Typography sx={heading}>Biggest folders</Typography>
            {(info.folders ?? []).map((f) => (
              <Row key={f.path} label={f.name} size={f.size} of={maxFolder} onClick={() => onOpen(f.path)} />
            ))}
          </Box>
          <Box>
            <Typography sx={heading}>Largest files</Typography>
            {(info.largest ?? []).map((f) => {
              const dir = f.path.slice(0, f.path.lastIndexOf('/'));
              return (
                <Row
                  key={f.path}
                  label={f.path.replace(ROOT + '/', '')}
                  size={f.size}
                  of={maxFile}
                  onClick={() => onOpen(dir, f.path)}
                />
              );
            })}
          </Box>
        </Box>

        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: 2, mt: 2 }}>
          <Box>
            <Typography sx={heading}>Memory (RAM)</Typography>
            <Box sx={{ px: 1 }}>
              <Bar
                parts={[
                  { size: ramUsed, color: ram.low ? beacon.danger : beacon.accent },
                ]}
                total={ram.total}
                height={8}
              />
              <Typography sx={{ fontSize: '0.78rem', color: beacon.textSecondary, mt: 0.75, lineHeight: 1.6 }}>
                {formatSize(ramUsed)} in use · {formatSize(ram.available)} available of {formatSize(ram.total)}
                <br />
                {formatSize(ram.cached)} cached (freed when needed)
                {ram.swapTotal > 0 && (
                  <>
                    <br />
                    Swap {formatSize(swapUsed)} used of {formatSize(ram.swapTotal)}
                  </>
                )}
                <br />
                {ram.low ? (
                  <b style={{ color: beacon.danger }}>The phone is low on memory and closing apps</b>
                ) : (
                  `Low-memory point: ${formatSize(ram.threshold)} available`
                )}
              </Typography>
            </Box>
          </Box>
          {(info.volumes ?? []).length > 0 && (
            <Box>
              <Typography sx={heading}>Other storage</Typography>
              {(info.volumes ?? []).map((v) => (
                <Box key={v.path} sx={{ px: 1, mb: 1 }}>
                  <Typography sx={{ fontSize: '0.82rem', mb: 0.5 }}>
                    {v.name}
                    {v.removable ? ' (removable)' : ''}
                  </Typography>
                  <Bar parts={[{ size: v.total - v.free, color: beacon.relay }]} total={v.total} height={8} />
                  <Typography sx={{ fontSize: '0.76rem', color: beacon.textSecondary, mt: 0.5 }}>
                    {formatSize(v.total - v.free)} used · {formatSize(v.free)} free of {formatSize(v.total)}
                  </Typography>
                </Box>
              ))}
            </Box>
          )}
        </Box>
      </Collapse>
    </Box>
  );
};

export default PhoneStoragePanel;
