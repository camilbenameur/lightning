/// <reference types="vitest/config" />
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// Dev/preview servers forward API calls, so the app always talks to its own origin.
const apiProxy = { '/api': process.env.LIGHTNING_API_URL ?? 'http://localhost:8080' };

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: [
      // Monaco vendors DOMPurify 3.4.15 (GHSA-p98j-92pf-mc4p, GHSA-6688-9rhm-gjv2): builds use the
      // patched npm package instead, kept current by Dependabot. The dev server's pre-bundled
      // dependencies still use Monaco's copy (they are never deployed).
      { find: /^\.\/dompurify\/dompurify\.js$/, replacement: 'dompurify' },
    ],
  },
  server: { proxy: apiProxy },
  preview: { proxy: apiProxy },
  build: {
    target: 'es2022',
    // Monaco is lazy-loaded as its own ~3 MB chunk; everything else stays small.
    chunkSizeWarningLimit: 4096,
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    restoreMocks: true,
    unstubGlobals: true,
    // user-event driven jsdom tests can be slow on busy CI runners.
    testTimeout: 15_000,
    // Transforming Monaco (thousands of modules, for MonacoEditor.test.tsx) dominates cold runs.
    fsModuleCache: true,
  },
});
