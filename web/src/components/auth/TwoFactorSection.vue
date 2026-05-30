<script setup lang="ts">
import { ref } from "vue";
import Dialog from "primevue/dialog";
import Button from "primevue/button";
import { totpApi } from "../../api/twofactor";
import { useAuthStore } from "../../stores/auth";
import { useNotify } from "../../composables/useNotify";
import { dialogRoot, btnPrimary } from "../../primevue/preset";
import AppIcon from "../AppIcon.vue";
import TwoFactorEnroll from "./TwoFactorEnroll.vue";
import TwoFactorCodeDialog from "./TwoFactorCodeDialog.vue";
import RecoveryCodesList from "./RecoveryCodesList.vue";

const auth = useAuthStore();
const notify = useNotify();

const showEnroll = ref(false);
const showDisable = ref(false);
const showRegenerate = ref(false);
const showNewCodes = ref(false);
const newCodes = ref<string[]>([]);

function onEnabled(): void {
  showEnroll.value = false;
  notify.success("Two-factor authentication enabled");
}

async function disable(code: string): Promise<void> {
  await totpApi.disable(code);
  await auth.refresh();
}

function onDisabled(): void {
  notify.success("Two-factor authentication disabled");
}

async function regenerate(code: string): Promise<void> {
  newCodes.value = (await totpApi.regenerateRecoveryCodes(code)).recoveryCodes;
}

function onRegenerated(): void {
  showNewCodes.value = true;
}
</script>

<template>
  <section
    class="flex min-w-0 flex-col gap-4 rounded-xl border border-surface-200 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900"
  >
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <h2
          class="text-sm font-semibold text-surface-900 dark:text-surface-100"
        >
          Two-factor authentication
        </h2>
        <p class="mt-1 text-sm text-surface-500 dark:text-surface-400">
          Protect your account with a time-based code from an authenticator app.
        </p>
      </div>
      <span
        v-if="auth.twoFactorEnabled"
        class="inline-flex shrink-0 items-center gap-1.5 rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'shield-check' }" :size="13" />
        Enabled
      </span>
    </div>

    <div v-if="!auth.twoFactorEnabled" class="flex justify-start">
      <Button
        type="button"
        label="Enable 2FA"
        :pt="{ root: btnPrimary }"
        @click="showEnroll = true"
      />
    </div>
    <div v-else class="flex flex-wrap gap-2">
      <Button
        type="button"
        severity="secondary"
        outlined
        @click="showRegenerate = true"
      >
        Regenerate recovery codes
      </Button>
      <Button
        type="button"
        severity="danger"
        outlined
        @click="showDisable = true"
      >
        Disable 2FA
      </Button>
    </div>

    <Dialog
      v-model:visible="showEnroll"
      modal
      header="Enable two-factor authentication"
      :pt="{ root: dialogRoot('max-w-lg'), content: 'p-5' }"
    >
      <TwoFactorEnroll v-if="showEnroll" @enabled="onEnabled" />
    </Dialog>

    <TwoFactorCodeDialog
      v-model:visible="showDisable"
      title="Disable two-factor authentication"
      description="Enter a current code to turn off 2FA. Your recovery codes will be invalidated."
      confirm-label="Disable"
      danger
      :action="disable"
      @done="onDisabled"
    />

    <TwoFactorCodeDialog
      v-model:visible="showRegenerate"
      title="Regenerate recovery codes"
      description="Enter a current code to generate a new set. Your previous recovery codes will stop working."
      confirm-label="Regenerate"
      :action="regenerate"
      @done="onRegenerated"
    />

    <Dialog
      v-model:visible="showNewCodes"
      modal
      header="Your new recovery codes"
      :pt="{ root: dialogRoot(), content: 'p-5' }"
    >
      <div class="flex flex-col gap-4">
        <p class="text-sm text-surface-600 dark:text-surface-300">
          Save these somewhere safe. They won't be shown again.
        </p>
        <RecoveryCodesList :codes="newCodes" />
        <div class="flex justify-end">
          <Button
            type="button"
            label="Done"
            :pt="{ root: btnPrimary }"
            @click="showNewCodes = false"
          />
        </div>
      </div>
    </Dialog>
  </section>
</template>
