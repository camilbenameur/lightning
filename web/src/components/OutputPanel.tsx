import { useEffect, useLayoutEffect, useRef, type ReactNode } from 'react';
import type { OutputChunk, RunStatus } from '../hooks/useRunner';
import { MOD_KEY } from '../lib/platform';
import { IconButton } from './IconButton';
import { AlertIcon, ClearIcon, CopyIcon } from './icons';
import { Panel } from './Panel';
import './OutputPanel.css';

/** Distance from the bottom (px) within which new output keeps the view pinned to the end. */
const STICKY_THRESHOLD = 24;

const PLACEHOLDERS: Record<RunStatus, ReactNode> = {
  idle: (
    <>
      Run your code to see its output here. <kbd className="kbd output__kbd">{MOD_KEY} ↵</kbd>
    </>
  ),
  starting: 'Starting sandbox…',
  running: 'Waiting for output…',
  finished: 'The program produced no output.',
  stopped: 'Stopped.',
  failed: null,
};

interface OutputPanelProps {
  status: RunStatus;
  output: OutputChunk[];
  errorMessage: string | null;
  onCopy: () => void;
  onClear: () => void;
}

export function OutputPanel({ status, output, errorMessage, onCopy, onClear }: OutputPanelProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const pinnedRef = useRef(true);
  const isEmpty = output.length === 0;
  const placeholder = isEmpty && !errorMessage ? PLACEHOLDERS[status] : null;

  // On phones the output sits below the editor: bring it into view when a run starts.
  useEffect(() => {
    if (status !== 'starting') return;
    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    scrollRef.current?.scrollIntoView({
      block: 'nearest',
      behavior: reduceMotion ? 'auto' : 'smooth',
    });
  }, [status]);

  // Follow new output unless the user scrolled up to read something.
  useLayoutEffect(() => {
    const element = scrollRef.current;
    if (!element) return;
    if (isEmpty) pinnedRef.current = true;
    if (pinnedRef.current) element.scrollTop = element.scrollHeight;
  }, [output, errorMessage, isEmpty]);

  const onScroll = () => {
    const element = scrollRef.current;
    if (!element) return;
    const distance = element.scrollHeight - element.scrollTop - element.clientHeight;
    pinnedRef.current = distance < STICKY_THRESHOLD;
  };

  return (
    <Panel
      title="Output"
      actions={
        <>
          <IconButton label="Copy output" onClick={onCopy} disabled={isEmpty}>
            <CopyIcon />
          </IconButton>
          <IconButton
            label="Clear output"
            onClick={onClear}
            disabled={status === 'idle' && isEmpty && !errorMessage}
          >
            <ClearIcon />
          </IconButton>
        </>
      }
    >
      <div
        ref={scrollRef}
        className="output"
        role="region"
        aria-label="Program output"
        tabIndex={0}
        onScroll={onScroll}
      >
        {placeholder && <p className="output__placeholder">{placeholder}</p>}
        {!isEmpty && (
          <pre className="output__text">
            {output.map((chunk, index) => (
              <span key={index} className={`output__${chunk.stream}`}>
                {chunk.text}
              </span>
            ))}
          </pre>
        )}
        {errorMessage && (
          <p className="output__error">
            <AlertIcon />
            {errorMessage}
          </p>
        )}
      </div>
    </Panel>
  );
}
