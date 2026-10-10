import { useCallback, useEffect, useState } from 'react';
import { readItem, writeItem } from '../lib/storage';
import { useMediaQuery } from './useMediaQuery';

export type ThemePreference = 'system' | 'light' | 'dark';
export type Theme = 'light' | 'dark';

/** Must match public/theme.js, which applies the saved theme before first paint. */
const STORAGE_KEY = 'theme';
const THEME_COLORS: Record<Theme, string> = { light: '#ffffff', dark: '#0b0b0c' };

function readPreference(): ThemePreference {
  const stored = readItem(STORAGE_KEY);
  return stored === 'light' || stored === 'dark' ? stored : 'system';
}

/** Light/dark/system theme, persisted, applied to <html data-theme> and the browser chrome. */
export function useTheme() {
  const [preference, setPreferenceState] = useState(readPreference);
  const systemIsDark = useMediaQuery('(prefers-color-scheme: dark)');
  const theme: Theme = preference === 'system' ? (systemIsDark ? 'dark' : 'light') : preference;

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document
      .querySelector('meta[name="theme-color"]')
      ?.setAttribute('content', THEME_COLORS[theme]);
  }, [theme]);

  const setPreference = useCallback((next: ThemePreference) => {
    setPreferenceState(next);
    writeItem(STORAGE_KEY, next);
  }, []);

  return { preference, theme, setPreference };
}
