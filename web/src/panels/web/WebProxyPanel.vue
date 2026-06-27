<script setup lang="ts">
import { computed } from "vue";
import Button from "primevue/button";
import AppIcon from "@/components/AppIcon.vue";
import type { PanelProps } from "@/panels/core/types";
import PanelError from "@/panels/shared/PanelError.vue";
import { usePersistentStagePanel } from "@/panels/shared/usePersistentStagePanel";
import type { WebProxyPanelConfig } from "@/types/projection";
import {
  activateWebProxyPanel,
  deactivateWebProxyPanel,
  registerWebProxyPanel,
  reloadWebProxyPanel,
  unregisterWebProxyPanel,
  updateWebProxyPanelRect,
  webProxyFrameURL,
  webProxyStageEntries,
} from "./webProxyStage";

const props = defineProps<PanelProps>();

const config = computed(() => props.config as WebProxyPanelConfig | undefined);
const stageKey = computed(
  () =>
    props.panelKey ??
    JSON.stringify({
      panel: "web_proxy",
      connectionId: props.connectionId,
      source: props.source,
      resource: props.resource?.uid,
      record: props.record,
      config: props.config,
    }),
);
const entry = computed(() =>
  webProxyStageEntries.value.find((item) => item.key === stageKey.value),
);
const configError = computed(() =>
  config.value ? null : "Web proxy panel config is required.",
);
const handle = computed(() =>
  config.value
    ? {
        key: stageKey.value,
        connectionId: props.connectionId,
        config: config.value,
        resource: props.resource,
        record: props.record,
      }
    : null,
);

const { setPlaceholder } = usePersistentStagePanel({
  stageKey,
  handle,
  watchSource: () => [
    stageKey.value,
    props.connectionId,
    props.resource,
    props.record,
    props.config,
  ],
  deep: true,
  register: registerWebProxyPanel,
  activate: activateWebProxyPanel,
  deactivate: deactivateWebProxyPanel,
  unregister: unregisterWebProxyPanel,
  updateRect: updateWebProxyPanelRect,
});

function reload(): void {
  reloadWebProxyPanel(stageKey.value);
}

function openExternal(): void {
  const src =
    entry.value?.src ||
    (config.value ? webProxyFrameURL(props.connectionId, config.value) : null);
  if (!src) return;
  window.open(src, "_blank", "noopener,noreferrer");
}
</script>

<template>
  <PanelError v-if="configError" :message="configError" />
  <PanelError v-else-if="entry?.error" :message="entry.error" />
  <section
    v-else
    class="flex h-full min-h-0 flex-col bg-surface-0 dark:bg-surface-950"
  >
    <div
      class="flex h-10 shrink-0 items-center justify-end gap-1 border-b border-surface-200 bg-surface-50 px-2 dark:border-surface-800 dark:bg-surface-900"
    >
      <Button
        type="button"
        severity="secondary"
        text
        rounded
        aria-label="Reload"
        title="Reload"
        class="h-8 w-8"
        :disabled="!entry?.src"
        @click="reload"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'refresh-cw' }" :size="16" />
      </Button>
      <Button
        v-if="config?.openExternal"
        type="button"
        severity="secondary"
        text
        rounded
        aria-label="Open in new tab"
        title="Open in new tab"
        class="h-8 w-8"
        :disabled="!entry?.src"
        @click="openExternal"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'external-link' }"
          :size="16"
        />
      </Button>
    </div>
    <div
      :ref="setPlaceholder"
      class="relative min-h-0 flex-1 bg-surface-0 dark:bg-surface-950"
      data-test="web-proxy-panel-placeholder"
    >
      <div
        v-if="!entry?.loaded"
        class="pointer-events-none absolute inset-x-0 top-0 z-10 h-0.5 overflow-hidden bg-surface-200 dark:bg-surface-800"
        aria-hidden="true"
      >
        <span class="block h-full w-1/3 animate-pulse bg-primary-500" />
      </div>
      <p class="sr-only">
        {{
          config?.instructions ||
          "This panel embeds a connection-scoped web surface through the gateway proxy."
        }}
      </p>
    </div>
  </section>
</template>
