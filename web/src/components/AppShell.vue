<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";
import { useDocumentVisibility, useIntervalFn } from "@vueuse/core";
import { useConnectionsStore } from "../stores/connections";
import { useWorkspaceStore } from "../stores/workspace";
import { useAuthStore } from "../stores/auth";
import { useTheme } from "../composables/useTheme";
import AppIcon from "./AppIcon.vue";
import AppLogo from "./AppLogo.vue";
import ConnectionFormDialog from "./ConnectionFormDialog.vue";
import { searchInputClass } from "../primevue/preset";
import type { ConnectionSummary } from "../types/projection";

const conns = useConnectionsStore();
const ws = useWorkspaceStore();
const auth = useAuthStore();
const route = useRoute();
const router = useRouter();
const { isDark, toggle: toggleTheme } = useTheme();

const userLabel = computed(
  () => auth.user?.displayName || auth.user?.username || "",
);

async function onLogout(): Promise<void> {
  await auth.logout();
  await router.push({ name: "login" });
}

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

// Keep the presence dots honest: re-fetch the catalog on a slow cadence so a
// session opening or closing flips its dot without a manual reload. Paused
// while the tab is hidden to avoid background churn.
const visibility = useDocumentVisibility();
const { pause, resume } = useIntervalFn(
  () => {
    if (conns.loaded) void conns.refresh().catch(() => undefined);
  },
  15000,
  { immediate: false },
);
watch(visibility, (state) => (state === "visible" ? resume() : pause()), {
  immediate: true,
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

function dotClass(c: ConnectionSummary): string {
  switch (c.status) {
    case "active":
      return "bg-emerald-400";
    case "offline":
      return "bg-red-500";
    default:
      return "bg-surface-300 dark:bg-surface-600";
  }
}

function dotTitle(c: ConnectionSummary): string {
  switch (c.status) {
    case "active":
      return "Open — live session";
    case "offline":
      return "Agent offline";
    default:
      return "Idle";
  }
}

function go(c: ConnectionSummary): void {
  ws.open(c.id);
  router.push({ name: "connection", params: { id: c.id } });
}

const showCreate = ref(false);

function onConnectionSaved(payload: { id: string; created: boolean }): void {
  if (payload.created) {
    ws.open(payload.id);
    void router.push({ name: "connection", params: { id: payload.id } });
  }
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
          <AppLogo :size="28" class="shrink-0 text-primary-600" />
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
            aria-label="Search connections"
            :class="searchInputClass"
          />
        </label>
      </div>

      <nav class="flex-1 overflow-y-auto px-2 pb-3">
        <p v-if="error" class="px-2 py-4 text-sm text-red-500">{{ error }}</p>

        <div class="flex items-center justify-between px-2 pb-1 pt-3">
          <p
            class="text-xs font-medium uppercase tracking-wide text-surface-400"
          >
            Connections
          </p>
          <button
            type="button"
            class="rounded p-1 text-surface-400 hover:bg-surface-200 hover:text-surface-700 dark:hover:bg-surface-800"
            title="Add connection"
            aria-label="Add connection"
            @click="showCreate = true"
          >
            <AppIcon :icon="{ type: 'name', value: 'plus' }" :size="15" />
          </button>
        </div>
        <div class="space-y-1">
          <button
            v-for="c in filtered"
            :key="c.id"
            type="button"
            class="group flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left text-sm transition-colors hover:bg-surface-200 dark:hover:bg-surface-800"
            :class="
              activeId === c.id
                ? 'bg-primary-50 font-medium text-primary-700 ring-1 ring-primary-200/70 dark:bg-primary-950/40 dark:text-primary-200 dark:ring-primary-900/60'
                : ''
            "
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
              class="h-2 w-2 shrink-0 rounded-full"
              :class="dotClass(c)"
              :title="dotTitle(c)"
            />
          </button>
        </div>
        <!-- Loading: skeleton rows while the catalog is fetched. -->
        <div v-if="!conns.loaded && !error" class="space-y-1.5 px-1 pt-1">
          <div
            v-for="n in 5"
            :key="n"
            class="h-9 animate-pulse rounded-md bg-surface-200/60 dark:bg-surface-800/60"
          />
        </div>
        <p
          v-else-if="conns.loaded && !filtered.length && query"
          class="px-2 py-6 text-center text-sm text-surface-400"
        >
          No connections match “{{ query }}”.
        </p>
        <!-- Empty: a single create affordance lives in the header (+), so this
             stays purely informational. -->
        <div
          v-else-if="conns.loaded && !conns.connections.length"
          class="flex flex-col items-center gap-1.5 px-4 py-10 text-center"
        >
          <span
            class="mb-1 flex h-10 w-10 items-center justify-center rounded-full bg-surface-100 text-surface-400 dark:bg-surface-800"
          >
            <AppIcon :icon="{ type: 'name', value: 'server' }" :size="18" />
          </span>
          <p class="text-sm font-medium text-surface-600 dark:text-surface-300">
            No connections yet
          </p>
          <p class="text-xs text-surface-400">
            Use the + above to add your first one.
          </p>
        </div>
      </nav>

      <div class="border-t border-surface-200 dark:border-surface-800">
        <RouterLink
          v-if="auth.isAdmin"
          :to="{ name: 'users' }"
          class="mx-2 mt-2 flex items-center gap-2.5 rounded-md px-2 py-2 text-sm text-surface-500 transition-colors hover:bg-surface-200 dark:hover:bg-surface-800"
          :class="
            route.name === 'users'
              ? 'bg-primary-50 font-medium text-primary-700 ring-1 ring-primary-200/70 dark:bg-primary-950/40 dark:text-primary-200 dark:ring-primary-900/60'
              : ''
          "
        >
          <AppIcon :icon="{ type: 'name', value: 'users' }" :size="16" />
          Users
        </RouterLink>
        <RouterLink
          :to="{ name: 'credentials' }"
          class="mx-2 mt-1 flex items-center gap-2.5 rounded-md px-2 py-2 text-sm text-surface-500 transition-colors hover:bg-surface-200 dark:hover:bg-surface-800"
          :class="
            route.name === 'credentials'
              ? 'bg-primary-50 font-medium text-primary-700 ring-1 ring-primary-200/70 dark:bg-primary-950/40 dark:text-primary-200 dark:ring-primary-900/60'
              : ''
          "
        >
          <AppIcon :icon="{ type: 'name', value: 'key' }" :size="16" />
          Credentials
        </RouterLink>
        <RouterLink
          :to="{ name: 'recordings' }"
          class="mx-2 mt-1 flex items-center gap-2.5 rounded-md px-2 py-2 text-sm text-surface-500 transition-colors hover:bg-surface-200 dark:hover:bg-surface-800"
          :class="
            route.name === 'recordings'
              ? 'bg-primary-50 font-medium text-primary-700 ring-1 ring-primary-200/70 dark:bg-primary-950/40 dark:text-primary-200 dark:ring-primary-900/60'
              : ''
          "
        >
          <AppIcon :icon="{ type: 'name', value: 'video' }" :size="16" />
          Recordings
        </RouterLink>
        <RouterLink
          :to="{ name: 'settings' }"
          class="mx-2 my-1 flex items-center gap-2.5 rounded-md px-2 py-2 text-sm text-surface-500 transition-colors hover:bg-surface-200 dark:hover:bg-surface-800"
          :class="
            route.name === 'settings'
              ? 'bg-primary-50 font-medium text-primary-700 ring-1 ring-primary-200/70 dark:bg-primary-950/40 dark:text-primary-200 dark:ring-primary-900/60'
              : ''
          "
        >
          <AppIcon :icon="{ type: 'name', value: 'settings' }" :size="16" />
          Settings
        </RouterLink>

        <div
          class="flex items-center gap-1 border-t border-surface-200 px-2 py-2 dark:border-surface-800"
        >
          <RouterLink
            :to="{ name: 'profile' }"
            class="flex min-w-0 flex-1 items-center gap-2.5 rounded-md px-2 py-1.5 hover:bg-surface-200 dark:hover:bg-surface-800"
            :class="
              route.name === 'profile'
                ? 'bg-primary-50 dark:bg-primary-950/40'
                : ''
            "
            title="Your profile"
          >
            <span
              class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-surface-200 text-surface-600 dark:bg-surface-800 dark:text-surface-300"
            >
              <AppIcon :icon="{ type: 'name', value: 'user' }" :size="15" />
            </span>
            <span
              class="min-w-0 flex-1 truncate text-sm text-surface-700 dark:text-surface-200"
              :title="userLabel"
            >
              {{ userLabel }}
            </span>
          </RouterLink>
          <button
            type="button"
            class="shrink-0 rounded-md p-1.5 text-surface-500 hover:bg-surface-200 hover:text-surface-700 dark:hover:bg-surface-800"
            title="Sign out"
            aria-label="Sign out"
            @click="onLogout"
          >
            <AppIcon :icon="{ type: 'name', value: 'log-out' }" :size="16" />
          </button>
        </div>
      </div>
    </aside>

    <main class="min-w-0 flex-1 overflow-hidden">
      <!-- Keep each connection's workspace alive (bounded LRU) so terminals,
           consoles and log streams resume exactly as left when navigating back. -->
      <RouterView v-slot="{ Component }">
        <KeepAlive :max="6">
          <component :is="Component" :key="route.fullPath" />
        </KeepAlive>
      </RouterView>
    </main>

    <ConnectionFormDialog
      v-model:visible="showCreate"
      @saved="onConnectionSaved"
    />
  </div>
</template>
