// Logout wipes every session-scoped store via these cleanups. Keyed by id so a
// fresh Pinia (tests) replaces rather than accumulates.
const cleanups = new Map<string, () => void>();

export function registerSessionCleanup(id: string, cleanup: () => void): void {
  cleanups.set(id, cleanup);
}

export function resetSession(): void {
  for (const cleanup of cleanups.values()) {
    try {
      cleanup();
    } catch {
      /* one store's failure must not block the rest */
    }
  }
}
