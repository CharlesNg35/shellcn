<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import Button from "primevue/button";
import { RECORDINGS_PAGE_SIZE, recordingsApi } from "../api/recordings";
import AppIcon from "../components/AppIcon.vue";
import AppPage from "../components/AppPage.vue";
import SkeletonList from "../components/SkeletonList.vue";
import RecordingsTable from "../components/recordings/RecordingsTable.vue";
import type { RecordingFilters, RecordingSummary } from "../types/projection";

// Ceiling on the window the view widens a page at a time.
const MAX_LOADED = 2000;

const route = useRoute();

const items = ref<RecordingSummary[]>([]);
const windowSize = ref(RECORDINGS_PAGE_SIZE);
const loading = ref(false);
const error = ref<string | null>(null);

// Recordings are private to their creator; the list is always the viewer's own.
const filters = computed<RecordingFilters>(() => {
  const f: RecordingFilters = {};
  if (typeof route.query.connection === "string")
    f.connection = route.query.connection;
  return f;
});

const hasItems = computed(() => items.value.length > 0);
const truncated = computed(() => items.value.length >= windowSize.value);
const canLoadMore = computed(
  () => truncated.value && windowSize.value < MAX_LOADED,
);
const countLabel = computed(() => {
  const count = items.value.length;
  const noun = count === 1 ? "recording" : "recordings";
  return truncated.value ? `First ${count} ${noun}` : `${count} ${noun}`;
});

async function load(): Promise<void> {
  loading.value = true;
  error.value = null;
  try {
    items.value =
      windowSize.value === RECORDINGS_PAGE_SIZE
        ? await recordingsApi.list(filters.value)
        : await recordingsApi.list(filters.value, windowSize.value);
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

function reload(): void {
  windowSize.value = RECORDINGS_PAGE_SIZE;
  void load();
}

function loadMore(): void {
  if (!canLoadMore.value || loading.value) return;
  windowSize.value += RECORDINGS_PAGE_SIZE;
  void load();
}

onMounted(load);
watch(filters, reload);
</script>

<template>
  <AppPage>
    <h1 class="text-2xl font-semibold text-surface-900 dark:text-surface-0">
      My Recordings
    </h1>

    <p v-if="error" class="text-sm text-red-500">{{ error }}</p>
    <SkeletonList v-else-if="loading && !hasItems" :rows="6" />

    <div
      v-else-if="!hasItems"
      class="flex flex-col items-center gap-3 rounded-lg border border-dashed border-surface-300 py-16 text-center dark:border-surface-700"
    >
      <AppIcon
        :icon="{ type: 'lucide', value: 'video' }"
        :size="28"
        class="text-surface-400"
      />
      <p class="text-surface-500">No recordings yet.</p>
    </div>

    <template v-else>
      <RecordingsTable :items="items" @changed="load" />
      <div class="flex items-center gap-3 text-xs text-surface-400">
        <span class="tabular-nums">{{ countLabel }}</span>
        <Button
          v-if="canLoadMore"
          type="button"
          text
          size="small"
          class="ml-auto"
          :label="loading ? 'Loading…' : 'Load more'"
          :disabled="loading"
          @click="loadMore"
        />
        <span v-else-if="truncated" class="ml-auto">
          listing capped — filter by connection to narrow
        </span>
      </div>
    </template>
  </AppPage>
</template>
