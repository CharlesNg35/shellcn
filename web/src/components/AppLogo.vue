<script setup lang="ts">
withDefaults(
  defineProps<{
    size?: number | string;
    label?: string;
  }>(),
  {
    size: 32,
    label: "",
  },
);
</script>

<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 32 32"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    :role="label ? 'img' : undefined"
    :aria-label="label || undefined"
    :aria-hidden="label ? undefined : 'true'"
  >
    <rect width="32" height="32" rx="6" fill="currentColor" />
    <g transform="translate(16 16)">
      <path
        d="M3 -6Q6 -6 6 -3Q6 0 3 0H-3Q-6 0 -6 3Q-6 6 -3 6H3"
        stroke="#fff"
        stroke-width="2"
        stroke-linecap="round"
        fill="none"
      />
      <path
        d="M-5 -5L-3.5 -3.5L-5 -2"
        stroke="#fff"
        stroke-width="1"
        stroke-linecap="round"
        stroke-linejoin="round"
        fill="none"
      />
    </g>
  </svg>
</template>
