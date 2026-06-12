<script setup lang="ts">
import { computed, ref } from "vue";
import Button from "primevue/button";
import InputText from "primevue/inputtext";
import Select from "primevue/select";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Tag from "primevue/tag";
import { useToast } from "primevue/usetoast";
import { runFormAction } from "@/api/dataSource";
import type { HTTPClientConfig } from "@/types/projection";
import AppIcon from "@/components/AppIcon.vue";
import SkeletonList from "@/components/SkeletonList.vue";
import type { PanelProps } from "../core/types";
import CodeTextEditor from "../shared/CodeTextEditor.vue";
import PanelError from "../shared/PanelError.vue";
import { inputClass } from "@/primevue/preset";

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
const requestLanguage = computed(() =>
  languageForContentType(
    headerValue(headers.value, "content-type"),
    body.value,
  ),
);
const responseLanguage = computed(() =>
  languageForContentType(
    headerValue(responseHeaders.value, "content-type"),
    responseBody.value,
  ),
);

const responseHeaders = computed<HeaderRow[]>(() => {
  const raw = response.value?.headers;
  if (!raw) return [];
  if (Array.isArray(raw)) return raw;
  return Object.entries(raw).map(([key, value]) => ({ key, value }));
});

const statusLabel = computed(() => {
  const r = response.value;
  if (!r || r.status == null) return "No response";
  return r.statusText?.trim() || String(r.status);
});

// Map an HTTP status class to a preset Tag severity; neutral until a response.
function statusSeverity(status?: number): string {
  if (status == null) return "secondary";
  if (status >= 400) return "danger";
  if (status >= 300) return "warn";
  if (status >= 200) return "success";
  return "info";
}

function addHeader(): void {
  headers.value = [...headers.value, { key: "", value: "" }];
}

function removeHeader(index: number): void {
  headers.value = headers.value.filter((_, i) => i !== index);
}

function headerValue(rows: HeaderRow[], name: string): string {
  const needle = name.toLowerCase();
  return (
    rows.find((header) => header.key.toLowerCase() === needle)?.value ?? ""
  );
}

function languageForContentType(contentType: string, value: string): string {
  const normalized = contentType.toLowerCase();
  const trimmed = value.trim();
  if (
    normalized.includes("json") ||
    ((!normalized || normalized.includes("text/plain")) &&
      (trimmed.startsWith("{") || trimmed.startsWith("[")))
  ) {
    return "json";
  }
  if (normalized.includes("xml")) return "xml";
  if (normalized.includes("html")) return "html";
  if (normalized.includes("yaml") || normalized.includes("yml")) return "yaml";
  if (normalized.includes("javascript")) return "javascript";
  if (normalized.includes("css")) return "css";
  return "plaintext";
}

async function send(): Promise<void> {
  const routeId = config.value?.executeRouteId ?? props.source?.routeId;
  if (!routeId) {
    error.value = "No execute route configured.";
    return;
  }
  loading.value = true;
  error.value = null;
  response.value = null;
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
      class="grid grid-cols-1 items-end gap-2 border-b border-surface-200 p-3 sm:grid-cols-[7.5rem_minmax(12rem,1fr)_auto] dark:border-surface-800"
    >
      <label class="flex min-w-0 flex-col gap-1">
        <span
          class="text-xs font-medium text-surface-500 dark:text-surface-400"
        >
          Method
        </span>
        <Select
          v-model="method"
          :options="methods"
          option-label="label"
          option-value="value"
          class="w-full"
          aria-label="Request method"
        />
      </label>
      <label class="flex min-w-0 flex-col gap-1">
        <span
          class="text-xs font-medium text-surface-500 dark:text-surface-400"
        >
          Request URL
        </span>
        <InputText
          v-model="url"
          placeholder="/api/health or https://example.com"
          aria-label="Request URL"
          :class="inputClass"
          @keyup.enter="send"
        />
      </label>
      <Button type="button" class="self-end" :disabled="loading" @click="send">
        <AppIcon
          :icon="{ type: 'lucide', value: 'send' }"
          :size="14"
          :loading="loading"
        />
        Send
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
                <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="14" />
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
          <CodeTextEditor
            v-model:value="body"
            class="min-h-0 flex-1"
            :language="requestLanguage"
            aria-label="Request body"
          />
        </div>
      </section>

      <section class="flex min-h-0 flex-col" aria-label="Response">
        <div
          class="flex min-h-[2.75rem] items-center gap-2 border-b border-surface-200 px-3 py-2 dark:border-surface-800"
          aria-live="polite"
        >
          <Tag
            :severity="statusSeverity(response?.status)"
            :value="statusLabel"
          />
          <span
            v-if="response?.durationMs != null"
            class="text-xs text-surface-500 dark:text-surface-400"
          >
            {{ response.durationMs.toFixed(1) }} ms
          </span>
        </div>
        <SkeletonList v-if="loading" :rows="6" />
        <PanelError
          v-else-if="error"
          :message="error"
          retryable
          @retry="send"
        />
        <div
          v-else-if="!response"
          class="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 px-6 py-10 text-center"
        >
          <span
            class="flex h-11 w-11 items-center justify-center rounded-full bg-surface-100 text-surface-400 dark:bg-surface-800 dark:text-surface-500"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'send' }" :size="20" />
          </span>
          <p class="max-w-xs text-sm text-surface-500 dark:text-surface-400">
            Send a request to see the response here.
          </p>
        </div>
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
          <CodeTextEditor
            :value="responseBody"
            class="min-h-0 flex-1"
            :language="responseLanguage"
            readonly
            aria-label="Response body"
          />
        </template>
      </section>
    </div>
  </div>
</template>
