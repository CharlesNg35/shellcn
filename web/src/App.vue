<script setup lang="ts">
import { onMounted, onUnmounted } from "vue";
import { RouterView, useRouter } from "vue-router";
import { useToast } from "primevue/usetoast";
import { setApiErrorHandler, type ApiError } from "./api/client";
import { useAuthStore } from "./stores/auth";
import AppLogo from "./components/AppLogo.vue";
import AppToast from "./components/AppToast.vue";

const toast = useToast();
const router = useRouter();
const auth = useAuthStore();

// 401 → re-login; 403/network/server errors → toast. 400/404/409 pass through
// to the caller for inline handling so feedback isn't duplicated.
onMounted(() => {
  setApiErrorHandler((err: ApiError) => {
    if (err.status === 401) {
      if (router.currentRoute.value.name !== "login") {
        auth.clear();
        void router.push({
          name: "login",
          query: { redirect: router.currentRoute.value.fullPath },
        });
      }
      return;
    }
    if (err.status === 403 || err.status === 0 || err.status >= 500) {
      toast.add({
        severity: "error",
        summary: err.status === 403 ? "Not allowed" : "Something went wrong",
        detail: err.message,
        life: 5000,
      });
    }
  });
});
onUnmounted(() => setApiErrorHandler(null));
</script>

<template>
  <!-- Session bootstrap gate: a branded loader covers the brief window between
       mount and the first route resolving (auth /me), so there's no blank flash
       or premature login flicker. Hands off seamlessly from the index.html splash. -->
  <div
    v-if="!auth.ready"
    class="flex h-full flex-col items-center justify-center gap-[18px] bg-surface-50 dark:bg-surface-950"
  >
    <AppLogo :size="44" class="text-primary-600" />
    <span
      class="h-[22px] w-[22px] animate-spin rounded-full border-[2.5px] border-surface-200 border-t-primary-500 dark:border-surface-800 dark:border-t-primary-500"
      role="status"
      aria-label="Loading"
    />
  </div>
  <template v-else>
    <RouterView />
  </template>
  <AppToast />
</template>
