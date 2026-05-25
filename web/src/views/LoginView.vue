<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import InputText from "primevue/inputtext";
import Password from "primevue/password";
import Button from "primevue/button";
import { useAuthStore } from "../stores/auth";
import { ApiError } from "../api/client";
import AppIcon from "../components/AppIcon.vue";

const auth = useAuthStore();
const route = useRoute();
const router = useRouter();

const username = ref("");
const password = ref("");
const error = ref<string | null>(null);
const busy = ref(false);

async function onSubmit(): Promise<void> {
  error.value = null;
  busy.value = true;
  try {
    await auth.login(username.value.trim(), password.value);
    const redirect = route.query.redirect;
    await router.replace(typeof redirect === "string" ? redirect : "/");
  } catch (e) {
    error.value =
      e instanceof ApiError && e.status === 401
        ? "Invalid username or password."
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
          <AppIcon :icon="{ type: 'name', value: 'terminal' }" :size="24" />
        </span>
        <h1
          class="text-xl font-semibold tracking-tight text-surface-900 dark:text-surface-0"
        >
          Sign in to ShellCN
        </h1>
      </div>

      <form
        class="flex flex-col gap-4 rounded-xl border border-surface-200 bg-surface-0 p-6 shadow-sm dark:border-surface-800 dark:bg-surface-900"
        @submit.prevent="onSubmit"
      >
        <div class="flex flex-col gap-1.5">
          <label
            for="login-username"
            class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >
            Username
          </label>
          <InputText
            id="login-username"
            v-model="username"
            autocomplete="username"
            autofocus
            required
            :pt="{ root: 'w-full' }"
          />
        </div>

        <div class="flex flex-col gap-1.5">
          <label
            for="login-password"
            class="text-sm font-medium text-surface-700 dark:text-surface-200"
          >
            Password
          </label>
          <Password
            v-model="password"
            input-id="login-password"
            :feedback="false"
            toggle-mask
            :input-props="{ autocomplete: 'current-password', required: true }"
            :pt="{ root: 'w-full', pcInputText: { root: 'w-full' } }"
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
          :disabled="busy"
          :pt="{
            root: 'flex w-full items-center justify-center rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-primary-700 disabled:opacity-50',
          }"
        >
          {{ busy ? "Signing in…" : "Sign in" }}
        </Button>
      </form>
    </div>
  </div>
</template>
