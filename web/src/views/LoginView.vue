<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import InputText from "primevue/inputtext";
import Password from "primevue/password";
import Button from "primevue/button";
import { useAuthStore } from "../stores/auth";
import { ApiError } from "../api/client";
import AppLogo from "../components/AppLogo.vue";
import AppIcon from "../components/AppIcon.vue";
import ThemeToggle from "../components/ThemeToggle.vue";

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
  <div class="flex min-h-screen bg-surface-50 dark:bg-surface-950">
    <!-- Brand panel (large screens only) -->
    <aside
      class="relative hidden w-1/2 flex-col justify-between overflow-hidden bg-linear-to-br from-primary-500 via-primary-700 to-primary-900 p-12 text-white lg:flex"
    >
      <div
        class="pointer-events-none absolute inset-0 opacity-[0.18]"
        style="
          background-image: radial-gradient(
            circle at 1px 1px,
            white 1px,
            transparent 0
          );
          background-size: 32px 32px;
        "
      />
      <div
        class="pointer-events-none absolute -top-28 -left-28 h-96 w-96 rounded-full bg-primary-300/30 blur-3xl"
      />
      <div
        class="pointer-events-none absolute -right-24 -bottom-32 h-96 w-96 rounded-full bg-primary-950/40 blur-3xl"
      />

      <div class="relative flex items-center gap-2.5">
        <span
          class="flex h-9 w-9 items-center justify-center rounded-xl bg-white shadow-sm"
        >
          <AppLogo :size="22" class="text-primary-600" />
        </span>
        <span class="text-lg font-semibold tracking-tight">ShellCN</span>
      </div>

      <div class="relative max-w-md">
        <h2 class="text-3xl leading-tight font-semibold tracking-tight">
          One gateway for everything you log into.
        </h2>
        <p class="mt-4 text-base leading-relaxed text-white/70">
          Reach your servers, containers, databases, and desktops from one
          secure, audited place in the browser.
        </p>
      </div>

      <div
        class="relative flex flex-wrap items-center gap-2 text-xs font-medium text-white/80"
      >
        <span
          class="rounded-full bg-white/10 px-3 py-1 ring-1 ring-white/15 ring-inset"
        >
          Self-hosted
        </span>
        <span
          class="rounded-full bg-white/10 px-3 py-1 ring-1 ring-white/15 ring-inset"
        >
          39+ protocols
        </span>
      </div>
    </aside>

    <!-- Form panel -->
    <main
      class="relative flex w-full flex-col justify-center px-6 py-12 sm:px-12 lg:w-1/2"
    >
      <ThemeToggle class="absolute top-5 right-5" />

      <div class="mx-auto w-full max-w-sm">
        <div class="mb-8">
          <AppLogo :size="40" class="mb-6 text-primary-600 lg:hidden" />
          <h1
            class="text-2xl font-semibold tracking-tight text-surface-900 dark:text-surface-0"
          >
            Welcome back
          </h1>
          <p class="mt-1.5 text-sm text-surface-500 dark:text-surface-400">
            Sign in to continue to your cockpit.
          </p>
        </div>

        <form class="flex flex-col gap-5" @submit.prevent="onSubmit">
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
              placeholder="Enter your username"
              autofocus
              required
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
              placeholder="Enter your password"
              :feedback="false"
              toggle-mask
              :input-props="{
                autocomplete: 'current-password',
                required: true,
              }"
            />
          </div>

          <p
            v-if="error"
            class="flex items-center gap-2 rounded-md bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:bg-rose-950/50 dark:text-rose-300"
            role="alert"
          >
            <AppIcon
              :icon="{ type: 'lucide', value: 'circle-alert' }"
              :size="15"
              class="shrink-0"
            />
            {{ error }}
          </p>

          <Button
            type="submit"
            label="Sign in"
            :loading="busy"
            :disabled="busy"
            :pt="{
              root: 'mt-1 flex w-full items-center justify-center gap-1.5 rounded-md bg-primary-600 px-4 py-2.5 text-sm font-medium text-white shadow-sm transition-colors hover:bg-primary-700 focus-visible:ring-2 focus-visible:ring-primary-500/40 disabled:opacity-50',
            }"
          />
        </form>
      </div>
    </main>
  </div>
</template>
