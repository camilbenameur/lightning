# Lightning web

The browser UI of Lightning: a React 19 + TypeScript app built with Vite, with a
self-hosted Monaco editor. Monaco is bundled (no CDN, compatible with a strict
`script-src 'self'` CSP), trimmed to the editor core plus syntax highlighting for the
supported languages, and lazy-loaded in its own chunk behind a skeleton.

## Development

Requires Node.js 24 (see `.nvmrc`).

```sh
npm install
npm run dev        # http://localhost:5173, proxies /api to http://localhost:8080
```

Start the API on port 8080 (see the repository README) to actually run code, or point
the proxy elsewhere with `LIGHTNING_API_URL=http://host:port npm run dev`.

| Script              | What it does                                          |
| ------------------- | ----------------------------------------------------- |
| `npm run build`     | Type-check and build the production bundle to `dist/` |
| `npm run preview`   | Serve `dist/` locally (also proxies `/api`)           |
| `npm run lint`      | ESLint (type-aware, React hooks rules)                |
| `npm run typecheck` | `tsc -b`                                              |
| `npm test`          | Vitest + Testing Library (jsdom, API mocked)          |
| `npm run format`    | Prettier (`format:check` verifies, as CI does)        |

## Configuration

`VITE_API_BASE_URL` (build time) sets the API origin, e.g. `https://api.example.com`.
It defaults to empty, i.e. the same origin, which is how the production image serves
it (Caddy serves the app and proxies `/api`). A cross-origin API must allow this origin
(`LIGHTNING_CORS_ORIGINS`).

## Dependencies

Monaco vendors its own copy of DOMPurify. `vite.config.ts` aliases that copy to the
`dompurify` npm package, which `overrides` in `package.json` also imposes on Monaco, so
DOMPurify security fixes ship without waiting for a Monaco release. When upgrading Monaco,
check that the bundle still contains a single DOMPurify, at the npm version.

## Layout

```
src/
  api/          typed API client, NDJSON stream reader, error mapping
  components/   UI (editor/ holds the lazy Monaco setup)
  examples/     code examples per language
  hooks/        state: runner, workspace (per-language drafts), theme, …
  lib/          share links, localStorage, formatting
  styles/       design tokens (light/dark) and shared controls
```
