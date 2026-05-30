<script setup lang="ts">
import { ref } from "vue";
import InputText from "primevue/inputtext";
import Password from "primevue/password";
import Button from "primevue/button";
import { ApiError } from "../api/client";
import { authApi } from "../api/auth";
import { useAuthStore } from "../stores/auth";
import { useNotify } from "../composables/useNotify";
import { btnPrimary } from "../primevue/preset";
import TwoFactorSection from "../components/auth/TwoFactorSection.vue";

const auth = useAuthStore();
const notify = useNotify();

const displayName = ref(auth.user?.displayName ?? "");
const email = ref(auth.user?.email ?? "");
const savingProfile = ref(false);

const currentPassword = ref("");
const newPassword = ref("");
const confirmPassword = ref("");
const pwError = ref<string | null>(null);
const savingPassword = ref(false);

async function saveProfile(): Promise<void> {
  savingProfile.value = true;
  try {
    const updated = await authApi.updateProfile({
      displayName: displayName.value.trim(),
      email: email.value.trim(),
    });
    auth.user = updated;
    notify.success("Profile updated");
  } catch (e) {
    if (e instanceof ApiError && e.status === 400) {
      notify.error("Could not update profile", e.message);
    }
  } finally {
    savingProfile.value = false;
  }
}

async function savePassword(): Promise<void> {
  pwError.value = null;
  if (newPassword.value.length < 8) {
    pwError.value = "New password must be at least 8 characters.";
    return;
  }
  if (newPassword.value !== confirmPassword.value) {
    pwError.value = "New passwords do not match.";
    return;
  }
  savingPassword.value = true;
  try {
    await auth.changePassword(currentPassword.value, newPassword.value);
    currentPassword.value = "";
    newPassword.value = "";
    confirmPassword.value = "";
    notify.success("Password updated");
  } catch (e) {
    if (e instanceof ApiError && e.status === 400) {
      pwError.value = e.message;
    }
  } finally {
    savingPassword.value = false;
  }
}
</script>

<template>
  <div class="mx-auto flex h-full max-w-2xl flex-col gap-6 overflow-auto p-8">
    <h1 class="text-2xl font-semibold text-surface-900 dark:text-surface-0">
      Your profile
    </h1>

    <section
      class="flex min-w-0 flex-col gap-4 rounded-xl border border-surface-200 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900"
    >
      <h2 class="text-sm font-semibold text-surface-900 dark:text-surface-100">
        Account
      </h2>

      <div class="flex min-w-0 flex-col gap-1.5">
        <label
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >Username</label
        >
        <div
          class="rounded-md border border-surface-200 bg-surface-50 px-2.5 py-1.5 text-sm text-surface-500 dark:border-surface-800 dark:bg-surface-950/60 dark:text-surface-400"
        >
          {{ auth.user?.username }}
        </div>
        <p class="text-xs text-surface-400">Your username can’t be changed.</p>
      </div>

      <div class="flex min-w-0 flex-col gap-1.5">
        <label
          for="profile-name"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >Display name</label
        >
        <InputText
          id="profile-name"
          :model-value="displayName"
          @update:model-value="displayName = $event ?? ''"
        />
      </div>

      <div class="flex min-w-0 flex-col gap-1.5">
        <label
          for="profile-email"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >Email</label
        >
        <InputText
          id="profile-email"
          :model-value="email"
          type="email"
          @update:model-value="email = $event ?? ''"
        />
      </div>

      <div class="flex justify-end">
        <Button
          type="button"
          label="Save profile"
          :loading="savingProfile"
          :disabled="savingProfile"
          :pt="{ root: btnPrimary }"
          @click="saveProfile"
        />
      </div>
    </section>

    <TwoFactorSection />

    <section
      class="flex min-w-0 flex-col gap-4 rounded-xl border border-surface-200 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900"
    >
      <h2 class="text-sm font-semibold text-surface-900 dark:text-surface-100">
        Change password
      </h2>

      <div class="flex min-w-0 flex-col gap-1.5">
        <label
          for="cur-pw"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >Current password</label
        >
        <Password
          v-model="currentPassword"
          input-id="cur-pw"
          :feedback="false"
          toggle-mask
          :input-props="{ autocomplete: 'current-password' }"
        />
      </div>

      <div class="flex min-w-0 flex-col gap-1.5">
        <label
          for="new-pw"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >New password</label
        >
        <Password
          v-model="newPassword"
          input-id="new-pw"
          :feedback="false"
          toggle-mask
          :input-props="{ autocomplete: 'new-password' }"
        />
      </div>

      <div class="flex min-w-0 flex-col gap-1.5">
        <label
          for="confirm-pw"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >Confirm new password</label
        >
        <Password
          v-model="confirmPassword"
          input-id="confirm-pw"
          :feedback="false"
          toggle-mask
          :input-props="{ autocomplete: 'new-password' }"
        />
      </div>

      <p v-if="pwError" class="text-xs text-red-500" role="alert">
        {{ pwError }}
      </p>

      <div class="flex justify-end">
        <Button
          type="button"
          label="Update password"
          :loading="savingPassword"
          :disabled="savingPassword || !currentPassword || !newPassword"
          :pt="{ root: btnPrimary }"
          @click="savePassword"
        />
      </div>
    </section>
  </div>
</template>
