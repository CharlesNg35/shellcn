<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { useElementSize } from "@vueuse/core";
import SkeletonList from "@/components/SkeletonList.vue";
import { useTheme } from "@/composables/useTheme";
import { DiffMode } from "@/types/projection";
import type { CodeMirrorDiffMode, CodeMirrorDiffView } from "@/codemirror";

const props = withDefaults(
  defineProps<{
    original: string;
    modified: string;
    language?: string;
    originalLabel?: string;
    modifiedLabel?: string;
    mode?: CodeMirrorDiffMode;
    collapseUnchanged?: boolean;
  }>(),
  {
    language: "plaintext",
    originalLabel: "Original",
    modifiedLabel: "Modified",
    mode: DiffMode.SideBySide,
    collapseUnchanged: false,
  },
);

const root = ref<HTMLElement | null>(null);
const host = ref<HTMLElement | null>(null);
const loading = ref(true);
const useFallback = ref(false);
const { isDark } = useTheme();
let diff: CodeMirrorDiffView | null = null;

// Two side-by-side editors below ~640px leave each pane too narrow to align
// changes, so the view falls back to unified until there is room again.
// A hidden or not-yet-measured pane reports 0, so the last real measurement is
// latched: the mode must not flip back and rebuild the editor on hide/show.
const { width } = useElementSize(root);
const measuredWidth = ref(0);
watch(width, (w) => {
  if (w > 0) measuredWidth.value = w;
});
const effectiveMode = computed<CodeMirrorDiffMode>(() =>
  measuredWidth.value > 0 && measuredWidth.value < 640
    ? DiffMode.Unified
    : props.mode,
);

async function mountDiff(): Promise<void> {
  await nextTick();
  if (!host.value) {
    useFallback.value = true;
    loading.value = false;
    return;
  }
  loading.value = true;
  try {
    const helpers = await import("@/codemirror");
    diff?.destroy();
    host.value.replaceChildren();
    diff = helpers.createCodeMirrorDiffView(host.value, {
      original: props.original,
      modified: props.modified,
      language: props.language,
      mode: effectiveMode.value,
      collapseUnchanged: props.collapseUnchanged,
    });
    useFallback.value = false;
  } catch {
    useFallback.value = true;
  } finally {
    loading.value = false;
  }
}

onMounted(mountDiff);

watch(
  () =>
    [
      props.original,
      props.modified,
      props.language,
      effectiveMode.value,
      props.collapseUnchanged,
    ] as const,
  mountDiff,
);

watch(isDark, () => diff?.syncTheme());

onUnmounted(() => {
  diff?.destroy();
});
</script>

<template>
  <div ref="root" class="flex h-full min-h-0 flex-col">
    <div
      class="grid shrink-0 grid-cols-2 border-b border-surface-200 bg-surface-0 text-xs font-medium text-surface-500 dark:border-surface-800 dark:bg-surface-950 dark:text-surface-400"
      :class="{ 'grid-cols-1': effectiveMode === DiffMode.Unified }"
    >
      <div class="truncate px-3 py-2">
        {{ effectiveMode === DiffMode.Unified ? modifiedLabel : originalLabel }}
      </div>
      <div
        v-if="effectiveMode !== DiffMode.Unified"
        class="truncate border-l border-surface-200 px-3 py-2 dark:border-surface-800"
      >
        {{ modifiedLabel }}
      </div>
    </div>
    <SkeletonList v-if="loading" :rows="8" />
    <div
      v-else-if="useFallback"
      class="grid min-h-0 flex-1 grid-cols-2 overflow-hidden text-xs"
      :class="{ 'grid-cols-1': effectiveMode === DiffMode.Unified }"
    >
      <pre
        class="m-0 overflow-auto bg-surface-0 p-4 font-mono leading-relaxed whitespace-pre-wrap text-surface-700 dark:bg-surface-950 dark:text-surface-200"
        >{{ effectiveMode === DiffMode.Unified ? modified : original }}</pre
      >
      <pre
        v-if="effectiveMode !== DiffMode.Unified"
        class="m-0 overflow-auto border-l border-surface-200 bg-surface-0 p-4 font-mono leading-relaxed whitespace-pre-wrap text-surface-700 dark:border-surface-800 dark:bg-surface-950 dark:text-surface-200"
        >{{ modified }}</pre
      >
    </div>
    <div
      v-show="!loading && !useFallback"
      ref="host"
      class="shellcn-diff-host min-h-0 flex-1"
    />
  </div>
</template>
