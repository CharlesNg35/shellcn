<script setup lang="ts">
import { computed } from "vue";
import Button from "primevue/button";
import SkeletonList from "../../components/SkeletonList.vue";
import type { FileContent } from "../../types/projection";
import CodeTextEditor from "../shared/CodeTextEditor.vue";
import PanelError from "../shared/PanelError.vue";
import { formatBytes, languageFor, viewerFor } from "./fileTypes";

const props = withDefaults(
  defineProps<{
    name: string;
    content: FileContent | null;
    streamSrc?: string;
    loading?: boolean;
    error?: string | null;
  }>(),
  { streamSrc: "", loading: false, error: null },
);

const emit = defineEmits<{ retry: [] }>();

const viewer = computed(() => viewerFor(props.name, props.content?.mime));
const codeLanguage = computed(() => languageFor(props.name));
const showTruncatedNotice = computed(
  () => props.content?.truncated === true && viewer.value === "code",
);

const src = computed(() => props.streamSrc || props.content?.url || "");
</script>

<template>
  <div class="flex h-full flex-col">
    <SkeletonList v-if="loading" />
    <PanelError
      v-else-if="error"
      :message="error"
      retryable
      @retry="emit('retry')"
    />

    <template v-else-if="content">
      <CodeTextEditor
        v-if="viewer === 'code'"
        :value="content.content ?? ''"
        :language="codeLanguage"
        readonly
        :aria-label="name ? `${name} preview` : 'File preview'"
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
        v-if="showTruncatedNotice"
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
