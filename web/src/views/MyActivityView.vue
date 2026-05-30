<script setup lang="ts">
import { onMounted, ref } from "vue";
import { activityApi } from "../api/activity";
import AppBreadcrumb from "../components/AppBreadcrumb.vue";
import AuditTable from "../components/admin/AuditTable.vue";
import type { AuditEntry } from "../types/projection";

const crumbs = [
  { label: "Settings", to: { name: "settings" } },
  { label: "My activity" },
];

const items = ref<AuditEntry[]>([]);
const total = ref(0);
const first = ref(0);
const rows = ref(25);
const loading = ref(false);

async function load(): Promise<void> {
  loading.value = true;
  try {
    const page = await activityApi.mine(rows.value, first.value);
    items.value = page.items;
    total.value = page.total;
  } finally {
    loading.value = false;
  }
}

function onPage(e: { first: number; rows: number }): void {
  first.value = e.first;
  rows.value = e.rows;
  void load();
}

onMounted(load);
</script>

<template>
  <div class="mx-auto flex h-full max-w-4xl flex-col gap-5 p-8">
    <AppBreadcrumb :items="crumbs" />
    <h1 class="text-2xl font-semibold text-surface-900 dark:text-surface-0">
      My activity
    </h1>
    <AuditTable
      :items="items"
      :total="total"
      :rows="rows"
      :first="first"
      :loading="loading"
      @page="onPage"
    />
  </div>
</template>
