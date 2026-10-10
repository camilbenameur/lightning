import { useCallback } from 'react';
import { fetchInfo, fetchLanguages } from './api/client';
import { describeError } from './api/errors';
import { EditorPanel } from './components/editor/EditorPanel';
import { Header } from './components/Header';
import { OutputPanel } from './components/OutputPanel';
import { StatusBar } from './components/StatusBar';
import { StdinPanel } from './components/StdinPanel';
import { Toast } from './components/Toast';
import { WorkspaceLayout } from './components/WorkspaceLayout';
import { useHotkeys } from './hooks/useHotkeys';
import { useResource } from './hooks/useResource';
import { isRunActive, useRunner } from './hooks/useRunner';
import { useShareLink } from './hooks/useShareLink';
import { useTheme } from './hooks/useTheme';
import { useToast } from './hooks/useToast';
import { useWorkspace } from './hooks/useWorkspace';
import { buildShareUrl } from './lib/share';
import { examplesFor } from './lib/templates';
import './App.css';

/** Highlighting to use until the server's language list (which carries Monaco ids) has loaded. */
const MONACO_FALLBACK: Record<string, string> = { bash: 'shell' };

export function App() {
  const { preference, theme, setPreference } = useTheme();
  const { toast, showToast } = useToast();
  const [languages, retryLanguages] = useResource(fetchLanguages);
  const [info] = useResource(fetchInfo);
  const { state: run, run: startRun, stop, clear } = useRunner();

  const { workspace, setCode, setStdin, selectLanguage, open, loadExample, resetToTemplate } =
    useWorkspace();
  // A share link takes precedence over the saved drafts.
  useShareLink(languages, {
    onOpen: open,
    onUnavailable: () => showToast('This share link’s language isn’t available on this server.'),
    onInvalid: () => showToast('This share link is invalid or incomplete.'),
  });

  const available = languages.status === 'ready' ? languages.data : [];
  const language = available.find((l) => l.id === workspace.language);
  const [firstAvailable] = available;
  if (!language && firstAvailable) {
    // The saved language is not enabled on this server.
    selectLanguage(firstAvailable.id);
  }

  const limits = info.status === 'ready' ? info.data.limits : null;
  const running = isRunActive(run.status);
  const errorMessage = run.error ? describeError(run.error, limits) : null;

  const runCode = useCallback(() => {
    void startRun({ language: workspace.language, code: workspace.code, stdin: workspace.stdin });
  }, [startRun, workspace]);

  useHotkeys({ onRun: runCode, onStop: stop });

  const share = async () => {
    const url = buildShareUrl(workspace, window.location.href);
    try {
      await navigator.clipboard.writeText(url);
      showToast('Share link copied to clipboard.');
    } catch {
      window.history.replaceState(window.history.state, '', url);
      showToast('Couldn’t copy the link: it is in the address bar instead.');
    }
  };

  const copyOutput = async () => {
    try {
      await navigator.clipboard.writeText(run.output.map((chunk) => chunk.text).join(''));
      showToast('Output copied to clipboard.');
    } catch {
      showToast('Couldn’t access the clipboard.');
    }
  };

  const reset = useCallback(() => {
    resetToTemplate();
    showToast('Code reset to the template.');
  }, [resetToTemplate, showToast]);

  return (
    <div className="app">
      <Header
        languages={languages}
        language={workspace.language}
        onLanguageChange={selectLanguage}
        onRetryLanguages={retryLanguages}
        examples={examplesFor(workspace.language)}
        onExample={loadExample}
        running={running}
        onRun={runCode}
        onStop={stop}
        onShare={() => void share()}
        themePreference={preference}
        onThemeChange={setPreference}
      />
      <main className="app__main">
        <WorkspaceLayout
          editor={
            <EditorPanel
              language={workspace.language}
              monacoLanguage={
                language?.monaco ?? MONACO_FALLBACK[workspace.language] ?? workspace.language
              }
              fileName={language?.fileName}
              value={workspace.code}
              onChange={setCode}
              onRun={runCode}
              onReset={reset}
              theme={theme}
            />
          }
          stdin={<StdinPanel value={workspace.stdin} onChange={setStdin} />}
          output={
            <OutputPanel
              status={run.status}
              output={run.output}
              errorMessage={errorMessage}
              onCopy={() => void copyOutput()}
              onClear={clear}
            />
          }
        />
      </main>
      <StatusBar run={run} errorMessage={errorMessage} limits={limits} />
      <Toast toast={toast} />
    </div>
  );
}
