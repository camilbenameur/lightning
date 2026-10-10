import { isRecord } from '../lib/guards';
import {
  ApiError,
  CONNECTION_LOST,
  INVALID_RESPONSE,
  NETWORK_ERROR,
  errorFromResponse,
} from './errors';
import { readNdjson } from './ndjson';
import type { Language, RunEvent, RunRequest, ServerInfo } from './types';

const API_BASE = `${(import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/+$/, '')}/api/v1`;

/** fetch() that maps transport failures and non-2xx responses to ApiError; aborts propagate as-is. */
async function request(path: string, init: RequestInit = {}): Promise<Response> {
  let response: Response;
  try {
    response = await fetch(API_BASE + path, init);
  } catch (error) {
    if (init.signal?.aborted) throw error;
    throw new ApiError({ status: 0, code: NETWORK_ERROR, message: 'Network error', cause: error });
  }
  if (!response.ok) throw await errorFromResponse(response);
  return response;
}

async function getJson(path: string, signal?: AbortSignal): Promise<Record<string, unknown>> {
  const response = await request(path, { signal, headers: { Accept: 'application/json' } });
  const body: unknown = await response.json().catch(() => null);
  if (!isRecord(body)) {
    throw new ApiError({
      status: response.status,
      code: INVALID_RESPONSE,
      message: 'Invalid JSON',
    });
  }
  return body;
}

export async function fetchLanguages(signal?: AbortSignal): Promise<Language[]> {
  const { languages } = await getJson('/languages', signal);
  if (!Array.isArray(languages)) {
    throw new ApiError({ status: 200, code: INVALID_RESPONSE, message: 'Missing languages' });
  }
  return languages.filter(isLanguage);
}

const LANGUAGE_FIELDS = ['id', 'name', 'version', 'monaco', 'fileName'] as const;

function isLanguage(value: unknown): value is Language {
  return isRecord(value) && LANGUAGE_FIELDS.every((field) => typeof value[field] === 'string');
}

export async function fetchInfo(signal?: AbortSignal): Promise<ServerInfo> {
  const info = await getJson('/info', signal);
  if (!isRecord(info.limits)) {
    throw new ApiError({ status: 200, code: INVALID_RESPONSE, message: 'Missing limits' });
  }
  return info as unknown as ServerInfo;
}

/**
 * Runs a program and yields its events as they arrive (`POST /run/stream`, NDJSON).
 * Unknown or malformed lines are ignored. Aborting `signal` cancels the request,
 * which makes the server kill the sandbox.
 */
export async function* runStream(
  run: RunRequest,
  signal?: AbortSignal,
): AsyncGenerator<RunEvent, void, undefined> {
  const response = await request('/run/stream', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'application/x-ndjson' },
    body: JSON.stringify(run),
    signal,
  });
  if (!response.body) {
    throw new ApiError({ status: response.status, code: INVALID_RESPONSE, message: 'Empty body' });
  }

  try {
    for await (const value of readNdjson(response.body)) {
      const event = toRunEvent(value);
      if (event) yield event;
    }
  } catch (error) {
    if (signal?.aborted) throw error;
    throw new ApiError({
      status: 0,
      code: CONNECTION_LOST,
      message: 'Stream interrupted',
      cause: error,
    });
  }
}

/** Validates one decoded NDJSON value against the run event schema. */
export function toRunEvent(value: unknown): RunEvent | null {
  if (!isRecord(value)) return null;
  switch (value.type) {
    case 'started':
      return { type: 'started' };
    case 'stdout':
    case 'stderr':
      return typeof value.data === 'string' ? { type: value.type, data: value.data } : null;
    case 'result':
      if (typeof value.exitCode !== 'number') return null;
      return {
        type: 'result',
        exitCode: value.exitCode,
        durationMs: typeof value.durationMs === 'number' ? value.durationMs : 0,
        timedOut: value.timedOut === true,
        oomKilled: value.oomKilled === true,
        truncated: value.truncated === true,
      };
    case 'error': {
      const error = isRecord(value.error) ? value.error : {};
      return {
        type: 'error',
        error: {
          code: typeof error.code === 'string' ? error.code : 'internal_error',
          message: typeof error.message === 'string' ? error.message : 'The run failed.',
        },
      };
    }
    default:
      return null;
  }
}
