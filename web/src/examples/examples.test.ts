import { describe, expect, it } from 'vitest';

import { EXAMPLES, getDefaultCode, type LanguageId } from './index.ts';
import { NAMES, source } from './shared.ts';

const LANGUAGES: LanguageId[] = [
  'python',
  'javascript',
  'typescript',
  'go',
  'rust',
  'c',
  'cpp',
  'java',
  'ruby',
  'php',
  'bash',
];

const MAX_LINES = 40;

describe('EXAMPLES', () => {
  it('covers exactly the supported languages', () => {
    expect(Object.keys(EXAMPLES).sort()).toEqual([...LANGUAGES].sort());
  });

  describe.each(LANGUAGES)('%s', (lang) => {
    const examples = EXAMPLES[lang];

    it('has at least four examples with unique ids and titles', () => {
      expect(examples.length).toBeGreaterThanOrEqual(4);
      expect(new Set(examples.map((e) => e.id)).size).toBe(examples.length);
      expect(new Set(examples.map((e) => e.title)).size).toBe(examples.length);
    });

    it('starts with the hello-world template', () => {
      const [hello] = examples;
      expect(hello?.id).toBe('hello');
      expect(hello?.title).toBe('Hello, world!');
      expect(hello?.code).toContain('Hello, world!');
      expect(hello?.stdin).toBeUndefined();
    });

    it('has a stdin example that ships sample input', () => {
      const withStdin = examples.filter((e) => e.stdin !== undefined);
      expect(withStdin.map((e) => e.id)).toEqual(['stdin']);
      expect(withStdin.map((e) => e.stdin)).toEqual([NAMES]);
    });

    it.each(examples.map((e) => [e.id, e] as const))('%s is well formed', (_id, example) => {
      const { code } = example;
      const lines = code.slice(0, -1).split('\n');

      expect(example.title.trim()).not.toBe('');
      expect(code.trim()).not.toBe('');
      expect(code.endsWith('\n') && !code.endsWith('\n\n')).toBe(true);
      expect(code).not.toContain('\r');
      expect(lines.length).toBeLessThanOrEqual(MAX_LINES);
      expect(lines.filter((line) => /\s$/.test(line))).toEqual([]);
      // Go is gofmt-formatted (tab indents); every other language indents with spaces.
      const indent = lang === 'go' ? /^ / : /^\t/;
      expect(lines.filter((line) => indent.test(line))).toEqual([]);
    });
  });
});

describe('getDefaultCode', () => {
  it.each(LANGUAGES)('returns the first example for %s', (lang) => {
    expect(getDefaultCode(lang)).toBe(EXAMPLES[lang][0]?.code);
  });

  it.each(['', 'cobol', 'Python', 'constructor', 'toString', '__proto__'])(
    'falls back to an empty string for %j',
    (lang) => {
      expect(getDefaultCode(lang)).toBe('');
    },
  );
});

describe('source', () => {
  it('drops one leading newline and keeps the rest verbatim', () => {
    expect(source`
a
  b
`).toBe('a\n  b\n');
  });

  it('keeps backslash escapes uninterpreted', () => {
    expect(source`printf("a\tb\n");`).toBe('printf("a\\tb\\n");');
  });

  it('unescapes backticks and dollar-brace sequences', () => {
    expect(source`echo "\${HOME}" \`date\``).toBe('echo "${HOME}" `date`');
  });
});
