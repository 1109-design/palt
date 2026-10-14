export interface HistoryFile {
  name: string;
  size: number;
}

export interface HistoryEntry {
  id: string;
  partnerName: string;
  files: HistoryFile[];
  totalSize: number;
  direction: 'incoming' | 'outgoing';
  timestamp: string; // ISO string
  status: 'completed' | 'error';
  errorMessage?: string;
  durationMillis: number;
  /** 'text' for messages; absent on file transfers and older entries. */
  kind?: 'files' | 'text';
  /** Message body when kind === 'text'. */
  text?: string;
  /** Incoming text was auto-copied to the clipboard. */
  copied?: boolean;
}
