import type { ThemePreference } from '../hooks/useTheme';
import { IconButton } from './IconButton';
import { MonitorIcon, MoonIcon, SunIcon } from './icons';

const NEXT: Record<ThemePreference, ThemePreference> = {
  system: 'light',
  light: 'dark',
  dark: 'system',
};

const ICONS = { system: MonitorIcon, light: SunIcon, dark: MoonIcon };

interface ThemeToggleProps {
  preference: ThemePreference;
  onChange: (preference: ThemePreference) => void;
}

/** Cycles system → light → dark. */
export function ThemeToggle({ preference, onChange }: ThemeToggleProps) {
  const Icon = ICONS[preference];
  return (
    <IconButton
      label={`Theme: ${preference} (switch to ${NEXT[preference]})`}
      onClick={() => onChange(NEXT[preference])}
    >
      <Icon />
    </IconButton>
  );
}
