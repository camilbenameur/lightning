import { useCallback, useEffect, useReducer, useRef } from 'react';
import { runStream } from '../api/client';
import { ApiError, CONNECTION_LOST } from '../api/errors';
import type { OutputStream, RunRequest, RunResult } from '../api/types';

export type RunStatus = 'idle' | 'starting' | 'running' | 'finished' | 'stopped' | 'failed';

export interface OutputChunk {
  stream: OutputStream;
  text: string;
}

export interface RunState {
  status: RunStatus;
  /** Output in arrival order; consecutive chunks of the same stream are merged. */
  output: OutputChunk[];
  result: RunResult | null;
  error: Error | null;
  /** Epoch ms when the current run was requested, while it is in flight. */
  startedAt: number | null;
}

type RunAction =
  | { type: 'start'; at: number }
  | { type: 'started' }
  | { type: 'output'; stream: OutputStream; text: string }
  | { type: 'result'; result: RunResult }
  | { type: 'fail'; error: Error }
  | { type: 'stop' }
  | { type: 'clear' };

export const initialRunState: RunState = {
  status: 'idle',
  output: [],
  result: null,
  error: null,
  startedAt: null,
};

export const isRunActive = (status: RunStatus) => status === 'starting' || status === 'running';

function append(output: OutputChunk[], stream: OutputStream, text: string): OutputChunk[] {
  const last = output.at(-1);
  if (last?.stream === stream) return [...output.slice(0, -1), { stream, text: last.text + text }];
  return [...output, { stream, text }];
}

export function runReducer(state: RunState, action: RunAction): RunState {
  switch (action.type) {
    case 'start':
      return { ...initialRunState, status: 'starting', startedAt: action.at };
    case 'started':
      return state.status === 'starting' ? { ...state, status: 'running' } : state;
    case 'output':
      return {
        ...state,
        status: 'running',
        output: append(state.output, action.stream, action.text),
      };
    case 'result':
      return { ...state, status: 'finished', result: action.result, startedAt: null };
    case 'fail':
      return { ...state, status: 'failed', error: action.error, startedAt: null };
    case 'stop':
      return isRunActive(state.status) ? { ...state, status: 'stopped', startedAt: null } : state;
    case 'clear':
      return isRunActive(state.status) ? { ...state, output: [] } : initialRunState;
  }
}

const toError = (error: unknown) => (error instanceof Error ? error : new Error(String(error)));

/** Runs programs through the streaming endpoint; starting a new run or stopping aborts the previous one. */
export function useRunner() {
  const [state, dispatch] = useReducer(runReducer, initialRunState);
  const controllerRef = useRef<AbortController | null>(null);

  const stop = useCallback(() => {
    if (!controllerRef.current) return;
    controllerRef.current.abort();
    controllerRef.current = null;
    dispatch({ type: 'stop' });
  }, []);

  const run = useCallback(async (request: RunRequest) => {
    controllerRef.current?.abort();
    const controller = new AbortController();
    controllerRef.current = controller;
    // Events of a superseded or stopped run must not touch the state of the current one.
    const emit = (action: RunAction) => {
      if (controllerRef.current === controller) dispatch(action);
    };

    dispatch({ type: 'start', at: Date.now() });
    try {
      for await (const event of runStream(request, controller.signal)) {
        switch (event.type) {
          case 'started':
            emit({ type: 'started' });
            break;
          case 'stdout':
          case 'stderr':
            emit({ type: 'output', stream: event.type, text: event.data });
            break;
          case 'result': {
            const { exitCode, durationMs, timedOut, oomKilled, truncated } = event;
            emit({
              type: 'result',
              result: { exitCode, durationMs, timedOut, oomKilled, truncated },
            });
            return;
          }
          case 'error':
            throw new ApiError({ status: 200, ...event.error });
        }
      }
      throw new ApiError({ status: 0, code: CONNECTION_LOST, message: 'Stream ended early' });
    } catch (error) {
      if (!controller.signal.aborted) emit({ type: 'fail', error: toError(error) });
    } finally {
      if (controllerRef.current === controller) controllerRef.current = null;
    }
  }, []);

  const clear = useCallback(() => dispatch({ type: 'clear' }), []);

  // Abort any in-flight run when the component unmounts.
  useEffect(() => stop, [stop]);

  return { state, run, stop, clear };
}
