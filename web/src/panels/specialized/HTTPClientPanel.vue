<script setup lang="ts">
import { computed, ref } from "vue";
import Button from "primevue/button";
import InputText from "primevue/inputtext";
import Select from "primevue/select";
import Textarea from "primevue/textarea";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import { useToast } from "primevue/usetoast";
import { runFormAction } from "../../api/dataSource";
import type { HTTPClientConfig } from "../../types/projection";
import AppIcon from "../../components/AppIcon.vue";
import type { PanelProps } from "../core/types";
import PanelError from "../shared/PanelError.vue";

interface HeaderRow {
  key: string;
  value: string;
}

interface HTTPResponse {
  status?: number;
  statusText?: string;
  durationMs?: number;
  headers?: HeaderRow[] | Record<string, string>;
  body?: unknown;
}

const props = defineProps<PanelProps>();
const toast = useToast();

const config = computed(() => props.config as HTTPClientConfig | undefined);
const methods = computed(() =>
  (config.value?.methods?.length
    ? config.value.methods
    : ["GET", "POST", "PUT", "PATCH", "DELETE"]
  ).map((method) => ({
    label: method,
    value: method,
  })),
);

const method = ref(config.value?.defaultMethod ?? "GET");
const url = ref(config.value?.defaultUrl ?? "");
const headers = ref<HeaderRow[]>(
  config.value?.defaultHeaders?.length
    ? config.value.defaultHeaders.map((h) => ({ ...h }))
    : [{ key: "Content-Type", value: "application/json" }],
);
const body = ref(config.value?.defaultBody ?? "");
const response = ref<HTTPResponse | null>(null);
const loading = ref(false);
const error = ref<string | null>(null);

const responseBody = computed(() => {
  if (!response.value) return "";
  return typeof response.value.body === "string"
    ? response.value.body
    : JSON.stringify(response.value.body ?? null, null, 2);
});

const responseHeaders = computed<HeaderRow[]>(() => {
  const raw = response.value?.headers;
  if (!raw) return [];
  if (Array.isArray(raw)) return raw;
  return Object.entries(raw).map(([key, value]) => ({ key, value }));
});

function addHeader(): void {
  headers.value = [...headers.value, { key: "", value: "" }];
}

function removeHeader(index: number): void {
  headers.value = headers.value.filter((_, i) => i !== index);
}

async function send(): Promise<void> {
  const routeId = config.value?.executeRouteId ?? props.source?.routeId;
  if (!routeId) {
    error.value = "No execute route configured.";
    return;
  }
  loading.value = true;
  error.value = null;
  try {
    response.value = (await runFormAction(
      props.connectionId,
      routeId,
      { resource: props.resource },
      {
        method: method.value,
        url: url.value,
        headers: headers.value.filter((header) => header.key.trim()),
        body: body.value,
      },
      props.source?.params ?? {},
      "POST",
    )) as HTTPResponse;
  } catch (e) {
    error.value = (e as Error).message;
    toast.add({
      severity: "error",
      summary: "Request failed",
      detail: error.value,
      life: 4500,
    });
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div
      class="flex items-center gap-2 border-b border-surface-200 p-3 dark:border-surface-800"
    >
      <Select
        v-model="method"
        :options="methods"
        option-label="label"
        option-value="value"
        class="w-32"
      />
      <InputText
        v-model="url"
        placeholder="Path or URL"
        aria-label="Request URL"
        class="min-w-0 flex-1"
      />
      <Button type="button" :disabled="loading" @click="send">
        {{ loading ? "Sending..." : "Send" }}
      </Button>
    </div>

    <div class="grid min-h-0 flex-1 grid-cols-2">
      <section
        class="flex min-h-0 flex-col border-r border-surface-200 dark:border-surface-800"
        aria-label="Request"
      >
        <div class="border-b border-surface-200 p-3 dark:border-surface-800">
          <div class="mb-2 flex items-center justify-between">
            <h2
              class="text-sm font-medium text-surface-700 dark:text-surface-200"
            >
              Headers
            </h2>
            <Button
              type="button"
              size="small"
              severity="secondary"
              @click="addHeader"
            >
              Add header
            </Button>
          </div>
          <div class="space-y-2">
            <div
              v-for="(header, index) in headers"
              :key="index"
              class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2.25rem] gap-2"
            >
              <InputText v-model="header.key" placeholder="Header" />
              <InputText v-model="header.value" placeholder="Value" />
              <Button
                type="button"
                text
                rounded
                severity="secondary"
                aria-label="Remove header"
                @click="removeHeader(index)"
              >
                <AppIcon :icon="{ type: 'name', value: 'x' }" :size="14" />
              </Button>
            </div>
          </div>
        </div>
        <div class="flex min-h-0 flex-1 flex-col p-3">
          <label
            class="mb-2 text-sm font-medium text-surface-700 dark:text-surface-200"
          >
            Body
          </label>
          <Textarea
            v-model="body"
            class="min-h-0 flex-1 font-mono text-xs"
            aria-label="Request body"
          />
        </div>
      </section>

      <section class="flex min-h-0 flex-col" aria-label="Response">
        <div
          class="flex items-center justify-between border-b border-surface-200 px-3 py-2 dark:border-surface-800"
        >
          <div class="flex items-center gap-2">
            <span
              class="rounded px-2 py-0.5 text-xs font-medium"
              :class="
                response?.status && response.status >= 400
                  ? 'bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300'
                  : 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300'
              "
            >
              {{ response?.status ?? "No response" }}
            </span>
            <span
              v-if="response?.durationMs != null"
              class="text-xs text-surface-400"
            >
              {{ response.durationMs.toFixed(1) }} ms
            </span>
          </div>
          <span v-if="error && response" class="text-xs text-red-500">{{
            error
          }}</span>
        </div>
        <PanelError
          v-if="error && !response"
          :message="error"
          retryable
          @retry="send"
        />
        <template v-else>
          <DataTable
            v-if="responseHeaders.length"
            :value="responseHeaders"
            scrollable
            scroll-height="10rem"
          >
            <Column field="key" header="Header" />
            <Column field="value" header="Value" />
          </DataTable>
          <Textarea
            :model-value="responseBody"
            readonly
            class="min-h-0 flex-1 rounded-none border-0 font-mono text-xs"
            aria-label="Response body"
          />
        </template>
      </section>
    </div>
  </div>
</template>
