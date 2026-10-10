import { EXAMPLES, type Example, type LanguageId } from '../examples';
import type { Draft } from './storage';

/** Examples for a language id coming from the server; unknown ids have none. */
export function examplesFor(language: string): Example[] {
  return Object.hasOwn(EXAMPLES, language) ? EXAMPLES[language as LanguageId] : [];
}

/** The starting point of a language: its first example ("Hello, world!"). */
export function templateFor(language: string): Draft {
  const template = examplesFor(language)[0];
  return { code: template?.code ?? '', stdin: template?.stdin ?? '' };
}
