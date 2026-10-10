import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { OutputChunk } from '../hooks/useRunner';
import { OutputPanel } from './OutputPanel';

const noop = () => undefined;

function renderPanel(output: OutputChunk[]) {
  const view = render(
    <OutputPanel
      status="running"
      output={output}
      errorMessage={null}
      onCopy={noop}
      onClear={noop}
    />,
  );
  const region = screen.getByRole('region', { name: 'Program output' });

  // jsdom has no layout: give the region a fixed geometry and a writable scroll position.
  let scrollTop = 0;
  Object.defineProperties(region, {
    scrollHeight: { configurable: true, get: () => 1000 },
    clientHeight: { configurable: true, get: () => 200 },
    scrollTop: {
      configurable: true,
      get: () => scrollTop,
      set: (value: number) => {
        scrollTop = value;
      },
    },
  });

  const update = (next: OutputChunk[]) =>
    view.rerender(
      <OutputPanel
        status="running"
        output={next}
        errorMessage={null}
        onCopy={noop}
        onClear={noop}
      />,
    );
  return { region, update };
}

describe('OutputPanel', () => {
  it('follows new output until the user scrolls up, and resumes at the bottom', () => {
    const { region, update } = renderPanel([{ stream: 'stdout', text: 'a\n' }]);

    update([{ stream: 'stdout', text: 'a\nb\n' }]);
    expect(region.scrollTop).toBe(1000);

    region.scrollTop = 100;
    fireEvent.scroll(region);
    update([{ stream: 'stdout', text: 'a\nb\nc\n' }]);
    expect(region.scrollTop).toBe(100);

    region.scrollTop = 790;
    fireEvent.scroll(region);
    update([{ stream: 'stdout', text: 'a\nb\nc\nd\n' }]);
    expect(region.scrollTop).toBe(1000);
  });

  it('renders stderr distinctly from stdout', () => {
    renderPanel([
      { stream: 'stdout', text: 'fine' },
      { stream: 'stderr', text: 'broken' },
    ]);

    expect(screen.getByText('fine')).toHaveClass('output__stdout');
    expect(screen.getByText('broken')).toHaveClass('output__stderr');
  });
});
