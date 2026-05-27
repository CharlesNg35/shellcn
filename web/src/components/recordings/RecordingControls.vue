<script setup lang="ts">
import { computed, watch } from "vue";
import Button from "primevue/button";
import { resolveParams } from "../../api/dataSource";
import {
  useRecordingControl,
  type RecordingDescriptor,
} from "../../composables/useRecordingControl";
import AppIcon from "../AppIcon.vue";
import type { DataSource, ResourceRef } from "../../types/projection";
import type { ChannelStatus } from "../../stores/sessions";

const props = defineProps<{
  connectionId: string;
  source: DataSource;
  resource?: ResourceRef | null;
  descriptor: RecordingDescriptor;
  streamStatus?: ChannelStatus;
}>();

const streamRef = computed(() => ({
  routeId: props.source.routeId,
  params: resolveParams(props.source.params, { resource: props.resource }),
}));

const { recording, forced, failed, busy, canControl, start, stop } =
  useRecordingControl(props.connectionId, streamRef.value, props.descriptor);

let resumeOnOpen = false;

const typeLabel = computed(() =>
  props.descriptor.class === "desktop" ? "desktop" : "terminal",
);

async function startRecording(): Promise<void> {
  resumeOnOpen = true;
  await start();
}

async function stopRecording(): Promise<void> {
  resumeOnOpen = false;
  await stop();
}

watch(
  () => props.streamStatus,
  (next) => {
    if (!canControl.value || !next) return;
    if ((next === "closed" || next === "error") && recording.value) {
      resumeOnOpen = true;
      recording.value = false;
      return;
    }
    if (next === "open" && resumeOnOpen && !recording.value && !busy.value) {
      void start();
    }
  },
);
</script>

<template>
  <div class="flex items-center gap-2 text-xs">
    <span
      v-if="recording"
      class="inline-flex items-center gap-1.5 rounded-full bg-rose-500/10 px-2 py-0.5 font-medium text-rose-600 dark:text-rose-300"
      role="status"
      :aria-label="`Recording this ${typeLabel} session`"
      :title="`Recording this ${typeLabel} session`"
    >
      <span
        class="h-2 w-2 animate-pulse rounded-full bg-rose-400"
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
      @click="startRecording"
    >
      <AppIcon
        v-if="busy"
        :icon="{ type: 'lucide', value: 'circle' }"
        :size="12"
        loading
      />
      <span v-else class="h-2 w-2 rounded-full bg-rose-400" />
      Record
    </Button>

    <Button
      v-if="canControl && recording && !forced"
      outlined
      severity="secondary"
      size="small"
      :disabled="busy"
      @click="stopRecording"
    >
      <AppIcon
        :icon="{ type: 'lucide', value: 'square' }"
        :size="12"
        :loading="busy"
      />
      Stop
    </Button>

    <span v-if="failed" class="text-amber-500" role="alert"
      >Recording error</span
    >
  </div>
</template>
