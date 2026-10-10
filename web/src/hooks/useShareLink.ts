import { useEffect, useEffectEvent, useRef } from 'react';
import type { Language } from '../api/types';
import { parseShareHash, type Snippet } from '../lib/share';
import type { Resource } from './useResource';

interface ShareLinkHandlers {
  /** Open the snippet: its language is enabled, or the language list failed to load. */
  onOpen: (snippet: Snippet) => void;
  /** The link is for a language this server does not run. */
  onUnavailable: () => void;
  /** The URL carries a `#code=` fragment that cannot be decoded. */
  onInvalid: () => void;
}

/** Drops the fragment so a reload shows the (auto-saved) draft rather than the original link. */
function clearHash() {
  const { pathname, search } = window.location;
  window.history.replaceState(window.history.state, '', pathname + search);
}

/**
 * Handles the share link the page was opened with and links pasted later in the same tab.
 * A link waits for the server's language list, so one for a language the server does not
 * run is reported (and never saved as a draft) instead of silently dropped.
 */
export function useShareLink(
  languages: Resource<Language[]>,
  { onOpen, onUnavailable, onInvalid }: ShareLinkHandlers,
) {
  const pending = useRef<Snippet | null>(null);

  const deliver = useEffectEvent(() => {
    const snippet = pending.current;
    if (!snippet || languages.status === 'loading') return;
    pending.current = null;
    const unavailable =
      languages.status === 'ready' && !languages.data.some(({ id }) => id === snippet.language);
    if (unavailable) onUnavailable();
    else onOpen(snippet);
  });

  const receive = useEffectEvent(() => {
    const shared = parseShareHash(window.location.hash);
    if (shared.kind === 'none') return;
    clearHash();
    if (shared.kind === 'invalid') {
      onInvalid();
      return;
    }
    pending.current = shared.snippet;
    deliver();
  });

  useEffect(() => {
    receive();
    window.addEventListener('hashchange', receive);
    return () => window.removeEventListener('hashchange', receive);
  }, []);

  useEffect(() => deliver(), [languages]);
}
