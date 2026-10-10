import { compressToEncodedURIComponent, decompressFromEncodedURIComponent } from 'lz-string';
import { isRecord } from './guards';

/** What a share link carries. Links are stateless: everything lives in the URL fragment. */
export interface Snippet {
  language: string;
  code: string;
  stdin: string;
}

export type SharedHash =
  { kind: 'none' } | { kind: 'invalid' } | { kind: 'snippet'; snippet: Snippet };

const HASH_PREFIX = '#code=';

/** Encodes a snippet as `lz(JSON {l, c, i})`, the format of `#code=…` share links (SPEC §6). */
export function encodeSnippet({ language, code, stdin }: Snippet): string {
  return compressToEncodedURIComponent(JSON.stringify({ l: language, c: code, i: stdin }));
}

export function buildShareUrl(snippet: Snippet, currentUrl: string): string {
  const url = new URL(currentUrl);
  url.hash = `code=${encodeSnippet(snippet)}`;
  return url.toString();
}

/** Reads a `location.hash`; anything that is not a well-formed share link is reported as invalid. */
export function parseShareHash(hash: string): SharedHash {
  if (!hash.startsWith(HASH_PREFIX)) return { kind: 'none' };
  try {
    const json = decompressFromEncodedURIComponent(hash.slice(HASH_PREFIX.length));
    const value: unknown = json ? JSON.parse(json) : null;
    if (!isRecord(value)) return { kind: 'invalid' };

    const { l, c, i = '' } = value;
    if (typeof l !== 'string' || l === '' || typeof c !== 'string' || typeof i !== 'string') {
      return { kind: 'invalid' };
    }
    return { kind: 'snippet', snippet: { language: l, code: c, stdin: i } };
  } catch {
    return { kind: 'invalid' };
  }
}
