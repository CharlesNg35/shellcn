<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Button from "primevue/button";
import AppIcon from "@/components/AppIcon.vue";
import PanelError from "@/panels/shared/PanelError.vue";
import type { PanelProps } from "@/panels/core/types";
import type {
  WebProxyCapability,
  WebProxyPanelConfig,
} from "@/types/projection";

const props = defineProps<PanelProps>();

const loaded = ref(false);
const reloadToken = ref(0);

const config = computed(() => props.config as WebProxyPanelConfig | undefined);

const proxyPath = computed(() => normalizeProxyPath(config.value?.path));
const frameSrc = computed(() => {
  if (!proxyPath.value) return "";
  return `/api/connections/${encodeURIComponent(props.connectionId)}/proxy${proxyPath.value}`;
});
const frameKey = computed(() => `${frameSrc.value}:${reloadToken.value}`);
const ariaLabel = computed(
  () => config.value?.ariaLabel?.trim() || "Proxied web surface",
);
const capabilitySet = computed(() => new Set(config.value?.capabilities ?? []));
const sandboxPolicy = computed(() => {
  const tokens = ["allow-scripts", "allow-forms", "allow-modals"];
  if (hasCapability("downloads")) tokens.push("allow-downloads");
  if (hasCapability("popups")) {
    tokens.push("allow-popups", "allow-popups-to-escape-sandbox");
  }
  if (hasCapability("same_origin")) tokens.push("allow-same-origin");
  return tokens.join(" ");
});
const allowPolicy = computed(() => {
  const policies: string[] = [];
  if (hasCapability("clipboard")) {
    policies.push("clipboard-read", "clipboard-write");
  }
  if (hasCapability("fullscreen")) policies.push("fullscreen");
  return policies.join("; ");
});

watch(
  frameSrc,
  () => {
    loaded.value = false;
  },
  { immediate: true },
);

function hasCapability(capability: WebProxyCapability): boolean {
  return capabilitySet.value.has(capability);
}

function reload(): void {
  loaded.value = false;
  reloadToken.value += 1;
}

function openExternal(): void {
  if (!frameSrc.value) return;
  window.open(frameSrc.value, "_blank", "noopener,noreferrer");
}

function normalizeProxyPath(path?: string): string | null {
  const raw = path?.trim() || "/";
  if (!raw.startsWith("/") || raw.startsWith("//") || raw.startsWith("/\\")) {
    return null;
  }
  try {
    const parsed = new URL(raw, "https://shellcn.local");
    if (parsed.origin !== "https://shellcn.local") return null;
    return `${parsed.pathname}${parsed.search}${parsed.hash}` || "/";
  } catch {
    return null;
  }
}
</script>

<template>
  <PanelError
    v-if="!proxyPath"
    message="This panel declares an invalid proxy path."
  />
  <section
    v-else
    class="flex h-full min-h-0 flex-col bg-surface-0 dark:bg-surface-950"
  >
    <div
      class="flex h-10 shrink-0 items-center justify-end gap-1 border-b border-surface-200 bg-surface-50 px-2 dark:border-surface-800 dark:bg-surface-900"
    >
      <Button
        type="button"
        severity="secondary"
        text
        rounded
        aria-label="Reload"
        title="Reload"
        class="h-8 w-8"
        @click="reload"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'refresh-cw' }" :size="16" />
      </Button>
      <Button
        v-if="config?.openExternal"
        type="button"
        severity="secondary"
        text
        rounded
        aria-label="Open in new tab"
        title="Open in new tab"
        class="h-8 w-8"
        @click="openExternal"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'external-link' }"
          :size="16"
        />
      </Button>
    </div>
    <div class="relative min-h-0 flex-1 bg-surface-0 dark:bg-surface-950">
      <div
        v-if="!loaded"
        class="pointer-events-none absolute inset-x-0 top-0 z-10 h-0.5 overflow-hidden bg-surface-200 dark:bg-surface-800"
        aria-hidden="true"
      >
        <span class="block h-full w-1/3 animate-pulse bg-primary-500" />
      </div>
      <iframe
        :key="frameKey"
        class="h-full w-full border-0 bg-white dark:bg-surface-950"
        :src="frameSrc"
        :title="ariaLabel"
        :aria-label="ariaLabel"
        :sandbox="sandboxPolicy"
        :allow="allowPolicy || undefined"
        :allowfullscreen="hasCapability('fullscreen') || undefined"
        referrerpolicy="no-referrer"
        @load="loaded = true"
      />
    </div>
  </section>
</template>
