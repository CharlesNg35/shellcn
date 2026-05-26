<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import InputText from "primevue/inputtext";
import Password from "primevue/password";
import Button from "primevue/button";
import { api, ApiError } from "../api/client";
import AppIcon from "../components/AppIcon.vue";

const route = useRoute();
const router = useRouter();
const token = String(route.params.token);

const email = ref("");
const username = ref("");
const password = ref("");
const loading = ref(true);
const invalid = ref(false);
const busy = ref(false);
const error = ref<string | null>(null);

onMounted(async () => {
  try {
    const inv = await api.get<{ email: string }>(`/invitations/${token}`);
    email.value = inv.email;
  } catch {
    invalid.value = true;
  } finally {
    loading.value = false;
  }
});

async function onSubmit(): Promise<void> {
  error.value = null;
  busy.value = true;
  try {
    await api.post(`/invitations/${token}/accept`, {
      username: username.value.trim(),
      password: password.value,
    });
    await router.replace({ name: "login" });
  } catch (e) {
    error.value =
      e instanceof ApiError && e.status === 409
        ? "That username is taken — choose another."
        : (e as Error).message;
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div
    class="flex min-h-screen items-center justify-center bg-surface-50 p-4 dark:bg-surface-950"
  >
    <div class="w-full max-w-sm">
      <div class="mb-8 flex flex-col items-center gap-3 text-center">
        <span
          class="flex h-12 w-12 items-center justify-center rounded-2xl bg-primary-600 text-white"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'user' }" :size="24" />
        </span>
        <h1
          class="text-xl font-semibold tracking-tight text-surface-900 dark:text-surface-0"
        >
          Accept your invitation
        </h1>
        <p v-if="email" class="text-sm text-surface-500">
          Set up the account for <span class="font-medium">{{ email }}</span>
        </p>
      </div>

      <p v-if="loading" class="text-center text-sm text-surface-400">
        Loading…
      </p>

      <div
        v-else-if="invalid"
        class="rounded-xl border border-surface-200 bg-surface-0 p-6 text-center dark:border-surface-800 dark:bg-surface-900"
      >
        <p class="text-sm text-surface-600 dark:text-surface-300">
          This invitation is invalid, expired, or already used.
        </p>
        <RouterLink
          :to="{ name: 'login' }"
          class="mt-3 inline-block text-sm text-primary-600 hover:underline"
        >
          Go to sign in
        </RouterLink>
      </div>

      <form
        v-else
        class="flex flex-col gap-4 rounded-xl border border-surface-200 bg-surface-0 p-6 shadow-sm dark:border-surface-800 dark:bg-surface-900"
        @submit.prevent="onSubmit"
      >
        <div class="flex flex-col gap-1.5">
          <label
            for="invite-username"
            class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >
            Username
          </label>
          <InputText
            id="invite-username"
            v-model="username"
            autocomplete="username"
            autofocus
            required
          />
        </div>

        <div class="flex flex-col gap-1.5">
          <label
            for="invite-password"
            class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >
            Password
          </label>
          <Password
            v-model="password"
            input-id="invite-password"
            :feedback="false"
            toggle-mask
            :input-props="{ autocomplete: 'new-password', required: true }"
          />
        </div>

        <p
          v-if="error"
          class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-600 dark:bg-red-950/50 dark:text-red-300"
          role="alert"
        >
          {{ error }}
        </p>

        <Button
          type="submit"
          label="Create account"
          :loading="busy"
          :disabled="busy"
          :pt="{
            root: 'flex w-full items-center justify-center gap-1.5 rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-primary-700 disabled:opacity-50',
          }"
        />
      </form>
    </div>
  </div>
</template>
