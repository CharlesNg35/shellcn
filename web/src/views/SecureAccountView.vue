<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import Button from "primevue/button";
import { totpApi } from "../api/twofactor";
import { useAuthStore } from "../stores/auth";
import AppIcon from "../components/AppIcon.vue";
import TwoFactorEnroll from "../components/auth/TwoFactorEnroll.vue";
import { decodeRedirectTarget } from "../router/redirect";

const auth = useAuthStore();
const route = useRoute();
const router = useRouter();

const enrolling = ref(false);
const busy = ref(false);

function destination(): string {
  return decodeRedirectTarget(route.query.redirect);
}

async function remindLater(): Promise<void> {
  busy.value = true;
  try {
    await totpApi.remind();
  } finally {
    auth.dismissReminder();
    await router.replace(destination());
  }
}

async function onEnabled(): Promise<void> {
  auth.dismissReminder();
  await router.replace(destination());
}
</script>

<template>
  <div class="mx-auto flex h-full max-w-lg flex-col justify-center gap-6 p-8">
    <div class="flex flex-col items-center gap-3 text-center">
      <span
        class="flex h-12 w-12 items-center justify-center rounded-2xl bg-primary-50 text-primary-600 dark:bg-primary-950/50 dark:text-primary-300"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'shield-check' }" :size="24" />
      </span>
      <h1
        class="text-2xl font-semibold tracking-tight text-surface-900 dark:text-surface-0"
      >
        Secure your account
      </h1>
      <p class="max-w-md text-sm text-surface-500 dark:text-surface-400">
        Add two-factor authentication so a password alone can't get into your
        account. It only takes a minute with an authenticator app.
      </p>
    </div>

    <section
      class="rounded-xl border border-surface-200 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900"
    >
      <TwoFactorEnroll v-if="enrolling" @enabled="onEnabled" />
      <div v-else class="flex flex-col gap-3 sm:flex-row sm:justify-end">
        <Button
          type="button"
          severity="secondary"
          outlined
          :loading="busy"
          :disabled="busy"
          @click="remindLater"
        >
          Remind me later
        </Button>
        <Button type="button" @click="enrolling = true"> Enable 2FA </Button>
      </div>
    </section>
  </div>
</template>
