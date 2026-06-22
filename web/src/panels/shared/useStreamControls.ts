import { computed, reactive, ref, type ComputedRef } from "vue";
import { fetchPage, type ResolveContext } from "@/api/dataSource";
import type { DataSource, Option, StreamControl } from "@/types/projection";

// useStreamControls is the shared logic behind a panel's StreamControl selectors
// (a manifest-driven set of pickers that re-parameterize a source). It owns the
// selected values + fetched options; each panel decides what a change does
// (reconnect a stream, reload a listing, …).
export function useStreamControls(
  connectionId: string,
  controls: ComputedRef<StreamControl[]>,
  ctx: ResolveContext,
): {
  values: Record<string, string>;
  options: ComputedRef<Record<string, Option[]>>;
  load: () => Promise<void>;
  visible: (param: string) => boolean;
  hasVisible: ComputedRef<boolean>;
  applyTo: (source: DataSource) => void;
} {
  const values = reactive<Record<string, string>>({});
  const options = ref<Record<string, Option[]>>({});

  async function load(): Promise<void> {
    for (const ctrl of controls.value) {
      if (!ctrl.optionsSource) continue;
      try {
        const page = await fetchPage<Option>(
          connectionId,
          ctrl.optionsSource,
          ctx,
          {
            limit: 200,
          },
        );
        options.value = { ...options.value, [ctrl.param]: page.items };
        if (values[ctrl.param] === undefined && page.items.length) {
          values[ctrl.param] = String(page.items[0].value);
        }
      } catch {
        options.value = { ...options.value, [ctrl.param]: [] };
      }
    }
  }

  function visible(param: string): boolean {
    return (options.value[param]?.length ?? 0) > 1;
  }

  const hasVisible = computed(() =>
    controls.value.some((ctrl) => visible(ctrl.param)),
  );

  // applyTo merges the current selection into a (reactive) source's params.
  function applyTo(source: DataSource): void {
    for (const ctrl of controls.value) {
      source.params = {
        ...(source.params ?? {}),
        [ctrl.param]: values[ctrl.param] ?? "",
      };
    }
  }

  return {
    values,
    options: computed(() => options.value),
    load,
    visible,
    hasVisible,
    applyTo,
  };
}
