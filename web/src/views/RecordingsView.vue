<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { recordingsApi } from "../api/recordings";
import AppIcon from "../components/AppIcon.vue";
import SkeletonList from "../components/SkeletonList.vue";
import RecordingsTable from "../components/recordings/RecordingsTable.vue";
import type { RecordingFilters, RecordingSummary } from "../types/projection";

const route = useRoute();

const items = ref<RecordingSummary[]>([]);
const loading = ref(false);
const error = ref<string | null>(null);

// Recordings are private to their creator; the list is always the viewer's own.
const filters = computed<RecordingFilters>(() => {
  const f: RecordingFilters = {};
  if (typeof route.query.connection === "string")
    f.connection = route.query.connection;
  return f;
});

async function load(): Promise<void> {
  loading.value = true;
  error.value = null;
  try {
    items.value = await recordingsApi.list(filters.value);
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

onMounted(load);
watch(filters, load);

const hasItems = computed(() => items.value.length > 0);
</script>

<template>
  <div class="mx-auto flex h-full max-w-5xl flex-col gap-5 p-8">
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

    <RecordingsTable v-else :items="items" @changed="load" />
  </div>
</template>
