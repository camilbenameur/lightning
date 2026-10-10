/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Origin of the API, e.g. "https://api.example.com". Empty (default) = same origin. */
  readonly VITE_API_BASE_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
