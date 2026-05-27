<script setup lang="ts">
import { computed, ref, useAttrs } from "vue";
import DOMPurify from "dompurify";
import { FALLBACK_ICON, resolveLucideIcon } from "./lucideIconRegistry";
import type { Icon } from "../types/projection";

defineOptions({ inheritAttrs: false });

const attrs = useAttrs();

const props = withDefaults(
  defineProps<{
    icon?: Icon | null;
    size?: number;
    fallback?: string;
    loading?: boolean;
  }>(),
  { icon: null, size: 18, fallback: FALLBACK_ICON, loading: false },
);

const imgFailed = ref(false);

// Bound inline SVG markup so a hostile manifest can't ship a huge payload.
const MAX_SVG_BYTES = 64 * 1024;

const resolvedIcon = computed<Icon | null>(() =>
  props.loading
    ? { type: "lucide", value: "loader-circle" }
    : (props.icon ?? null),
);

const kind = computed(() => {
  const t = resolvedIcon.value?.type;
  const v = resolvedIcon.value?.value;
  if (!t || !v) return "glyph";
  if ((t === "url" || t === "base64") && !imgFailed.value) {
    const safe =
      t === "base64" ? v.startsWith("data:image/") : v.startsWith("https://");
    return safe ? "image" : "glyph";
  }
  if (t === "emoji") return "emoji";
  if (t === "svg") return safeSvg.value ? "svg" : "glyph";
  return "glyph";
});

// A glyph's value is a Lucide name. resolveLucideIcon falls back to a
// placeholder for an empty or unknown name.
const glyphComponent = computed(() =>
  resolveLucideIcon(
    resolvedIcon.value?.type === "lucide"
      ? resolvedIcon.value.value
      : props.fallback,
  ),
);

// Sanitize raw inline SVG (svg profile only — no HTML/MathML, scripts and event
// handlers stripped) before it is ever injected into the DOM.
const safeSvg = computed(() => {
  const v =
    !props.loading && props.icon?.type === "svg" ? props.icon.value : "";
  if (!v || v.length > MAX_SVG_BYTES) return "";
  return DOMPurify.sanitize(v, {
    USE_PROFILES: { svg: true, svgFilters: true },
  });
});
</script>

<template>
  <span
    v-bind="attrs"
    class="inline-flex shrink-0 items-center justify-center"
    :style="{ width: `${size}px`, height: `${size}px` }"
  >
    <component
      :is="glyphComponent"
      v-if="kind === 'glyph'"
      :size="size"
      :stroke-width="2"
      :class="{ 'animate-spin': loading }"
      aria-hidden="true"
    />
    <span
      v-else-if="kind === 'emoji'"
      :style="{ fontSize: `${size}px`, lineHeight: 1 }"
      role="img"
    >
      {{ resolvedIcon?.value }}
    </span>
    <!-- eslint-disable vue/no-v-html -- safeSvg is sanitized with DOMPurify's SVG profile. -->
    <span
      v-else-if="kind === 'svg'"
      class="app-icon-svg inline-flex"
      :style="{ width: `${size}px`, height: `${size}px` }"
      aria-hidden="true"
      v-html="safeSvg"
    />
    <!-- eslint-enable vue/no-v-html -->
    <img
      v-else-if="kind === 'image'"
      :src="resolvedIcon?.value"
      :width="size"
      :height="size"
      alt=""
      class="object-contain"
      :style="{ maxWidth: `${size}px`, maxHeight: `${size}px` }"
      @error="imgFailed = true"
    />
  </span>
</template>

<style scoped>
.app-icon-svg :deep(svg) {
  width: 100%;
  height: 100%;
}
</style>
