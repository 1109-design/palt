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
    deviceId: 'a1b2c3d4e5f60718',
    paired: true,
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

let mockAutoCopy: string[] = [];

/** Returns the list of discovered peers. */
export const RejectOffer = (transferId: string): Promise<void> => Promise.resolve();
export const AcceptOffer = (transferId: string): Promise<void> => Promise.resolve();
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
export const GetAutoCopyDevices = (): Promise<string[]> => Promise.resolve(mockAutoCopy);
export const SetAutoCopy = (name: string, enabled: boolean): Promise<void> => {
  mockAutoCopy = enabled ? [...mockAutoCopy.filter((n) => n !== name), name] : mockAutoCopy.filter((n) => n !== name);
  return Promise.resolve();
};
export const OpenHistoryFile = (path: string, name: string): Promise<void> => {
  console.log('Mock: open', path || name);
  return Promise.resolve();
};
export const RevealHistoryFile = (path: string, name: string): Promise<void> =>
  Promise.reject(`${name} is no longer at ~/Downloads/PALT`);
let mockRelay = '';
const mockKey = 'bW9ja21vY2ttb2NrbW9ja21vY2ttb2NrbW9ja21vY2s';
const mockPairing = () => ({
  deviceId: '0123456789abcdef',
  code: `palt2:${mockKey}${mockRelay ? '@' + mockRelay : ''}`,
  relay: mockRelay,
});
const mockRelayStatus = () => ({
  configured: !!mockRelay,
  status: mockRelay ? 'online' : 'off',
  detail: '',
  server: mockRelay,
  code: mockPairing().code,
  peers: mockRelay ? 1 : 0,
});
export const GetRelayStatus = () => Promise.resolve(mockRelayStatus());
export const GetPairing = () => Promise.resolve(mockPairing());
export const JoinPairing = (code: string): Promise<void> =>
  code.startsWith('palt2:') || code.startsWith('palt1:') ? Promise.resolve() : Promise.reject('pairing code must start with palt2:');
export const ResetPairing = (): Promise<void> => Promise.resolve();
export const GetDeviceStatus = () =>
  Promise.resolve({ 'Pixel 8': { battery: 82, charging: true, network: 'cellular', ssid: '', cellular: '5G', signal: 3 } });
export const RingDevice = (name: string): Promise<void> => Promise.resolve();
export const PingDevice = (name: string): Promise<void> => Promise.resolve();
export const StopRinging = (): Promise<void> => Promise.resolve();
export const SendClipboard = (name: string): Promise<void> => Promise.resolve();
export const GetPhoneNotifications = (device: string) =>
  Promise.resolve([
    { id: 'n1', device, app: 'WhatsApp', appId: 'com.whatsapp', title: 'Mum', text: 'Are you coming for dinner?', time: Date.now() - 120000, actions: ['Mark as read'], replyable: true },
    { id: 'n2', device, app: 'Gmail', appId: 'com.google.android.gm', title: 'Your order shipped', text: 'Arrives Tuesday', time: Date.now() - 3600000, actions: ['Archive'], replyable: false },
  ]);
export const ReplyToNotification = (device: string, id: string, text: string): Promise<void> => Promise.resolve();
export const DismissPhoneNotification = (device: string, id: string): Promise<void> => Promise.resolve();
export const RunPhoneNotificationAction = (device: string, id: string, index: number): Promise<void> => Promise.resolve();
const mockSettings: Record<string, boolean> = { notifications: true, clipboardSend: true, clipboardReceive: true, pauseOnCall: true };
export const GetClipboardHistory = () =>
  Promise.resolve([
    { text: 'https://example.com/recipe', from: 'Pixel 8', time: Date.now() - 60000 },
    { text: 'ssh uriel@192.168.1.10', from: '', time: Date.now() - 900000 },
  ]);
export const GetClipboardHelper = () => Promise.resolve({ running: false, installed: false });
export const InstallClipboardHelper = (): Promise<void> => Promise.resolve();
export const CopyFromHistory = (text: string): Promise<void> => Promise.resolve();
export const GetDeviceSettings = (device: string) => Promise.resolve({ ...mockSettings });
export const SetDeviceSetting = (device: string, key: string, on: boolean): Promise<void> => {
  mockSettings[key] = on;
  return Promise.resolve();
};
export const ConnectByIP = (addr: string): Promise<void> =>
  new Promise((_, reject) => setTimeout(() => reject('could not connect: timeout'), 400));
export const SetRelayServer = (addr: string): Promise<void> => {
  mockRelay = addr && !addr.includes(':') ? `${addr}:9877` : addr;
  return Promise.resolve();
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
