/**
 * wailsjs/go/main/App.ts
 *
 * Wails auto-generates this file when you run `wails dev` or `wails build`.
 * During standalone development (npm run dev, without Wails), we provide
 * this mock so the app compiles and runs in a browser with fake data.
 *
 * When running inside the actual Wails binary, the real generated file
 * in wailsjs/ takes precedence via Vite's path resolution.
 */

import type { Peer } from '../types/peer';

/** Simulated peers for browser-only development */
const MOCK_PEERS: Peer[] = [
  {
    id: 'pixel-8',
    deviceName: 'Pixel 8',
    ipAddress: '192.168.1.42',
    port: 9876,
    os: 'android',
  },
  {
    id: 'macbook-uriel',
    deviceName: 'MacBook Pro',
    ipAddress: '192.168.1.55',
    port: 9876,
    os: 'darwin',
  },
  {
    id: 'home-server',
    deviceName: 'Home Server',
    ipAddress: 'relay',
    port: 0,
    os: 'linux',
    via: 'relay',
  },
];

let mockTrusted: string[] = ['Pixel 8'];
let mockAutoCopy: string[] = [];

/** Returns the list of discovered peers. */
export const RejectOffer = (transferId: string): Promise<void> => Promise.resolve();
export const AcceptOffer = (transferId: string): Promise<void> => Promise.resolve();
export const AddTrustedDevice = (name: string): Promise<void> => {
  if (!mockTrusted.includes(name)) mockTrusted = [...mockTrusted, name];
  return Promise.resolve();
};
const ago = (min: number) => new Date(Date.now() - min * 60_000).toISOString();
const MOCK_HISTORY = [
  { id: 'h5', partnerName: 'Pixel 8', direction: 'incoming', kind: 'files', status: 'completed', durationMillis: 900, totalSize: 3_500_000, timestamp: ago(2), route: 'relay',
    files: [{ name: 'beach.jpg', size: 3_500_000 }] },
  { id: 'h4', partnerName: 'Pixel 8', direction: 'outgoing', kind: 'files', status: 'completed', durationMillis: 4000, totalSize: 38_400_000, timestamp: ago(5), route: 'direct',
    files: [{ name: 'clip.mp4', size: 38_400_000, path: '/tmp/clip.mp4' }] },
  { id: 'h3', partnerName: 'Pixel 8', direction: 'outgoing', kind: 'files', status: 'completed', durationMillis: 400, totalSize: 52_000, timestamp: ago(9), route: 'direct',
    files: [{ name: 'notes.pdf', size: 52_000, path: '/tmp/notes.pdf' }] },
  { id: 'h2', partnerName: 'Pixel 8', direction: 'incoming', kind: 'text', text: 'sending the pics now', status: 'completed', durationMillis: 0, totalSize: 20, timestamp: ago(12), files: [], route: 'relay' },
];
export const GetHistory = (): Promise<any[]> => Promise.resolve(MOCK_HISTORY);
export const ClearHistory = (): Promise<void> => Promise.resolve();
export const GetTrustedDevices = (): Promise<string[]> => Promise.resolve(mockTrusted);
export const GetAutoCopyDevices = (): Promise<string[]> => Promise.resolve(mockAutoCopy);
export const SetAutoCopy = (name: string, enabled: boolean): Promise<void> => {
  if (enabled && !mockTrusted.includes(name)) return Promise.reject(`${name} must be trusted first`);
  mockAutoCopy = enabled ? [...mockAutoCopy.filter((n) => n !== name), name] : mockAutoCopy.filter((n) => n !== name);
  return Promise.resolve();
};
export const OpenHistoryFile = (path: string, name: string): Promise<void> => {
  console.log('Mock: open', path || name);
  return Promise.resolve();
};
export const RevealHistoryFile = (path: string, name: string): Promise<void> =>
  Promise.reject(`${name} is no longer at ~/Downloads/PALT`);
let mockRelayCode = '';
const mockRelayStatus = () => ({
  configured: !!mockRelayCode,
  status: mockRelayCode ? 'online' : 'off',
  detail: '',
  server: mockRelayCode ? mockRelayCode.split(':').slice(1, 3).join(':') : '',
  code: mockRelayCode,
  peers: mockRelayCode ? 1 : 0,
});
export const GetRelayStatus = () => Promise.resolve(mockRelayStatus());
export const SetRelayCode = (code: string): Promise<void> => {
  mockRelayCode = code;
  return Promise.resolve();
};
export const GenerateRelayCode = (addr: string): Promise<string> => {
  mockRelayCode = `palt1:${addr.includes(':') ? addr : addr + ':9877'}:bW9ja21vY2ttb2NrbW9ja21vY2ttb2NrbW9ja21vY2s`;
  return Promise.resolve(mockRelayCode);
};
export const GetMediaPreview = (path: string, name: string) => {
  const video = /\.(mp4|mov|m4v|webm|mkv|avi|3gp)$/i.test(name);
  return /\.(jpe?g|png|gif|webp)$/i.test(name) || video
    ? Promise.resolve({ url: `https://picsum.photos/seed/${encodeURIComponent(name)}/480/320`, video, duration: video ? 83 : 0 })
    : Promise.reject('not previewable');
};
export const DeleteHistoryEntry = (id: string, direction: string): Promise<void> => Promise.resolve();
export const SendText = (ip: string, port: number, name: string, text: string): Promise<void> =>
  new Promise((r) => setTimeout(r, 250));

export async function GetPeers(): Promise<Peer[]> {
  // In real Wails, this calls the Go App.GetPeers() binding.
  await new Promise((r) => setTimeout(r, 400)); // realistic latency
  return MOCK_PEERS;
}

/** Returns metadata about the local device. */
export async function GetLocalDevice(): Promise<Peer> {
  await new Promise((r) => setTimeout(r, 100));
  return {
    id: 'local',
    deviceName: 'My Linux PC',
    ipAddress: '127.0.0.1',
    port: 9876,
    os: 'linux',
  };
}
