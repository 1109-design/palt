/**
 * Peer — the shared data contract between the Go backend and React frontend.
 *
 * Must mirror the Go `models.Peer` struct exactly.
 */
export interface Peer {
  /** Stable unique identifier — the mDNS instance name. */
  id: string;
  /** Human-readable device name. */
  deviceName: string;
  /** Resolved IPv4 address. */
  ipAddress: string;
  /** TCP port the peer's PALT service listens on. */
  port: number;
  /** OS string: "linux" | "android" | "darwin" | "windows" | "unknown" */
  os: string;
  /** How the peer is reachable: same LAN ("direct") or through the relay. */
  via?: string;
  /** Stable PALT device ID ("" for builds before 2.0). */
  deviceId?: string;
  /** True while an authenticated link to this paired device is up. */
  paired?: boolean;
  /** How PALT reaches it: "usb" (cable) | "wifi" | "lan" | "relay". */
  path?: string;
}

export type Route = 'direct' | 'relay';

/** What a paired device reports about itself (Go DeviceStatus). */
export interface DeviceStatus {
  /** 0-100, -1 none/unknown */
  battery: number;
  charging: boolean;
  /** "wifi" | "cellular" | "ethernet" | "" */
  network: string;
  /** Wi-Fi name, "" when unknown */
  ssid: string;
  /** "5G", "LTE", … on mobile data */
  cellular: string;
  /** mobile signal 0-4, -1 unknown */
  signal: number;
  /** reaches the internet, not just the LAN */
  internet?: boolean;
}

/** A notification mirrored from a paired phone (Go PhoneNotification). */
export interface PhoneNotification {
  id: string;
  device: string;
  app: string;
  appId: string;
  title: string;
  text: string;
  /** ms since epoch */
  time: number;
  actions: string[];
  replyable: boolean;
  /** local icon file path */
  icon?: string;
}

/** One clipboard history item (Go ClipboardEntry). */
export interface ClipboardEntry {
  text: string;
  /** device name, "" for this computer */
  from: string;
  time: number;
}

/** State of the GNOME clipboard helper extension. */
export interface ClipboardHelper {
  running: boolean;
  installed: boolean;
}

/** One app playing media on a device (Go MediaPlayer). */
export interface MediaPlayer {
  id: string;
  name: string;
  title: string;
  artist: string;
  album: string;
  /** web URL or data: URL, '' none */
  art: string;
  playing: boolean;
  canNext: boolean;
  canPrev: boolean;
  canSeek: boolean;
  canControl: boolean;
  /** ms, as of when the state arrived */
  position: number;
  /** ms, 0 unknown */
  length: number;
  volume: number;
}

/** What a device reports about its media (Go MediaState). */
export interface MediaState {
  players: MediaPlayer[];
  volume: { level: number; muted: boolean };
  at: number;
}

/** One SMS, or a conversation's newest one (Go SMSMessage). */
export interface SMSMessage {
  id: number;
  threadId: number;
  address: string;
  body: string;
  /** unix ms */
  date: number;
  /** 1 received, 2 sent, 4 sending, 5 failed, 6 queued */
  type: number;
  read: boolean;
  name: string;
  unread?: number;
  /** SIM subscription id it came in on / went out from, -1 unknown */
  subscription?: number;
}

/** A phone contact with numbers (Go Contact). */
export interface Contact {
  id: string;
  name: string;
  numbers: string[];
  photo: string;
}

/** A call in progress on a paired phone (Go ActiveCall). */
export interface ActiveCall {
  device: string;
  /** ringing | talking */
  state: string;
  /** incoming | outgoing */
  direction: string;
  number: string;
  name: string;
  /** data: URL or '' */
  photo: string;
  /** unix ms picked up / dialled; 0 while ringing */
  since: number;
}

/** One call in a phone's call log (Go CallLogEntry). */
export interface CallLogEntry {
  number: string;
  name: string;
  /** incoming outgoing missed rejected blocked voicemail */
  type: string;
  date: number;
  /** seconds */
  duration: number;
  /** SIM slot, 0 unknown */
  sim: number;
}

/** One of a phone's SIMs (Go SIM). */
export interface SIM {
  /** subscription id, used to pick it */
  id: number;
  slot: number;
  name: string;
  carrier: string;
}
