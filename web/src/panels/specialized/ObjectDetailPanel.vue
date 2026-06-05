<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import Panel from "primevue/panel";
import Button from "primevue/button";
import { fetchDoc } from "../../api/dataSource";
import type {
  ColumnType,
  ObjectDetailField,
  ObjectDetailPanelConfig,
  ObjectDetailSection,
  Row,
} from "../../types/projection";
import type { PanelProps } from "../core/types";
import PanelError from "../shared/PanelError.vue";
import SkeletonList from "../../components/SkeletonList.vue";
import AppIcon from "../../components/AppIcon.vue";
import CodeTextEditor from "../shared/CodeTextEditor.vue";
import { badgeClassFor } from "../shared/severity";

const props = defineProps<PanelProps>();

const cfg = computed(
  () => (props.config as ObjectDetailPanelConfig | undefined) ?? {},
);
const doc = ref<unknown>(null);
const loading = ref(true);
const error = ref<string | null>(null);
const copiedKey = ref<string | null>(null);
const mode = ref<"fields" | "raw">("fields");
let copiedTimer: ReturnType<typeof setTimeout> | undefined;

function clearCopiedTimer(): void {
  if (copiedTimer) clearTimeout(copiedTimer);
  copiedTimer = undefined;
}

function record(): Row {
  return doc.value && typeof doc.value === "object" && !Array.isArray(doc.value)
    ? (doc.value as Row)
    : {};
}

function humanize(key: string): string {
  const spaced = key
    .replace(/[_-]+/g, " ")
    .replace(/([a-z\d])([A-Z])/g, "$1 $2");
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

const sections = computed<ObjectDetailSection[]>(() => {
  if (cfg.value.sections?.length) return cfg.value.sections;
  const fields = Object.keys(record())
    .filter((key) => !key.startsWith("_") && key !== "ref")
    .map((key) => ({ key, label: humanize(key) }));
  return [{ fields }];
});

const pretty = computed(() => JSON.stringify(doc.value ?? {}, null, 2));

async function load(): Promise<void> {
  if (!props.source) {
    doc.value = props.resource ?? {};
    loading.value = false;
    return;
  }
  loading.value = true;
  error.value = null;
  try {
    doc.value = await fetchDoc(props.connectionId, props.source, {
      resource: props.resource,
    });
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

function valueFor(field: ObjectDetailField): unknown {
  return record()[field.key];
}

function formatBytes(value: number): string {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let n = value;
  let i = 0;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i += 1;
  }
  return `${n.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function formatValue(value: unknown, type?: ColumnType): string {
  if (value === undefined || value === null || value === "") return "—";
  if (type === "bytes" && typeof value === "number") return formatBytes(value);
  if (type === "datetime" && typeof value === "string")
    return new Date(value).toLocaleString();
  if (type === "json" || typeof value === "object")
    return JSON.stringify(value, null, 2);
  return String(value);
}

async function copy(field: ObjectDetailField): Promise<void> {
  const value = formatValue(valueFor(field), field.type);
  if (!navigator.clipboard || value === "—") return;
  await navigator.clipboard.writeText(value);
  copiedKey.value = field.key;
  clearCopiedTimer();
  copiedTimer = window.setTimeout(() => {
    copiedKey.value = null;
  }, 1500);
}

watch(() => [props.connectionId, props.resource?.uid], load, {
  immediate: true,
});
onUnmounted(clearCopiedTimer);
</script>

<template>
  <div class="flex h-full flex-col">
    <div
      class="flex items-center justify-between gap-2 border-b border-surface-200 px-3 py-2 dark:border-surface-800"
    >
      <Button
        type="button"
        severity="secondary"
        :disabled="loading"
        @click="load"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'refresh-cw' }"
          :size="14"
          :loading="loading"
        />
        Refresh
      </Button>
      <Button
        v-if="cfg.rawToggle"
        type="button"
        severity="secondary"
        :label="mode === 'fields' ? 'Raw' : 'Fields'"
        @click="mode = mode === 'fields' ? 'raw' : 'fields'"
      />
    </div>

    <div class="min-h-0 flex-1">
      <SkeletonList v-if="loading" />
      <PanelError v-else-if="error" :message="error" retryable @retry="load" />
      <CodeTextEditor
        v-else-if="mode === 'raw'"
        :value="pretty"
        language="json"
        readonly
        aria-label="Raw object detail JSON"
      />
      <div v-else class="h-full overflow-auto p-4">
        <div class="space-y-4">
          <Panel
            v-for="(section, index) in sections"
            :key="section.title ?? index"
            :header="section.title"
          >
            <dl class="divide-y divide-surface-100 dark:divide-surface-800">
              <div
                v-for="field in section.fields ?? []"
                :key="field.key"
                class="grid grid-cols-[minmax(8rem,14rem)_1fr_auto] items-start gap-3 px-4 py-2.5 text-sm"
              >
                <dt class="text-surface-500 dark:text-surface-400">
                  {{ field.label ?? humanize(field.key) }}
                </dt>
                <dd
                  class="min-w-0 wrap-break-word whitespace-pre-wrap text-surface-900 dark:text-surface-100"
                >
                  <span v-if="field.redacted" class="font-mono text-surface-400"
                    >********</span
                  >
                  <span
                    v-else-if="field.type === 'badge'"
                    class="inline-block max-w-full truncate rounded-full px-2 py-0.5 align-bottom text-xs"
                    :class="badgeClassFor(field.severities, valueFor(field))"
                    >{{ formatValue(valueFor(field), field.type) }}</span
                  >
                  <span v-else>{{
                    formatValue(valueFor(field), field.type)
                  }}</span>
                </dd>
                <Button
                  v-if="field.copy && !field.redacted"
                  type="button"
                  text
                  rounded
                  severity="secondary"
                  :title="copiedKey === field.key ? 'Copied' : 'Copy value'"
                  :aria-label="
                    copiedKey === field.key ? 'Copied' : 'Copy value'
                  "
                  @click="copy(field)"
                >
                  <AppIcon
                    :icon="{
                      type: 'lucide',
                      value: copiedKey === field.key ? 'check' : 'copy',
                    }"
                    :size="14"
                  />
                </Button>
              </div>
            </dl>
          </Panel>
        </div>
      </div>
    </div>
  </div>
</template>
