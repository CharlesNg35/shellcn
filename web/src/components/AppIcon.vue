<script setup lang="ts">
import { computed, ref } from "vue";
import DOMPurify from "dompurify";
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

// Bound inline SVG markup so a hostile manifest can't ship a huge payload.
const MAX_SVG_BYTES = 64 * 1024;

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
  if (t === "svg") return safeSvg.value ? "svg" : "glyph";
  return "glyph";
});

const glyphBody = computed(() => {
  const name = props.icon?.type === "name" ? props.icon.value : props.fallback;
  return glyphs[name] ?? glyphs[props.fallback] ?? glyphs[FALLBACK_GLYPH];
});

// Sanitize raw inline SVG (svg profile only — no HTML/MathML, scripts and event
// handlers stripped) before it is ever injected into the DOM.
const safeSvg = computed(() => {
  const v = props.icon?.type === "svg" ? props.icon.value : "";
  if (!v || v.length > MAX_SVG_BYTES) return "";
  return DOMPurify.sanitize(v, {
    USE_PROFILES: { svg: true, svgFilters: true },
  });
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
  <span
    v-else-if="kind === 'emoji'"
    :style="{ fontSize: `${size}px`, lineHeight: 1 }"
    role="img"
  >
    {{ icon?.value }}
  </span>
  <span
    v-else-if="kind === 'svg'"
    class="app-icon-svg inline-flex"
    :style="{ width: `${size}px`, height: `${size}px` }"
    aria-hidden="true"
    v-html="safeSvg"
  />
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

<style scoped>
.app-icon-svg :deep(svg) {
  width: 100%;
  height: 100%;
}
</style>
