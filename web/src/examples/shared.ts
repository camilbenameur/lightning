/**
 * Tag for program listings. Unlike a plain template literal it keeps backslashes
 * verbatim (`\n` stays two characters), so a listing reads exactly as it appears in
 * the editor. Write \` for a backtick and \${ for a literal "${" (shell and JS
 * listings); the tag takes no substitutions. One leading newline is dropped so the
 * opening backtick can sit on its own line.
 */
export function source(strings: TemplateStringsArray): string {
  return strings.raw
    .join('')
    .replace(/^\n/, '')
    .replace(/\\(`|\$\{)/g, '$1');
}

/** Sample input shared by the "Read from stdin" examples. */
export const NAMES = 'Ada\nGrace\nLinus\n';
