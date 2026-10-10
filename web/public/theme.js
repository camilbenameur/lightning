// Applies the color theme before first paint, avoiding a flash of the wrong theme.
// A same-origin file rather than an inline script so that it passes the CSP (script-src 'self').
// Keep in sync with src/hooks/useTheme.ts.
{
  const system = matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
  let saved = null;
  try {
    saved = localStorage.getItem('lightning:theme');
  } catch {
    // Storage blocked: follow the system preference.
  }
  const theme = saved === 'light' || saved === 'dark' ? saved : system;
  document.documentElement.dataset.theme = theme;
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute('content', theme === 'light' ? '#ffffff' : '#0b0b0c');
}
