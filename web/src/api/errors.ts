import { formatBytes, pluralize } from '../lib/format';
import { isRecord } from '../lib/guards';
import type { Limits } from './types';

/** Client-side error codes, in addition to the server's (SPEC §4). */
export const NETWORK_ERROR = 'network_error';
export const CONNECTION_LOST = 'connection_lost';
export const INVALID_RESPONSE = 'invalid_response';

interface ApiErrorInit {
  /** HTTP status; 0 when no response was received. */
  status: number;
  code: string;
  message: string;
  retryAfterSeconds?: number;
  cause?: unknown;
}

/** An API failure: an HTTP error envelope, a stream `error` event or a transport problem. */
export class ApiError extends Error {
  override readonly name = 'ApiError';
  readonly status: number;
  readonly code: string;
  readonly retryAfterSeconds: number | undefined;

  constructor({ status, code, message, retryAfterSeconds, cause }: ApiErrorInit) {
    super(message, { cause });
    this.status = status;
    this.code = code;
    this.retryAfterSeconds = retryAfterSeconds;
  }
}

/** Builds an ApiError from a non-2xx response, tolerating bodies that are not the JSON envelope. */
export async function errorFromResponse(response: Response): Promise<ApiError> {
  let code = 'http_error';
  let message = `Request failed with HTTP ${response.status}`;
  try {
    const body: unknown = await response.json();
    const error = isRecord(body) ? body.error : undefined;
    if (isRecord(error) && typeof error.code === 'string') {
      code = error.code;
      if (typeof error.message === 'string') message = error.message;
    }
  } catch {
    // Not JSON (e.g. a proxy error page): keep the generic message.
  }
  return new ApiError({
    status: response.status,
    code,
    message,
    retryAfterSeconds: parseRetryAfter(response.headers.get('Retry-After')),
  });
}

/** Parses a Retry-After header (delta-seconds or HTTP date) into whole seconds. */
export function parseRetryAfter(value: string | null, now = Date.now()): number | undefined {
  if (value === null || value.trim() === '') return undefined;
  const seconds = Number(value);
  if (Number.isFinite(seconds)) return Math.max(0, Math.ceil(seconds));
  const date = Date.parse(value);
  return Number.isNaN(date) ? undefined : Math.max(0, Math.ceil((date - now) / 1000));
}

/** Turns any error thrown by the API client into a short, user-facing sentence. */
export function describeError(error: unknown, limits?: Limits | null): string {
  if (!(error instanceof ApiError)) return 'Something went wrong. Please try again.';

  switch (error.code) {
    case NETWORK_ERROR:
      return 'Can’t reach the Lightning server. Check your connection and try again.';
    case CONNECTION_LOST:
      return 'The connection to the server was lost before the program finished.';
    case INVALID_RESPONSE:
      return 'The server sent an unexpected response. Please try again.';
    case 'unsupported_language':
      return 'This language is not available on this server.';
    case 'rate_limited':
      return rateLimited(error.retryAfterSeconds);
    case 'server_busy':
      return 'All sandboxes are busy right now. Please try again in a few seconds.';
    case 'sandbox_unavailable':
      return 'The sandbox for this language is unavailable right now. Please try again later.';
    case 'payload_too_large':
      return tooLarge(limits);
  }

  switch (error.status) {
    case 413:
      return tooLarge(limits);
    case 429:
      return rateLimited(error.retryAfterSeconds);
    case 502:
    case 503:
    case 504:
      return 'The Lightning server is unavailable right now. Please try again in a moment.';
    default:
      return error.message;
  }
}

function rateLimited(retryAfterSeconds: number | undefined): string {
  return retryAfterSeconds
    ? `You’re running code too quickly. Try again in ${pluralize(retryAfterSeconds, 'second')}.`
    : 'You’re running code too quickly. Please wait a moment and try again.';
}

function tooLarge(limits: Limits | null | undefined): string {
  return limits
    ? `Your code or input is too large (limit: ${formatBytes(limits.maxCodeBytes)}).`
    : 'Your code or input is too large.';
}
