<script setup lang="ts">
import { onUnmounted, ref, watch } from "vue";
import Dialog from "primevue/dialog";
import Select from "primevue/select";
import InputText from "primevue/inputtext";
import Button from "primevue/button";
import { ApiError } from "../api/client";
import { invitationsApi } from "../api/invitations";
import { useNotify } from "../composables/useNotify";
import AppIcon from "./AppIcon.vue";
import { dialogRoot, btnPrimary, btnGhost } from "../primevue/preset";
import { Role, ROLE_OPTIONS } from "../constants/roles";
import type { InviteResult } from "../types/projection";

const props = defineProps<{ visible: boolean }>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  created: [];
}>();

const notify = useNotify();

const email = ref("");
const role = ref<Role>(Role.Viewer);
const result = ref<InviteResult | null>(null);
const error = ref<string | null>(null);
const busy = ref(false);
const copied = ref(false);
let copiedTimer: ReturnType<typeof setTimeout> | undefined;

function clearCopiedTimer(): void {
  if (copiedTimer) clearTimeout(copiedTimer);
  copiedTimer = undefined;
}

watch(
  () => props.visible,
  (open) => {
    clearCopiedTimer();
    if (!open) {
      copied.value = false;
      return;
    }
    email.value = "";
    role.value = Role.Viewer;
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
    result.value = await invitationsApi.create({
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
    clearCopiedTimer();
    copiedTimer = setTimeout(() => (copied.value = false), 1500);
  } catch {
    // clipboard unavailable
  }
}

onUnmounted(clearCopiedTimer);
</script>

<template>
  <Dialog
    :visible="visible"
    modal
    header="Invite a user"
    :pt="{
      root: dialogRoot(),
      content: 'min-h-0 overflow-auto p-5',
    }"
    @update:visible="emit('update:visible', $event)"
  >
    <!-- Step 1: choose who + what role -->
    <div v-if="!result" class="flex min-w-0 flex-col gap-4">
      <div class="flex min-w-0 flex-col gap-1.5">
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
      <div class="flex min-w-0 flex-col gap-1.5">
        <label
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Role
        </label>
        <Select
          :model-value="role"
          :options="ROLE_OPTIONS"
          option-label="label"
          option-value="value"
          @update:model-value="role = $event"
        />
      </div>
      <p v-if="error" class="text-xs text-red-500">{{ error }}</p>
    </div>

    <!-- Step 2: share the link -->
    <div v-else class="flex min-w-0 flex-col gap-3">
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
        <Button
          text
          severity="secondary"
          size="small"
          class="shrink-0"
          @click="copyLink"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'copy' }" :size="13" />
          {{ copied ? "Copied" : "Copy" }}
        </Button>
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
          :pt="{ root: btnGhost }"
          @click="emit('update:visible', false)"
        >
          Cancel
        </Button>
        <Button
          v-if="!result"
          type="button"
          label="Create invitation"
          :loading="busy"
          :disabled="busy"
          :pt="{ root: btnPrimary }"
          @click="invite"
        />
        <Button
          v-else
          type="button"
          :pt="{ root: btnPrimary }"
          @click="emit('update:visible', false)"
        >
          Done
        </Button>
      </div>
    </template>
  </Dialog>
</template>
