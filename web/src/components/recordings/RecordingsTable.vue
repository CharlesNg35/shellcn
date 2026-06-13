<script setup lang="ts">
import { ref } from "vue";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Button from "primevue/button";
import { ApiError } from "@/api/client";
import { recordingsApi } from "@/api/recordings";
import { useAuthStore } from "@/stores/auth";
import { useNotify } from "@/composables/useNotify";
import { useConfirmAction } from "@/composables/useConfirmAction";
import AppIcon from "../AppIcon.vue";
import RecordingPlayerDialog from "./RecordingPlayerDialog.vue";
import {
  IconType,
  RecordingClass,
  RecordingStatus,
  type RecordingSummary,
} from "@/types/projection";

defineProps<{ items: RecordingSummary[] }>();
const emit = defineEmits<{ changed: [] }>();

const auth = useAuthStore();
const notify = useNotify();
const { confirmDanger } = useConfirmAction();

const playing = ref<RecordingSummary | null>(null);
const showPlayer = ref(false);

const playable = (r: RecordingSummary): boolean =>
  r.status === RecordingStatus.Finalized;
const canDelete = (r: RecordingSummary): boolean => r.userId === auth.user?.id;

function play(r: RecordingSummary): void {
  playing.value = r;
  showPlayer.value = true;
}

function download(r: RecordingSummary): void {
  window.open(recordingsApi.contentUrl(r.id), "_blank");
}

function openDelete(r: RecordingSummary): void {
  confirmDanger({
    header: "Delete recording",
    message: "Delete this recording? This cannot be undone.",
    accept: () => onDelete(r),
  });
}

async function onDelete(r: RecordingSummary): Promise<void> {
  try {
    await recordingsApi.remove(r.id);
    notify.success("Recording deleted");
    emit("changed");
  } catch (e) {
    if (e instanceof ApiError) notify.error("Could not delete", e.message);
  }
}

const statusSeverity: Record<RecordingStatus, string> = {
  [RecordingStatus.Finalized]: "text-emerald-600 dark:text-emerald-400",
  [RecordingStatus.Active]: "text-sky-600 dark:text-sky-400",
  [RecordingStatus.Pending]: "text-amber-600 dark:text-amber-400",
  [RecordingStatus.Failed]: "text-rose-600 dark:text-rose-300",
  [RecordingStatus.Discarded]: "text-surface-400",
};

function classLabel(c: RecordingClass): string {
  return c === RecordingClass.Desktop ? "Desktop" : "Terminal";
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
</script>

<template>
  <DataTable :value="items" scrollable scroll-height="flex">
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
              type: IconType.Lucide,
              value:
                (data as RecordingSummary).class === RecordingClass.Desktop
                  ? 'server'
                  : 'terminal',
            }"
            :size="12"
          />
          {{ classLabel((data as RecordingSummary).class) }}
        </span>
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
          <Button
            v-if="playable(data as RecordingSummary)"
            text
            rounded
            severity="secondary"
            size="small"
            title="Play"
            aria-label="Play recording"
            @click="play(data as RecordingSummary)"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'play' }" :size="16" />
          </Button>
          <Button
            v-if="playable(data as RecordingSummary)"
            text
            rounded
            severity="secondary"
            size="small"
            title="Download"
            aria-label="Download recording"
            @click="download(data as RecordingSummary)"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'download' }" :size="16" />
          </Button>
          <Button
            v-if="canDelete(data as RecordingSummary)"
            text
            rounded
            severity="danger"
            size="small"
            title="Delete"
            aria-label="Delete recording"
            @click="openDelete(data as RecordingSummary)"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'trash' }" :size="16" />
          </Button>
        </div>
      </template>
    </Column>
    <template #empty>No recordings.</template>
  </DataTable>

  <RecordingPlayerDialog v-model:visible="showPlayer" :recording="playing" />
</template>
