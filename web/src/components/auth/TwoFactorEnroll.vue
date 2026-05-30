<script setup lang="ts">
import { onMounted, ref } from "vue";
import InputText from "primevue/inputtext";
import Checkbox from "primevue/checkbox";
import Button from "primevue/button";
import { ApiError } from "../../api/client";
import { totpApi, type TotpSetup } from "../../api/twofactor";
import { useAuthStore } from "../../stores/auth";
import { btnPrimary } from "../../primevue/preset";
import AppIcon from "../AppIcon.vue";
import RecoveryCodesList from "./RecoveryCodesList.vue";

const emit = defineEmits<{ enabled: [] }>();

const auth = useAuthStore();

type Step = "loading" | "scan" | "recovery" | "error";
const step = ref<Step>("loading");
const setup = ref<TotpSetup | null>(null);
const error = ref<string | null>(null);
const code = ref("");
const codeError = ref<string | null>(null);
const busy = ref(false);
const recoveryCodes = ref<string[]>([]);
const acknowledged = ref(false);

async function start(): Promise<void> {
  step.value = "loading";
  error.value = null;
  try {
    setup.value = await totpApi.setup();
    step.value = "scan";
  } catch (e) {
    error.value = (e as Error).message;
    step.value = "error";
  }
}
onMounted(start);

async function confirm(): Promise<void> {
  codeError.value = null;
  if (code.value.trim().length < 6) {
    codeError.value = "Enter the 6-digit code from your app.";
    return;
  }
  busy.value = true;
  try {
    const result = await totpApi.enable(code.value.trim());
    recoveryCodes.value = result.recoveryCodes;
    await auth.refresh();
    step.value = "recovery";
  } catch (e) {
    codeError.value =
      e instanceof ApiError ? e.message : "Could not verify the code.";
  } finally {
    busy.value = false;
  }
}

function finish(): void {
  emit("enabled");
}
</script>

<template>
  <div class="flex min-w-0 flex-col gap-4">
    <p v-if="step === 'loading'" class="text-sm text-surface-400">
      Preparing your authenticator…
    </p>

    <div v-else-if="step === 'error'" class="flex flex-col items-start gap-3">
      <p class="text-sm text-red-500">{{ error }}</p>
      <Button type="button" severity="secondary" outlined @click="start">
        Try again
      </Button>
    </div>

    <template v-else-if="step === 'scan' && setup">
      <ol
        class="flex list-inside list-decimal flex-col gap-1 text-sm text-surface-600 dark:text-surface-300"
      >
        <li>
          Open your authenticator app (Google Authenticator, 1Password, …).
        </li>
        <li>Scan this QR code, or enter the key manually.</li>
        <li>Enter the 6-digit code it shows to finish.</li>
      </ol>

      <div class="flex flex-col items-center gap-3 sm:flex-row sm:items-start">
        <img
          :src="setup.qr"
          alt="Two-factor QR code"
          class="h-40 w-40 shrink-0 rounded-lg border border-surface-200 bg-white p-2 dark:border-surface-700"
        />
        <div class="flex min-w-0 flex-1 flex-col gap-1.5">
          <span class="text-xs font-medium text-surface-500">
            Manual entry key
          </span>
          <code
            class="rounded-md border border-surface-200 bg-surface-50 px-2.5 py-1.5 font-mono text-xs break-all text-surface-700 dark:border-surface-800 dark:bg-surface-950 dark:text-surface-200"
          >
            {{ setup.secret }}
          </code>
        </div>
      </div>

      <div class="flex min-w-0 flex-col gap-1.5">
        <label
          for="totp-code"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Verification code
        </label>
        <InputText
          id="totp-code"
          :model-value="code"
          inputmode="numeric"
          autocomplete="one-time-code"
          maxlength="6"
          placeholder="123456"
          class="w-40 text-center font-mono tracking-[0.3em]"
          @update:model-value="code = ($event ?? '').replace(/\D/g, '')"
          @keyup.enter="confirm"
        />
        <p v-if="codeError" class="text-xs text-red-500" role="alert">
          {{ codeError }}
        </p>
      </div>

      <div class="flex justify-end">
        <Button
          type="button"
          label="Enable 2FA"
          :loading="busy"
          :disabled="busy"
          :pt="{ root: btnPrimary }"
          @click="confirm"
        />
      </div>
    </template>

    <template v-else-if="step === 'recovery'">
      <div
        class="flex items-start gap-2 rounded-md bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:bg-amber-950/40 dark:text-amber-300"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'triangle-alert' }"
          :size="16"
          class="mt-0.5 shrink-0"
        />
        <span>
          Save these recovery codes somewhere safe. Each works once if you lose
          your authenticator. They won't be shown again.
        </span>
      </div>

      <RecoveryCodesList :codes="recoveryCodes" />

      <label
        for="ack-codes"
        class="flex items-center gap-2 text-sm text-surface-600 dark:text-surface-300"
      >
        <Checkbox v-model="acknowledged" input-id="ack-codes" binary />
        I've saved my recovery codes.
      </label>

      <div class="flex justify-end">
        <Button
          type="button"
          label="Done"
          :disabled="!acknowledged"
          :pt="{ root: btnPrimary }"
          @click="finish"
        />
      </div>
    </template>
  </div>
</template>
