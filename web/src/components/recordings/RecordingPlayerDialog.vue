<script setup lang="ts">
import { computed } from "vue";
import Dialog from "primevue/dialog";
import { recordingsApi } from "../../api/recordings";
import CastPlayer from "./CastPlayer.vue";
import VideoPlayer from "./VideoPlayer.vue";
import type { RecordingSummary } from "../../types/projection";

const props = defineProps<{
  visible: boolean;
  recording: RecordingSummary | null;
}>();
defineEmits<{ "update:visible": [value: boolean] }>();

const src = computed(() =>
  props.recording ? recordingsApi.contentUrl(props.recording.id) : "",
);
const isTerminal = computed(() => props.recording?.class === "terminal");
const title = computed(
  () =>
    props.recording?.connectionName || props.recording?.protocol || "Recording",
);
</script>

<template>
  <Dialog
    :visible="visible"
    modal
    :header="title"
    :pt="{
      root: 'w-full max-w-4xl rounded-lg bg-surface-0 shadow-xl dark:bg-surface-900',
      content: 'p-4',
    }"
    @update:visible="$emit('update:visible', $event)"
  >
    <div v-if="recording" class="flex flex-col gap-3">
      <p
        v-if="!recording.authoritative"
        class="rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-950/40 dark:text-amber-300"
      >
        Browser capture. Not compliance-grade.
      </p>
      <CastPlayer v-if="isTerminal" :src="src" />
      <VideoPlayer v-else :src="src" />
    </div>
  </Dialog>
</template>
