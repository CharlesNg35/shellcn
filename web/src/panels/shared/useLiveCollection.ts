import type { Ref } from "vue";
import type { ResourceEvent, Row } from "@/types/projection";

interface LiveCollectionOptions {
  rows: Ref<Row[]>;
  keyOf: (row: Row) => string;
  // prepend puts new entries first (newest-first lists like a timeline).
  prepend?: boolean;
  max?: number;
}

// A live list has no natural end, so the cap is on by default: forgetting the
// option must not buy unbounded growth.
const DEFAULT_MAX = 500;

// useLiveCollection applies StreamResource deltas to a flat list by key: upsert on
// add/modify, drop on delete. It is the simple-list primitive; TablePanel keeps its
// own merge because it must also batch, preserve selection, and defer under
// server-side sort/filter/pagination.
export function useLiveCollection(opts: LiveCollectionOptions): {
  apply: (ev: ResourceEvent) => void;
} {
  function apply(ev: ResourceEvent): void {
    const row = ev.resource as Row | undefined;
    const key = ev.ref?.uid ?? (row ? opts.keyOf(row) : undefined);
    if (!key) return;
    // Mutate in place so Vue patches the touched row instead of rediffing a
    // freshly allocated array on every event.
    const rows = opts.rows.value;
    const index = rows.findIndex((r) => opts.keyOf(r) === key);
    if (ev.type === "deleted" || !row) {
      if (index >= 0) rows.splice(index, 1);
      return;
    }
    if (index >= 0) rows.splice(index, 1);
    if (opts.prepend) rows.unshift(row);
    else rows.push(row);
    const max = opts.max ?? DEFAULT_MAX;
    if (rows.length > max) rows.splice(max);
  }
  return { apply };
}
