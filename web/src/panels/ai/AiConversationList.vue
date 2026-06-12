<script setup lang="ts">
import Button from "primevue/button";
import InputText from "primevue/inputtext";
import { nextTick, ref, type VNodeRef } from "vue";
import AppIcon from "@/components/AppIcon.vue";
import { useConfirmAction } from "@/composables/useConfirmAction";
import type { AiConversation } from "@/api/ai";

defineProps<{
  conversations: AiConversation[];
  activeId: string | null;
  streamingId: string | null;
  busy: boolean;
}>();
const emit = defineEmits<{
  select: [id: string];
  create: [];
  close: [];
  rename: [id: string, title: string];
  remove: [id: string];
}>();

const { confirmDanger } = useConfirmAction();
const editingId = ref<string | null>(null);
const renameTitle = ref("");
const renameInputEl = ref<HTMLInputElement | null>(null);

const setRenameInput: VNodeRef = (el) => {
  const input =
    el instanceof HTMLInputElement
      ? el
      : el &&
          typeof el === "object" &&
          "$el" in el &&
          el.$el instanceof HTMLInputElement
        ? el.$el
        : null;
  renameInputEl.value = input;
};

function startRename(c: AiConversation): void {
  if (!c.id) return;
  editingId.value = c.id;
  renameTitle.value = c.title || "New chat";
  void nextTick(() => {
    renameInputEl.value?.focus();
    renameInputEl.value?.select();
  });
}

function cancelRename(): void {
  editingId.value = null;
  renameTitle.value = "";
}

function submitRename(c: AiConversation): void {
  if (editingId.value !== c.id) return;
  const nextTitle = renameTitle.value.trim();
  const currentTitle = (c.title || "New chat").trim();
  if (c.id && nextTitle && nextTitle !== currentTitle) {
    emit("rename", c.id, nextTitle);
  }
  cancelRename();
}

function remove(c: AiConversation): void {
  if (!c.id) return;
  const id = c.id;
  const title = c.title || "New chat";
  confirmDanger({
    header: "Delete conversation",
    message: `Delete "${title}"? This cannot be undone.`,
    accept: () => emit("remove", id),
  });
}
</script>

<template>
  <div
    class="flex h-full min-h-0 w-72 max-w-[82vw] flex-col border-r border-surface-200 bg-surface-0 shadow-2xl ring-1 ring-surface-950/5 dark:border-surface-800 dark:bg-surface-950 dark:ring-surface-0/10"
  >
    <div
      class="flex items-center justify-between gap-2 border-b border-surface-200 px-3 py-2.5 dark:border-surface-800"
    >
      <div class="flex min-w-0 items-center gap-2">
        <AppIcon
          :icon="{ type: 'lucide', value: 'messages-square' }"
          :size="15"
          class="text-surface-500 dark:text-surface-400"
        />
        <span
          class="truncate text-xs font-semibold tracking-wide text-surface-500 uppercase dark:text-surface-400"
        >
          History
        </span>
      </div>
      <div class="flex shrink-0 items-center gap-1">
        <Button
          text
          rounded
          size="small"
          :disabled="busy"
          aria-label="New chat"
          @click="emit('create')"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'plus' }" :size="14" />
        </Button>
        <Button
          text
          rounded
          severity="secondary"
          size="small"
          aria-label="Close history"
          @click="emit('close')"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="14" />
        </Button>
      </div>
    </div>
    <ul class="min-h-0 flex-1 space-y-1 overflow-y-auto p-2">
      <li
        v-for="c in conversations"
        :key="c.id"
        class="group relative flex min-w-0 items-center gap-1 rounded-lg border px-2 py-1.5 text-xs transition-colors"
        :class="
          c.id === activeId
            ? 'border-primary-200 bg-primary-50 text-primary-800 dark:border-primary-900/70 dark:bg-primary-500/10 dark:text-primary-200'
            : 'border-transparent text-surface-600 hover:border-surface-200 hover:bg-surface-0 dark:text-surface-300 dark:hover:border-surface-800 dark:hover:bg-surface-900'
        "
      >
        <Button
          type="button"
          text
          severity="secondary"
          class="flex min-w-0 flex-1 items-center gap-2 text-left"
          :disabled="busy"
          :class="editingId === c.id ? 'pointer-events-none opacity-40' : ''"
          @click="emit('select', c.id)"
        >
          <span
            v-if="c.id === streamingId"
            class="h-1.5 w-1.5 shrink-0 animate-pulse rounded-full bg-primary-500"
            aria-label="streaming"
          />
          <AppIcon
            v-else
            :icon="{ type: 'lucide', value: 'message-square' }"
            :size="13"
            class="shrink-0 text-current/55"
          />
          <span
            class="min-w-0 flex-1 truncate font-medium"
            :title="c.title || 'New chat'"
          >
            {{ c.title || "New chat" }}
          </span>
        </Button>
        <form
          v-if="editingId === c.id"
          class="absolute inset-x-2 top-1/2 z-10 flex -translate-y-1/2 items-center gap-1 rounded-lg bg-surface-0 dark:bg-surface-950"
          @submit.prevent="submitRename(c)"
          @keydown.esc.prevent.stop="cancelRename"
          @click.stop
        >
          <InputText
            :ref="setRenameInput"
            v-model="renameTitle"
            class="min-w-0 flex-1 px-2 py-1 text-xs"
            autocomplete="off"
            aria-label="Conversation title"
            @blur="submitRename(c)"
          />
          <Button
            type="submit"
            text
            rounded
            severity="secondary"
            size="small"
            aria-label="Save title"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'check' }" :size="12" />
          </Button>
          <Button
            type="button"
            text
            rounded
            severity="secondary"
            size="small"
            aria-label="Cancel rename"
            @mousedown.prevent
            @click="cancelRename"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="12" />
          </Button>
        </form>
        <Button
          type="button"
          text
          rounded
          severity="secondary"
          size="small"
          class="text-surface-400 opacity-0 transition-opacity group-hover:opacity-100 hover:text-surface-700 focus-visible:opacity-100 dark:hover:text-surface-100"
          aria-label="Rename"
          @click.stop="startRename(c)"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'pencil' }" :size="12" />
        </Button>
        <Button
          type="button"
          text
          rounded
          severity="danger"
          size="small"
          class="text-surface-400 opacity-0 transition-opacity group-hover:opacity-100 hover:text-red-500 focus-visible:opacity-100"
          aria-label="Delete"
          @click.stop="remove(c)"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'trash' }" :size="12" />
        </Button>
      </li>
      <li
        v-if="conversations.length === 0"
        class="grid gap-1 rounded-lg border border-dashed border-surface-200 px-3 py-5 text-center text-xs text-surface-400 dark:border-surface-800"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'message-circle' }"
          :size="18"
          class="mx-auto text-surface-300 dark:text-surface-600"
        />
        <span>No conversations yet.</span>
      </li>
    </ul>
  </div>
</template>
