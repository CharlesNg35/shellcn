<script setup lang="ts">
withDefaults(defineProps<{ rows?: number }>(), { rows: 4 });

// Alternating assistant/user placeholder bubbles; widths vary for a natural feel.
const shape = (n: number): { user: boolean; width: string } => ({
  user: n % 2 === 0,
  width: `${52 + ((n * 17) % 34)}%`,
});
</script>

<template>
  <div
    class="flex flex-col gap-3 px-4 py-3"
    aria-hidden="true"
    data-test="ai-message-skeleton"
  >
    <div
      v-for="n in rows"
      :key="n"
      class="flex gap-2"
      :class="shape(n).user ? 'justify-end' : 'justify-start'"
    >
      <div
        v-if="!shape(n).user"
        class="mt-1 size-6 shrink-0 rounded-full bg-surface-200 motion-safe:animate-pulse dark:bg-surface-800"
      />
      <div
        class="flex flex-col gap-1.5 rounded-2xl bg-surface-100 px-3 py-2.5 dark:bg-surface-800/70"
        :style="{ width: shape(n).width }"
      >
        <div
          class="h-3 rounded bg-surface-200 motion-safe:animate-pulse dark:bg-surface-700"
        />
        <div
          class="h-3 w-3/4 rounded bg-surface-200 motion-safe:animate-pulse dark:bg-surface-700"
        />
      </div>
    </div>
  </div>
</template>
