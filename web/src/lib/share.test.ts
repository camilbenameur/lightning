import { compressToEncodedURIComponent } from 'lz-string';
import { describe, expect, it } from 'vitest';
import { buildShareUrl, encodeSnippet, parseShareHash, type Snippet } from './share';

const snippet: Snippet = {
  language: 'python',
  code: 'name = input()\nprint(f"héllo {name} ⚡")\n',
  stdin: 'Ada\n',
};

describe('share links', () => {
  it('round-trips a snippet through the URL', () => {
    const url = new URL(buildShareUrl(snippet, 'https://lightning.dev/?ref=x#old'));

    expect(url.pathname + url.search).toBe('/?ref=x');
    expect(parseShareHash(url.hash)).toEqual({ kind: 'snippet', snippet });
  });

  it('uses the documented {l, c, i} payload', () => {
    const hash = `#code=${encodeSnippet(snippet)}`;
    const expected = compressToEncodedURIComponent(
      JSON.stringify({ l: snippet.language, c: snippet.code, i: snippet.stdin }),
    );
    expect(hash).toBe(`#code=${expected}`);
  });

  it('treats a missing stdin as empty', () => {
    const hash = `#code=${compressToEncodedURIComponent(JSON.stringify({ l: 'go', c: 'x' }))}`;
    expect(parseShareHash(hash)).toEqual({
      kind: 'snippet',
      snippet: { language: 'go', code: 'x', stdin: '' },
    });
  });

  it('ignores fragments that are not share links', () => {
    expect(parseShareHash('')).toEqual({ kind: 'none' });
    expect(parseShareHash('#section')).toEqual({ kind: 'none' });
  });

  it.each([
    ['garbage', '#code=%%%not-lz%%%'],
    ['empty', '#code='],
    ['not an object', `#code=${compressToEncodedURIComponent('"text"')}`],
    ['no language', `#code=${compressToEncodedURIComponent(JSON.stringify({ c: 'x' }))}`],
    [
      'code not a string',
      `#code=${compressToEncodedURIComponent(JSON.stringify({ l: 'go', c: 1 }))}`,
    ],
    [
      'stdin not a string',
      `#code=${compressToEncodedURIComponent(JSON.stringify({ l: 'go', c: '', i: [] }))}`,
    ],
  ])('rejects a %s payload', (_, hash) => {
    expect(parseShareHash(hash)).toEqual({ kind: 'invalid' });
  });
});
