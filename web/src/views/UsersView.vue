<script setup lang="ts">
import { onMounted, ref } from "vue";
import Tabs from "primevue/tabs";
import TabList from "primevue/tablist";
import Tab from "primevue/tab";
import TabPanels from "primevue/tabpanels";
import TabPanel from "primevue/tabpanel";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Button from "primevue/button";
import { api, ApiError } from "../api/client";
import { useAuthStore } from "../stores/auth";
import { useNotify } from "../composables/useNotify";
import AppIcon from "../components/AppIcon.vue";
import UserFormDialog from "../components/UserFormDialog.vue";
import InviteDialog from "../components/InviteDialog.vue";
import { useConfirmAction } from "../composables/useConfirmAction";
import type { AdminUser, InvitationSummary } from "../types/projection";

const auth = useAuthStore();
const notify = useNotify();

const tab = ref("users");
const users = ref<AdminUser[]>([]);
const invitations = ref<InvitationSummary[]>([]);

const showUserForm = ref(false);
const editingUser = ref<AdminUser | null>(null);
const showInvite = ref(false);
const { confirmDanger } = useConfirmAction();

async function loadUsers(): Promise<void> {
  users.value = await api.get<AdminUser[]>("/admin/users");
}
async function loadInvitations(): Promise<void> {
  invitations.value = await api.get<InvitationSummary[]>("/admin/invitations");
}
onMounted(() => {
  void loadUsers();
  void loadInvitations();
});

// The root admin can never be deleted; only the root admin may delete admins.
function canDelete(u: AdminUser): boolean {
  if (u.protected) return false;
  if (u.roles.includes("admin")) return auth.user?.protected === true;
  return true;
}

// Root admin edits anyone; a regular admin edits non-admin users and their own
// account, but not other admins (the backend enforces this too).
function canEdit(u: AdminUser): boolean {
  if (auth.user?.protected) return true;
  return !u.roles.includes("admin") || u.id === auth.user?.id;
}

function openCreate(): void {
  editingUser.value = null;
  showUserForm.value = true;
}
function openEdit(u: AdminUser): void {
  editingUser.value = u;
  showUserForm.value = true;
}

function askDeleteUser(u: AdminUser): void {
  confirmDanger({
    header: "Delete user",
    message: `Delete “${u.username}”? This cannot be undone.`,
    accept: () => deleteUser(u),
  });
}

async function deleteUser(u: AdminUser): Promise<void> {
  try {
    await api.del(`/admin/users/${u.id}`);
    notify.success("User deleted", u.username);
    await loadUsers();
  } catch (e) {
    if (e instanceof ApiError && e.status === 403) {
      notify.error("Not allowed", e.message);
    }
  }
}

function askRevoke(inv: InvitationSummary): void {
  confirmDanger({
    header: "Revoke invitation",
    message: `Revoke the invitation for “${inv.email}”?`,
    acceptLabel: "Revoke",
    accept: () => revokeInvite(inv),
  });
}

async function revokeInvite(inv: InvitationSummary): Promise<void> {
  await api.del(`/admin/invitations/${inv.id}`);
  notify.success("Invitation revoked");
  await loadInvitations();
}
</script>

