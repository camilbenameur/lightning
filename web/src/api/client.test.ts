import { describe, expect, it, vi } from 'vitest';
import { errorResponse, jsonResponse, LANGUAGES, ndjsonResponse } from '../test/http';
import { fetchInfo, fetchLanguages, runStream, toRunEvent } from './client';
import { ApiError } from './errors';
import type { RunEvent } from './types';

function stubFetch(response: Response | Promise<Response>) {
  const fetchMock = vi.fn(() => Promise.resolve(response));
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

async function collect(events: AsyncIterable<RunEvent>) {
  const collected: RunEvent[] = [];
  for await (const event of events) collected.push(event);
  return collected;
}

describe('fetchLanguages', () => {
  it('returns the languages of GET /api/v1/languages', async () => {
    const fetchMock = stubFetch(jsonResponse({ languages: LANGUAGES }));

    await expect(fetchLanguages()).resolves.toEqual(LANGUAGES);
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/languages', expect.anything());
  });

  it('drops malformed entries', async () => {
    stubFetch(jsonResponse({ languages: [LANGUAGES[0], { id: 'x' }, null, 'go'] }));
    await expect(fetchLanguages()).resolves.toEqual([LANGUAGES[0]]);
  });

  it('rejects a body without a languages array', async () => {
    stubFetch(jsonResponse({ nope: true }));
    await expect(fetchLanguages()).rejects.toMatchObject({ code: 'invalid_response' });
  });
});

describe('fetchInfo', () => {
  it('rejects a body without limits', async () => {
    stubFetch(jsonResponse({ version: '1' }));
    await expect(fetchInfo()).rejects.toMatchObject({ code: 'invalid_response' });
  });
});

describe('error mapping', () => {
  it('maps the error envelope, status and Retry-After header', async () => {
    stubFetch(errorResponse(429, 'rate_limited', 'Too many runs', { 'Retry-After': '12' }));

    const error: unknown = await fetchLanguages().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 429,
      code: 'rate_limited',
      message: 'Too many runs',
      retryAfterSeconds: 12,
    });
  });

  it('tolerates error bodies that are not JSON', async () => {
    stubFetch(new Response('<html>Bad gateway</html>', { status: 502 }));
    await expect(fetchLanguages()).rejects.toMatchObject({ status: 502, code: 'http_error' });
  });

  it('maps transport failures to network_error', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => Promise.reject(new TypeError('Failed to fetch'))),
    );
    await expect(fetchLanguages()).rejects.toMatchObject({ status: 0, code: 'network_error' });
  });

  it('lets aborts propagate untouched', async () => {
    const controller = new AbortController();
    controller.abort();
    const abort = new DOMException('Aborted', 'AbortError');
    vi.stubGlobal(
      'fetch',
      vi.fn(() => Promise.reject(abort)),
    );

    await expect(fetchLanguages(controller.signal)).rejects.toBe(abort);
  });
});

describe('runStream', () => {
  it('posts the run and yields validated events in order', async () => {
    const fetchMock = stubFetch(
      ndjsonResponse([
        { type: 'started' },
        { type: 'stdout', data: 'out' },
        { type: 'mystery' },
        { type: 'stderr', data: 'err' },
        {
          type: 'result',
          exitCode: 1,
          durationMs: 42,
          timedOut: false,
          oomKilled: false,
          truncated: true,
        },
      ]),
    );

    const events = await collect(runStream({ language: 'python', code: 'print(1)', stdin: '' }));

    expect(events).toEqual([
      { type: 'started' },
      { type: 'stdout', data: 'out' },
      { type: 'stderr', data: 'err' },
      {
        type: 'result',
        exitCode: 1,
        durationMs: 42,
        timedOut: false,
        oomKilled: false,
        truncated: true,
      },
    ]);
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/run/stream',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ language: 'python', code: 'print(1)', stdin: '' }),
      }),
    );
  });

  it('throws the error envelope of a rejected run', async () => {
    stubFetch(errorResponse(503, 'server_busy'));
    await expect(collect(runStream({ language: 'go', code: '' }))).rejects.toMatchObject({
      status: 503,
      code: 'server_busy',
    });
  });

  it('reports a stream that breaks mid-way as connection_lost', async () => {
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(new TextEncoder().encode('{"type":"started"}\n'));
        controller.error(new TypeError('network error'));
      },
    });
    stubFetch(new Response(body));

    await expect(collect(runStream({ language: 'go', code: '' }))).rejects.toMatchObject({
      code: 'connection_lost',
    });
  });
});

describe('toRunEvent', () => {
  it.each([
    null,
    'text',
    42,
    {},
    { type: 'stdout' },
    { type: 'stdout', data: 1 },
    { type: 'result' },
  ])('rejects %j', (value) => {
    expect(toRunEvent(value)).toBeNull();
  });

  it('fills in defaults for optional result fields', () => {
    expect(toRunEvent({ type: 'result', exitCode: 0 })).toEqual({
      type: 'result',
      exitCode: 0,
      durationMs: 0,
      timedOut: false,
      oomKilled: false,
      truncated: false,
    });
  });

  it('normalises error events', () => {
    expect(
      toRunEvent({ type: 'error', error: { code: 'internal_error', message: 'boom' } }),
    ).toEqual({
      type: 'error',
      error: { code: 'internal_error', message: 'boom' },
    });
    expect(toRunEvent({ type: 'error' })).toEqual({
      type: 'error',
      error: { code: 'internal_error', message: 'The run failed.' },
    });
  });
});
