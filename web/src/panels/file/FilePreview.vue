<script setup lang="ts">
import { computed } from "vue";
import Button from "primevue/button";
import SkeletonList from "../../components/SkeletonList.vue";
import type { FileContent } from "../../types/projection";
import FileCodeEditor from "./FileCodeEditor.vue";
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
  if (c.encoding === "utf8" && c.content && c.mime?.startsWith("image/")) {
    return `data:${c.mime};charset=utf-8,${encodeURIComponent(c.content)}`;
  }
  return c.url ?? "";
});
</script>

<template>
  <div class="flex h-full flex-col">
    <SkeletonList v-if="loading" />

    <template v-else-if="content">
      <FileCodeEditor
        v-if="viewer === 'code'"
        :name="name"
        :value="content.content ?? ''"
        readonly
      />

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
        <Button
          v-if="src"
          as="a"
          :href="src"
          :download="name"
          label="Download"
        />
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
