<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import { ApiError } from "../api/client";
import { recordingsApi } from "../api/recordings";
import { useAuthStore } from "../stores/auth";
import { useNotify } from "../composables/useNotify";
import AppIcon from "../components/AppIcon.vue";
import SkeletonList from "../components/SkeletonList.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import RecordingPlayerDialog from "../components/recordings/RecordingPlayerDialog.vue";
import type { RecordingFilters, RecordingSummary } from "../types/projection";

const route = useRoute();
const auth = useAuthStore();
const notify = useNotify();

const items = ref<RecordingSummary[]>([]);
const loading = ref(false);
const error = ref<string | null>(null);

const playing = ref<RecordingSummary | null>(null);
const showPlayer = ref(false);
const deleteTarget = ref<RecordingSummary | null>(null);
const showDelete = ref(false);
const deleting = ref(false);

const filters = computed<RecordingFilters>(() => {
  const f: RecordingFilters = {};
  if (auth.isAdmin && typeof route.query.user === "string")
    f.user = route.query.user;
  if (typeof route.query.connection === "string")
    f.connection = route.query.connection;
  return f;
});

const heading = computed(() =>
  auth.isAdmin && filters.value.user && filters.value.user !== auth.user?.id
    ? "User Recordings"
    : auth.isAdmin
      ? "All Recordings"
      : "My Recordings",
);

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

function canDelete(r: RecordingSummary): boolean {
  return auth.isAdmin || r.userId === auth.user?.id;
}

function play(r: RecordingSummary): void {
  playing.value = r;
  showPlayer.value = true;
}

function download(r: RecordingSummary): void {
  window.open(recordingsApi.contentUrl(r.id), "_blank");
}

function openDelete(r: RecordingSummary): void {
  deleteTarget.value = r;
  showDelete.value = true;
}

async function onDelete(): Promise<void> {
  if (!deleteTarget.value) return;
  deleting.value = true;
  try {
    await recordingsApi.remove(deleteTarget.value.id);
    notify.success("Recording deleted");
    showDelete.value = false;
    await load();
  } catch (e) {
    if (e instanceof ApiError) notify.error("Could not delete", e.message);
  } finally {
    deleting.value = false;
  }
}

const statusSeverity: Record<string, string> = {
  finalized: "text-emerald-600 dark:text-emerald-400",
  active: "text-sky-600 dark:text-sky-400",
  pending: "text-amber-600 dark:text-amber-400",
  failed: "text-red-600 dark:text-red-400",
  discarded: "text-surface-400",
};

function classLabel(c: string): string {
  return c === "desktop" ? "Desktop" : "Terminal";
}

function formatBytes(n: number): string {
  if (!n) return "—";
  const units = ["B", "KB", "MB", "GB"];
  let v = n;
  let u = 0;
  while (v >= 1024 && u < units.length - 1) {
    v /= 1024;
    u++;
  }
  return `${v.toFixed(u === 0 ? 0 : 1)} ${units[u]}`;
}

function formatDuration(ms: number): string {
  if (!ms) return "—";
  const s = Math.round(ms / 1000);
  const m = Math.floor(s / 60);
  return m > 0 ? `${m}m ${s % 60}s` : `${s}s`;
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString();
}

const hasItems = computed(() => items.value.length > 0);
const playable = (r: RecordingSummary): boolean => r.status === "finalized";
</script>

