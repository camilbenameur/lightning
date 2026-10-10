import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { App } from './App';
import { EXAMPLES } from './examples';
import { encodeSnippet, parseShareHash } from './lib/share';
import {
  errorResponse,
  INFO,
  jsonResponse,
  LANGUAGES,
  liveNdjsonResponse,
  ndjsonResponse,
  routes,
} from './test/http';

vi.mock('./components/editor/MonacoEditor', () => import('./test/MonacoEditorMock'));

type Routes = Parameters<typeof routes>[0];

const [pythonTemplate] = EXAMPLES.python;
const [goTemplate] = EXAMPLES.go;

function setup(extra: Routes = {}) {
  const fetchMock = vi.fn(
    routes({
      'GET /languages': () => jsonResponse({ languages: LANGUAGES }),
      'GET /info': () => jsonResponse(INFO),
      ...extra,
    }),
  );
  vi.stubGlobal('fetch', fetchMock);
  const user = userEvent.setup();
  render(<App />);
  return { user, fetchMock };
}

/** Resolves once the language list has loaded. */
const languagesLoaded = () => screen.findByRole('option', { name: 'Python 3.14' });
const codeEditor = () => screen.findByRole('textbox', { name: 'Code' });
const stdinInput = () => screen.getByRole('textbox', { name: 'Standard input' });
const output = () => screen.getByRole('region', { name: 'Program output' });
const languageSelect = () => screen.getByRole('combobox', { name: 'Language' });

