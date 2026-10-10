import { describe, expect, it } from 'vitest';
import { INFO } from '../test/http';
import { formatBytes, formatDuration, formatLimits } from './format';

describe('format', () => {
  it.each([
    [0, '0 ms'],
    [850, '850 ms'],
    [1234, '1.2 s'],
    [59_949, '59.9 s'],
    [65_000, '1m 05s'],
  ])('formats %i ms as %s', (ms, expected) => {
    expect(formatDuration(ms)).toBe(expected);
  });

  it('formats byte sizes', () => {
    expect(formatBytes(512)).toBe('512 B');
    expect(formatBytes(65536)).toBe('64 KB');
    expect(formatBytes(1024 * 1024)).toBe('1 MB');
  });

  it('summarises sandbox limits', () => {
    expect(formatLimits(INFO.limits)).toBe('no network · 256 MB · 10 s');
    expect(formatLimits({ ...INFO.limits, network: true, timeoutMs: 2500 })).toBe(
      'network on · 256 MB · 2.5 s',
    );
  });
});
