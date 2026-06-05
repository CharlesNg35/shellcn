<script setup lang="ts">
import { ref } from "vue";
import Button from "primevue/button";
import Popover from "primevue/popover";
import Tag from "primevue/tag";
import AppIcon from "../../components/AppIcon.vue";
import type { MarketEntry } from "../../types/projection";
import { marketAction, marketStatus, marketVersionLabel } from "./market";
import MarketPluginDetails from "./MarketPluginDetails.vue";

const props = defineProps<{
  entry: MarketEntry;
  installing: boolean;
  uninstalling: boolean;
}>();

const emit = defineEmits<{
  (e: "install", entry: MarketEntry): void;
  (e: "uninstall", entry: MarketEntry): void;
}>();

const details = ref<{ toggle: (event: Event) => void } | null>(null);

function toggleDetails(event: Event): void {
  details.value?.toggle(event);
}
</script>

<template>
  <article
    class="grid gap-3 rounded-lg border border-surface-200 bg-surface-0 p-4 transition-colors hover:border-surface-300 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center dark:border-surface-800 dark:bg-surface-950 dark:hover:border-surface-700"
  >
    <div class="flex min-w-0 gap-3">
      <span
        class="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-surface-100 text-surface-500 dark:bg-surface-800 dark:text-surface-300"
      >
        <AppIcon
          v-if="props.entry.latest"
          :icon="props.entry.latest.icon"
          :size="21"
        />
        <AppIcon
          v-else
          :icon="{ type: 'lucide', value: 'puzzle' }"
          :size="19"
        />
      </span>

      <div class="min-w-0 flex-1">
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <h3
            class="truncate text-sm font-semibold text-surface-900 dark:text-surface-0"
          >
            {{ props.entry.displayName }}
          </h3>
          <Tag
            :value="marketStatus(props.entry).value"
            :severity="marketStatus(props.entry).severity"
          />
        </div>

        <p
          class="mt-1 line-clamp-2 max-w-3xl text-sm leading-5 text-surface-600 dark:text-surface-300"
        >
          {{ props.entry.description }}
        </p>

        <div
          class="mt-2 flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 text-xs text-surface-400"
        >
          <span>{{ props.entry.name }}</span>
          <span
            >Latest {{ marketVersionLabel(props.entry.latest?.version) }}</span
          >
          <span v-if="props.entry.installedVersion">
            Installed {{ marketVersionLabel(props.entry.installedVersion) }}
          </span>
        </div>
      </div>
    </div>

    <div
      class="flex w-full items-center justify-end gap-2 sm:w-auto sm:min-w-72"
    >
      <Button
        class="w-9 shrink-0 justify-center"
        severity="secondary"
        variant="text"
        size="small"
        aria-label="Show plugin details"
        @click="toggleDetails"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'info' }" :size="15" />
      </Button>
      <Button
        v-if="marketAction(props.entry)"
        class="w-28 shrink-0 justify-center"
        :label="marketAction(props.entry)!"
        size="small"
        :loading="props.installing"
        :disabled="props.uninstalling"
        :aria-label="`${marketAction(props.entry)} ${props.entry.displayName}`"
        @click="emit('install', props.entry)"
      />
      <Button
        v-else-if="!props.entry.compatible"
        class="w-28 shrink-0 justify-center"
        label="Unavailable"
        size="small"
        severity="secondary"
        disabled
      />

      <Button
        v-if="props.entry.managed"
        class="w-28 shrink-0 justify-center"
        label="Uninstall"
        severity="danger"
        variant="outlined"
        size="small"
        :loading="props.uninstalling"
        :disabled="props.installing"
        :aria-label="`Uninstall ${props.entry.displayName}`"
        @click="emit('uninstall', props.entry)"
      />
    </div>

    <Popover ref="details">
      <MarketPluginDetails :entry="props.entry" />
    </Popover>
  </article>
</template>
