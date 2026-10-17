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
}

export type Route = 'direct' | 'relay';
