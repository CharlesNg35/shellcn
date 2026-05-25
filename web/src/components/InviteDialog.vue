<script setup lang="ts">
import { ref, watch } from "vue";
import Dialog from "primevue/dialog";
import Select from "primevue/select";
import InputText from "primevue/inputtext";
import Button from "primevue/button";
import { api, ApiError } from "../api/client";
import { useNotify } from "../composables/useNotify";
import AppIcon from "./AppIcon.vue";
import type { InviteResult } from "../types/projection";

const props = defineProps<{ visible: boolean }>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  created: [];
}>();

const notify = useNotify();

const roleOptions = [
  { label: "Admin", value: "admin" },
  { label: "Operator", value: "operator" },
  { label: "Viewer", value: "viewer" },
];

const email = ref("");
const role = ref("viewer");
const result = ref<InviteResult | null>(null);
const error = ref<string | null>(null);
const busy = ref(false);
const copied = ref(false);

watch(
  () => props.visible,
  (open) => {
    if (!open) return;
    email.value = "";
    role.value = "viewer";
    result.value = null;
    error.value = null;
    copied.value = false;
  },
  { immediate: true },
);

async function invite(): Promise<void> {
  error.value = null;
  if (!email.value.includes("@")) {
    error.value = "Enter a valid email address.";
    return;
  }
  busy.value = true;
  try {
    result.value = await api.post<InviteResult>("/admin/invitations", {
      email: email.value.trim(),
      role: role.value,
    });
    emit("created");
  } catch (e) {
    if (e instanceof ApiError) error.value = e.message;
  } finally {
    busy.value = false;
  }
}

async function copyLink(): Promise<void> {
  if (!result.value) return;
  try {
    await navigator.clipboard?.writeText(result.value.link);
    copied.value = true;
    notify.success("Link copied");
    setTimeout(() => (copied.value = false), 1500);
  } catch {
    // clipboard unavailable
  }
}
</script>

<template>
  <Dialog
    :visible="visible"
    modal
    header="Invite a user"
    :pt="{
      root: 'w-full max-w-md rounded-lg bg-surface-0 shadow-xl dark:bg-surface-900',
      content: 'p-5',
    }"
    @update:visible="emit('update:visible', $event)"
  >
    <!-- Step 1: choose who + what role -->
    <div v-if="!result" class="flex flex-col gap-4">
      <div class="flex flex-col gap-1.5">
        <label
          for="invite-email"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Email <span class="text-red-500">*</span>
        </label>
        <InputText
          id="invite-email"
          :model-value="email"
          placeholder="person@example.com"
          @update:model-value="email = $event ?? ''"
        />
      </div>
      <div class="flex flex-col gap-1.5">
        <label
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Role
        </label>
        <Select
          :model-value="role"
          :options="roleOptions"
          option-label="label"
          option-value="value"
          @update:model-value="role = $event"
        />
      </div>
      <p v-if="error" class="text-xs text-red-500">{{ error }}</p>
    </div>

    <!-- Step 2: share the link -->
    <div v-else class="flex flex-col gap-3">
      <p class="text-sm text-surface-600 dark:text-surface-300">
        Invitation created for
        <span class="font-medium">{{ result.invitation.email }}</span
        >.
        <span v-if="result.emailSent">An email has been sent.</span>
        <span v-else>Email is not configured — share this link:</span>
      </p>
      <div
        class="flex items-center gap-2 rounded-md border border-surface-200 bg-surface-50 px-2.5 py-1.5 dark:border-surface-700 dark:bg-surface-950"
      >
        <span class="min-w-0 flex-1 truncate font-mono text-xs">{{
          result.link
        }}</span>
        <button
          type="button"
          class="flex shrink-0 items-center gap-1 rounded px-2 py-1 text-xs text-primary-600 hover:bg-surface-100 dark:hover:bg-surface-800"
          @click="copyLink"
        >
          <AppIcon :icon="{ type: 'name', value: 'copy' }" :size="13" />
          {{ copied ? "Copied" : "Copy" }}
        </button>
      </div>
      <p class="text-xs text-surface-400">
        The link expires
        {{ new Date(result.invitation.expiresAt).toLocaleString() }}.
      </p>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <Button
          v-if="!result"
          type="button"
          :disabled="busy"
          :pt="{
            root: 'rounded-md px-3 py-1.5 text-sm text-surface-600 hover:bg-surface-100 dark:text-surface-300 dark:hover:bg-surface-800',
          }"
          @click="emit('update:visible', false)"
        >
          Cancel
        </Button>
        <Button
          v-if="!result"
          type="button"
          :disabled="busy"
          :pt="{
            root: 'rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700 disabled:opacity-50',
          }"
          @click="invite"
        >
          {{ busy ? "Creating…" : "Create invitation" }}
        </Button>
        <Button
          v-else
          type="button"
          :pt="{
            root: 'rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700',
          }"
          @click="emit('update:visible', false)"
        >
          Done
        </Button>
      </div>
    </template>
  </Dialog>
</template>
