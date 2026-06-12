type ConnectionCleanup = (connectionId: string) => void;

const cleanups = new Set<ConnectionCleanup>();

export function registerConnectionCleanup(
  cleanup: ConnectionCleanup,
): () => void {
  cleanups.add(cleanup);
  return () => cleanups.delete(cleanup);
}

export function cleanupConnection(connectionId: string): void {
  for (const cleanup of cleanups) cleanup(connectionId);
}
