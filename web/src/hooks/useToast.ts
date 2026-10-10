import { useCallback, useEffect, useState } from 'react';

export interface Toast {
  id: number;
  message: string;
}

const TOAST_DURATION_MS = 3000;
let nextId = 0;

/** A single transient notification; showing a new one replaces the current one. */
export function useToast() {
  const [toast, setToast] = useState<Toast | null>(null);

  const showToast = useCallback((message: string) => {
    nextId += 1;
    setToast({ id: nextId, message });
  }, []);

  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(null), TOAST_DURATION_MS);
    return () => clearTimeout(timer);
  }, [toast]);

  return { toast, showToast };
}
