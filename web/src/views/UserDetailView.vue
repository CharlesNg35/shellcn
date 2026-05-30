<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import Tabs from "primevue/tabs";
import TabList from "primevue/tablist";
import Tab from "primevue/tab";
import TabPanels from "primevue/tabpanels";
import TabPanel from "primevue/tabpanel";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Button from "primevue/button";
import { adminUsersApi } from "../api/admin";
import { useAuthStore } from "../stores/auth";
import { useNotify } from "../composables/useNotify";
import AppIcon from "../components/AppIcon.vue";
import AppBreadcrumb from "../components/AppBreadcrumb.vue";
import SkeletonList from "../components/SkeletonList.vue";
import AuditTable from "../components/admin/AuditTable.vue";
import { Role } from "../constants/roles";
import type {
  AdminUser,
  AuditEntry,
  UserConnectionSummary,
} from "../types/projection";

const props = defineProps<{ id: string }>();
const auth = useAuthStore();
const notify = useNotify();

const crumbs = computed(() => [
  { label: "Settings", to: { name: "settings" } },
  { label: "Users", to: { name: "users" } },
  { label: user.value?.displayName || user.value?.username || "User" },
]);

const tab = ref("overview");
const user = ref<AdminUser | null>(null);
const loading = ref(false);
const error = ref<string | null>(null);
const busy = ref(false);

const connections = ref<UserConnectionSummary[]>([]);
const connectionsLoaded = ref(false);

const audit = ref<AuditEntry[]>([]);
const auditTotal = ref(0);
const auditFirst = ref(0);
const auditRows = ref(25);
const auditLoaded = ref(false);
const auditLoading = ref(false);

async function loadUser(): Promise<void> {
  loading.value = true;
  error.value = null;
  try {
    user.value = await adminUsersApi.get(props.id);
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

async function loadConnections(): Promise<void> {
  if (connectionsLoaded.value) return;
  connections.value = await adminUsersApi.connections(props.id);
  connectionsLoaded.value = true;
}

async function loadAudit(): Promise<void> {
  auditLoading.value = true;
  try {
    const page = await adminUsersApi.audit(
      props.id,
      auditRows.value,
      auditFirst.value,
    );
    audit.value = page.items;
    auditTotal.value = page.total;
    auditLoaded.value = true;
  } finally {
    auditLoading.value = false;
  }
}

function onAuditPage(e: { first: number; rows: number }): void {
  auditFirst.value = e.first;
  auditRows.value = e.rows;
  void loadAudit();
}

// Lazy-load each tab's data the first time it is opened.
watch(tab, (t) => {
  if (t === "connections") void loadConnections();
  if (t === "audit" && !auditLoaded.value) void loadAudit();
});

onMounted(loadUser);

// Mirrors the backend rule: never self, never the protected root, and only the
// root admin may manage another admin.
const canDeactivate = computed(() => {
  const u = user.value;
  if (!u || u.protected || u.id === auth.user?.id) return false;
  if (u.roles.includes(Role.Admin) && !auth.user?.protected) return false;
  return true;
});

async function setActive(active: boolean): Promise<void> {
  if (!user.value) return;
  busy.value = true;
  try {
    user.value = active
      ? await adminUsersApi.activate(user.value.id)
      : await adminUsersApi.deactivate(user.value.id);
    notify.success(active ? "Account activated" : "Account deactivated");
  } catch (e) {
    notify.error("Could not update account", (e as Error).message);
  } finally {
    busy.value = false;
  }
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString();
}
</script>

<template>
  <div class="mx-auto flex h-full max-w-4xl flex-col gap-5 p-8">
    <AppBreadcrumb :items="crumbs" />
    <h1 class="text-2xl font-semibold text-surface-900 dark:text-surface-0">
      {{ user?.displayName || user?.username || "User" }}
    </h1>

    <p v-if="error" class="text-sm text-red-500">{{ error }}</p>
    <SkeletonList v-else-if="loading && !user" :rows="4" />

    <Tabs v-else-if="user" :value="tab" @update:value="tab = String($event)">
      <TabList>
        <Tab value="overview">
          <AppIcon
            :icon="{ type: 'lucide', value: 'circle-user' }"
            :size="14"
          />
          Overview
        </Tab>
        <Tab value="connections">
          <AppIcon :icon="{ type: 'lucide', value: 'server' }" :size="14" />
          Connections
        </Tab>
        <Tab value="audit">
          <AppIcon
            :icon="{ type: 'lucide', value: 'scroll-text' }"
            :size="14"
          />
          Audit
        </Tab>
      </TabList>
      <TabPanels>
        <TabPanel value="overview" class="flex flex-col gap-5 pt-2">
          <dl class="grid grid-cols-[8rem_1fr] gap-y-3 text-sm">
            <dt class="text-surface-400">Username</dt>
            <dd class="text-surface-800 dark:text-surface-100">
              {{ user.username }}
            </dd>
            <dt class="text-surface-400">Email</dt>
            <dd class="text-surface-800 dark:text-surface-100">
              {{ user.email || "—" }}
            </dd>
            <dt class="text-surface-400">Role</dt>
            <dd class="text-surface-800 capitalize dark:text-surface-100">
              {{ user.roles.join(", ") }}
            </dd>
            <dt class="text-surface-400">Status</dt>
            <dd :class="user.disabled ? 'text-amber-600' : 'text-emerald-600'">
              {{ user.disabled ? "Deactivated" : "Active" }}
            </dd>
          </dl>

          <div v-if="canDeactivate">
            <Button
              v-if="user.disabled"
              type="button"
              :loading="busy"
              @click="setActive(true)"
            >
              Activate account
            </Button>
            <Button
              v-else
              type="button"
              severity="danger"
              outlined
              :loading="busy"
              @click="setActive(false)"
            >
              Deactivate account
            </Button>
          </div>
        </TabPanel>

        <TabPanel value="connections" class="pt-2">
          <DataTable :value="connections" scrollable scroll-height="flex">
            <Column header="Name">
              <template #body="{ data }">
                <span class="flex items-center gap-2">
                  <AppIcon
                    :icon="(data as UserConnectionSummary).icon"
                    :size="16"
                    class="text-surface-400"
                  />
                  {{ (data as UserConnectionSummary).name }}
                </span>
              </template>
            </Column>
            <Column field="protocol" header="Protocol" />
            <Column header="Created">
              <template #body="{ data }">
                {{ formatDate((data as UserConnectionSummary).createdAt) }}
              </template>
            </Column>
            <template #empty>No connections.</template>
          </DataTable>
        </TabPanel>

        <TabPanel value="audit" class="pt-2">
          <AuditTable
            :items="audit"
            :total="auditTotal"
            :rows="auditRows"
            :first="auditFirst"
            :loading="auditLoading"
            @page="onAuditPage"
          />
        </TabPanel>
      </TabPanels>
    </Tabs>
  </div>
</template>
