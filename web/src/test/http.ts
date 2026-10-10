/** Helpers to fake the Lightning API in tests. */

const encoder = new TextEncoder();

export function jsonResponse(body: unknown, status = 200, headers: Record<string, string> = {}) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  });
}

export function errorResponse(
  status: number,
  code: string,
  message = code,
  headers: Record<string, string> = {},
) {
  return jsonResponse({ error: { code, message } }, status, headers);
}

/** A byte stream that emits the given chunks (strings are UTF-8 encoded) and closes. */
export function byteStream(chunks: (string | Uint8Array)[]): ReadableStream<Uint8Array> {
  return new ReadableStream({
    start(controller) {
      for (const chunk of chunks) {
        controller.enqueue(typeof chunk === 'string' ? encoder.encode(chunk) : chunk);
      }
      controller.close();
    },
  });
}

export function ndjsonResponse(events: unknown[]) {
  return new Response(byteStream(events.map((event) => `${JSON.stringify(event)}\n`)), {
    headers: { 'Content-Type': 'application/x-ndjson' },
  });
}

/**
 * An NDJSON response whose events are pushed by the test, like a program that is
 * still running. Aborting `signal` errors the body, as fetch does.
 */
export function liveNdjsonResponse(signal: AbortSignal | null | undefined) {
  let controller!: ReadableStreamDefaultController<Uint8Array>;
  const body = new ReadableStream<Uint8Array>({
    start(c) {
      controller = c;
    },
  });
  signal?.addEventListener('abort', () => {
    controller.error(new DOMException('The operation was aborted.', 'AbortError'));
  });
  return {
    response: new Response(body, { headers: { 'Content-Type': 'application/x-ndjson' } }),
    push: (event: unknown) => controller.enqueue(encoder.encode(`${JSON.stringify(event)}\n`)),
    close: () => controller.close(),
  };
}

export const LANGUAGES = [
  { id: 'python', name: 'Python', version: '3.14', monaco: 'python', fileName: 'main.py' },
  { id: 'go', name: 'Go', version: '1.27', monaco: 'go', fileName: 'main.go' },
  { id: 'bash', name: 'Bash', version: '5.2', monaco: 'shell', fileName: 'main.sh' },
];

export const INFO = {
  version: 'test',
  commit: 'abc1234',
  limits: {
    timeoutMs: 10000,
    memoryMb: 256,
    maxCodeBytes: 65536,
    maxStdinBytes: 65536,
    maxOutputBytes: 65536,
    network: false,
  },
};

type Handler = (init: RequestInit | undefined) => Response | Promise<Response>;

/** Routes fetch calls by "METHOD /path" (path relative to /api/v1). Unrouted calls reject. */
export function routes(table: Record<string, Handler>) {
  return async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const url = new URL(input instanceof Request ? input.url : String(input), 'http://localhost');
    const key = `${init?.method ?? 'GET'} ${url.pathname.replace(/^\/api\/v1/, '')}`;
    const handler = table[key];
    if (!handler) throw new TypeError(`Unexpected request: ${key}`);
    return handler(init);
  };
}
