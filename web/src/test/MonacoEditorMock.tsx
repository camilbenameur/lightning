import type { CodeEditorProps } from '../components/editor/EditorPanel';

/** Stands in for Monaco (which needs a real browser) in component tests. */
export default function MonacoEditorMock({ value, onChange, monacoLanguage }: CodeEditorProps) {
  return (
    <textarea
      aria-label="Code"
      data-language={monacoLanguage}
      value={value}
      onChange={(event) => onChange(event.target.value)}
    />
  );
}
