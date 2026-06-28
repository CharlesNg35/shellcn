<script setup lang="ts">
import { computed } from "vue";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import Tooltip from "primevue/tooltip";
import { recordingsApi } from "@/api/recordings";
import AppIcon from "@/components/AppIcon.vue";
import CastPlayer from "./CastPlayer.vue";
import VideoPlayer from "./VideoPlayer.vue";
import { dialogRoot } from "@/primevue/preset";
import {
  RecordingClass,
  RecordingFormat,
  type RecordingSummary,
} from "@/types/projection";

const props = defineProps<{
  visible: boolean;
  recording: RecordingSummary | null;
}>();
defineEmits<{ "update:visible": [value: boolean] }>();
const vTooltip = Tooltip;

const src = computed(() =>
  props.recording ? recordingsApi.contentUrl(props.recording.id) : "",
);
const downloadSrc = computed(() =>
  props.recording
    ? recordingsApi.contentUrl(props.recording.id, { download: true })
    : "",
);
const isTerminal = computed(
  () => props.recording?.class === RecordingClass.Terminal,
);
const isBrowserCapture = computed(
  () =>
    props.recording?.format === RecordingFormat.WebmCanvas &&
    !props.recording.authoritative,
);
const title = computed(
  () =>
    props.recording?.connectionName || props.recording?.protocol || "Recording",
);
const captureNotice = computed(() => ({
  value: "Browser capture. Not compliance-grade.",
  showDelay: 300,
}));
const downloadName = computed(() => {
  if (!props.recording) return "recording";
  const ext =
    props.recording.format === RecordingFormat.WebmCanvas ? "webm" : "cast";
  return `${props.recording.connectionName || props.recording.protocol || props.recording.id}-${props.recording.id}.${ext}`;
});
</script>

<template>
  <Dialog
    :visible="visible"
    modal
    :pt="{
      root: dialogRoot('max-w-4xl'),
      content: 'min-h-0 overflow-auto p-4',
    }"
    @update:visible="$emit('update:visible', $event)"
  >
    <template #header>
      <div class="flex min-w-0 items-center gap-2">
        <span class="truncate font-semibold">{{ title }}</span>
        <Button
          v-if="isBrowserCapture"
          v-tooltip.bottom="captureNotice"
          type="button"
          text
          rounded
          size="small"
          severity="warn"
          aria-label="Browser capture. Not compliance-grade."
        >
          <AppIcon :icon="{ type: 'lucide', value: 'info' }" :size="14" />
        </Button>
      </div>
    </template>
    <div v-if="recording" class="flex flex-col gap-3">
      <CastPlayer v-if="isTerminal" :src="src" />
      <VideoPlayer
        v-else
        :src="src"
        :download-src="downloadSrc"
        :download-name="downloadName"
      />
    </div>
  </Dialog>
</template>
