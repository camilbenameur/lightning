import type { Language } from '../api/types';
import type { Example } from '../examples';
import type { Resource } from '../hooks/useResource';
import type { ThemePreference } from '../hooks/useTheme';
import { ExamplesMenu } from './ExamplesMenu';
import { BoltIcon, GitHubIcon, LinkIcon } from './icons';
import { LanguagePicker } from './LanguagePicker';
import { RunButton } from './RunButton';
import { ThemeToggle } from './ThemeToggle';
import './Header.css';

const REPOSITORY_URL = 'https://github.com/camilbenameur/lightning';

interface HeaderProps {
  languages: Resource<Language[]>;
  language: string;
  onLanguageChange: (language: string) => void;
  onRetryLanguages: () => void;
  examples: Example[];
  onExample: (example: Example) => void;
  running: boolean;
  onRun: () => void;
  onStop: () => void;
  onShare: () => void;
  themePreference: ThemePreference;
  onThemeChange: (preference: ThemePreference) => void;
}

export function Header(props: HeaderProps) {
  return (
    <header className="header">
      <h1 className="brand">
        <BoltIcon className="brand__logo" width={18} height={18} />
        Lightning
      </h1>

      <div className="header__toolbar">
        <LanguagePicker
          languages={props.languages}
          value={props.language}
          onChange={props.onLanguageChange}
          onRetry={props.onRetryLanguages}
        />
        <ExamplesMenu examples={props.examples} onSelect={props.onExample} />
        <RunButton running={props.running} onRun={props.onRun} onStop={props.onStop} />
      </div>

      <div className="header__actions">
        <button
          type="button"
          className="button button--ghost"
          onClick={props.onShare}
          aria-label="Share"
          title="Copy a link to this code"
        >
          <LinkIcon />
          <span className="header__label">Share</span>
        </button>
        <ThemeToggle preference={props.themePreference} onChange={props.onThemeChange} />
        <a
          className="icon-button"
          href={REPOSITORY_URL}
          target="_blank"
          rel="noopener noreferrer"
          aria-label="Source code on GitHub"
          title="Source code on GitHub"
        >
          <GitHubIcon />
        </a>
      </div>
    </header>
  );
}
