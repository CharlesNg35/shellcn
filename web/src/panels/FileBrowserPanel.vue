<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { fetchDoc, fetchPage } from "../api/dataSource";
import type { FileContent, FileEntry } from "../types/projection";
import type { PanelProps } from "./types";
import AppIcon from "../components/AppIcon.vue";
import FilePreview from "./file/FilePreview.vue";
import { formatBytes } from "./file/fileTypes";

const props = defineProps<PanelProps>();

const pathParam = computed(() => (props.config?.pathParam as string) ?? "path");
const readRouteId = computed(
  () => props.config?.readRouteId as string | undefined,
);

const startPath = (props.source?.params?.[pathParam.value] as string) ?? "/";
const cwd = ref(startPath);
const entries = ref<FileEntry[]>([]);
const loadingList = ref(false);
const listError = ref<string | null>(null);

const selected = ref<FileEntry | null>(null);
const content = ref<FileContent | null>(null);
const loadingContent = ref(false);

const sorted = computed(() =>
  [...entries.value].sort((a, b) => {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
    return a.name.localeCompare(b.name);
  }),
);

const crumbs = computed(() => {
  const parts = cwd.value.split("/").filter(Boolean);
  const acc: { label: string; path: string }[] = [{ label: "/", path: "/" }];
  let p = "";
  for (const part of parts) {
    p += `/${part}`;
    acc.push({ label: part, path: p });
  }
  return acc;
});

async function loadList(path: string): Promise<void> {
  if (!props.source) return;
  loadingList.value = true;
  listError.value = null;
  selected.value = null;
  content.value = null;
  try {
    const page = await fetchPage<FileEntry>(props.connectionId, {
      routeId: props.source.routeId,
      params: { ...props.source.params, [pathParam.value]: path },
    });
    entries.value = page.items;
    cwd.value = path;
  } catch (e) {
    listError.value = (e as Error).message;
  } finally {
    loadingList.value = false;
  }
}

async function openEntry(entry: FileEntry): Promise<void> {
  if (entry.isDir) {
    await loadList(entry.path);
    return;
  }
  selected.value = entry;
  if (!readRouteId.value) return;
  loadingContent.value = true;
  content.value = null;
  try {
    content.value = await fetchDoc<FileContent>(props.connectionId, {
      routeId: readRouteId.value,
      params: { [pathParam.value]: entry.path },
    });
  } finally {
    loadingContent.value = false;
  }
}

watch(
  () => props.connectionId,
  () => loadList(startPath),
  { immediate: true },
);
</script>

<template>
  <div class="flex h-full flex-col">
    <div
      class="flex items-center gap-1 overflow-x-auto border-b border-surface-200 px-3 py-2 text-sm dark:border-surface-800"
    >
      <template v-for="(c, i) in crumbs" :key="c.path">
        <span v-if="i > 0" class="text-surface-300">/</span>
        <button
          type="button"
          class="rounded px-1.5 py-0.5 text-surface-500 hover:bg-surface-100 hover:text-surface-800 dark:hover:bg-surface-800"
          @click="loadList(c.path)"
        >
          {{ c.label }}
        </button>
      </template>
    </div>

    <div class="flex min-h-0 flex-1">
      <div
        class="w-72 shrink-0 overflow-y-auto border-r border-surface-200 dark:border-surface-800"
      >
        <p v-if="loadingList" class="p-3 text-sm text-surface-400">Loading…</p>
        <p v-else-if="listError" class="p-3 text-sm text-red-500">
          {{ listError }}
        </p>
        <ul v-else>
          <li v-for="entry in sorted" :key="entry.path">
            <button
              type="button"
              class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm hover:bg-surface-100 dark:hover:bg-surface-800"
              :class="
                selected?.path === entry.path
                  ? 'bg-surface-100 dark:bg-surface-800'
                  : ''
              "
              @click="openEntry(entry)"
            >
              <AppIcon
                :icon="{ type: 'name', value: entry.isDir ? 'folder' : 'code' }"
                :size="15"
                class="shrink-0 text-surface-400"
              />
              <span
                class="flex-1 truncate text-surface-700 dark:text-surface-200"
                >{{ entry.name }}</span
              >
              <span v-if="!entry.isDir" class="text-xs text-surface-400">{{
                formatBytes(entry.size)
              }}</span>
            </button>
          </li>
        </ul>
      </div>

      <div class="min-w-0 flex-1">
        <FilePreview
          :name="selected?.name ?? ''"
          :content="content"
          :loading="loadingContent"
        />
      </div>
    </div>
  </div>
</template>
