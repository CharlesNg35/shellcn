import type { Ref } from "vue";
import type { ResourceEvent, Row } from "@/types/projection";

interface LiveCollectionOptions {
  rows: Ref<Row[]>;
  keyOf: (row: Row) => string;
  // prepend puts new entries first (newest-first lists like a timeline).
  prepend?: boolean;
  max?: number;
}

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
    const next = opts.rows.value.filter((r) => opts.keyOf(r) !== key);
    if (ev.type !== "deleted" && row) {
      if (opts.prepend) next.unshift(row);
      else next.push(row);
    }
    opts.rows.value = opts.max ? next.slice(0, opts.max) : next;
  }
  return { apply };
}
