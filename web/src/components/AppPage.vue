<script setup lang="ts">
withDefaults(
  defineProps<{
    fill?: boolean;
    scroll?: boolean;
  }>(),
  {
    fill: true,
    scroll: false,
  },
);
</script>

<template>
  <div
    :class="[
      'mx-auto flex w-full max-w-5xl flex-col gap-5 px-4 py-6 sm:p-6 lg:p-8',
      fill ? 'h-full' : '',
      scroll ? 'overflow-auto' : '',
    ]"
  >
    <slot />
  </div>
</template>
