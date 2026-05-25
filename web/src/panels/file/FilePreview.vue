<script setup lang="ts">
import { computed } from "vue";
import type { FileContent } from "../../types/projection";
import { formatBytes, viewerFor } from "./fileTypes";

const props = defineProps<{
  name: string;
  content: FileContent | null;
  loading?: boolean;
}>();

const viewer = computed(() => viewerFor(props.name, props.content?.mime));

const src = computed(() => {
  const c = props.content;
  if (!c) return "";
  if (c.encoding === "url" && c.url) return c.url;
  if (c.encoding === "base64" && c.content)
    return `data:${c.mime ?? "application/octet-stream"};base64,${c.content}`;
  return c.url ?? "";
});
</script>

<template>
  <div class="flex h-full flex-col">
    <p v-if="loading" class="p-6 text-sm text-surface-400">Loading preview…</p>

    <template v-else-if="content">
      <pre
        v-if="viewer === 'code'"
        class="m-0 h-full overflow-auto whitespace-pre-wrap break-words p-4 font-mono text-xs leading-relaxed text-surface-700 dark:text-surface-200"
        >{{ content.content }}</pre
      >

      <div
        v-else-if="viewer === 'image'"
        class="flex h-full items-center justify-center overflow-auto p-4"
      >
        <img
          :src="src"
          :alt="name"
          class="max-h-full max-w-full object-contain"
        />
      </div>

      <iframe
        v-else-if="viewer === 'pdf'"
        :src="src"
        class="h-full w-full border-0"
        :title="name"
      />

      <div
        v-else-if="viewer === 'audio'"
        class="flex h-full items-center justify-center p-6"
      >
        <audio :src="src" controls />
      </div>

      <div
        v-else-if="viewer === 'video'"
        class="flex h-full items-center justify-center bg-black p-4"
      >
        <video :src="src" controls class="max-h-full max-w-full" />
      </div>

      <div
        v-else
        class="flex h-full flex-col items-center justify-center gap-2 p-6 text-center"
      >
        <p class="text-surface-600 dark:text-surface-300">
          No inline preview for this file.
        </p>
        <p class="text-sm text-surface-400">
          {{ name }} · {{ formatBytes(content.size) }}
        </p>
        <a
          v-if="src"
          :href="src"
          :download="name"
          class="rounded-md bg-primary-500 px-3 py-1.5 text-sm font-medium text-white"
          >Download</a
        >
      </div>

      <p
        v-if="content.truncated"
        class="border-t border-surface-200 px-4 py-1.5 text-xs text-amber-500 dark:border-surface-800"
      >
        Preview truncated — download for the full file.
      </p>
    </template>

    <div
      v-else
      class="flex h-full items-center justify-center p-6 text-sm text-surface-400"
    >
      Select a file to preview.
    </div>
  </div>
</template>
