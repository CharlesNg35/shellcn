<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Dialog from "primevue/dialog";
import Select from "primevue/select";
import AutoComplete from "primevue/autocomplete";
import InputText from "primevue/inputtext";
import Button from "primevue/button";
import { api, ApiError } from "../api/client";
import { useAuthStore } from "../stores/auth";
import { useNotify } from "../composables/useNotify";
import AppIcon from "./AppIcon.vue";
import { useConfirmAction } from "../composables/useConfirmAction";
import { dialogRoot, btnPrimary } from "../primevue/preset";
import type { GrantAccess, ShareGrant, UserSummary } from "../types/projection";

const props = defineProps<{
  visible: boolean;
  // The control-plane collection: "connections" or "credentials".
  resource: "connections" | "credentials";
  resourceId: string;
  resourceName: string;
  // Connections grant use/manage; credentials grant use only.
  allowManage?: boolean;
}>();
const emit = defineEmits<{ "update:visible": [value: boolean] }>();

const notify = useNotify();
const auth = useAuthStore();

const grants = ref<ShareGrant[]>([]);
type UserOption = UserSummary & { label: string };
const users = ref<UserOption[]>([]);
const subject = ref<UserOption | null>(null);
// Operators can't enumerate users; they share by exact email instead.
const email = ref("");
const access = ref<GrantAccess>("use");
const loading = ref(false);
const busy = ref(false);
const { confirmDanger } = useConfirmAction();

const canAdd = computed(() =>
  auth.isAdmin ? Boolean(subject.value) : email.value.trim().length > 0,
);

const base = computed(() => `/${props.resource}/${props.resourceId}/grants`);
const accessChoices = [
  { label: "Use", value: "use" },
  { label: "Manage", value: "manage" },
];

const subjectChoices = computed(() => {
  const taken = new Set(grants.value.map((g) => g.subjectId));
  return users.value.filter((u) => !taken.has(u.id));
});

async function load(): Promise<void> {
  loading.value = true;
  subject.value = null;
  email.value = "";
  access.value = "use";
  try {
    grants.value = await api.get<ShareGrant[]>(base.value);
    if (auth.isAdmin) await searchUsers("");
  } finally {
    loading.value = false;
  }
}

function userLabel(user: UserSummary): string {
  return user.displayName
    ? `${user.displayName} (${user.username})`
    : user.username;
}

async function searchUsers(query: string): Promise<void> {
  const params = new URLSearchParams();
  if (query.trim()) params.set("query", query.trim());
  const found = await api.get<UserSummary[]>(
    `/admin/users/search${params.toString() ? `?${params.toString()}` : ""}`,
  );
  users.value = found.map((u) => ({ ...u, label: userLabel(u) }));
}

function completeUsers(event: { query: string }): void {
  void searchUsers(event.query);
}

watch(
  () => props.visible,
  (open) => {
    if (open) void load();
  },
  { immediate: true },
);

async function add(): Promise<void> {
  if (!canAdd.value) return;
  busy.value = true;
  try {
    const body: Record<string, unknown> = {
      access: props.allowManage ? access.value : "use",
    };
    if (auth.isAdmin && subject.value) body.subjectId = subject.value.id;
    else body.email = email.value.trim();
    const grant = await api.post<ShareGrant>(base.value, body);
    grants.value = [...grants.value, grant];
    subject.value = null;
    email.value = "";
    access.value = "use";
    notify.success("Access granted", grant.username);
    if (auth.isAdmin) await searchUsers("");
  } catch (e) {
    if (e instanceof ApiError)
      notify.error("Could not grant access", e.message);
  } finally {
    busy.value = false;
  }
}

function requestRevoke(grant: ShareGrant): void {
  confirmDanger({
    header: "Revoke access",
    message: `Revoke access for ${grant.username || grant.subjectId}?`,
    acceptLabel: "Revoke",
    accept: () => revoke(grant),
  });
}

async function revoke(grant: ShareGrant): Promise<void> {
  try {
    await api.del(`${base.value}/${grant.id}`);
    grants.value = grants.value.filter((g) => g.id !== grant.id);
    notify.success("Access revoked", grant.username);
    await searchUsers("");
  } catch (e) {
    if (e instanceof ApiError)
      notify.error("Could not revoke access", e.message);
  }
}
</script>

<template>
  <Dialog
    :visible="visible"
    modal
    :header="`Share “${resourceName}”`"
    :pt="{
      root: dialogRoot(),
      content: 'min-h-0 max-h-[70vh] overflow-auto p-5',
    }"
    @update:visible="emit('update:visible', $event)"
  >
    <p v-if="loading" class="py-6 text-center text-sm text-surface-400">
      Loading…
    </p>

    <div v-else class="flex min-w-0 flex-col gap-4">
      <!-- Add a subject -->
      <div class="flex items-end gap-2">
        <div class="min-w-0 flex-1">
          <label
            class="mb-1 block text-xs font-medium text-surface-500 dark:text-surface-400"
          >
            {{ auth.isAdmin ? "User" : "Email" }}
          </label>
          <AutoComplete
            v-if="auth.isAdmin"
            :model-value="subject"
            :suggestions="subjectChoices"
            option-label="label"
            force-selection
            placeholder="Select a user"
            input-class="w-full"
            @complete="completeUsers"
            @update:model-value="subject = $event"
          />
          <InputText
            v-else
            :model-value="email"
            type="email"
            placeholder="person@example.com"
            class="w-full"
            @update:model-value="email = $event ?? ''"
            @keyup.enter="add"
          />
        </div>
        <div v-if="allowManage" class="w-28">
          <label
            class="mb-1 block text-xs font-medium text-surface-500 dark:text-surface-400"
          >
            Access
          </label>
          <Select
            :model-value="access"
            :options="accessChoices"
            option-label="label"
            option-value="value"
            @update:model-value="access = $event"
          />
        </div>
        <Button
          type="button"
          label="Add"
          :loading="busy"
          :disabled="busy || !canAdd"
          :pt="{ root: btnPrimary }"
          @click="add"
        />
      </div>

      <!-- Current grants -->
      <ul
        v-if="grants.length"
        class="divide-y divide-surface-200 rounded-md border border-surface-200 dark:divide-surface-800 dark:border-surface-800"
      >
        <li
          v-for="g in grants"
          :key="g.id"
          class="flex items-center gap-2 px-3 py-2"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: 'user' }"
            :size="15"
            class="text-surface-400"
          />
          <span
            class="min-w-0 flex-1 truncate text-sm text-surface-700 dark:text-surface-200"
          >
            {{ g.username || g.subjectId }}
          </span>
          <span
            class="rounded bg-surface-100 px-1.5 py-0.5 text-xs text-surface-500 capitalize dark:bg-surface-800"
          >
            {{ g.access }}
          </span>
          <Button
            text
            rounded
            severity="danger"
            size="small"
            :title="`Revoke ${g.username || g.subjectId}`"
            :aria-label="`Revoke ${g.username || g.subjectId}`"
            @click="requestRevoke(g)"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="15" />
          </Button>
        </li>
      </ul>
      <p v-else class="text-sm text-surface-400">Not shared with anyone yet.</p>
    </div>
  </Dialog>
</template>
