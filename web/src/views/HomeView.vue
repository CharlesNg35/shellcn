<script setup lang="ts">
import { computed } from "vue";
import { useConnectionsStore } from "../stores/connections";
import AppIcon from "../components/AppIcon.vue";

const conns = useConnectionsStore();
const count = computed(() => conns.connections.length);
const protocols = computed(() => conns.plugins.length);
</script>

<template>
  <div
    class="flex h-full flex-col items-center justify-center gap-4 p-10 text-center"
  >
    <span
      class="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary-600 text-white"
    >
      <AppIcon :icon="{ type: 'name', value: 'terminal' }" :size="28" />
    </span>

    <h1
      class="text-3xl font-semibold tracking-tight text-surface-900 dark:text-surface-0"
    >
      Infrastructure access gateway
    </h1>
    <p class="max-w-md text-surface-500">
      Select a connection from the sidebar to open its workspace.
    </p>
    <p class="text-sm text-surface-400">
      {{ count }} connection{{ count === 1 ? "" : "s" }} ·
      {{ protocols }} protocol{{ protocols === 1 ? "" : "s" }} available
    </p>
  </div>
</template>