<template>
  <div class="mx-auto flex h-full max-w-5xl flex-col gap-5 p-8">
    <h1 class="text-2xl font-semibold text-surface-900 dark:text-surface-0">
      {{ heading }}
    </h1>

    <p v-if="error" class="text-sm text-red-500">{{ error }}</p>
    <SkeletonList v-else-if="loading && !hasItems" :rows="6" />

    <div
      v-else-if="!hasItems"
      class="flex flex-col items-center gap-3 rounded-lg border border-dashed border-surface-300 py-16 text-center dark:border-surface-700"
    >
      <AppIcon
        :icon="{ type: 'name', value: 'video' }"
        :size="28"
        class="text-surface-400"
      />
      <p class="text-surface-500">No recordings yet.</p>
    </div>

    <DataTable v-else :value="items" scrollable scroll-height="flex">
      <Column header="Connection">
        <template #body="{ data }">
          <span class="font-medium text-surface-800 dark:text-surface-100">{{
            (data as RecordingSummary).connectionName ||
            (data as RecordingSummary).connectionId
          }}</span>
          <span class="block text-xs text-surface-400">{{
            (data as RecordingSummary).protocol
          }}</span>
        </template>
      </Column>
      <Column header="Type">
        <template #body="{ data }">
          <span
            class="inline-flex items-center gap-1.5 rounded-full bg-surface-100 px-2 py-0.5 text-xs text-surface-600 dark:bg-surface-800 dark:text-surface-300"
          >
            <AppIcon
              :icon="{
                type: 'name',
                value:
                  (data as RecordingSummary).class === 'desktop'
                    ? 'server'
                    : 'terminal',
              }"
              :size="12"
            />
            {{ classLabel((data as RecordingSummary).class) }}
          </span>
        </template>
      </Column>
      <Column v-if="auth.isAdmin" header="User">
        <template #body="{ data }">
          {{
            (data as RecordingSummary).username ||
            (data as RecordingSummary).userId
          }}
        </template>
      </Column>
      <Column header="Started">
        <template #body="{ data }">
          <span class="text-sm text-surface-600 dark:text-surface-300">{{
            formatTime((data as RecordingSummary).startedAt)
          }}</span>
        </template>
      </Column>
      <Column header="Duration">
        <template #body="{ data }">
          {{ formatDuration((data as RecordingSummary).durationMs) }}
        </template>
      </Column>
      <Column header="Size">
        <template #body="{ data }">
          {{ formatBytes((data as RecordingSummary).size) }}
        </template>
      </Column>
      <Column header="Status">
        <template #body="{ data }">
          <span
            class="text-xs font-medium capitalize"
            :class="statusSeverity[(data as RecordingSummary).status]"
            >{{ (data as RecordingSummary).status }}</span
          >
        </template>
      </Column>
      <Column header="" :pt="{ bodyCell: 'text-right' }">
        <template #body="{ data }">
          <div class="flex items-center justify-end gap-1">
            <button
              v-if="playable(data as RecordingSummary)"
              type="button"
              class="rounded p-1.5 text-surface-500 hover:bg-surface-100 hover:text-primary-600 dark:hover:bg-surface-800"
              title="Play"
              :aria-label="`Play recording`"
              @click="play(data as RecordingSummary)"
            >
              <AppIcon :icon="{ type: 'name', value: 'play' }" :size="16" />
            </button>
            <button
              v-if="playable(data as RecordingSummary)"
              type="button"
              class="rounded p-1.5 text-surface-500 hover:bg-surface-100 hover:text-surface-700 dark:hover:bg-surface-800"
              title="Download"
              :aria-label="`Download recording`"
              @click="download(data as RecordingSummary)"
            >
              <AppIcon :icon="{ type: 'name', value: 'download' }" :size="16" />
            </button>
            <button
              v-if="canDelete(data as RecordingSummary)"
              type="button"
              class="rounded p-1.5 text-surface-500 hover:bg-surface-100 hover:text-red-500 dark:hover:bg-surface-800"
              title="Delete"
              :aria-label="`Delete recording`"
              @click="openDelete(data as RecordingSummary)"
            >
              <AppIcon :icon="{ type: 'name', value: 'trash' }" :size="16" />
            </button>
          </div>
        </template>
      </Column>
      <template #empty>No recordings.</template>
    </DataTable>

    <RecordingPlayerDialog v-model:visible="showPlayer" :recording="playing" />
    <ConfirmDialog
      v-model:visible="showDelete"
      title="Delete recording"
      message="Delete this recording? The captured data cannot be recovered."
      confirm-label="Delete"
      danger
      :busy="deleting"
      @confirm="onDelete"
    />
  </div>
</template>
