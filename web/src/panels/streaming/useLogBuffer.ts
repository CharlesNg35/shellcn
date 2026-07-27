import { onScopeDispose, ref, type Ref } from "vue";

export interface LogLine {
  id: number;
  text: string;
}

// Log-ish streams deliver one line per frame. Frames land in a plain array and
// flush together once the current turn drains, so a burst costs one render
// rather than one per frame. Lines carry a stable id so rolling the cap patches
// the two ends of the list instead of rediffing every row behind index keys.
export function useLogBuffer(
  max: number,
  onFlush?: () => void,
): {
  lines: Ref<LogLine[]>;
  append: (text: string) => void;
  clear: () => void;
} {
  const lines = ref<LogLine[]>([]);
  let pending: LogLine[] = [];
  let scheduled = false;
  let seq = 0;

  function flush(): void {
    scheduled = false;
    if (!pending.length) return;
    const batch = pending;
    pending = [];
    if (batch.length > max) batch.splice(0, batch.length - max);
    // Publish a new array reference: PrimeVue's VirtualScroller only recomputes
    // its scroll extent when the items length changes between watcher runs, and
    // an in-place mutation hands it the same array as both old and new value.
    const next = lines.value.concat(batch);
    lines.value = next.length > max ? next.slice(next.length - max) : next;
    onFlush?.();
  }

  function append(text: string): void {
    pending.push({ id: seq++, text });
    if (scheduled) return;
    scheduled = true;
    queueMicrotask(flush);
  }

  function clear(): void {
    pending = [];
    lines.value = [];
  }

  onScopeDispose(() => {
    pending = [];
  });

  return { lines, append, clear };
}
