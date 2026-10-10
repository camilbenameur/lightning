import { lazy, memo, Suspense } from 'react';
import type { Theme } from '../../hooks/useTheme';
import { IconButton } from '../IconButton';
import { ResetIcon } from '../icons';
import { Panel } from '../Panel';
import { EditorBoundary } from './EditorBoundary';
import { EditorSkeleton } from './EditorSkeleton';
import './EditorPanel.css';

// Monaco is several MB: it is only downloaded once the shell has rendered.
const MonacoEditor = lazy(() => import('./MonacoEditor'));

export interface CodeEditorProps {
  /** Lightning language id; one editor model is kept per language. */
  language: string;
  /** Monaco language id used for highlighting. */
  monacoLanguage: string;
  value: string;
  onChange: (value: string) => void;
  onRun: () => void;
  theme: Theme;
}

interface EditorPanelProps extends CodeEditorProps {
  fileName: string | undefined;
  onReset: () => void;
}

export const EditorPanel = memo(function EditorPanel({
  fileName,
  onReset,
  ...editor
}: EditorPanelProps) {
  return (
    <Panel
      title={
        <>
          Editor
          {fileName && <span className="editor-panel__file">{fileName}</span>}
        </>
      }
      actions={
        <IconButton label="Reset to template" onClick={onReset}>
          <ResetIcon />
        </IconButton>
      }
    >
      <EditorBoundary value={editor.value} onChange={editor.onChange}>
        <Suspense fallback={<EditorSkeleton />}>
          <MonacoEditor {...editor} />
        </Suspense>
      </EditorBoundary>
    </Panel>
  );
});
