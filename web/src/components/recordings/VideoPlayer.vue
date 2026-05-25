<script setup lang="ts">
defineProps<{ src: string }>();
</script>

<template>
  <div class="overflow-hidden rounded-lg bg-black">
    <video :src="src" controls preload="metadata" class="h-auto w-full">
      Your browser cannot play this recording.
    </video>
  </div>
</template>
