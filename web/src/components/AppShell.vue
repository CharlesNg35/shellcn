<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";
import { useDocumentVisibility, useIntervalFn, useStorage } from "@vueuse/core";
import Button from "primevue/button";
import { useConnectionsStore } from "../stores/connections";
import { useAuthStore } from "../stores/auth";
import { useConnectionSessionsStore } from "../stores/connectionSessions";
import { KEEP_ALIVE_CONNECTION_WORKSPACES_MAX } from "../stores/sessionLimits";
import AppIcon from "./AppIcon.vue";
import AppLogo from "./AppLogo.vue";
import ThemeToggle from "./ThemeToggle.vue";
import ConnectionFormDialog from "./ConnectionFormDialog.vue";
import ConnectionSidebar from "./ConnectionSidebar.vue";
import { searchInputClass } from "../primevue/preset";

const conns = useConnectionsStore();
const auth = useAuthStore();
const connectionSessions = useConnectionSessionsStore();
const route = useRoute();
const router = useRouter();

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
  connectionSessions.start();
  if (conns.loaded) return;
  try {
    await conns.load();
  } catch (e) {
    error.value = (e as Error).message;
  }
});

onUnmounted(() => connectionSessions.stop());

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

const showCreate = ref(false);
const sidebarMenuOpen = useStorage("shellcn:sidebar-menu:open", true);

function onConnectionSaved(payload: { id: string; created: boolean }): void {
  if (payload.created) {
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
        <ThemeToggle size="small" :icon-size="16" />
      </div>

      <div class="px-3 pb-2">
        <label class="relative block">
          <span
            class="pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-surface-400"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'search' }" :size="15" />
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

      <nav class="flex min-h-0 flex-1 flex-col overflow-hidden px-2 pb-3">
        <p v-if="error" class="px-2 py-4 text-sm text-red-500">{{ error }}</p>
        <ConnectionSidebar v-else :active-id="activeId" :query="query">
          <template #create>
            <Button
              v-if="auth.canCreate"
              text
              rounded
              severity="secondary"
              size="small"
              title="Add connection"
              aria-label="Add connection"
              @click="showCreate = true"
            >
              <AppIcon :icon="{ type: 'lucide', value: 'plus' }" :size="15" />
            </Button>
          </template>
        </ConnectionSidebar>
      </nav>

      <div class="relative border-t border-surface-200 dark:border-surface-800">
        <button
          type="button"
          class="absolute top-0 left-1/2 z-10 flex h-5 w-5 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border border-surface-200 bg-surface-50 text-surface-400 shadow-sm transition hover:border-surface-300 hover:text-surface-600 dark:border-surface-700 dark:bg-surface-900 dark:text-surface-500 dark:hover:border-surface-600 dark:hover:text-surface-300"
          :title="sidebarMenuOpen ? 'Hide menu' : 'Show menu'"
          :aria-label="
            sidebarMenuOpen ? 'Hide sidebar menu' : 'Show sidebar menu'
          "
          :aria-expanded="sidebarMenuOpen"
          aria-controls="sidebar-utility-menu"
          @click="sidebarMenuOpen = !sidebarMenuOpen"
        >
          <AppIcon
            :icon="{
              type: 'lucide',
              value: sidebarMenuOpen ? 'chevron-down' : 'chevron-up',
            }"
            :size="12"
          />
        </button>

        <Transition
          enter-active-class="overflow-hidden transition-[max-height,opacity,transform] duration-150 ease-out"
          enter-from-class="max-h-0 -translate-y-1 opacity-0"
          enter-to-class="max-h-80 translate-y-0 opacity-100"
          leave-active-class="overflow-hidden transition-[max-height,opacity,transform] duration-150 ease-in"
          leave-from-class="max-h-80 translate-y-0 opacity-100"
          leave-to-class="max-h-0 -translate-y-1 opacity-0"
        >
          <div
            v-show="sidebarMenuOpen"
            id="sidebar-utility-menu"
            :aria-hidden="!sidebarMenuOpen"
            class="pt-1"
          >
            <RouterLink
              :to="{ name: 'credentials' }"
              class="mx-2 flex items-center gap-2.5 rounded-md px-2 py-2 text-sm text-surface-500 transition-colors hover:bg-surface-200 dark:hover:bg-surface-800"
              :class="
                route.name === 'credentials'
                  ? 'bg-primary-50 font-medium text-primary-700 ring-1 ring-primary-200/70 dark:bg-primary-950/40 dark:text-primary-200 dark:ring-primary-900/60'
                  : ''
              "
            >
              <AppIcon :icon="{ type: 'lucide', value: 'key' }" :size="16" />
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
              <AppIcon :icon="{ type: 'lucide', value: 'video' }" :size="16" />
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
              <AppIcon
                :icon="{ type: 'lucide', value: 'settings' }"
                :size="16"
              />
              Settings
            </RouterLink>
          </div>
        </Transition>

        <div
          class="flex items-center gap-1 border-t border-surface-200 px-2 py-2 dark:border-surface-800"
        >
          <RouterLink
            :to="{ name: 'profile' }"
            class="my-1 flex min-w-0 flex-1 items-center gap-2.5 rounded-md px-2 py-1.5 hover:bg-surface-200 dark:hover:bg-surface-800"
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
              <AppIcon :icon="{ type: 'lucide', value: 'user' }" :size="15" />
            </span>
            <span
              class="min-w-0 flex-1 truncate text-sm text-surface-700 dark:text-surface-200"
              :title="userLabel"
            >
              {{ userLabel }}
            </span>
          </RouterLink>
          <Button
            text
            rounded
            severity="secondary"
            size="small"
            class="shrink-0"
            title="Sign out"
            aria-label="Sign out"
            @click="onLogout"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'log-out' }" :size="16" />
          </Button>
        </div>
      </div>
    </aside>

    <main class="min-w-0 flex-1 overflow-hidden">
      <!-- Keep each connection's workspace alive (bounded LRU) so terminals,
           consoles and log streams resume exactly as left when navigating back. -->
      <RouterView v-slot="{ Component }">
        <KeepAlive :max="KEEP_ALIVE_CONNECTION_WORKSPACES_MAX">
          <component :is="Component" :key="route.path" />
        </KeepAlive>
      </RouterView>
    </main>

    <ConnectionFormDialog
      v-model:visible="showCreate"
      @saved="onConnectionSaved"
    />
  </div>
</template>
