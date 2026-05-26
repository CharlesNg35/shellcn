<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Button from "primevue/button";
import { api, ApiError } from "../api/client";
import { useAuthStore } from "../stores/auth";
import { useNotify } from "../composables/useNotify";
import AppIcon from "../components/AppIcon.vue";
import SkeletonList from "../components/SkeletonList.vue";
import CredentialFormDialog from "../components/CredentialFormDialog.vue";
import ShareDialog from "../components/ShareDialog.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import type {
  CredentialKindInfo,
  CredentialSummary,
} from "../types/projection";

const auth = useAuthStore();
const notify = useNotify();

const items = ref<CredentialSummary[]>([]);
const kinds = ref<CredentialKindInfo[]>([]);
const loading = ref(false);
const error = ref<string | null>(null);

const showForm = ref(false);
const editing = ref<CredentialSummary | null>(null);
const showShare = ref(false);
const shareTarget = ref<CredentialSummary | null>(null);
const showDelete = ref(false);
const deleteTarget = ref<CredentialSummary | null>(null);
const deleting = ref(false);

function canManage(c: CredentialSummary): boolean {
  return auth.isAdmin || c.ownerId === auth.user?.id;
}

function kindInfo(kind: string): CredentialKindInfo | undefined {
  return kinds.value.find((k) => k.kind === kind);
}

function kindLabel(kind: string): string {
  return kindInfo(kind)?.label ?? kind;
}

function identityLabel(c: CredentialSummary): string {
  const label = kindInfo(c.kind)?.identityLabel;
  if (!label || !c.identity) return "—";
  return `${label}: ${c.identity}`;
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = null;
  try {
    const [nextItems, nextKinds] = await Promise.all([
      api.get<CredentialSummary[]>("/credentials"),
      api.get<CredentialKindInfo[]>("/credential-kinds"),
    ]);
    items.value = nextItems;
    kinds.value = nextKinds;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}
onMounted(load);

function openCreate(): void {
  editing.value = null;
  showForm.value = true;
}
function openEdit(c: CredentialSummary): void {
  editing.value = c;
  showForm.value = true;
}
function openShare(c: CredentialSummary): void {
  shareTarget.value = c;
  showShare.value = true;
}
function openDelete(c: CredentialSummary): void {
  deleteTarget.value = c;
  showDelete.value = true;
}

async function onDelete(): Promise<void> {
  if (!deleteTarget.value) return;
  deleting.value = true;
  try {
    await api.del(`/credentials/${deleteTarget.value.id}`);
    notify.success("Credential deleted", deleteTarget.value.name);
    showDelete.value = false;
    await load();
  } catch (e) {
    if (e instanceof ApiError && e.status === 409) {
      notify.error(
        "In use",
        "This credential is still referenced by a connection.",
      );
    }
  } finally {
    deleting.value = false;
  }
}

const hasItems = computed(() => items.value.length > 0);
</script>

<template>
  <div class="mx-auto flex h-full max-w-4xl flex-col gap-5 p-8">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-semibold text-surface-900 dark:text-surface-0">
          Credentials
        </h1>
      </div>
      <Button type="button" @click="openCreate">
        <AppIcon :icon="{ type: 'name', value: 'plus' }" :size="15" />
        New credential
      </Button>
    </div>

    <p v-if="error" class="text-sm text-red-500">{{ error }}</p>
    <SkeletonList v-else-if="loading && !hasItems" :rows="6" />

    <div
      v-else-if="!hasItems"
      class="flex flex-col items-center gap-3 rounded-lg border border-dashed border-surface-300 py-16 text-center dark:border-surface-700"
    >
      <AppIcon
        :icon="{ type: 'name', value: 'key' }"
        :size="28"
        class="text-surface-400"
      />
      <p class="text-surface-500">No credentials yet.</p>
      <Button type="button" @click="openCreate">
        Create your first credential
      </Button>
    </div>

    <DataTable v-else :value="items" scrollable scroll-height="flex">
      <Column field="name" header="Name" />
      <Column header="Kind">
        <template #body="{ data }">
          {{ kindLabel((data as CredentialSummary).kind) }}
        </template>
      </Column>
      <Column header="Identity">
        <template #body="{ data }">
          {{ identityLabel(data as CredentialSummary) }}
        </template>
      </Column>
      <Column header="Protocols">
        <template #body="{ data }">
          <span class="text-surface-500">{{
            (data as CredentialSummary).protocols?.join(", ") || "any"
          }}</span>
        </template>
      </Column>
      <Column header="" :pt="{ bodyCell: 'text-right' }">
        <template #body="{ data }">
          <div
            v-if="canManage(data as CredentialSummary)"
            class="flex items-center justify-end gap-1"
          >
            <button
              type="button"
              class="rounded p-1.5 text-surface-500 hover:bg-surface-100 hover:text-surface-700 dark:hover:bg-surface-800"
              title="Edit / rotate"
              :aria-label="`Edit ${(data as CredentialSummary).name}`"
              @click="openEdit(data as CredentialSummary)"
            >
              <AppIcon :icon="{ type: 'name', value: 'pencil' }" :size="16" />
            </button>
            <button
              type="button"
              class="rounded p-1.5 text-surface-500 hover:bg-surface-100 hover:text-surface-700 dark:hover:bg-surface-800"
              title="Share"
              :aria-label="`Share ${(data as CredentialSummary).name}`"
              @click="openShare(data as CredentialSummary)"
            >
              <AppIcon :icon="{ type: 'name', value: 'users' }" :size="16" />
            </button>
            <button
              type="button"
              class="rounded p-1.5 text-surface-500 hover:bg-surface-100 hover:text-red-500 dark:hover:bg-surface-800"
              title="Delete"
              :aria-label="`Delete ${(data as CredentialSummary).name}`"
              @click="openDelete(data as CredentialSummary)"
            >
              <AppIcon :icon="{ type: 'name', value: 'trash' }" :size="16" />
            </button>
          </div>
          <span v-else class="text-xs text-surface-400">shared with you</span>
        </template>
      </Column>
      <template #empty>No credentials.</template>
    </DataTable>

    <CredentialFormDialog
      v-model:visible="showForm"
      :credential="editing"
      @saved="load"
    />
    <ShareDialog
      v-if="shareTarget"
      v-model:visible="showShare"
      resource="credentials"
      :resource-id="shareTarget.id"
      :resource-name="shareTarget.name"
    />
    <ConfirmDialog
      v-model:visible="showDelete"
      title="Delete credential"
      :message="`Delete “${deleteTarget?.name}”? Connections that reference it must be updated first.`"
      confirm-label="Delete"
      danger
      :busy="deleting"
      @confirm="onDelete"
    />
  </div>
</template>
