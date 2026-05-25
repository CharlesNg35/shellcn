<script setup lang="ts">
import { computed } from "vue";
import { useConnectionsStore } from "../stores/connections";

const conns = useConnectionsStore();
const count = computed(() => conns.connections.length);
const protocols = computed(() => conns.plugins.length);
</script>

<template>
  <div
    class="flex h-full flex-col items-center justify-center gap-3 p-10 text-center"
  >
    <h1 class="text-3xl font-semibold text-surface-900 dark:text-surface-0">
      Infrastructure access gateway
    </h1>
    <p class="max-w-md text-surface-500">
      Select a connection from the sidebar to open its workspace. Every protocol
      renders from its manifest — no per-plugin UI.
    </p>
    <p class="text-sm text-surface-400">
      {{ count }} connection{{ count === 1 ? "" : "s" }} ·
      {{ protocols }} protocol{{ protocols === 1 ? "" : "s" }} available
    </p>
  </div>
</template>
