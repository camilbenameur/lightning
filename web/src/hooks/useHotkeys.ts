import { useEffect, useEffectEvent } from 'react';

interface Hotkeys {
  /** Ctrl/Cmd+Enter. Inside Monaco the editor handles it itself (see MonacoEditor). */
  onRun: () => void;
  /** Escape. */
  onStop: () => void;
}

/** Global keyboard shortcuts. Keys already handled by a focused widget are ignored. */
export function useHotkeys({ onRun, onStop }: Hotkeys) {
  const onKeyDown = useEffectEvent((event: KeyboardEvent) => {
    if (event.defaultPrevented || event.isComposing) return;
    if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      onRun();
    } else if (event.key === 'Escape') {
      onStop();
    }
  });

  useEffect(() => {
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);
}
