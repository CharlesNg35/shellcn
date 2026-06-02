<script setup lang="ts">
import { computed, defineAsyncComponent, h, onMounted, ref, watch } from "vue";
import Button from "primevue/button";
import Drawer from "primevue/drawer";
import AppIcon from "./AppIcon.vue";
import { drawerRoot } from "../primevue/preset";
import { useAiProvidersStore } from "../stores/aiProviders";

const props = defineProps<{
  connectionId: string;
  connected: boolean;
  aiMode?: string;
}>();

const aiProviders = useAiProvidersStore();
const open = ref(false);
const opened = ref(false); // mount the panel only once the drawer is first opened
const visible = computed(
  () => aiProviders.available && props.connected && props.aiMode !== "disabled",
);

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
            h(Button, {
              label: "Reload",
              size: "small",
              severity: "secondary",
              outlined: true,
              onClick: () => window.location.reload(),
            }),
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
    await aiProviders.load();
  } catch {
    return;
  }
});

watch(visible, (next) => {
  if (!next) open.value = false;
});
</script>

<template>
  <Button
    v-if="visible"
    text
    rounded
    severity="secondary"
    title="AI assistant"
    :aria-label="open ? 'Close AI assistant' : 'Open AI assistant'"
    @click="toggle"
  >
    <AppIcon :icon="{ type: 'lucide', value: 'sparkles' }" :size="17" />
  </Button>

  <Drawer
    v-model:visible="open"
    position="right"
    :dismissable="false"
    header="Assistant"
    :pt="{
      root: drawerRoot('max-w-lg'),
      header: 'hidden',
      content: 'flex min-h-0 flex-1 flex-col p-0',
    }"
  >
    <AiChatPanel
      v-if="opened"
      :connection-id="connectionId"
      class="h-full"
      @close="open = false"
    />
  </Drawer>
</template>
