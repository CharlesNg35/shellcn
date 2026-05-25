<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";

const props = defineProps<{ src: string }>();

const container = ref<HTMLElement | null>(null);
const failed = ref(false);
let player: { dispose: () => void } | null = null;

function dispose(): void {
  try {
    player?.dispose();
  } catch {
    /* already disposed */
  }
  player = null;
}

async function mount(): Promise<void> {
  if (!container.value) return;
  dispose();
  failed.value = false;
  try {
    const AsciinemaPlayer = await import("asciinema-player");
    await import("asciinema-player/dist/bundle/asciinema-player.css");
    player = AsciinemaPlayer.create(props.src, container.value, {
      fit: "width",
    });
  } catch {
    failed.value = true;
  }
}

onMounted(mount);
watch(() => props.src, mount);
onBeforeUnmount(dispose);
</script>

<template>
  <div class="overflow-hidden rounded-lg bg-[#0b0f17]">
    <p v-if="failed" class="p-4 text-sm text-surface-400">
      Playback unavailable in this environment.
    </p>
    <div ref="container" />
  </div>
</template>
