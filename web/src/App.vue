<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { RouterView, useRouter } from "vue-router";
import { useToast } from "primevue/usetoast";
import ConfirmDialog from "primevue/confirmdialog";
import { setApiErrorHandler, type ApiError } from "./api/client";
import { useAuthStore } from "./stores/auth";
import AppToast from "./components/AppToast.vue";
import AppIcon from "./components/AppIcon.vue";
import AppRouteLoader from "./components/AppRouteLoader.vue";

const toast = useToast();
const router = useRouter();
const auth = useAuthStore();
const routeLoading = ref(false);
let routeLoadingTimer: ReturnType<typeof window.setTimeout> | undefined;
let removeBeforeGuard: (() => void) | undefined;
let removeAfterGuard: (() => void) | undefined;
let removeErrorGuard: (() => void) | undefined;

function stopRouteLoading(): void {
  if (routeLoadingTimer) {
    window.clearTimeout(routeLoadingTimer);
    routeLoadingTimer = undefined;
  }
  routeLoading.value = false;
}

// 401 → re-login; 403/network/server errors → toast. 400/404/409 pass through
// to the caller for inline handling so feedback isn't duplicated.
onMounted(() => {
  removeBeforeGuard = router.beforeEach((to, from) => {
    if (to.fullPath === from.fullPath) return;
    stopRouteLoading();
    routeLoadingTimer = window.setTimeout(() => {
      routeLoading.value = true;
    }, 120);
  });
  removeAfterGuard = router.afterEach(stopRouteLoading);
  removeErrorGuard = router.onError(stopRouteLoading);

  setApiErrorHandler((err: ApiError) => {
    if (err.status === 401 && err.authRequired) {
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
onUnmounted(() => {
  setApiErrorHandler(null);
  removeBeforeGuard?.();
  removeAfterGuard?.();
  removeErrorGuard?.();
  stopRouteLoading();
});
</script>

<template>
  <!-- Session bootstrap gate: a branded loader covers the brief window between
       mount and the first route resolving (auth /me), so there's no blank flash
       or premature login flicker. Hands off seamlessly from the index.html splash. -->
  <AppRouteLoader v-if="!auth.ready" label="Loading ShellCN" :fixed="false" />
  <template v-else>
    <RouterView v-slot="{ Component }">
      <component :is="Component" />
    </RouterView>
    <Transition
      enter-active-class="transition-opacity duration-150"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition-opacity duration-150"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <AppRouteLoader v-if="routeLoading" label="Loading view" />
    </Transition>
  </template>
  <AppToast />
  <ConfirmDialog>
    <template #message="{ message }">
      <div class="flex items-start gap-3">
        <AppIcon
          :icon="{ type: 'lucide', value: 'triangle-alert' }"
          :size="20"
          class="mt-0.5 shrink-0 text-rose-500"
        />
        <p class="text-sm text-surface-600 dark:text-surface-300">
          {{ message.message }}
        </p>
      </div>
    </template>
  </ConfirmDialog>
</template>
