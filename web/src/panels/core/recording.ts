import { inject, provide, type InjectionKey } from "vue";
import type { RecordingDescriptor } from "@/composables/useRecordingControl";
import type { DataSource } from "@/types/projection";

export type PanelRecordingResolver = (
  source?: DataSource,
) => RecordingDescriptor | null;

const PANEL_RECORDING_RESOLVER: InjectionKey<PanelRecordingResolver> = Symbol(
  "panel-recording-resolver",
);

export function providePanelRecordingResolver(
  resolver: PanelRecordingResolver,
): void {
  provide(PANEL_RECORDING_RESOLVER, resolver);
}

export function usePanelRecordingResolver(): PanelRecordingResolver {
  return inject(PANEL_RECORDING_RESOLVER, () => null);
}
