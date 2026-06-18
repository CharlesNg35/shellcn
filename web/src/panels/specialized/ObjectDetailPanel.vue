<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import Panel from "primevue/panel";
import Button from "primevue/button";
import { fetchDoc } from "@/api/dataSource";
import type {
  ObjectDetailField,
  ObjectDetailPanelConfig,
  ObjectDetailSection,
  Row,
} from "@/types/projection";
import type { PanelProps } from "../core/types";
import PanelError from "../shared/PanelError.vue";
import SkeletonList from "@/components/SkeletonList.vue";
import AppIcon from "@/components/AppIcon.vue";
import CodeTextEditor from "../shared/CodeTextEditor.vue";
import ObjectDetailFieldRow from "./ObjectDetailFieldRow.vue";
import { formatValue, humanize, valueFor } from "./objectDetailFormat";
import { useRefreshableSource } from "../shared/useRefreshableSource";

const props = defineProps<PanelProps>();

const cfg = computed(
  () => (props.config as ObjectDetailPanelConfig | undefined) ?? {},
);
const copiedKey = ref<string | null>(null);
const mode = ref<"fields" | "raw">("fields");
let copiedTimer: ReturnType<typeof setTimeout> | undefined;

async function loadDetail(): Promise<unknown> {
  if (!props.source) return props.resource ?? {};
  return fetchDoc(props.connectionId, props.source, {
    resource: props.resource,
    record: props.record,
  });
}

const {
  data: doc,
  refreshing,
  error,
  showInitialLoader,
  blockingError,
  load,
  reset,
} = useRefreshableSource<unknown>(loadDetail, {
  initialValue: () => null,
});

function clearCopiedTimer(): void {
  if (copiedTimer) clearTimeout(copiedTimer);
  copiedTimer = undefined;
}

function record(): Row {
  return doc.value && typeof doc.value === "object" && !Array.isArray(doc.value)
    ? (doc.value as Row)
    : {};
}

const sections = computed<ObjectDetailSection[]>(() => {
  if (cfg.value.sections?.length) return cfg.value.sections;
  const fields = Object.keys(record())
    .filter((key) => !key.startsWith("_") && key !== "ref")
    .map((key) => ({ key, label: humanize(key) }));
  return [{ fields }];
});

function redactedRawValue(): unknown {
  const source = doc.value;
  if (!source || typeof source !== "object" || Array.isArray(source)) {
    return source ?? {};
  }
  const copy: Row = { ...(source as Row) };
  for (const section of sections.value) {
    for (const field of section.fields ?? []) {
      if (field.redacted && field.key in copy) copy[field.key] = "********";
    }
  }
  return copy;
}

const pretty = computed(() => JSON.stringify(redactedRawValue(), null, 2));

async function copy(field: ObjectDetailField): Promise<void> {
  const value = formatValue(valueFor(record(), field), field.type);
  if (!navigator.clipboard || value === "—") return;
  await navigator.clipboard.writeText(value);
  copiedKey.value = field.key;
  clearCopiedTimer();
  copiedTimer = window.setTimeout(() => {
    copiedKey.value = null;
  }, 1500);
}

watch(
  () => [
    props.connectionId,
    props.resource?.uid,
    props.source?.routeId,
    JSON.stringify(props.source?.params ?? {}),
    JSON.stringify(props.record ?? {}),
  ],
  () => {
    reset();
    void load();
  },
  {
    immediate: true,
  },
);
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
      <Button
        v-if="cfg.rawToggle"
        type="button"
        severity="secondary"
        :label="mode === 'fields' ? 'Raw' : 'Fields'"
        @click="mode = mode === 'fields' ? 'raw' : 'fields'"
      />
    </div>

    <div class="min-h-0 flex-1">
      <SkeletonList v-if="showInitialLoader" />
      <PanelError
        v-else-if="blockingError"
        :message="error ?? ''"
        retryable
        @retry="load"
      />
      <CodeTextEditor
        v-else-if="mode === 'raw'"
        :value="pretty"
        language="json"
        readonly
        aria-label="Raw object detail JSON"
      />
      <div v-else class="h-full overflow-auto p-4">
        <PanelError
          v-if="error"
          class="mb-4"
          :message="error"
          retryable
          @retry="load"
        />
        <div class="space-y-4">
          <Panel
            v-for="(section, index) in sections"
            :key="section.title ?? index"
            :header="section.title"
          >
            <dl class="divide-y divide-surface-100 dark:divide-surface-800">
              <ObjectDetailFieldRow
                v-for="field in section.fields ?? []"
                :key="field.key"
                :field="field"
                :record="record()"
                :copied="copiedKey === field.key"
                @copy="copy"
              />
            </dl>
          </Panel>
        </div>
      </div>
    </div>
  </div>
</template>
