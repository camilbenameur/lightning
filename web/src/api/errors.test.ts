import { describe, expect, it } from 'vitest';
import { INFO } from '../test/http';
import { ApiError, describeError, parseRetryAfter } from './errors';

const apiError = (
  status: number,
  code: string,
  message = 'server message',
  retryAfterSeconds?: number,
) => new ApiError({ status, code, message, retryAfterSeconds });

describe('describeError', () => {
  it.each([
    [
      apiError(429, 'rate_limited', '', 12),
      'You’re running code too quickly. Try again in 12 seconds.',
    ],
    [
      apiError(429, 'rate_limited', '', 1),
      'You’re running code too quickly. Try again in 1 second.',
    ],
    [
      apiError(429, 'rate_limited'),
      'You’re running code too quickly. Please wait a moment and try again.',
    ],
    [
      apiError(503, 'server_busy'),
      'All sandboxes are busy right now. Please try again in a few seconds.',
    ],
    [
      apiError(503, 'sandbox_unavailable'),
      'The sandbox for this language is unavailable right now. Please try again later.',
    ],
    [
      apiError(0, 'network_error'),
      'Can’t reach the Lightning server. Check your connection and try again.',
    ],
    [
      apiError(0, 'connection_lost'),
      'The connection to the server was lost before the program finished.',
    ],
    [apiError(400, 'invalid_request', 'code is required'), 'code is required'],
    [
      apiError(502, 'http_error'),
      'The Lightning server is unavailable right now. Please try again in a moment.',
    ],
  ])('describes %o', (error, expected) => {
    expect(describeError(error)).toBe(expected);
  });

  it('mentions the size limit of payload_too_large when known', () => {
    expect(describeError(apiError(413, 'payload_too_large'), INFO.limits)).toBe(
      'Your code or input is too large (limit: 64 KB).',
    );
    expect(describeError(apiError(413, 'http_error'))).toBe('Your code or input is too large.');
  });

  it('falls back to a generic sentence for unexpected errors', () => {
    expect(describeError(new Error('boom'))).toBe('Something went wrong. Please try again.');
  });
});

describe('parseRetryAfter', () => {
  it('parses delta-seconds', () => {
    expect(parseRetryAfter('30')).toBe(30);
    expect(parseRetryAfter('1.2')).toBe(2);
  });

  it('parses HTTP dates relative to now', () => {
    const now = Date.parse('2026-01-01T00:00:00Z');
    expect(parseRetryAfter('Thu, 01 Jan 2026 00:00:05 GMT', now)).toBe(5);
    expect(parseRetryAfter('Wed, 31 Dec 2025 23:59:00 GMT', now)).toBe(0);
  });

  it('ignores missing or invalid values', () => {
    expect(parseRetryAfter(null)).toBeUndefined();
    expect(parseRetryAfter('')).toBeUndefined();
    expect(parseRetryAfter('soon')).toBeUndefined();
  });
});
