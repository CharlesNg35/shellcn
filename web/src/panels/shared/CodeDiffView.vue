<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import SkeletonList from "@/components/SkeletonList.vue";
import { useTheme } from "@/composables/useTheme";
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
    mode: "side_by_side",
    collapseUnchanged: false,
  },
);

const host = ref<HTMLElement | null>(null);
const loading = ref(true);
const useFallback = ref(false);
const { isDark } = useTheme();
let diff: CodeMirrorDiffView | null = null;

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
      mode: props.mode,
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
      props.mode,
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
  <div class="flex h-full min-h-0 flex-col">
    <div
      class="grid shrink-0 grid-cols-2 border-b border-surface-200 bg-surface-0 text-xs font-medium text-surface-500 dark:border-surface-800 dark:bg-surface-950 dark:text-surface-400"
      :class="{ 'grid-cols-1': mode === 'unified' }"
    >
      <div class="truncate px-3 py-2">
        {{ mode === "unified" ? modifiedLabel : originalLabel }}
      </div>
      <div
        v-if="mode !== 'unified'"
        class="truncate border-l border-surface-200 px-3 py-2 dark:border-surface-800"
      >
        {{ modifiedLabel }}
      </div>
    </div>
    <SkeletonList v-if="loading" :rows="8" />
    <div
      v-else-if="useFallback"
      class="grid min-h-0 flex-1 grid-cols-2 overflow-hidden text-xs"
      :class="{ 'grid-cols-1': mode === 'unified' }"
    >
      <pre
        class="m-0 overflow-auto bg-surface-0 p-4 font-mono leading-relaxed whitespace-pre-wrap text-surface-700 dark:bg-surface-950 dark:text-surface-200"
        >{{ mode === "unified" ? modified : original }}</pre
      >
      <pre
        v-if="mode !== 'unified'"
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
