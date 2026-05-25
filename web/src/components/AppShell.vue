<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";
import { useConnectionsStore } from "../stores/connections";
import { useWorkspaceStore } from "../stores/workspace";
import { useTheme } from "../composables/useTheme";
import AppIcon from "./AppIcon.vue";
import AppToast from "./AppToast.vue";
import type { ConnectionSummary } from "../types/projection";

const conns = useConnectionsStore();
const ws = useWorkspaceStore();
const route = useRoute();
const router = useRouter();
const { isDark, toggle: toggleTheme } = useTheme();

const query = ref("");
const error = ref<string | null>(null);

onMounted(async () => {
  if (conns.loaded) return;
  try {
    await conns.load();
  } catch (e) {
    error.value = (e as Error).message;
  }
});

const activeId = computed(() =>
  route.name === "connection" ? String(route.params.id) : null,
);

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase();
  if (!q) return conns.connections;
  return conns.connections.filter((c) =>
    `${c.name} ${c.protocol}`.toLowerCase().includes(q),
  );
});

const recent = computed(() =>
  ws.recent
    .map((id) => conns.byId(id))
    .filter((c): c is ConnectionSummary => Boolean(c)),
);

function dotClass(c: ConnectionSummary): string {
  if (c.transport === "agent" && !c.online) return "bg-amber-400";
  return c.online === false ? "bg-surface-400" : "bg-emerald-400";
}

function go(c: ConnectionSummary): void {
  ws.open(c.id);
  router.push({ name: "connection", params: { id: c.id } });
}
</script>

<template>
  <div
    class="flex h-full bg-surface-0 text-surface-700 dark:bg-surface-950 dark:text-surface-200"
  >
    <aside
      class="flex w-64 shrink-0 flex-col border-r border-surface-200 bg-surface-50 dark:border-surface-800 dark:bg-surface-900"
    >
      <div class="flex items-center justify-between px-4 py-3.5">
        <RouterLink
          :to="{ name: 'home' }"
          class="flex items-center gap-2 font-semibold text-surface-900 dark:text-surface-0"
        >
          <span
            class="flex h-7 w-7 items-center justify-center rounded-md bg-primary-500 text-white"
          >
            <AppIcon :icon="{ type: 'name', value: 'terminal' }" :size="16" />
          </span>
          ShellCN
        </RouterLink>
        <button
          type="button"
          class="rounded-md p-1.5 text-surface-500 hover:bg-surface-200 dark:hover:bg-surface-800"
          :title="isDark ? 'Switch to light' : 'Switch to dark'"
          @click="toggleTheme"
        >
          {{ isDark ? "☀" : "☾" }}
        </button>
      </div>

      <div class="px-3 pb-2">
        <label class="relative block">
          <span
            class="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-surface-400"
          >
            <AppIcon :icon="{ type: 'name', value: 'search' }" :size="15" />
          </span>
          <input
            v-model="query"
            type="search"
            placeholder="Search connections"
            class="w-full rounded-md border border-surface-200 bg-surface-0 py-1.5 pl-8 pr-2 text-sm outline-none placeholder:text-surface-400 focus:border-primary-400 dark:border-surface-700 dark:bg-surface-950"
          />
        </label>
      </div>

      <nav class="flex-1 overflow-y-auto px-2 pb-3">
        <p v-if="error" class="px-2 py-4 text-sm text-red-500">{{ error }}</p>

        <template v-if="recent.length && !query">
          <p
            class="px-2 pb-1 pt-2 text-xs font-medium uppercase tracking-wide text-surface-400"
          >
            Recent
          </p>
          <button
            v-for="c in recent"
            :key="`recent-${c.id}`"
            type="button"
            class="group flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left text-sm hover:bg-surface-200 dark:hover:bg-surface-800"
            :class="
              activeId === c.id ? 'bg-surface-200 dark:bg-surface-800' : ''
            "
            @click="go(c)"
          >
            <AppIcon :icon="c.icon" :size="16" class="text-surface-500" />
            <span
              class="flex-1 truncate text-surface-800 dark:text-surface-100"
              >{{ c.name }}</span
            >
            <span class="h-2 w-2 rounded-full" :class="dotClass(c)" />
          </button>
        </template>

        <p
          class="px-2 pb-1 pt-3 text-xs font-medium uppercase tracking-wide text-surface-400"
        >
          Connections
        </p>
        <button
          v-for="c in filtered"
          :key="c.id"
          type="button"
          class="group flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left text-sm hover:bg-surface-200 dark:hover:bg-surface-800"
          :class="activeId === c.id ? 'bg-surface-200 dark:bg-surface-800' : ''"
          @click="go(c)"
        >
          <AppIcon :icon="c.icon" :size="16" class="text-surface-500" />
          <span class="flex min-w-0 flex-1 flex-col">
            <span class="truncate text-surface-800 dark:text-surface-100">{{
              c.name
            }}</span>
            <span class="truncate text-xs text-surface-400">{{
              c.protocol
            }}</span>
          </span>
          <span
            class="h-2 w-2 rounded-full"
            :class="dotClass(c)"
            :title="c.status ?? (c.online ? 'online' : 'offline')"
          />
        </button>
        <p
          v-if="conns.loaded && !filtered.length"
          class="px-2 py-4 text-sm text-surface-400"
        >
          No connections match.
        </p>
      </nav>

      <RouterLink
        :to="{ name: 'settings' }"
        class="flex items-center gap-2.5 border-t border-surface-200 px-4 py-3 text-sm text-surface-500 hover:bg-surface-200 dark:border-surface-800 dark:hover:bg-surface-800"
      >
        <AppIcon :icon="{ type: 'name', value: 'settings' }" :size="16" />
        Settings
      </RouterLink>
    </aside>

    <main class="min-w-0 flex-1 overflow-hidden">
      <RouterView />
    </main>

    <AppToast />
  </div>
</template>
