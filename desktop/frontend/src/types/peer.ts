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
  /** True when a direct peer is reached over a USB cable (tethering). */
  usb?: boolean;
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
