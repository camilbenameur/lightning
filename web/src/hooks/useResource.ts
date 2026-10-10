import { useCallback, useEffect, useState } from 'react';

export type Resource<T> =
  { status: 'loading' } | { status: 'ready'; data: T } | { status: 'error'; error: unknown };

/** Loads data once on mount (abortable) and exposes a retry. `load` must be a stable function. */
export function useResource<T>(load: (signal: AbortSignal) => Promise<T>) {
  const [attempt, setAttempt] = useState(0);
  const [resource, setResource] = useState<Resource<T>>({ status: 'loading' });

  useEffect(() => {
    const controller = new AbortController();
    load(controller.signal).then(
      (data) => setResource({ status: 'ready', data }),
      (error: unknown) => {
        if (!controller.signal.aborted) setResource({ status: 'error', error });
      },
    );
    return () => controller.abort();
  }, [load, attempt]);

  const retry = useCallback(() => {
    setResource({ status: 'loading' });
    setAttempt((n) => n + 1);
  }, []);

  return [resource, retry] as const;
}
