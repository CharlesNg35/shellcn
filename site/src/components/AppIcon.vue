<script setup lang="ts">
defineProps<{ name: keyof typeof icons }>();

const s = 'fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"';
const icons = {
  terminal: `<path d="m4 17 6-5-6-5M12 19h8" ${s}/>`,
  shield: `<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10Z" ${s}/><path d="m9 12 2 2 4-4" ${s}/>`,
  record: `<rect x="2" y="4" width="20" height="16" rx="2" ${s}/><circle cx="12" cy="12" r="3.4" fill="currentColor"/>`,
  network: `<rect x="9" y="2" width="6" height="6" rx="1" ${s}/><rect x="2" y="16" width="6" height="6" rx="1" ${s}/><rect x="16" y="16" width="6" height="6" rx="1" ${s}/><path d="M12 8v4M12 12H5v4M12 12h7v4" ${s}/>`,
  binary: `<path d="M20 7 12 3 4 7v10l8 4 8-4V7Z" ${s}/><path d="m4 7 8 4 8-4M12 21V11" ${s}/>`,
  globe: `<circle cx="12" cy="12" r="9" ${s}/><path d="M3 12h18M12 3c2.5 2.5 3.5 6 3.5 9S14.5 18.5 12 21c-2.5-2.5-3.5-6-3.5-9S9.5 5.5 12 3Z" ${s}/>`,
  plug: `<path d="M12 2v6M8 6v3a4 4 0 0 0 8 0V6M12 15v3a4 4 0 0 1-4 4H6" ${s}/>`,
  copy: `<rect x="9" y="9" width="12" height="12" rx="2" ${s}/><path d="M5 15V5a2 2 0 0 1 2-2h10" ${s}/>`,
  check: `<path d="m5 12 5 5 9-11" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"/>`,
  arrow: `<path d="M5 12h14m-6-6 6 6-6 6" ${s}/>`,
  github: `<path fill="currentColor" d="M12 2a10 10 0 0 0-3.16 19.49c.5.09.68-.22.68-.48v-1.7c-2.78.6-3.37-1.34-3.37-1.34-.45-1.16-1.11-1.47-1.11-1.47-.9-.62.07-.6.07-.6 1 .07 1.53 1.03 1.53 1.03.9 1.52 2.34 1.08 2.91.83.09-.65.35-1.09.63-1.34-2.22-.25-4.55-1.11-4.55-4.94 0-1.09.39-1.98 1.03-2.68-.1-.25-.45-1.27.1-2.64 0 0 .84-.27 2.75 1.02a9.4 9.4 0 0 1 5 0c1.91-1.29 2.75-1.02 2.75-1.02.55 1.37.2 2.39.1 2.64.64.7 1.03 1.59 1.03 2.68 0 3.84-2.34 4.68-4.57 4.93.36.31.68.92.68 1.85v2.74c0 .27.18.58.69.48A10 10 0 0 0 12 2Z"/>`,
};
</script>

<template>
  <svg viewBox="0 0 24 24" class="inline-block h-[1em] w-[1em] align-[-0.125em]" v-html="icons[name]" />
</template>
