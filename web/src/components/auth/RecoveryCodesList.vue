<script setup lang="ts">
import Button from "primevue/button";
import { useNotify } from "../../composables/useNotify";
import AppIcon from "../AppIcon.vue";

const props = defineProps<{ codes: string[] }>();
const notify = useNotify();

async function copy(): Promise<void> {
  try {
    await navigator.clipboard?.writeText(props.codes.join("\n"));
    notify.success("Recovery codes copied");
  } catch {
    // clipboard unavailable
  }
}

function download(): void {
  const blob = new Blob([props.codes.join("\n") + "\n"], {
    type: "text/plain",
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "shellcn-recovery-codes.txt";
  a.click();
  URL.revokeObjectURL(url);
}
</script>

<template>
  <div class="flex flex-col gap-3">
    <ul
      class="grid grid-cols-2 gap-2 rounded-md border border-surface-200 bg-surface-50 p-3 font-mono text-sm dark:border-surface-800 dark:bg-surface-950"
    >
      <li
        v-for="code in codes"
        :key="code"
        class="text-surface-700 dark:text-surface-200"
      >
        {{ code }}
      </li>
    </ul>

    <div class="flex flex-wrap items-center gap-2">
      <Button type="button" severity="secondary" outlined @click="copy">
        <AppIcon :icon="{ type: 'lucide', value: 'copy' }" :size="15" />
        Copy
      </Button>
      <Button type="button" severity="secondary" outlined @click="download">
        <AppIcon :icon="{ type: 'lucide', value: 'download' }" :size="15" />
        Download
      </Button>
    </div>
  </div>
</template>
