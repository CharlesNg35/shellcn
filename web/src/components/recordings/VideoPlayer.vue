<script setup lang="ts">
import { computed, ref } from "vue";
import Button from "primevue/button";
import AppIcon from "../AppIcon.vue";

const props = defineProps<{
  src: string;
  downloadSrc?: string;
  downloadName?: string;
}>();
const failed = ref(false);
const video = ref<HTMLVideoElement | null>(null);
const playbackRate = ref(1);
const rates = [0.5, 1, 1.25, 1.5, 2];

const downloadHref = computed(() => props.downloadSrc || props.src);

function setRate(rate: number): void {
  playbackRate.value = rate;
  if (video.value) video.value.playbackRate = rate;
}
</script>

<template>
  <div class="overflow-hidden rounded-lg bg-black">
    <p v-if="failed" class="p-4 text-sm text-surface-300" role="alert">
      This recording could not be loaded.
    </p>
    <video
      v-else
      ref="video"
      :src="src"
      controls
      preload="metadata"
      class="h-auto w-full"
      @error="failed = true"
    >
      Your browser cannot play this recording.
    </video>
    <div
      class="flex flex-wrap items-center justify-between gap-2 border-t border-white/10 bg-surface-950 px-3 py-2"
    >
      <div class="flex items-center gap-1">
        <Button
          v-for="rate in rates"
          :key="rate"
          type="button"
          size="small"
          :severity="playbackRate === rate ? 'primary' : 'secondary'"
          :outlined="playbackRate !== rate"
          @click="setRate(rate)"
        >
          {{ rate }}x
        </Button>
      </div>
      <a
        :href="downloadHref"
        :download="downloadName"
        class="inline-flex h-8 items-center gap-1.5 rounded-md px-2 text-sm text-surface-200 transition-colors hover:bg-white/10 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-400"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'download' }" :size="15" />
        Download
      </a>
    </div>
  </div>
</template>
