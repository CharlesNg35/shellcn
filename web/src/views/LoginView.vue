<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { ApiError } from "../api/client";
import AppLogo from "../components/AppLogo.vue";
import ThemeToggle from "../components/ThemeToggle.vue";
import LoginPasswordForm from "../components/auth/LoginPasswordForm.vue";
import LoginOtpForm from "../components/auth/LoginOtpForm.vue";

const auth = useAuthStore();
const route = useRoute();
const router = useRouter();

const step = ref<"password" | "otp">("password");
const error = ref<string | null>(null);
const busy = ref(false);

const highlights = ["Self-hosted", "39+ protocols"];

// After a completed sign-in, send users who haven't enabled 2FA to the nudge
// page; everyone else continues to their original destination.
async function finishLogin(): Promise<void> {
  const redirect = route.query.redirect;
  const dest = typeof redirect === "string" ? redirect : "/";
  if (auth.mfaReminder) {
    await router.replace({ name: "secure-account", query: { redirect: dest } });
  } else {
    await router.replace(dest);
  }
}

async function onPassword(credentials: {
  username: string;
  password: string;
}): Promise<void> {
  error.value = null;
  busy.value = true;
  try {
    const { mfaRequired } = await auth.login(
      credentials.username,
      credentials.password,
    );
    if (mfaRequired) {
      step.value = "otp";
      return;
    }
    await finishLogin();
  } catch (e) {
    error.value =
      e instanceof ApiError && e.status === 401
        ? "Invalid username or password."
        : (e as Error).message;
  } finally {
    busy.value = false;
  }
}

async function onOtp(code: string): Promise<void> {
  error.value = null;
  busy.value = true;
  try {
    await auth.completeMfa(code);
    await finishLogin();
  } catch (e) {
    error.value =
      e instanceof ApiError && e.status === 401
        ? "Invalid code. Try again, or use a recovery code."
        : (e as Error).message;
  } finally {
    busy.value = false;
  }
}

function backToPassword(): void {
  auth.cancelMfa();
  step.value = "password";
  error.value = null;
}
</script>

<template>
  <div class="flex min-h-screen bg-surface-50 dark:bg-surface-950">
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
          v-for="item in highlights"
          :key="item"
          class="rounded-full bg-white/10 px-3 py-1 ring-1 ring-white/15 ring-inset"
        >
          {{ item }}
        </span>
      </div>
    </aside>

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
            {{ step === "otp" ? "Two-factor authentication" : "Welcome back" }}
          </h1>
          <p class="mt-1.5 text-sm text-surface-500 dark:text-surface-400">
            {{
              step === "otp"
                ? "Enter the code from your authenticator app, or a recovery code."
                : "Sign in to continue to your cockpit."
            }}
          </p>
        </div>

        <LoginPasswordForm
          v-if="step === 'password'"
          :busy="busy"
          :error="error"
          @submit="onPassword"
        />
        <LoginOtpForm
          v-else
          :busy="busy"
          :error="error"
          @submit="onOtp"
          @back="backToPassword"
        />
      </div>
    </main>
  </div>
</template>
