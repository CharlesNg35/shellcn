<script setup lang="ts">
import { computed } from "vue";
import Message from "primevue/message";
import AppIcon from "./AppIcon.vue";

type AlertTone = "info" | "success" | "warning" | "danger" | "neutral";

const props = withDefaults(
  defineProps<{
    tone?: AlertTone;
    title?: string;
    closable?: boolean;
  }>(),
  {
    tone: "info",
    title: "",
    closable: false,
  },
);

const severity = computed(() => {
  switch (props.tone) {
    case "success":
      return "success";
    case "warning":
      return "warn";
    case "danger":
      return "error";
    case "neutral":
      return "secondary";
    default:
      return "info";
  }
});

const icon = computed(() => {
  switch (props.tone) {
    case "success":
      return "circle-check";
    case "warning":
    case "danger":
      return "triangle-alert";
    case "neutral":
      return "message-circle";
    default:
      return "info";
  }
});
</script>

<template>
  <Message
    :severity="severity"
    :closable="closable"
    class="w-full"
    role="alert"
  >
    <template #icon>
      <AppIcon :icon="{ type: 'lucide', value: icon }" :size="16" />
    </template>
    <div class="min-w-0 space-y-0.5">
      <p v-if="title" class="font-medium">{{ title }}</p>
      <div class="text-xs break-words opacity-90">
        <slot />
      </div>
    </div>
  </Message>
</template>
