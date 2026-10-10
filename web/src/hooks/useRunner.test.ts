import { describe, expect, it } from 'vitest';
import { initialRunState, runReducer, type RunState } from './useRunner';

const result = { exitCode: 0, durationMs: 5, timedOut: false, oomKilled: false, truncated: false };

function reduce(...actions: Parameters<typeof runReducer>[1][]): RunState {
  return actions.reduce(runReducer, initialRunState);
}

describe('runReducer', () => {
  it('walks through a successful run', () => {
    const starting = reduce({ type: 'start', at: 1000 });
    expect(starting).toMatchObject({ status: 'starting', startedAt: 1000 });

    const running = runReducer(starting, { type: 'started' });
    expect(running.status).toBe('running');

    const finished = runReducer(running, { type: 'result', result });
    expect(finished).toMatchObject({ status: 'finished', result, startedAt: null });
  });

  it('merges consecutive chunks of the same stream and keeps arrival order', () => {
    const state = reduce(
      { type: 'start', at: 0 },
      { type: 'output', stream: 'stdout', text: 'a' },
      { type: 'output', stream: 'stdout', text: 'b' },
      { type: 'output', stream: 'stderr', text: 'c' },
      { type: 'output', stream: 'stdout', text: 'd' },
    );
    expect(state.output).toEqual([
      { stream: 'stdout', text: 'ab' },
      { stream: 'stderr', text: 'c' },
      { stream: 'stdout', text: 'd' },
    ]);
  });

  it('starts from a clean slate on each run', () => {
    const state = reduce(
      { type: 'start', at: 0 },
      { type: 'output', stream: 'stdout', text: 'old' },
      { type: 'fail', error: new Error('x') },
      { type: 'start', at: 1 },
    );
    expect(state).toEqual({ ...initialRunState, status: 'starting', startedAt: 1 });
  });

  it('only stops active runs', () => {
    expect(reduce({ type: 'start', at: 0 }, { type: 'stop' }).status).toBe('stopped');
    expect(reduce({ type: 'stop' }).status).toBe('idle');
  });

  it('clears output but keeps an active run going', () => {
    const running = reduce(
      { type: 'start', at: 0 },
      { type: 'output', stream: 'stdout', text: 'x' },
      { type: 'clear' },
    );
    expect(running).toMatchObject({ status: 'running', output: [] });

    const done = runReducer(runReducer(running, { type: 'result', result }), { type: 'clear' });
    expect(done).toEqual(initialRunState);
  });
});
