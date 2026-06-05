<script setup lang="ts">
import type { MarketEntry } from "../../types/projection";
import { marketRepoUrl, marketVersionLabel } from "./market";

defineProps<{
  entry: MarketEntry;
}>();
</script>

<template>
  <div class="w-[min(24rem,calc(100vw-2rem))] space-y-4 text-sm">
    <div>
      <p class="font-medium text-surface-900 dark:text-surface-0">
        {{ entry.displayName }}
      </p>
      <p class="mt-1 text-surface-500 dark:text-surface-400">
        {{ entry.description }}
      </p>
    </div>

    <dl class="grid grid-cols-[7rem_minmax(0,1fr)] gap-x-3 gap-y-2 text-xs">
      <dt class="text-surface-400">Plugin</dt>
      <dd class="min-w-0 truncate text-surface-700 dark:text-surface-200">
        {{ entry.name }}
      </dd>

      <dt class="text-surface-400">Latest</dt>
      <dd class="text-surface-700 dark:text-surface-200">
        {{ marketVersionLabel(entry.latest?.version) }}
      </dd>

      <dt class="text-surface-400">Installed</dt>
      <dd class="text-surface-700 dark:text-surface-200">
        {{
          entry.installedVersion
            ? marketVersionLabel(entry.installedVersion)
            : "Not installed"
        }}
      </dd>

      <dt class="text-surface-400">License</dt>
      <dd class="text-surface-700 dark:text-surface-200">
        {{ entry.license || "Not specified" }}
      </dd>

      <dt class="text-surface-400">Maintainers</dt>
      <dd class="text-surface-700 dark:text-surface-200">
        {{
          entry.maintainers.length ? entry.maintainers.join(", ") : "Community"
        }}
      </dd>

      <template v-if="entry.latest?.platforms.length">
        <dt class="text-surface-400">Platforms</dt>
        <dd class="text-surface-700 dark:text-surface-200">
          {{ entry.latest.platforms.join(", ") }}
        </dd>
      </template>

      <dt class="text-surface-400">Repository</dt>
      <dd class="min-w-0">
        <a
          :href="marketRepoUrl(entry)"
          target="_blank"
          rel="noopener noreferrer"
          class="truncate text-primary-600 hover:underline dark:text-primary-300"
        >
          {{ entry.repo }}
        </a>
      </dd>
    </dl>
  </div>
</template>
