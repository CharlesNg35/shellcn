import { onScopeDispose, ref, watch as vueWatch, type Ref } from "vue";
import { watch as watchResource, type ResolveContext } from "@/api/dataSource";
import type { DataSource, ResourceEvent } from "@/types/projection";
import {
  useRefreshableSource,
  type RefreshableSource,
} from "./useRefreshableSource";

interface LiveObjectOptions<T> {
  initialValue: () => T;
  // accept gates incoming pushes: false stashes the update instead of applying it
  // (e.g. an editor with unsaved edits).
  accept?: () => boolean;
  // active pauses the subscription when false (e.g. an inactive KeepAlive tab).
  active?: Ref<boolean>;
}

export interface LiveObject<T> extends RefreshableSource<T> {
  externalChanged: Ref<boolean>;
  deleted: Ref<boolean>;
  applyPending: () => void;
}

// useLiveObject is the generic single-resource live primitive: an initial fetch
// plus, when watchSource is set, a StreamResource subscription pushing the current
// object. The payload is whatever the route emits (object for detail, string for
// an editor), so it stays plugin-agnostic.
export function useLiveObject<T>(
  connectionId: string,
  watchSource: DataSource | undefined,
  ctx: ResolveContext,
  loader: () => Promise<T>,
  options: LiveObjectOptions<T>,
): LiveObject<T> {
  const base = useRefreshableSource<T>(loader, {
    initialValue: options.initialValue,
  });
  const externalChanged = ref(false);
  const deleted = ref(false);
  let pending: T | null = null;
  const accept = options.accept ?? (() => true);

  function onEvent(ev: ResourceEvent): void {
    if (ev.type === "deleted") {
      deleted.value = true;
      return;
    }
    deleted.value = false;
    const next = ev.resource as T | undefined;
    if (next === undefined || next === null) return;
    if (accept()) {
      base.data.value = next;
      base.loadedOnce.value = true;
      externalChanged.value = false;
      pending = null;
    } else {
      pending = next;
      externalChanged.value = true;
    }
  }

  function applyPending(): void {
    if (pending !== null) {
      base.data.value = pending;
      base.loadedOnce.value = true;
      pending = null;
    }
    externalChanged.value = false;
  }

  let stop: (() => void) | null = null;
  function start(): void {
    if (stop || !watchSource) return;
    stop = watchResource(connectionId, watchSource, ctx, onEvent);
  }
  function halt(): void {
    stop?.();
    stop = null;
  }

  if (options.active) {
    vueWatch(options.active, (a) => (a ? start() : halt()), {
      immediate: true,
    });
  } else {
    start();
  }
  onScopeDispose(halt);

  return { ...base, externalChanged, deleted, applyPending };
}
