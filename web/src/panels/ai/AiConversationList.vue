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
    class="flex h-full min-h-0 w-64 flex-col border-r border-surface-200 bg-surface-50/80 dark:border-surface-800 dark:bg-surface-950"
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
    </div>
    <ul class="min-h-0 flex-1 space-y-1 overflow-y-auto p-2">
      <li
        v-for="c in conversations"
        :key="c.id"
        class="group flex min-w-0 items-center gap-1 rounded-lg border px-2 py-1.5 text-xs transition-colors"
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
          <span class="min-w-0 flex-1 truncate font-medium">
            {{ c.title }}
          </span>
        </Button>
        <Button
          type="button"
          text
          rounded
          severity="secondary"
          size="small"
          class="text-surface-400 opacity-0 transition-opacity group-hover:opacity-100 hover:text-surface-700 focus-visible:opacity-100 dark:hover:text-surface-100"
          aria-label="Rename"
          @click="rename(c)"
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
          @click="remove(c)"
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
