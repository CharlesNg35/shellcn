<script setup lang="ts">
import {
  computed,
  onActivated,
  onDeactivated,
  ref,
  watch as vueWatch,
} from "vue";
import { useDocumentVisibility, useIntervalFn } from "@vueuse/core";
import Timeline from "primevue/timeline";
import Button from "primevue/button";
import { fetchPage } from "@/api/dataSource";
import type { PanelProps } from "../core/types";
import type { Row, TimelinePanelConfig } from "@/types/projection";
import PanelError from "../shared/PanelError.vue";
import SkeletonList from "@/components/SkeletonList.vue";
import AppIcon from "@/components/AppIcon.vue";
import { badgeClassFor } from "../shared/severity";

const props = defineProps<PanelProps>();

const cfg = computed(
  () => (props.config as TimelinePanelConfig | undefined) ?? {},
);
const rows = ref<Row[]>([]);
const loadedOnce = ref(false);
const refreshing = ref(false);
const error = ref<string | null>(null);
const active = ref(true);
const visibility = useDocumentVisibility();

const timestampField = computed(() => cfg.value.timestampField ?? "time");
const titleField = computed(() => cfg.value.titleField ?? "title");
const bodyField = computed(() => cfg.value.bodyField ?? "body");
const severityField = computed(() => cfg.value.severityField ?? "severity");
const iconField = computed(() => cfg.value.iconField ?? "icon");
const emptyText = computed(() => cfg.value.emptyText ?? "No events.");
const refreshMs = computed(() => cfg.value.refreshIntervalMs ?? 0);
const showInitialLoader = computed(() => refreshing.value && !loadedOnce.value);

async function load(): Promise<void> {
  if (!props.source) {
    loadedOnce.value = true;
    return;
  }
  if (refreshing.value) return;
  refreshing.value = true;
  error.value = null;
  try {
    const page = await fetchPage<Row>(
      props.connectionId,
      props.source,
      { resource: props.resource },
      { limit: 100 },
    );
    rows.value = page.items;
    loadedOnce.value = true;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    refreshing.value = false;
  }
}

function text(row: Row, key: string): string {
  const value = row[key];
  if (value === undefined || value === null) return "";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

function timeText(row: Row): string {
  const value = text(row, timestampField.value);
  if (!value) return "";
  const ts = Date.parse(value);
  return Number.isNaN(ts) ? value : new Date(ts).toLocaleString();
}

const { pause, resume } = useIntervalFn(load, () => refreshMs.value || 1000, {
  immediate: false,
});

onActivated(() => {
  active.value = true;
});

onDeactivated(() => {
  active.value = false;
});

vueWatch(
  () => [props.connectionId, props.resource?.uid, props.source?.routeId],
  () => {
    rows.value = [];
    loadedOnce.value = false;
    void load();
  },
  { immediate: true },
);

vueWatch(
  () => refreshMs.value > 0 && active.value && visibility.value === "visible",
  (on) => {
    if (!on) {
      pause();
      return;
    }
    resume();
  },
  { immediate: true },
);
</script>

<template>
  <div class="flex h-full flex-col">
    <div
      class="flex items-center justify-end border-b border-surface-200 px-3 py-2 dark:border-surface-800"
    >
      <Button
        type="button"
        severity="secondary"
        :disabled="refreshing"
        @click="load"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'refresh-cw' }"
          :size="14"
          :loading="refreshing"
        />
        Refresh
      </Button>
    </div>

    <div class="min-h-0 flex-1 overflow-auto p-4">
      <SkeletonList v-if="showInitialLoader" />
      <PanelError v-else-if="error" :message="error" retryable @retry="load" />
      <div v-else-if="!rows.length" class="text-sm text-surface-500">
        {{ emptyText }}
      </div>
      <Timeline v-else :value="rows">
        <template #opposite="{ item }">
          {{ timeText(item as Row) }}
        </template>
        <template #marker="{ item }">
          <AppIcon
            :icon="{
              type: 'lucide',
              value: text(item as Row, iconField) || 'circle',
            }"
            :size="13"
          />
        </template>
        <template #content="{ item }">
          <article class="min-w-0">
            <div class="flex min-w-0 items-center gap-2">
              <h3
                class="min-w-0 truncate text-sm font-medium text-surface-900 dark:text-surface-0"
              >
                {{ text(item as Row, titleField) || "Event" }}
              </h3>
              <span
                v-if="text(item as Row, severityField)"
                class="rounded-full px-2 py-0.5 text-xs"
                :class="
                  badgeClassFor(undefined, text(item as Row, severityField))
                "
                >{{ text(item as Row, severityField) }}</span
              >
            </div>
            <p
              v-if="text(item as Row, bodyField)"
              class="mt-1 text-sm leading-relaxed whitespace-pre-wrap text-surface-600 dark:text-surface-300"
            >
              {{ text(item as Row, bodyField) }}
            </p>
          </article>
        </template>
      </Timeline>
    </div>
  </div>
</template>
