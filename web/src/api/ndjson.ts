/**
 * Reads a newline-delimited JSON stream and yields one parsed value per line.
 *
 * Lines may be split across chunks at any byte (including in the middle of a
 * multi-byte UTF-8 sequence). Blank lines are skipped; malformed lines are
 * reported through `onMalformed` instead of aborting the stream.
 * If the consumer stops early, the underlying stream is cancelled.
 */
export async function* readNdjson(
  stream: ReadableStream<Uint8Array>,
  onMalformed: (line: string) => void = () => undefined,
): AsyncGenerator<unknown, void, undefined> {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let finished = false;

  try {
    while (!finished) {
      const { done, value } = await reader.read();
      finished = done;
      buffer += done ? decoder.decode() : decoder.decode(value, { stream: true });

      const lines = buffer.split('\n');
      buffer = finished ? '' : (lines.pop() ?? '');

      for (const raw of lines) {
        const line = raw.trim();
        if (line === '') continue;
        let parsed: unknown;
        try {
          parsed = JSON.parse(line);
        } catch {
          onMalformed(line);
          continue;
        }
        yield parsed;
      }
    }
  } finally {
    if (finished) reader.releaseLock();
    else await reader.cancel().catch(() => undefined);
  }
}
