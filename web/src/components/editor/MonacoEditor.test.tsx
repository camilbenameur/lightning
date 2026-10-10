import { render } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import type { CodeEditorProps } from './EditorPanel';

// Node's, which the browser tsconfig does not type.
declare const process: object;

// Monaco defaults new text to CRLF except on Linux and macOS, reading the platform once
// when it is imported (from `process.platform` under Node): load it as on Windows.
const realPlatform = vi.hoisted(() => {
  const descriptor = Object.getOwnPropertyDescriptor(process, 'platform');
  Object.defineProperty(process, 'platform', { value: 'win32' });
  return descriptor;
});

import MonacoEditor from './MonacoEditor';
import { monaco } from './monaco';

if (realPlatform) Object.defineProperty(process, 'platform', realPlatform);

beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe = vi.fn();
      unobserve = vi.fn();
      disconnect = vi.fn();
    },
  );
  // jsdom has no canvas; Monaco copes without one.
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null);
});

afterEach(() => {
  for (const model of monaco.editor.getModels()) model.dispose();
});

function renderEditor(props: Partial<CodeEditorProps>) {
  const onChange = vi.fn<(value: string) => void>();
  const element = (next: Partial<CodeEditorProps>) => (
    <MonacoEditor
      language="bash"
      monacoLanguage="shell"
      value=""
      onRun={() => undefined}
      theme="dark"
      onChange={onChange}
      {...next}
    />
  );
  const view = render(element(props));
  const [editor] = monaco.editor.getEditors();
  if (!editor) throw new Error('Monaco did not create an editor');
  return {
    editor,
    onChange,
    update: (next: Partial<CodeEditorProps>) => view.rerender(element(next)),
  };
}

it('reports LF line endings even where Monaco defaults to CRLF', () => {
  const { editor, onChange } = renderEditor({ value: '' });

  editor.trigger('test', 'type', { text: 'if true; then\necho hi\nfi' });

  expect(onChange).toHaveBeenLastCalledWith('if true; then\necho hi\nfi');
});

it('converts CRLF code it is given (e.g. an old draft) to LF', () => {
  const { onChange } = renderEditor({ value: 'echo a\r\necho b\r\n' });

  expect(onChange).toHaveBeenLastCalledWith('echo a\necho b\n');
});

it('applies outside changes, such as examples, as undoable edits', () => {
  const { editor, onChange, update } = renderEditor({ value: 'echo draft\n' });

  update({ value: 'echo example\n' });
  expect(editor.getValue()).toBe('echo example\n');

  editor.trigger('test', 'undo', null);
  expect(onChange).toHaveBeenLastCalledWith('echo draft\n');
});

it('keeps one model, with its undo history, per language', () => {
  const python = { language: 'python', monacoLanguage: 'python' };
  const { editor, onChange, update } = renderEditor({ ...python, value: 'print(1)\n' });
  editor.trigger('test', 'type', { text: '# edited\n' });
  const edited = editor.getValue();

  update({ language: 'go', monacoLanguage: 'go', value: 'package main\n' });
  expect(editor.getModel()?.getLanguageId()).toBe('go');
  update({ ...python, value: edited });
  editor.trigger('test', 'undo', null);

  expect(onChange).toHaveBeenLastCalledWith('print(1)\n');
});
