import { useEffect, useLayoutEffect, useRef } from 'react';
import type { CodeEditorProps } from './EditorPanel';
import { monaco } from './monaco';

const THEMES = { light: 'lightning-light', dark: 'lightning-dark' } as const;

const OPTIONS: monaco.editor.IStandaloneEditorConstructionOptions = {
  ariaLabel: 'Code editor',
  automaticLayout: true,
  fontFamily:
    "ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, 'Cascadia Mono', 'Liberation Mono', monospace",
  fontSize: 13,
  lineHeight: 20,
  padding: { top: 10, bottom: 10 },
  minimap: { enabled: false },
  scrollBeyondLastLine: false,
  scrollbar: { verticalScrollbarSize: 10, horizontalScrollbarSize: 10, useShadows: false },
  overviewRulerLanes: 0,
  overviewRulerBorder: false,
  hideCursorInOverviewRuler: true,
  renderLineHighlight: 'line',
  fixedOverflowWidgets: true,
  tabSize: 4,
  wordBasedSuggestions: 'currentDocument',
};

/** One model per language, so switching back and forth keeps each language's undo history. */
function modelFor(language: string, monacoLanguage: string, initialValue: string) {
  const uri = monaco.Uri.from({ scheme: 'inmemory', path: `/${language}` });
  let model = monaco.editor.getModel(uri);
  if (!model) {
    model = monaco.editor.createModel(initialValue, monacoLanguage, uri);
    // Text without line breaks gets the platform's EOL, i.e. CRLF on Windows, but
    // programs must receive LF (Bash rejects `then\r`).
    model.setEOL(monaco.editor.EndOfLineSequence.LF);
  }
  if (model.getLanguageId() !== monacoLanguage)
    monaco.editor.setModelLanguage(model, monacoLanguage);
  return model;
}

const textOf = (model: monaco.editor.ITextModel) =>
  model.getValue(monaco.editor.EndOfLinePreference.LF);

/** The Monaco editor, loaded lazily by EditorPanel. */
export default function MonacoEditor({
  language,
  monacoLanguage,
  value,
  onChange,
  onRun,
  theme,
}: CodeEditorProps) {
  const hostRef = useRef<HTMLDivElement>(null);
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);
  const handlers = useRef({ onChange, onRun });

  useEffect(() => {
    handlers.current = { onChange, onRun };
  });

  // Themes are global in Monaco; set before the editor is created to avoid a flash.
  useLayoutEffect(() => {
    monaco.editor.setTheme(THEMES[theme]);
  }, [theme]);

  useLayoutEffect(() => {
    if (!hostRef.current) return;
    const editor = monaco.editor.create(hostRef.current, { ...OPTIONS, model: null });
    editor.addAction({
      id: 'lightning.run',
      label: 'Run Code',
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter],
      run: () => handlers.current.onRun(),
    });
    const subscription = editor.onDidChangeModelContent(() => {
      const model = editor.getModel();
      if (model) handlers.current.onChange(textOf(model));
    });
    editorRef.current = editor;
    return () => {
      subscription.dispose();
      editor.dispose();
      editorRef.current = null;
    };
  }, []);

  // Sync the model with the props; outside changes (examples, reset, share links) are undoable edits.
  useLayoutEffect(() => {
    const editor = editorRef.current;
    if (!editor) return;
    const model = modelFor(language, monacoLanguage, value);
    if (editor.getModel() !== model) editor.setModel(model);
    if (textOf(model) !== value) {
      editor.pushUndoStop();
      editor.executeEdits('lightning', [{ range: model.getFullModelRange(), text: value }]);
      editor.pushUndoStop();
    }
  }, [language, monacoLanguage, value]);

  return <div ref={hostRef} className="monaco-host" />;
}
