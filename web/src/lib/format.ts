import type { Limits } from '../api/types';

export function pluralize(count: number, noun: string): string {
  return `${count} ${noun}${count === 1 ? '' : 's'}`;
}

/** 850 → "850 ms", 1234 → "1.2 s", 65000 → "1m 05s". */
export function formatDuration(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)} ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)} s`;
  const totalSeconds = Math.floor(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = String(totalSeconds % 60).padStart(2, '0');
  return `${minutes}m ${seconds}s`;
}

/** 65536 → "64 KB", 1048576 → "1 MB". */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${Math.round(bytes / (1024 * 1024))} MB`;
}

/** Summarises the sandbox limits, e.g. "no network · 256 MB · 10 s". */
export function formatLimits(limits: Limits): string {
  const seconds = limits.timeoutMs / 1000;
  return [
    limits.network ? 'network on' : 'no network',
    `${limits.memoryMb} MB`,
    `${Number.isInteger(seconds) ? seconds : seconds.toFixed(1)} s`,
  ].join(' · ');
}
