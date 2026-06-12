<script setup lang="ts">
import { ref } from "vue";
import InputText from "primevue/inputtext";
import Button from "primevue/button";
import { btnPrimaryBlock } from "@/primevue/preset";
import AuthAlert from "./AuthAlert.vue";

defineProps<{ busy: boolean; error: string | null }>();
const emit = defineEmits<{ submit: [code: string]; back: [] }>();

const code = ref("");

function submit(): void {
  emit("submit", code.value.trim());
}
</script>

<template>
  <form class="flex flex-col gap-5" @submit.prevent="submit">
    <div class="flex flex-col gap-1.5">
      <label
        for="login-otp"
        class="text-sm font-medium text-surface-700 dark:text-surface-200"
      >
        Authentication code
      </label>
      <InputText
        id="login-otp"
        :model-value="code"
        inputmode="text"
        autocomplete="one-time-code"
        placeholder="123456"
        autofocus
        class="text-center font-mono tracking-[0.3em]"
        @update:model-value="code = $event ?? ''"
      />
    </div>

    <AuthAlert v-if="error" :message="error" />

    <Button
      type="submit"
      label="Verify"
      :loading="busy"
      :disabled="busy || !code.trim()"
      :pt="{ root: btnPrimaryBlock + ' mt-1' }"
    />

    <button
      type="button"
      class="text-sm text-surface-500 transition-colors hover:text-surface-800 dark:hover:text-surface-200"
      @click="emit('back')"
    >
      Back to sign in
    </button>
  </form>
</template>
