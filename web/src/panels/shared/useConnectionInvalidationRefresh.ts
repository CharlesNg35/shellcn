import {
  computed,
  onActivated,
  onDeactivated,
  onScopeDispose,
  ref,
  watch,
  type Ref,
} from "vue";
import { getActivePinia } from "pinia";
import { useWorkspaceInvalidationStore } from "@/stores/workspaceInvalidation";

interface Options {
  connectionId: () => string;
  refresh: () => unknown | Promise<unknown>;
  active?: Ref<boolean>;
  canRefresh?: () => boolean;
  debounceMs?: number;
}

export function useConnectionInvalidationRefresh(options: Options): void {
  if (!getActivePinia()) return;

  const invalidations = useWorkspaceInvalidationStore();
  const localActive = ref(true);
  const active = options.active ?? localActive;
  const debounceMs = options.debounceMs ?? 150;
  const version = computed(() => invalidations.version(options.connectionId()));
  let stale = false;
  let timer: ReturnType<typeof setTimeout> | undefined;

  function clearTimer(): void {
    if (timer) clearTimeout(timer);
    timer = undefined;
  }

  function canRefresh(): boolean {
    return active.value && (options.canRefresh?.() ?? true);
  }

  function run(): void {
    clearTimer();
    if (!canRefresh()) {
      stale = true;
      return;
    }
    stale = false;
    timer = setTimeout(() => {
      timer = undefined;
      if (!canRefresh()) {
        stale = true;
        return;
      }
      void options.refresh();
    }, debounceMs);
  }

  watch(
    () => [options.connectionId(), version.value] as const,
    ([connectionId, nextVersion], previous) => {
      if (!previous) {
        return;
      }
      const [previousConnection, previousVersion] = previous;
      if (previousConnection !== connectionId) {
        stale = false;
        clearTimer();
        return;
      }
      if (nextVersion !== previousVersion) run();
    },
    { immediate: true },
  );

  onActivated(() => {
    localActive.value = true;
    if (stale) run();
  });

  onDeactivated(() => {
    localActive.value = false;
    clearTimer();
  });

  onScopeDispose(clearTimer);
}
