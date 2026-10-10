/** Narrows untrusted data (JSON from the network or storage) to a plain object. */
export const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null;
