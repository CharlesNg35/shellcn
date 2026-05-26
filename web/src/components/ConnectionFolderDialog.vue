<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Dialog from "primevue/dialog";
import InputText from "primevue/inputtext";
import Button from "primevue/button";
import { useConnectionsStore } from "../stores/connections";
import { useNotify } from "../composables/useNotify";
import { dialogRoot, btnGhost, btnPrimary } from "../primevue/preset";
import type { ConnectionFolder, FolderColor } from "../types/projection";

const props = defineProps<{
  visible: boolean;
  folder?: ConnectionFolder | null;
}>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  saved: [folder: ConnectionFolder];
}>();

const conns = useConnectionsStore();
const notify = useNotify();
const name = ref("");
const color = ref<FolderColor>("blue");
const busy = ref(false);
const error = ref<string | null>(null);

const isEdit = computed(() => Boolean(props.folder));

const swatches: Array<{
  value: FolderColor;
  label: string;
  class: string;
}> = [
  { value: "slate", label: "Slate", class: "bg-slate-500" },
  { value: "blue", label: "Blue", class: "bg-blue-500" },
  { value: "teal", label: "Teal", class: "bg-teal-500" },
  { value: "emerald", label: "Emerald", class: "bg-emerald-500" },
  { value: "amber", label: "Amber", class: "bg-amber-500" },
  { value: "rose", label: "Rose", class: "bg-rose-500" },
  { value: "violet", label: "Violet", class: "bg-violet-500" },
  { value: "cyan", label: "Cyan", class: "bg-cyan-500" },
];

watch(
  () => props.visible,
  (open) => {
    if (!open) return;
    name.value = props.folder?.name ?? "";
    color.value = props.folder?.color ?? "blue";
    error.value = null;
  },
  { immediate: true },
);

function close(): void {
  emit("update:visible", false);
}

async function save(): Promise<void> {
  const trimmed = name.value.trim();
  error.value = trimmed ? null : "A folder name is required.";
  if (error.value) return;
  busy.value = true;
  try {
    const saved = props.folder
      ? await conns.updateFolder(props.folder.id, {
          name: trimmed,
          color: color.value,
        })
      : await conns.createFolder({ name: trimmed, color: color.value });
    notify.success(
      props.folder ? "Folder updated" : "Folder created",
      saved.name,
    );
    emit("saved", saved);
    close();
  } catch (e) {
    notify.error("Could not save folder", (e as Error).message);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <Dialog
    :visible="visible"
    modal
    :header="isEdit ? 'Edit folder' : 'New folder'"
    :closable="!busy"
    :pt="{ root: dialogRoot('max-w-sm') }"
    @update:visible="emit('update:visible', $event)"
  >
    <div class="flex min-w-0 flex-col gap-4">
      <div class="flex min-w-0 flex-col gap-1.5">
        <label
          for="folder-name"
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Name <span class="text-red-500">*</span>
        </label>
        <InputText
          id="folder-name"
          :model-value="name"
          placeholder="e.g. Production"
          @update:model-value="name = $event ?? ''"
        />
        <p v-if="error" class="text-xs text-red-500">{{ error }}</p>
      </div>

      <fieldset class="flex min-w-0 flex-col gap-2">
        <legend
          class="text-sm font-medium text-surface-700 dark:text-surface-200"
        >
          Color
        </legend>
        <div class="grid grid-cols-8 gap-1.5">
          <Button
            v-for="swatch in swatches"
            :key="swatch.value"
            type="button"
            text
            rounded
            severity="secondary"
            class="h-8 w-8 p-0"
            :title="swatch.label"
            :aria-label="`${swatch.label} folder color`"
            :aria-pressed="color === swatch.value"
            @click="color = swatch.value"
          >
            <span
              class="h-4 w-4 rounded-full ring-2 ring-offset-2 ring-offset-surface-0 dark:ring-offset-surface-900"
              :class="[
                swatch.class,
                color === swatch.value
                  ? 'ring-surface-900 dark:ring-surface-0'
                  : 'ring-transparent',
              ]"
            />
          </Button>
        </div>
      </fieldset>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <Button
          type="button"
          :disabled="busy"
          :pt="{ root: btnGhost }"
          @click="close"
        >
          Cancel
        </Button>
        <Button
          type="button"
          :disabled="busy"
          :pt="{ root: btnPrimary }"
          @click="save"
        >
          {{ busy ? "Saving..." : isEdit ? "Save changes" : "Create folder" }}
        </Button>
      </div>
    </template>
  </Dialog>
</template>
