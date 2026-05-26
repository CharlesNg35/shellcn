<script setup lang="ts">
import { computed } from "vue";
import Button from "primevue/button";
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
      role="status"
      :aria-label="`Recording this ${typeLabel} session`"
      :title="`Recording this ${typeLabel} session`"
    >
      <span
        class="h-2 w-2 animate-pulse rounded-full bg-red-500"
        aria-hidden="true"
      />
      REC
    </span>

    <Button
      v-if="canControl && !recording"
      outlined
      severity="secondary"
      size="small"
      :disabled="busy"
      @click="start"
    >
      <span class="h-2 w-2 rounded-full bg-red-500" />
      Record
    </Button>

    <Button
      v-if="canControl && recording && !forced"
      outlined
      severity="secondary"
      size="small"
      :disabled="busy"
      @click="stop"
    >
      <AppIcon :icon="{ type: 'name', value: 'stop' }" :size="12" />
      Stop
    </Button>

    <span v-if="failed" class="text-amber-500" role="alert"
      >Recording error</span
    >
  </div>
</template>
