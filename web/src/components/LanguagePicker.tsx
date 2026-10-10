import type { Language } from '../api/types';
import type { Resource } from '../hooks/useResource';
import { AlertIcon } from './icons';

interface LanguagePickerProps {
  languages: Resource<Language[]>;
  value: string;
  onChange: (language: string) => void;
  onRetry: () => void;
}

export function LanguagePicker({ languages, value, onChange, onRetry }: LanguagePickerProps) {
  if (languages.status === 'error') {
    return (
      <div className="language-error" role="alert">
        <AlertIcon />
        <span>Languages unavailable</span>
        <button type="button" className="button button--ghost" onClick={onRetry}>
          Retry
        </button>
      </div>
    );
  }

  if (languages.status === 'loading') {
    return (
      <select className="select language-picker" aria-label="Language" disabled>
        <option>Loading languages…</option>
      </select>
    );
  }

  return (
    <select
      className="select language-picker"
      aria-label="Language"
      value={value}
      onChange={(event) => onChange(event.target.value)}
    >
      {languages.data.map((language) => (
        <option key={language.id} value={language.id}>
          {language.name} {language.version}
        </option>
      ))}
    </select>
  );
}