describe('App', () => {
  it('lists the server languages and the sandbox limits', async () => {
    setup();

    await languagesLoaded();
    const options = within(languageSelect()).getAllByRole('option');
    expect(options.map((option) => option.textContent)).toEqual([
      'Python 3.14',
      'Go 1.27',
      'Bash 5.2',
    ]);
    expect(await screen.findByText('no network · 256 MB · 10 s')).toBeInTheDocument();
    expect(await codeEditor()).toHaveValue(pythonTemplate?.code);
    expect(screen.getByText('main.py')).toBeInTheDocument();
  });

  it('offers a retry when the languages cannot be loaded', async () => {
    let available = false;
    const { user } = setup({
      'GET /languages': () =>
        available
          ? jsonResponse({ languages: LANGUAGES })
          : errorResponse(503, 'sandbox_unavailable'),
    });

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Languages unavailable');

    available = true;
    await user.click(within(alert).getByRole('button', { name: 'Retry' }));
    expect(await languagesLoaded()).toBeInTheDocument();
  });

  it('streams stdout and stderr into the output panel in arrival order', async () => {
    const { user, fetchMock } = setup({
      'POST /run/stream': () =>
        ndjsonResponse([
          { type: 'started' },
          { type: 'stdout', data: 'Hello\n' },
          { type: 'stderr', data: 'warning\n' },
          { type: 'stdout', data: 'done\n' },
          {
            type: 'result',
            exitCode: 1,
            durationMs: 1234,
            timedOut: false,
            oomKilled: false,
            truncated: true,
          },
        ]),
    });
    await languagesLoaded();

    await user.click(screen.getByRole('button', { name: 'Run' }));

    await screen.findByText('Exit 1');
    expect(output()).toHaveTextContent('Hello warning done');
    expect(within(output()).getByText('warning')).toHaveClass('output__stderr');
    expect(screen.getByText('1.2 s')).toBeInTheDocument();
    expect(screen.getByText('Output truncated')).toBeInTheDocument();
    expect(screen.getByText(/Exited with code 1 in 1\.2 s\. Output truncated/)).toBeInTheDocument();

    const init = fetchMock.mock.calls.find(([url]) => url === '/api/v1/run/stream')?.[1];
    expect(JSON.parse(init?.body as string)).toEqual({
      language: 'python',
      code: pythonTemplate?.code,
      stdin: pythonTemplate?.stdin ?? '',
    });
  });

  it('flags timeouts and out-of-memory kills', async () => {
    const { user } = setup({
      'POST /run/stream': () =>
        ndjsonResponse([
          {
            type: 'result',
            exitCode: 137,
            durationMs: 10000,
            timedOut: true,
            oomKilled: true,
            truncated: false,
          },
        ]),
    });
    await languagesLoaded();

    await user.click(screen.getByRole('button', { name: 'Run' }));

    expect(await screen.findByText('Timed out')).toBeInTheDocument();
    expect(screen.getByText('Memory limit exceeded')).toBeInTheDocument();
    expect(screen.getByText('The program produced no output.')).toBeInTheDocument();
  });

  it('runs with Ctrl+Enter', async () => {
    const { user, fetchMock } = setup({
      'POST /run/stream': () => ndjsonResponse([{ type: 'result', exitCode: 0, durationMs: 3 }]),
    });
    await languagesLoaded();

    await user.keyboard('{Control>}{Enter}{/Control}');

    expect(await screen.findByText('Exit 0')).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/run/stream', expect.anything());
  });

  it.each([
    [
      'the Stop button',
      (user: ReturnType<typeof userEvent.setup>) =>
        user.click(screen.getByRole('button', { name: 'Stop' })),
    ],
    ['Escape', (user: ReturnType<typeof userEvent.setup>) => user.keyboard('{Escape}')],
  ])('stops a running program with %s', async (_, stop) => {
    let signal: AbortSignal | null | undefined;
    let live: ReturnType<typeof liveNdjsonResponse> | undefined;
    const { user } = setup({
      'POST /run/stream': (init) => {
        signal = init?.signal;
        live = liveNdjsonResponse(signal);
        return live.response;
      },
    });
    await languagesLoaded();

    await user.click(screen.getByRole('button', { name: 'Run' }));
    await waitFor(() => expect(live).toBeDefined());
    act(() => {
      live?.push({ type: 'started' });
      live?.push({ type: 'stdout', data: 'tick\n' });
    });
    expect(await within(output()).findByText('tick')).toBeInTheDocument();

    await stop(user);

    expect(signal?.aborted).toBe(true);
    expect(screen.getByText('Stopped')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Run' })).toBeInTheDocument();
    expect(output()).toHaveTextContent('tick');
  });

  it.each<[string, Routes[string], string]>([
    [
      'rate limiting',
      () => errorResponse(429, 'rate_limited', 'slow down', { 'Retry-After': '12' }),
      'You’re running code too quickly. Try again in 12 seconds.',
    ],
    [
      'a busy server',
      () => errorResponse(503, 'server_busy'),
      'All sandboxes are busy right now. Please try again in a few seconds.',
    ],
    [
      'an unavailable sandbox',
      () => errorResponse(503, 'sandbox_unavailable'),
      'The sandbox for this language is unavailable right now. Please try again later.',
    ],
    [
      'an oversized payload',
      () => errorResponse(413, 'payload_too_large'),
      'Your code or input is too large (limit: 64 KB).',
    ],
    [
      'a network failure',
      () => Promise.reject(new TypeError('Failed to fetch')),
      'Can’t reach the Lightning server. Check your connection and try again.',
    ],
    [
      'an error event mid-stream',
      () =>
        ndjsonResponse([
          { type: 'started' },
          { type: 'error', error: { code: 'internal_error', message: 'Sandbox crashed' } },
        ]),
      'Sandbox crashed',
    ],
  ])('explains %s', async (_, handler, message) => {
    const { user } = setup({ 'POST /run/stream': handler });
    await languagesLoaded();
    await screen.findByText('no network · 256 MB · 10 s');

    await user.click(screen.getByRole('button', { name: 'Run' }));

    expect(await within(output()).findByText(message)).toBeInTheDocument();
    expect(screen.getByText('Failed')).toBeInTheDocument();
  });

  it('ignores malformed stream lines', async () => {
    const { user } = setup({
      'POST /run/stream': () =>
        new Response(
          [
            '{"type":"stdout","data":"ok\\n"}',
            'garbage',
            '{"type":"stdout"}',
            '{"type":"result","exitCode":0}',
            '',
          ].join('\n'),
        ),
    });
    await languagesLoaded();

    await user.click(screen.getByRole('button', { name: 'Run' }));

    expect(await screen.findByText('Exit 0')).toBeInTheDocument();
    expect(output()).toHaveTextContent(/^ok$/);
  });

  it('remembers the code of each language', async () => {
    const { user } = setup();
    await languagesLoaded();

    const editor = await codeEditor();
    await user.clear(editor);
    await user.type(editor, 'print("mine")');

    await user.selectOptions(languageSelect(), 'go');
    expect(await codeEditor()).toHaveValue(goTemplate?.code);
    expect(screen.getByText('main.go')).toBeInTheDocument();

    await user.selectOptions(languageSelect(), 'python');
    expect(await codeEditor()).toHaveValue('print("mine")');
  });

  it('resets the code to the template', async () => {
    const { user } = setup();
    const editor = await codeEditor();
    await user.type(editor, '# edited');

    await user.click(screen.getByRole('button', { name: 'Reset to template' }));

    expect(editor).toHaveValue(pythonTemplate?.code);
  });

  it('loads an example with its stdin', async () => {
    const { user } = setup();
    await languagesLoaded();
    const example = EXAMPLES.python.at(-1);

    await user.click(screen.getByRole('button', { name: 'Examples' }));
    const menu = screen.getByRole('menu', { name: 'Examples' });
    expect(within(menu).getAllByRole('menuitem')).toHaveLength(EXAMPLES.python.length);
    await user.click(within(menu).getByRole('menuitem', { name: example?.title }));

    expect(await codeEditor()).toHaveValue(example?.code);
    expect(stdinInput()).toHaveValue(example?.stdin ?? '');
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });

  it('falls back to the first language when the saved one is not enabled', async () => {
    window.localStorage.setItem('lightning:language', 'rust');
    setup();
    await languagesLoaded();

    await waitFor(() => expect(languageSelect()).toHaveValue('python'));
    expect(await codeEditor()).toHaveValue(pythonTemplate?.code);
  });

  it('copies a share link that reproduces the workspace', async () => {
    const { user } = setup();
    await languagesLoaded();
    await user.type(stdinInput(), 'Ada');

    await user.click(screen.getByRole('button', { name: 'Share' }));

    expect(await screen.findByText('Share link copied to clipboard.')).toBeInTheDocument();
    const link = new URL(await navigator.clipboard.readText());
    expect(parseShareHash(link.hash)).toEqual({
      kind: 'snippet',
      snippet: {
        language: 'python',
        code: pythonTemplate?.code,
        stdin: `${pythonTemplate?.stdin ?? ''}Ada`,
      },
    });
  });

  it('opens a share link, which takes precedence over saved drafts', async () => {
    window.localStorage.setItem('lightning:draft:go', JSON.stringify({ code: 'draft', stdin: '' }));
    const snippet = { language: 'go', code: 'package main // shared', stdin: '42' };
    window.history.replaceState(null, '', `/#code=${encodeSnippet(snippet)}`);

    setup();
    await languagesLoaded();

    expect(await codeEditor()).toHaveValue(snippet.code);
    expect(stdinInput()).toHaveValue('42');
    expect(languageSelect()).toHaveValue('go');
    expect(window.location.hash).toBe('');
  });

  it('opens a share link pasted into the address bar', async () => {
    setup();
    await languagesLoaded();
    const snippet = { language: 'bash', code: 'echo shared', stdin: '' };

    act(() => {
      window.history.replaceState(null, '', `/#code=${encodeSnippet(snippet)}`);
      window.dispatchEvent(new HashChangeEvent('hashchange'));
    });

    expect(await codeEditor()).toHaveValue(snippet.code);
    expect(languageSelect()).toHaveValue('bash');
    expect(window.location.hash).toBe('');
  });

  it('reports a share link for a language the server does not run', async () => {
    window.localStorage.setItem(
      'lightning:draft:python',
      JSON.stringify({ code: 'mine', stdin: '' }),
    );
    const snippet = { language: 'cobol', code: 'DISPLAY "HI".', stdin: '' };
    window.history.replaceState(null, '', `/#code=${encodeSnippet(snippet)}`);

    setup();

    expect(
      await screen.findByText('This share link’s language isn’t available on this server.'),
    ).toBeInTheDocument();
    expect(languageSelect()).toHaveValue('python');
    expect(await codeEditor()).toHaveValue('mine');
    expect(window.localStorage.getItem('lightning:draft:cobol')).toBeNull();
  });

  it('still opens a share link when the languages cannot be loaded', async () => {
    const snippet = { language: 'go', code: 'package main // shared', stdin: '' };
    window.history.replaceState(null, '', `/#code=${encodeSnippet(snippet)}`);

    setup({ 'GET /languages': () => errorResponse(503, 'sandbox_unavailable') });

    expect(await screen.findByDisplayValue(snippet.code)).toBeInTheDocument();
  });

  it('reports an invalid share link and keeps the default workspace', async () => {
    window.history.replaceState(null, '', '/#code=definitely-not-valid');

    setup();

    expect(
      await screen.findByText('This share link is invalid or incomplete.'),
    ).toBeInTheDocument();
    expect(await codeEditor()).toHaveValue(pythonTemplate?.code);
  });

  it('cycles and remembers the theme', async () => {
    const { user } = setup();

    await user.click(screen.getByRole('button', { name: 'Theme: system (switch to light)' }));
    expect(document.documentElement.dataset.theme).toBe('light');
    await user.click(screen.getByRole('button', { name: 'Theme: light (switch to dark)' }));

    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(window.localStorage.getItem('lightning:theme')).toBe('dark');
  });
});
