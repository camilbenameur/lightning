/**
 * Self-hosted Monaco: only the editor core, the features a playground needs and
 * syntax highlighting for the supported languages. The heavy language services
 * (TypeScript/JSON/CSS/HTML workers) are deliberately left out.
 */
import * as monaco from 'monaco-editor/editor/editor.api';
import EditorWorker from 'monaco-editor/editor/editor.worker?worker';

import 'monaco-editor/features/codicon/register';
import 'monaco-editor/features/codeEditor/register';
import 'monaco-editor/features/bracketMatching/register';
import 'monaco-editor/features/clipboard/register';
import 'monaco-editor/features/comment/register';
import 'monaco-editor/features/contextmenu/register';
import 'monaco-editor/features/cursorUndo/register';
import 'monaco-editor/features/dnd/register';
import 'monaco-editor/features/find/register';
import 'monaco-editor/features/folding/register';
import 'monaco-editor/features/gotoLine/register';
import 'monaco-editor/features/hover/register';
import 'monaco-editor/features/indentation/register';
import 'monaco-editor/features/iPadShowKeyboard/register';
import 'monaco-editor/features/lineSelection/register';
import 'monaco-editor/features/linesOperations/register';
import 'monaco-editor/features/links/register';
import 'monaco-editor/features/multicursor/register';
import 'monaco-editor/features/quickCommand/register';
import 'monaco-editor/features/smartSelect/register';
import 'monaco-editor/features/snippet/register';
import 'monaco-editor/features/suggest/register';
// The suggest widget itself (Ctrl+Space, word-based suggestions): the full build only gets it
// through the inline-completions feature, which a playground does not need.
import 'monaco-editor/editor/contrib/suggest/browser/suggestController';
import 'monaco-editor/features/unicodeHighlighter/register';
import 'monaco-editor/features/wordHighlighter/register';
import 'monaco-editor/features/wordOperations/register';

// Grammars are registered eagerly but downloaded on first use.
import 'monaco-editor/languages/definitions/cpp/register';
import 'monaco-editor/languages/definitions/go/register';
import 'monaco-editor/languages/definitions/java/register';
import 'monaco-editor/languages/definitions/javascript/register';
import 'monaco-editor/languages/definitions/php/register';
import 'monaco-editor/languages/definitions/python/register';
import 'monaco-editor/languages/definitions/ruby/register';
import 'monaco-editor/languages/definitions/rust/register';
import 'monaco-editor/languages/definitions/shell/register';
import 'monaco-editor/languages/definitions/typescript/register';

// Word-based suggestions, links, etc. run in the generic editor worker (bundled, same origin).
self.MonacoEnvironment = { getWorker: () => new EditorWorker() };

monaco.editor.defineTheme('lightning-dark', {
  base: 'vs-dark',
  inherit: true,
  rules: [],
  colors: {
    'editor.background': '#111113',
    'editor.lineHighlightBackground': '#18181b',
    'editor.lineHighlightBorder': '#00000000',
    'editorLineNumber.foreground': '#52525b',
    'editorLineNumber.activeForeground': '#a1a1aa',
    'editorCursor.foreground': '#fde047',
    'editorIndentGuide.background1': '#232327',
    'editorWidget.background': '#18181b',
    'editorWidget.border': '#34343a',
    focusBorder: '#fde04766',
  },
});

monaco.editor.defineTheme('lightning-light', {
  base: 'vs',
  inherit: true,
  rules: [],
  colors: {
    'editor.background': '#ffffff',
    'editor.lineHighlightBackground': '#f6f6f7',
    'editor.lineHighlightBorder': '#00000000',
    'editorLineNumber.foreground': '#a1a1aa',
    'editorLineNumber.activeForeground': '#52525b',
    'editorCursor.foreground': '#18181b',
    'editorIndentGuide.background1': '#e4e4e7',
    focusBorder: '#a1620766',
  },
});

export { monaco };
