<script setup lang="ts">
import { reactive } from "vue";
import { heavyPanelLoaders, type HeavyPanelKind } from "./lazy/heavyPanels";

const loaded = reactive<Record<string, boolean>>({});

async function load(kind: string) {
  await heavyPanelLoaders[kind as HeavyPanelKind]();
  loaded[kind] = true;
}
</script>

<template>
  <main class="mx-auto flex min-h-full max-w-3xl flex-col gap-6 p-8">
    <header class="flex flex-col gap-1">
      <h1 class="text-2xl font-semibold text-surface-900 dark:text-surface-0">
        ShellCN
      </h1>
      <p class="text-surface-500">
        Infrastructure access gateway — bootstrap shell.
      </p>
    </header>
    <section class="flex flex-wrap gap-2">
      <button
        v-for="kind in Object.keys(heavyPanelLoaders)"
        :key="kind"
        type="button"
        class="rounded border border-surface-300 px-3 py-1 text-sm hover:bg-surface-100 dark:border-surface-700 dark:hover:bg-surface-800"
        @click="load(kind)"
      >
        {{ loaded[kind] ? `${kind} ready` : `Load ${kind}` }}
      </button>
    </section>
  </main>
</template>
