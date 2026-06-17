import { computed, ref, type ComputedRef, type Ref } from "vue";

interface RefreshableSourceOptions<T> {
  initialValue: () => T;
}

export interface RefreshableSource<T> {
  data: Ref<T>;
  loadedOnce: Ref<boolean>;
  refreshing: Ref<boolean>;
  error: Ref<string | null>;
  showInitialLoader: ComputedRef<boolean>;
  blockingError: ComputedRef<string | null>;
  load: () => Promise<T | undefined>;
  reset: () => void;
}

export function useRefreshableSource<T>(
  loader: () => Promise<T>,
  options: RefreshableSourceOptions<T>,
): RefreshableSource<T> {
  const data = ref(options.initialValue()) as Ref<T>;
  const loadedOnce = ref(false);
  const refreshing = ref(false);
  const error = ref<string | null>(null);
  let requestId = 0;

  const showInitialLoader = computed(
    () => refreshing.value && !loadedOnce.value,
  );
  const blockingError = computed(() =>
    error.value && !loadedOnce.value ? error.value : null,
  );

  async function load(): Promise<T | undefined> {
    if (refreshing.value) return undefined;
    const request = ++requestId;
    refreshing.value = true;
    error.value = null;
    try {
      const next = await loader();
      if (request !== requestId) return undefined;
      data.value = next;
      loadedOnce.value = true;
      return next;
    } catch (e) {
      if (request === requestId) error.value = (e as Error).message;
      return undefined;
    } finally {
      if (request === requestId) refreshing.value = false;
    }
  }

  function reset(): void {
    requestId += 1;
    data.value = options.initialValue();
    loadedOnce.value = false;
    refreshing.value = false;
    error.value = null;
  }

  return {
    data,
    loadedOnce,
    refreshing,
    error,
    showInitialLoader,
    blockingError,
    load,
    reset,
  };
}
