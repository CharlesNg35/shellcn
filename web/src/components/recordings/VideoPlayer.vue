<script setup lang="ts">
import { ref } from "vue";

defineProps<{ src: string }>();
const failed = ref(false);
</script>

<template>
  <div class="overflow-hidden rounded-lg bg-black">
    <p v-if="failed" class="p-4 text-sm text-surface-300" role="alert">
      This recording could not be loaded.
    </p>
    <video
      v-else
      :src="src"
      controls
      preload="metadata"
      class="h-auto w-full"
      @error="failed = true"
    >
      Your browser cannot play this recording.
    </video>
  </div>
</template>
