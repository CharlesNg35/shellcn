<script setup lang="ts">
import Button from "primevue/button";
import AppIcon from "../../components/AppIcon.vue";
import { useConfirmAction } from "../../composables/useConfirmAction";
import type { AiConversation } from "../../api/ai";

defineProps<{
  conversations: AiConversation[];
  activeId: string | null;
  streamingId: string | null;
  busy: boolean;
}>();
const emit = defineEmits<{
  select: [id: string];
  create: [];
  rename: [id: string, title: string];
  remove: [id: string];
}>();

const { confirmDanger } = useConfirmAction();

function rename(c: AiConversation): void {
  const next = window.prompt("Rename conversation", c.title);
  if (next && next.trim() && next.trim() !== c.title)
    emit("rename", c.id, next.trim());
}

function remove(c: AiConversation): void {
  confirmDanger({
    header: "Delete conversation",
    message: `Delete "${c.title}"? This cannot be undone.`,
    accept: () => emit("remove", c.id),
  });
}
</script>

<template>
  <div
    class="flex h-full min-h-0 w-56 flex-col border-r border-surface-200 dark:border-surface-800"
  >
    <div class="flex items-center gap-1 p-2">
      <Button
        :pt="{ root: 'flex-1' }"
        severity="secondary"
        outlined
        size="small"
        :disabled="busy"
        @click="emit('create')"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'plus' }" :size="14" />
        New chat
      </Button>
    </div>
    <ul class="min-h-0 flex-1 overflow-y-auto px-1 pb-2">
      <li
        v-for="c in conversations"
        :key="c.id"
        class="group flex items-center gap-1 rounded-md px-2 py-1.5 text-xs"
        :class="
          c.id === activeId
            ? 'bg-surface-100 dark:bg-surface-800'
            : 'hover:bg-surface-50 dark:hover:bg-surface-800/50'
        "
      >
        <button
          type="button"
          class="flex min-w-0 flex-1 items-center gap-1.5 text-left"
          :disabled="busy"
          @click="emit('select', c.id)"
        >
          <span
            v-if="c.id === streamingId"
            class="h-1.5 w-1.5 shrink-0 animate-pulse rounded-full bg-primary-500"
            aria-label="streaming"
          />
          <span class="truncate text-surface-700 dark:text-surface-200">{{
            c.title
          }}</span>
        </button>
        <button
          type="button"
          class="hidden text-surface-400 group-hover:block hover:text-surface-700 dark:hover:text-surface-100"
          aria-label="Rename"
          @click="rename(c)"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'pencil' }" :size="12" />
        </button>
        <button
          type="button"
          class="hidden text-surface-400 group-hover:block hover:text-red-500"
          aria-label="Delete"
          @click="remove(c)"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'trash' }" :size="12" />
        </button>
      </li>
      <li
        v-if="conversations.length === 0"
        class="px-2 py-3 text-center text-xs text-surface-400"
      >
        No conversations yet.
      </li>
    </ul>
  </div>
</template>
