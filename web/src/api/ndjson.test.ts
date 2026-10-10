import { describe, expect, it, vi } from 'vitest';
import { byteStream } from '../test/http';
import { readNdjson } from './ndjson';

async function collect(stream: ReadableStream<Uint8Array>, onMalformed?: (line: string) => void) {
  const values: unknown[] = [];
  for await (const value of readNdjson(stream, onMalformed)) values.push(value);
  return values;
}

describe('readNdjson', () => {
  it('parses one value per line', async () => {
    const values = await collect(byteStream(['{"a":1}\n{"b":2}\n']));
    expect(values).toEqual([{ a: 1 }, { b: 2 }]);
  });

  it('reassembles lines split across chunks', async () => {
    const values = await collect(
      byteStream(['{"type":"std', 'out","data":"hel', 'lo"}\n{"n":', '2}\n']),
    );
    expect(values).toEqual([{ type: 'stdout', data: 'hello' }, { n: 2 }]);
  });

  it('decodes multi-byte characters split between chunks', async () => {
    const bytes = new TextEncoder().encode('{"data":"€ 😀 ü"}\n');
    const euro = bytes.indexOf(0xe2);
    const emoji = bytes.indexOf(0xf0);
    const chunks = [
      bytes.slice(0, euro + 1),
      bytes.slice(euro + 1, emoji + 2),
      bytes.slice(emoji + 2, emoji + 3),
      bytes.slice(emoji + 3),
    ];

    expect(await collect(byteStream(chunks))).toEqual([{ data: '€ 😀 ü' }]);
  });

  it('handles CRLF, blank lines and a final line without a newline', async () => {
    const values = await collect(byteStream(['{"a":1}\r\n\n  \n{"b":2}']));
    expect(values).toEqual([{ a: 1 }, { b: 2 }]);
  });

  it('skips malformed lines and keeps reading', async () => {
    const onMalformed = vi.fn();
    const values = await collect(byteStream(['{"a":1}\nnot json\n{"b":\n{"c":3}\n']), onMalformed);

    expect(values).toEqual([{ a: 1 }, { c: 3 }]);
    expect(onMalformed.mock.calls).toEqual([['not json'], ['{"b":']]);
  });

  it('cancels the stream when the consumer stops early', async () => {
    const cancel = vi.fn();
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(new TextEncoder().encode('{"a":1}\n{"b":2}\n'));
      },
      cancel,
    });

    for await (const value of readNdjson(stream)) {
      expect(value).toEqual({ a: 1 });
      break;
    }
    expect(cancel).toHaveBeenCalledOnce();
  });
});
