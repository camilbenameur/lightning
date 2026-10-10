/**
 * Best-effort localStorage persistence. Storage may be unavailable (privacy mode,
 * disabled cookies) or full; the app must keep working in that case, so every
 * access is guarded and failures degrade to "nothing stored".
 */

import { isRecord } from './guards';

const PREFIX = 'lightning:';

export function readItem(key: string): string | null {
  try {
    return window.localStorage.getItem(PREFIX + key);
  } catch {
    return null;
  }
}

export function writeItem(key: string, value: string): void {
  try {
    window.localStorage.setItem(PREFIX + key, value);
  } catch {
    // Quota exceeded or storage disabled: persistence is optional.
  }
}

/** The editor contents remembered for one language. */
export interface Draft {
  code: string;
  stdin: string;
}

const draftKey = (language: string) => `draft:${language}`;

export function loadDraft(language: string): Draft | null {
  const raw = readItem(draftKey(language));
  if (raw === null) return null;
  try {
    const value: unknown = JSON.parse(raw);
    if (!isRecord(value)) return null;
    const { code, stdin } = value;
    return typeof code === 'string'
      ? { code, stdin: typeof stdin === 'string' ? stdin : '' }
      : null;
  } catch {
    return null;
  }
}

export function saveDraft(language: string, draft: Draft): void {
  writeItem(draftKey(language), JSON.stringify(draft));
}
