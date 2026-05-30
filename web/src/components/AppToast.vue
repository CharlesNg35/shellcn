<script setup lang="ts">
import Toast from "primevue/toast";
import Button from "primevue/button";
import AppIcon from "./AppIcon.vue";
import type { Icon } from "../types/projection";

// A custom container keeps full styling control while reusing PrimeVue's
// positioning, lifecycle, grouping and accessibility. Severity reads from a
// coloured leading icon over a neutral surface card, rather than a loud tinted
// background.
interface Tone {
  icon: Icon;
  accent: string;
}

const tones: Record<string, Tone> = {
  success: {
    icon: { type: "lucide", value: "circle-check" },
    accent: "text-emerald-500",
  },
  error: {
    icon: { type: "lucide", value: "circle-alert" },
    accent: "text-rose-500",
  },
  warn: {
    icon: { type: "lucide", value: "triangle-alert" },
    accent: "text-amber-500",
  },
  info: { icon: { type: "lucide", value: "info" }, accent: "text-sky-500" },
};

function tone(severity?: string): Tone {
  return tones[severity ?? "info"] ?? tones.info;
}
</script>

<template>
  <Toast
    position="bottom-right"
    :pt="{ root: 'fixed bottom-4 right-4 z-[100] flex w-80 flex-col gap-2' }"
  >
    <template #container="{ message, closeCallback }">
      <div
        class="flex items-start gap-3 rounded-lg border border-surface-200 bg-surface-0 px-3.5 py-3 text-sm shadow-lg dark:border-surface-700 dark:bg-surface-900"
      >
        <AppIcon
          :icon="tone(message.severity).icon"
          :size="18"
          class="mt-0.5"
          :class="tone(message.severity).accent"
        />
        <div class="min-w-0 flex-1">
          <p
            v-if="message.summary"
            class="font-medium text-surface-900 dark:text-surface-0"
          >
            {{ message.summary }}
          </p>
          <p
            v-if="message.detail"
            class="mt-0.5 break-words text-surface-500 dark:text-surface-400"
          >
            {{ message.detail }}
          </p>
        </div>
        <Button
          text
          rounded
          severity="secondary"
          size="small"
          aria-label="Dismiss"
          class="-mt-1 -mr-1 shrink-0"
          @click="closeCallback"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="14" />
        </Button>
      </div>
    </template>
  </Toast>
</template>
