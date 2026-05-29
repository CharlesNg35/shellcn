<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Dialog from "primevue/dialog";
import Select from "primevue/select";
import InputText from "primevue/inputtext";
import Password from "primevue/password";
import ToggleSwitch from "primevue/toggleswitch";
import Button from "primevue/button";
import { api, ApiError } from "../api/client";
import { useNotify } from "../composables/useNotify";
import { dialogRoot, btnPrimary, btnGhost } from "../primevue/preset";
import { Role, ROLE_OPTIONS } from "../constants/roles";
import type { AdminUser } from "../types/projection";

const props = defineProps<{ visible: boolean; user?: AdminUser | null }>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  saved: [];
}>();

const notify = useNotify();

const roleOptions = ROLE_OPTIONS;

const roleHint = computed(
  () => roleOptions.find((o) => o.value === role.value)?.description ?? "",
);

const isEdit = computed(() => Boolean(props.user));
const protectedUser = computed(() => props.user?.protected ?? false);

const username = ref("");
const email = ref("");
const displayName = ref("");
const role = ref<Role>(Role.Viewer);
const password = ref("");
const disabled = ref(false);
const errors = ref<Record<string, string>>({});
const busy = ref(false);

watch(
  () => props.visible,
  (open) => {
    if (!open) return;
    errors.value = {};
    password.value = "";
    if (props.user) {
      username.value = props.user.username;
      email.value = props.user.email ?? "";
      displayName.value = props.user.displayName ?? "";
      role.value = props.user.roles[0] ?? Role.Viewer;
      disabled.value = props.user.disabled;
    } else {
      username.value = "";
      email.value = "";
      displayName.value = "";
      role.value = Role.Viewer;
      disabled.value = false;
    }
  },
  { immediate: true },
);

function validate(): boolean {
  const next: Record<string, string> = {};
  if (!isEdit.value && !username.value.trim()) next.username = "Required.";
  if (!isEdit.value && !password.value.trim()) next.password = "Required.";
  errors.value = next;
  return Object.keys(next).length === 0;
}

async function save(): Promise<void> {
  if (!validate()) return;
  busy.value = true;
  try {
    if (isEdit.value && props.user) {
      await api.put(`/admin/users/${props.user.id}`, {
        email: email.value.trim(),
        displayName: displayName.value.trim(),
        role: role.value,
        disabled: disabled.value,
      });
      notify.success("User updated", username.value);
    } else {
      await api.post("/admin/users", {
        username: username.value.trim(),
        email: email.value.trim(),
        displayName: displayName.value.trim(),
        role: role.value,
        password: password.value,
      });
      notify.success("User created", username.value);
    }
    emit("saved");
    emit("update:visible", false);
  } catch (e) {
    if (e instanceof ApiError && (e.status === 400 || e.status === 409)) {
      notify.error("Could not save user", e.message);
    }
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <Dialog
    :visible="visible"
    modal
    :header="isEdit ? 'Edit user' : 'New user'"
    :closable="!busy"
    :pt="{
      root: dialogRoot(),
      content: 'min-h-0 max-h-[70vh] overflow-auto p-5',
    }"
    @update:visible="emit('update:visible', $event)"
  >
    <div class="flex min-w-0 flex-col gap-4">
      <div v-if="!isEdit" class="flex min-w-0 flex-col gap-1.5">
        <label
          for="user-username"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Username <span class="text-red-500">*</span>
        </label>
        <InputText
          id="user-username"
          :model-value="username"
          @update:model-value="username = $event ?? ''"
        />
        <p v-if="errors.username" class="text-xs text-red-500">
          {{ errors.username }}
        </p>
      </div>

      <div class="flex min-w-0 flex-col gap-1.5">
        <label
          for="user-email"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Email
        </label>
        <InputText
          id="user-email"
          :model-value="email"
          @update:model-value="email = $event ?? ''"
        />
      </div>

      <div class="flex min-w-0 flex-col gap-1.5">
        <label
          for="user-display"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Display name
        </label>
        <InputText
          id="user-display"
          :model-value="displayName"
          @update:model-value="displayName = $event ?? ''"
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
          :options="roleOptions"
          option-label="label"
          option-value="value"
          :disabled="protectedUser"
          @update:model-value="role = $event"
        />
        <p class="text-xs text-surface-400">{{ roleHint }}</p>
      </div>

      <div v-if="!isEdit" class="flex min-w-0 flex-col gap-1.5">
        <label
          for="user-password"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Password <span class="text-red-500">*</span>
        </label>
        <Password
          v-model="password"
          input-id="user-password"
          :feedback="false"
          toggle-mask
          :input-props="{ autocomplete: 'new-password' }"
        />
        <p v-if="errors.password" class="text-xs text-red-500">
          {{ errors.password }}
        </p>
      </div>

      <label
        v-if="isEdit"
        class="flex items-center justify-between gap-3 text-sm text-surface-700 dark:text-surface-200"
      >
        <span>Disabled</span>
        <ToggleSwitch
          :model-value="disabled"
          :disabled="protectedUser"
          @update:model-value="disabled = $event"
        />
      </label>

      <p v-if="protectedUser" class="text-xs text-surface-400">
        The root admin must remain an enabled admin.
      </p>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <Button
          type="button"
          :disabled="busy"
          :pt="{ root: btnGhost }"
          @click="emit('update:visible', false)"
        >
          Cancel
        </Button>
        <Button
          type="button"
          :label="isEdit ? 'Save changes' : 'Create user'"
          :loading="busy"
          :disabled="busy"
          :pt="{ root: btnPrimary }"
          @click="save"
        />
      </div>
    </template>
  </Dialog>
</template>
