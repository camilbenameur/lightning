import { MOD_KEY } from '../lib/platform';
import { PlayIcon, StopIcon } from './icons';

interface RunButtonProps {
  running: boolean;
  onRun: () => void;
  onStop: () => void;
}

/** Run, or Stop while a program is running. Both states keep the same element so focus stays put. */
export function RunButton({ running, onRun, onStop }: RunButtonProps) {
  return (
    <button
      type="button"
      className={`button run-button ${running ? 'button--danger' : 'button--primary'}`}
      onClick={running ? onStop : onRun}
      aria-keyshortcuts={running ? 'Escape' : 'Control+Enter Meta+Enter'}
    >
      {running ? <StopIcon /> : <PlayIcon />}
      {running ? 'Stop' : 'Run'}
      <kbd className="kbd run-button__shortcut" aria-hidden="true">
        {running ? 'Esc' : `${MOD_KEY} ↵`}
      </kbd>
    </button>
  );
}
