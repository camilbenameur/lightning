/** Wire types of the Lightning HTTP API (see SPEC §4). */

export interface Language {
  id: string;
  name: string;
  version: string;
  /** Monaco language id used for syntax highlighting. */
  monaco: string;
  fileName: string;
}

export interface Limits {
  timeoutMs: number;
  memoryMb: number;
  maxCodeBytes: number;
  maxStdinBytes: number;
  maxOutputBytes: number;
  network: boolean;
}

export interface ServerInfo {
  version: string;
  commit: string;
  limits: Limits;
}

export interface RunRequest {
  language: string;
  code: string;
  stdin?: string;
}

export interface RunResult {
  exitCode: number;
  durationMs: number;
  timedOut: boolean;
  oomKilled: boolean;
  truncated: boolean;
}

export type OutputStream = 'stdout' | 'stderr';

export interface ErrorBody {
  code: string;
  message: string;
}

export type RunEvent =
  | { type: 'started' }
  | { type: OutputStream; data: string }
  | ({ type: 'result' } & RunResult)
  | { type: 'error'; error: ErrorBody };
