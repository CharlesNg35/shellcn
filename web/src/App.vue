<script setup lang="ts">
import { onMounted, onUnmounted } from "vue";
import { RouterView, useRouter } from "vue-router";
import { useToast } from "primevue/usetoast";
import { setApiErrorHandler, type ApiError } from "./api/client";
import { useAuthStore } from "./stores/auth";
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
  <RouterView />
  <AppToast />
</template>
