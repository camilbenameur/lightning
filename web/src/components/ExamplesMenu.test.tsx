import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { ExamplesMenu } from './ExamplesMenu';

const examples = [
  { id: 'hello', title: 'Hello, world!', code: 'hello' },
  { id: 'stdin', title: 'Read from stdin', code: 'stdin', stdin: 'Ada' },
  { id: 'fizzbuzz', title: 'FizzBuzz', code: 'fizz' },
];

describe('ExamplesMenu', () => {
  it('is operable with the keyboard', async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(<ExamplesMenu examples={examples} onSelect={onSelect} />);
    const button = screen.getByRole('button', { name: 'Examples' });

    button.focus();
    await user.keyboard('{ArrowDown}');
    expect(button).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByRole('menuitem', { name: 'Hello, world!' })).toHaveFocus();

    await user.keyboard('{ArrowUp}');
    expect(screen.getByRole('menuitem', { name: 'FizzBuzz' })).toHaveFocus();
    await user.keyboard('{Home}{ArrowDown}');
    expect(screen.getByRole('menuitem', { name: 'Read from stdin' })).toHaveFocus();

    await user.keyboard('{Enter}');
    expect(onSelect).toHaveBeenCalledWith(examples[1]);
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
    expect(button).toHaveFocus();
  });

  it('closes on Escape and on outside clicks', async () => {
    const user = userEvent.setup();
    render(<ExamplesMenu examples={examples} onSelect={vi.fn()} />);
    const button = screen.getByRole('button', { name: 'Examples' });

    await user.click(button);
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
    expect(button).toHaveFocus();

    await user.click(button);
    expect(screen.getByRole('menu')).toBeInTheDocument();
    await user.click(document.body);
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });

  it('is disabled when the language has no examples', () => {
    render(<ExamplesMenu examples={[]} onSelect={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Examples' })).toBeDisabled();
  });
});
