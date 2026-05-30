<script setup lang="ts">
import { ref, watch } from "vue";
import Dialog from "primevue/dialog";
import InputText from "primevue/inputtext";
import Button from "primevue/button";
import { ApiError } from "../../api/client";
import {
  dialogRoot,
  btnPrimary,
  btnDanger,
  btnGhost,
} from "../../primevue/preset";
import AuthAlert from "./AuthAlert.vue";

const props = defineProps<{
  visible: boolean;
  title: string;
  description: string;
  confirmLabel: string;
  danger?: boolean;
  // The parent supplies the verified action; the dialog owns the code, busy, and
  // error state so the disable/regenerate callers stay small.
  action: (code: string) => Promise<void>;
}>();
const emit = defineEmits<{ "update:visible": [value: boolean]; done: [] }>();

const code = ref("");
const error = ref<string | null>(null);
const busy = ref(false);

watch(
  () => props.visible,
  (open) => {
    if (open) {
      code.value = "";
      error.value = null;
      busy.value = false;
    }
  },
);

async function submit(): Promise<void> {
  error.value = null;
  if (!code.value.trim()) {
    error.value = "Enter a code from your authenticator, or a recovery code.";
    return;
  }
  busy.value = true;
  try {
    await props.action(code.value.trim());
    emit("update:visible", false);
    emit("done");
  } catch (e) {
    error.value =
      e instanceof ApiError ? e.message : "Could not verify the code.";
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <Dialog
    :visible="visible"
    modal
    :header="title"
    :pt="{ root: dialogRoot(), content: 'p-5' }"
    @update:visible="emit('update:visible', $event)"
  >
    <div class="flex min-w-0 flex-col gap-4">
      <p class="text-sm text-surface-600 dark:text-surface-300">
        {{ description }}
      </p>
      <InputText
        :model-value="code"
        inputmode="text"
        autocomplete="one-time-code"
        placeholder="123456"
        class="font-mono tracking-[0.2em]"
        @update:model-value="code = $event ?? ''"
        @keyup.enter="submit"
      />
      <AuthAlert v-if="error" :message="error" />
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <Button
          type="button"
          label="Cancel"
          :disabled="busy"
          :pt="{ root: btnGhost }"
          @click="emit('update:visible', false)"
        />
        <Button
          type="button"
          :label="confirmLabel"
          :loading="busy"
          :disabled="busy"
          :pt="{ root: danger ? btnDanger : btnPrimary }"
          @click="submit"
        />
      </div>
    </template>
  </Dialog>
</template>
