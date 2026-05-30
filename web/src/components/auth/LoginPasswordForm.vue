<script setup lang="ts">
import { ref } from "vue";
import InputText from "primevue/inputtext";
import Password from "primevue/password";
import Button from "primevue/button";
import { btnPrimaryBlock } from "../../primevue/preset";
import AuthAlert from "./AuthAlert.vue";

defineProps<{ busy: boolean; error: string | null }>();
const emit = defineEmits<{
  submit: [credentials: { username: string; password: string }];
}>();

const username = ref("");
const password = ref("");

function submit(): void {
  emit("submit", { username: username.value.trim(), password: password.value });
}
</script>

<template>
  <form class="flex flex-col gap-5" @submit.prevent="submit">
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
        :input-props="{ autocomplete: 'current-password', required: true }"
      />
    </div>

    <AuthAlert v-if="error" :message="error" />

    <Button
      type="submit"
      label="Sign in"
      :loading="busy"
      :disabled="busy"
      :pt="{ root: btnPrimaryBlock + ' mt-1' }"
    />
  </form>
</template>
