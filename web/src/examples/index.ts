import { bash } from './bash.ts';
import { c } from './c.ts';
import { cpp } from './cpp.ts';
import { go } from './go.ts';
import { java } from './java.ts';
import { javascript } from './javascript.ts';
import { php } from './php.ts';
import { python } from './python.ts';
import { ruby } from './ruby.ts';
import { rust } from './rust.ts';
import type { Example, LanguageId } from './types.ts';
import { typescript } from './typescript.ts';

export type { Example, LanguageId } from './types.ts';

/** Curated programs per language. The first entry is the default "Hello, world!" template. */
export const EXAMPLES: Record<LanguageId, Example[]> = {
  python,
  javascript,
  typescript,
  go,
  rust,
  c,
  cpp,
  java,
  ruby,
  php,
  bash,
};

function isLanguageId(lang: string): lang is LanguageId {
  return Object.hasOwn(EXAMPLES, lang);
}

/** The starter program for a language id, or an empty string for unknown ids. */
export function getDefaultCode(lang: string): string {
  return isLanguageId(lang) ? (EXAMPLES[lang][0]?.code ?? '') : '';
}
