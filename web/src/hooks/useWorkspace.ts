import { useCallback, useEffect, useState } from 'react';
import type { Example } from '../examples';
import type { Snippet } from '../lib/share';
import { loadDraft, readItem, saveDraft, writeItem } from '../lib/storage';
import { templateFor } from '../lib/templates';

export type Workspace = Snippet;

const LANGUAGE_KEY = 'language';
const DEFAULT_LANGUAGE = 'python';

/** A language's last code and stdin, or its template when it has never been edited. */
function restore(language: string): Workspace {
  return { language, ...(loadDraft(language) ?? templateFor(language)) };
}

/**
 * The code, stdin and language being edited. Every change is saved as the
 * draft of the current language, so switching languages (or reloading) resumes
 * where the user left off.
 */
export function useWorkspace() {
  const [workspace, setWorkspace] = useState<Workspace>(() =>
    restore(readItem(LANGUAGE_KEY) ?? DEFAULT_LANGUAGE),
  );

  useEffect(() => {
    const { language, code, stdin } = workspace;
    saveDraft(language, { code, stdin });
    writeItem(LANGUAGE_KEY, language);
  }, [workspace]);

  const setCode = useCallback((code: string) => setWorkspace((w) => ({ ...w, code })), []);
  const setStdin = useCallback((stdin: string) => setWorkspace((w) => ({ ...w, stdin })), []);
  const selectLanguage = useCallback((language: string) => setWorkspace(restore(language)), []);
  const open = useCallback((snippet: Snippet) => setWorkspace(snippet), []);
  const loadExample = useCallback(
    ({ code, stdin = '' }: Example) => setWorkspace((w) => ({ ...w, code, stdin })),
    [],
  );
  const resetToTemplate = useCallback(
    () => setWorkspace((w) => ({ ...w, ...templateFor(w.language) })),
    [],
  );

  return { workspace, setCode, setStdin, selectLanguage, open, loadExample, resetToTemplate };
}
