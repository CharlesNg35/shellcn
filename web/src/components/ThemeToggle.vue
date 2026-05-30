<script setup lang="ts">
import Button from "primevue/button";
import { useTheme } from "../composables/useTheme";
import AppIcon from "./AppIcon.vue";

withDefaults(
  defineProps<{
    size?: "small" | "large";
    iconSize?: number;
  }>(),
  { size: undefined, iconSize: 18 },
);

const { isDark, toggle } = useTheme();
</script>

<template>
  <Button
    text
    rounded
    severity="secondary"
    :size="size"
    :title="isDark ? 'Switch to light theme' : 'Switch to dark theme'"
    :aria-label="isDark ? 'Switch to light theme' : 'Switch to dark theme'"
    @click="toggle"
  >
    <Transition
      mode="out-in"
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="-rotate-90 scale-0 opacity-0"
      enter-to-class="rotate-0 scale-100 opacity-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="rotate-0 scale-100 opacity-100"
      leave-to-class="rotate-90 scale-0 opacity-0"
    >
      <AppIcon
        :key="isDark ? 'sun' : 'moon'"
        :icon="{ type: 'lucide', value: isDark ? 'sun' : 'moon' }"
        :size="iconSize"
      />
    </Transition>
  </Button>
</template>
