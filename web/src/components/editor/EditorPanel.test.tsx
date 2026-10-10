import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import { EditorPanel } from './EditorPanel';

// Simulates the lazy Monaco chunk failing to download.
vi.mock('./MonacoEditor', () => {
  throw new Error('Failed to fetch dynamically imported module');
});

it('falls back to a plain text area when the editor cannot be loaded', async () => {
  vi.spyOn(console, 'error').mockImplementation(() => undefined);
  const onChange = vi.fn();
  render(
    <EditorPanel
      language="python"
      monacoLanguage="python"
      fileName="main.py"
      value="print(1)"
      onChange={onChange}
      onRun={() => undefined}
      onReset={() => undefined}
      theme="dark"
    />,
  );

  expect(await screen.findByRole('alert')).toHaveTextContent(
    'The code editor could not be loaded.',
  );
  const textarea = screen.getByRole('textbox', { name: 'Code' });
  expect(textarea).toHaveValue('print(1)');

  await userEvent.type(textarea, '!');
  expect(onChange).toHaveBeenLastCalledWith('print(1)!');
});
