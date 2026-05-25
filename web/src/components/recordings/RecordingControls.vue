<script setup lang="ts">
import { computed } from "vue";
import { resolveParams } from "../../api/dataSource";
import {
  useRecordingControl,
  type RecordingDescriptor,
} from "../../composables/useRecordingControl";
import AppIcon from "../AppIcon.vue";
import type { DataSource, ResourceRef } from "../../types/projection";

const props = defineProps<{
  connectionId: string;
  source: DataSource;
  resource?: ResourceRef | null;
  descriptor: RecordingDescriptor;
}>();

const streamRef = computed(() => ({
  routeId: props.source.routeId,
  params: resolveParams(props.source.params, { resource: props.resource }),
}));

const { recording, forced, failed, busy, canControl, start, stop } =
  useRecordingControl(props.connectionId, streamRef.value, props.descriptor);

const typeLabel = computed(() =>
  props.descriptor.class === "desktop" ? "desktop" : "terminal",
);
</script>

<template>
  <div class="flex items-center gap-2 text-xs">
    <span
      v-if="recording"
      class="inline-flex items-center gap-1.5 rounded-full bg-red-500/10 px-2 py-0.5 font-medium text-red-600 dark:text-red-400"
      :title="`Recording this ${typeLabel} session`"
    >
      <span class="h-2 w-2 animate-pulse rounded-full bg-red-500" />
      REC
    </span>

    <button
      v-if="canControl && !recording"
      type="button"
      :disabled="busy"
      class="inline-flex items-center gap-1.5 rounded-md border border-surface-300 px-2 py-1 text-surface-600 hover:border-red-400 hover:text-red-600 disabled:opacity-50 dark:border-surface-600 dark:text-surface-300"
      @click="start"
    >
      <span class="h-2 w-2 rounded-full bg-red-500" />
      Record
    </button>

    <button
      v-if="canControl && recording && !forced"
      type="button"
      :disabled="busy"
      class="inline-flex items-center gap-1.5 rounded-md border border-surface-300 px-2 py-1 text-surface-600 hover:bg-surface-100 disabled:opacity-50 dark:border-surface-600 dark:text-surface-300 dark:hover:bg-surface-800"
      @click="stop"
    >
      <AppIcon :icon="{ type: 'name', value: 'stop' }" :size="12" />
      Stop
    </button>

    <span v-if="failed" class="text-amber-500">Recording error</span>
  </div>
</template>