<template>
  <div class="mx-auto flex h-full max-w-4xl flex-col gap-5 p-8">
    <h1 class="text-2xl font-semibold text-surface-900 dark:text-surface-0">
      Users &amp; access
    </h1>

    <Tabs
      :value="tab"
      :pt="{ root: 'flex min-h-0 flex-1 flex-col' }"
      @update:value="tab = String($event)"
    >
      <TabList>
        <Tab value="users">Users</Tab>
        <Tab value="invitations">Invitations</Tab>
      </TabList>
      <TabPanels>
        <!-- Users -->
        <TabPanel value="users" class="flex h-full flex-col">
          <div class="mb-3 flex shrink-0 justify-end">
            <Button type="button" @click="openCreate">
              <AppIcon :icon="{ type: 'lucide', value: 'plus' }" :size="15" />
              New user
            </Button>
          </div>
          <div class="min-h-0 flex-1">
            <DataTable :value="users" scrollable scroll-height="flex">
              <Column field="username" header="Username">
                <template #body="{ data }">
                  <span class="flex items-center gap-1.5">
                    {{ (data as AdminUser).username }}
                    <span
                      v-if="(data as AdminUser).protected"
                      class="rounded bg-primary-50 px-1.5 py-0.5 text-xs text-primary-600 dark:bg-primary-950/40 dark:text-primary-300"
                      >root</span
                    >
                  </span>
                </template>
              </Column>
              <Column field="email" header="Email">
                <template #body="{ data }">
                  {{ (data as AdminUser).email || "—" }}
                </template>
              </Column>
              <Column header="Roles">
                <template #body="{ data }">
                  <span class="text-surface-500 capitalize">{{
                    (data as AdminUser).roles.join(", ")
                  }}</span>
                </template>
              </Column>
              <Column header="Status">
                <template #body="{ data }">
                  <span
                    :class="
                      (data as AdminUser).disabled
                        ? 'text-amber-600'
                        : 'text-emerald-600'
                    "
                  >
                    {{ (data as AdminUser).disabled ? "Disabled" : "Active" }}
                  </span>
                </template>
              </Column>
              <Column header="" :pt="{ bodyCell: 'text-right' }">
                <template #body="{ data }">
                  <div class="flex items-center justify-end gap-1">
                    <Button
                      as="router-link"
                      :to="{
                        name: 'recordings',
                        query: { user: (data as AdminUser).id },
                      }"
                      text
                      rounded
                      severity="secondary"
                      size="small"
                      title="View recordings"
                      :aria-label="`View recordings for ${(data as AdminUser).username}`"
                    >
                      <AppIcon
                        :icon="{ type: 'lucide', value: 'video' }"
                        :size="16"
                      />
                    </Button>
                    <Button
                      v-if="canEdit(data as AdminUser)"
                      text
                      rounded
                      severity="secondary"
                      size="small"
                      title="Edit"
                      :aria-label="`Edit ${(data as AdminUser).username}`"
                      @click="openEdit(data as AdminUser)"
                    >
                      <AppIcon
                        :icon="{ type: 'lucide', value: 'pencil' }"
                        :size="16"
                      />
                    </Button>
                    <Button
                      v-if="canDelete(data as AdminUser)"
                      text
                      rounded
                      severity="danger"
                      size="small"
                      title="Delete"
                      :aria-label="`Delete ${(data as AdminUser).username}`"
                      @click="askDeleteUser(data as AdminUser)"
                    >
                      <AppIcon
                        :icon="{ type: 'lucide', value: 'trash' }"
                        :size="16"
                      />
                    </Button>
                  </div>
                </template>
              </Column>
              <template #empty>No users.</template>
            </DataTable>
          </div>
        </TabPanel>

        <!-- Invitations -->
        <TabPanel value="invitations" class="flex h-full flex-col">
          <div class="mb-3 flex shrink-0 justify-end">
            <Button type="button" @click="showInvite = true">
              <AppIcon :icon="{ type: 'lucide', value: 'plus' }" :size="15" />
              Invite user
            </Button>
          </div>
          <div class="min-h-0 flex-1">
            <DataTable :value="invitations" scrollable scroll-height="flex">
              <Column field="email" header="Email" />
              <Column header="Role">
                <template #body="{ data }">
                  <span class="text-surface-500 capitalize">{{
                    (data as InvitationSummary).role
                  }}</span>
                </template>
              </Column>
              <Column header="Status">
                <template #body="{ data }">
                  <span class="capitalize">{{
                    (data as InvitationSummary).status
                  }}</span>
                </template>
              </Column>
              <Column header="Expires">
                <template #body="{ data }">
                  {{
                    new Date(
                      (data as InvitationSummary).expiresAt,
                    ).toLocaleDateString()
                  }}
                </template>
              </Column>
              <Column header="" :pt="{ bodyCell: 'text-right' }">
                <template #body="{ data }">
                  <Button
                    v-if="(data as InvitationSummary).status === 'pending'"
                    text
                    rounded
                    severity="danger"
                    size="small"
                    title="Revoke"
                    :aria-label="`Revoke ${(data as InvitationSummary).email}`"
                    @click="askRevoke(data as InvitationSummary)"
                  >
                    <AppIcon
                      :icon="{ type: 'lucide', value: 'x' }"
                      :size="16"
                    />
                  </Button>
                </template>
              </Column>
              <template #empty>No invitations.</template>
            </DataTable>
          </div>
        </TabPanel>
      </TabPanels>
    </Tabs>

    <UserFormDialog
      v-model:visible="showUserForm"
      :user="editingUser"
      @saved="loadUsers"
    />
    <InviteDialog v-model:visible="showInvite" @created="loadInvitations" />
  </div>
</template>
