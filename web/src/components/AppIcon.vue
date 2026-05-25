<script setup lang="ts">
import { computed, ref } from "vue";
import type { Icon } from "../types/projection";
import { glyphs, FALLBACK_GLYPH } from "./icons/glyphs";

const props = withDefaults(
  defineProps<{
    icon?: Icon | null;
    size?: number;
    fallback?: string;
  }>(),
  { icon: null, size: 18, fallback: FALLBACK_GLYPH },
);

const imgFailed = ref(false);

const kind = computed(() => {
  const t = props.icon?.type;
  const v = props.icon?.value;
  // No icon declared → render nothing (a fallback glyph would just be noise).
  if (!t || !v) return "none";
  if ((t === "url" || t === "base64") && !imgFailed.value) {
    const safe =
      t === "base64" ? v.startsWith("data:image/") : v.startsWith("https://");
    return safe ? "image" : "glyph";
  }
  if (t === "emoji") return "emoji";
  return "glyph";
});

const glyphBody = computed(() => {
  const name = props.icon?.type === "name" ? props.icon.value : props.fallback;
  return glyphs[name] ?? glyphs[props.fallback] ?? glyphs[FALLBACK_GLYPH];
});
</script>

<template>
  <!-- eslint-disable vue/no-v-html -- glyphBody is a static, trusted SVG path set -->
  <svg
    v-if="kind === 'glyph'"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    v-html="glyphBody"
  />
  <!-- eslint-enable vue/no-v-html -->
  <span
    v-else-if="kind === 'emoji'"
    :style="{ fontSize: `${size}px`, lineHeight: 1 }"
    role="img"
    >{{ icon?.value }}</span
  >
  <img
    v-else-if="kind === 'image'"
    :src="icon?.value"
    :width="size"
    :height="size"
    alt=""
    class="object-contain"
    :style="{ maxWidth: `${size}px`, maxHeight: `${size}px` }"
    @error="imgFailed = true"
  />
</template>
