<script setup lang="ts">
import { defineAsyncComponent, h, onMounted, ref } from "vue";
import Button from "primevue/button";
import Drawer from "primevue/drawer";
import AppIcon from "./AppIcon.vue";
import { aiApi } from "../api/ai";

// AiChatLauncher is the ONLY AI code in the main bundle: a header icon + Drawer
// shell. The heavy chat panel (store, markdown, highlight) is lazy-loaded on
// first open, so first paint stays constant whether or not AI is configured.
defineProps<{ connectionId: string; connected: boolean }>();

const available = ref(false);
const open = ref(false);
const opened = ref(false); // mount the panel only once the drawer is first opened

const AiChatPanel = defineAsyncComponent({
  loader: () => import("../panels/ai/AiChatPanel.vue"),
  delay: 150,
  timeout: 20000,
  loadingComponent: {
    render: () =>
      h(
        "div",
        {
          class:
            "flex h-full items-center justify-center text-sm text-surface-400",
        },
        [
          h("span", {
            class:
              "h-5 w-5 animate-spin rounded-full border-2 border-surface-200 border-t-primary-500",
            role: "status",
            "aria-label": "Loading assistant",
          }),
        ],
      ),
  },
  errorComponent: {
    setup() {
      return () =>
        h(
          "div",
          {
            class:
              "flex h-full flex-col items-center justify-center gap-3 text-sm text-surface-500",
          },
          [
            h("span", "Failed to load the assistant."),
            h(
              "button",
              {
                class:
                  "rounded-md border border-surface-300 px-3 py-1.5 text-xs dark:border-surface-700",
                onClick: () => window.location.reload(),
              },
              "Reload",
            ),
          ],
        );
    },
  },
});

function toggle(): void {
  open.value = !open.value;
  if (open.value) opened.value = true;
}

onMounted(async () => {
  try {
    const [global, list] = await Promise.all([aiApi.global(), aiApi.list()]);
    available.value = global.configured || list.length > 0;
  } catch {
    available.value = false;
  }
});
</script>

<template>
  <Button
    v-if="available && connected"
    text
    rounded
    severity="secondary"
    title="AI assistant"
    aria-label="Open AI assistant"
    @click="toggle"
  >
    <AppIcon :icon="{ type: 'lucide', value: 'sparkles' }" :size="17" />
  </Button>

  <Drawer
    v-model:visible="open"
    position="right"
    :modal="false"
    :dismissable="false"
    header="Assistant"
    :pt="{
      root: 'w-full max-w-md',
      header: 'hidden',
      content: 'flex min-h-0 flex-1 flex-col p-0',
    }"
  >
    <AiChatPanel v-if="opened" :connection-id="connectionId" class="h-full" />
  </Drawer>
</template>
