import type { Limits, RunResult } from '../api/types';
import { useElapsed } from '../hooks/useElapsed';
import type { RunState } from '../hooks/useRunner';
import { formatDuration, formatLimits } from '../lib/format';
import { MOD_KEY } from '../lib/platform';
import './StatusBar.css';

type Tone = 'neutral' | 'success' | 'danger' | 'warning';

function Badge({ tone, children }: { tone: Tone; children: string }) {
  return <span className={`badge badge--${tone}`}>{children}</span>;
}

function resultFlags({ timedOut, oomKilled, truncated }: RunResult): string[] {
  return [
    timedOut && 'Timed out',
    oomKilled && 'Memory limit exceeded',
    truncated && 'Output truncated',
  ].filter((flag) => flag !== false);
}

/** What screen readers hear; deliberately free of the ticking timer. */
function announcement({ status, result }: RunState, errorMessage: string | null): string {
  switch (status) {
    case 'idle':
      return '';
    case 'starting':
      return 'Starting…';
    case 'running':
      return 'Running…';
    case 'stopped':
      return 'Stopped.';
    case 'failed':
      return `Run failed. ${errorMessage ?? ''}`;
    case 'finished':
      if (!result) return 'Finished.';
      return [
        `Exited with code ${result.exitCode} in ${formatDuration(result.durationMs)}.`,
        ...resultFlags(result),
      ].join(' ');
  }
}

function RunStatusView({ state }: { state: RunState }) {
  const elapsed = useElapsed(state.startedAt);
  const { status, result } = state;

  switch (status) {
    case 'idle':
      return <span className="status status--idle">Ready</span>;
    case 'starting':
    case 'running':
      return (
        <span className={`status status--${status}`}>
          {status === 'starting' ? 'Starting' : 'Running'}
          <span className="status__timer">{(elapsed / 1000).toFixed(1)} s</span>
        </span>
      );
    case 'stopped':
      return <span className="status status--stopped">Stopped</span>;
    case 'failed':
      return <span className="status status--failed">Failed</span>;
    case 'finished':
      if (!result) return null;
      return (
        <span className="status status--finished">
          <Badge tone={result.exitCode === 0 ? 'success' : 'danger'}>
            {`Exit ${result.exitCode}`}
          </Badge>
          <span className="status__timer">{formatDuration(result.durationMs)}</span>
          {resultFlags(result).map((flag) => (
            <Badge key={flag} tone="warning">
              {flag}
            </Badge>
          ))}
        </span>
      );
  }
}

interface StatusBarProps {
  run: RunState;
  errorMessage: string | null;
  limits: Limits | null;
}

export function StatusBar({ run, errorMessage, limits }: StatusBarProps) {
  return (
    <footer className="statusbar">
      <RunStatusView state={run} />
      <p className="visually-hidden" role="status">
        {announcement(run, errorMessage)}
      </p>
      <div className="statusbar__meta">
        {limits && (
          <span className="statusbar__limits" title="Sandbox limits per run">
            {formatLimits(limits)}
          </span>
        )}
        <span className="statusbar__hint" aria-hidden="true">
          <kbd className="kbd">{MOD_KEY} ↵</kbd> run <kbd className="kbd">Esc</kbd> stop
        </span>
      </div>
    </footer>
  );
}
