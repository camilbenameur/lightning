import { useEffect, useState } from 'react';

const TICK_MS = 100;

/** Milliseconds elapsed since `since` (epoch ms), ticking while it is non-null. */
export function useElapsed(since: number | null): number {
  const [now, setNow] = useState(Date.now);

  useEffect(() => {
    if (since === null) return;
    const timer = setInterval(() => setNow(Date.now()), TICK_MS);
    return () => clearInterval(timer);
  }, [since]);

  return since === null ? 0 : Math.max(0, now - since);
}
