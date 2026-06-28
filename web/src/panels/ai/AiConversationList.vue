<script setup lang="ts">
import Button from "primevue/button";
import InputText from "primevue/inputtext";
import { computed, nextTick, onMounted, ref, type VNodeRef } from "vue";
import AppIcon from "@/components/AppIcon.vue";
import { useConfirmAction } from "@/composables/useConfirmAction";
import { groupConversations, relativeTimeLabel } from "./conversationGroups";
import type { AiConversation } from "@/api/ai";

const props = defineProps<{
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
const panelEl = ref<HTMLElement | null>(null);

const groups = computed(() => groupConversations(props.conversations));
const timeLabel = (c: AiConversation): string => relativeTimeLabel(c);

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

onMounted(() => panelEl.value?.focus());
</script>

<template>
  <section
    ref="panelEl"
    class="flex h-full min-h-0 flex-col bg-surface-0 outline-none dark:bg-surface-950"
    role="dialog"
    aria-modal="true"
    aria-label="Conversation history"
    tabindex="-1"
    @keydown.esc.stop="emit('close')"
  >
    <div
      class="flex items-center gap-2 border-b border-surface-200 px-3 py-2.5 dark:border-surface-800"
    >
      <AppIcon
        :icon="{ type: 'lucide', value: 'messages-square' }"
        :size="16"
        class="shrink-0 text-surface-500 dark:text-surface-400"
      />
      <span
        class="min-w-0 flex-1 truncate text-sm font-semibold text-surface-800 dark:text-surface-100"
      >
        Conversations
      </span>
      <Button
        text
        rounded
        severity="secondary"
        size="small"
        aria-label="Close history"
        @click="emit('close')"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="15" />
      </Button>
    </div>

    <div class="px-3 pt-3">
      <Button
        type="button"
        outlined
        severity="secondary"
        class="flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-surface-300 py-2 text-sm font-medium text-surface-600 hover:border-primary-400 hover:text-primary-600 dark:border-surface-700 dark:text-surface-300 dark:hover:border-primary-500 dark:hover:text-primary-300"
        :disabled="busy"
        @click="emit('create')"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'plus' }" :size="15" />
        New chat
      </Button>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto px-2 py-2">
      <template v-for="group in groups" :key="group.key">
        <h3
          class="sticky top-0 z-10 bg-surface-0/95 px-2 py-1.5 text-[11px] font-semibold tracking-wide text-surface-400 uppercase backdrop-blur dark:bg-surface-950/95 dark:text-surface-500"
        >
          {{ group.label }}
        </h3>
        <ul class="mb-2 space-y-0.5">
          <li
            v-for="c in group.items"
            :key="c.id"
            class="group relative rounded-lg transition-colors"
            :class="
              c.id === activeId
                ? 'bg-primary-50 dark:bg-primary-500/10'
                : 'hover:bg-surface-100 dark:hover:bg-surface-900'
            "
          >
            <Button
              type="button"
              text
              severity="secondary"
              class="flex w-full items-start gap-2.5 rounded-lg px-2.5 py-2 pr-16 text-left"
              :disabled="busy"
              :class="editingId === c.id ? 'pointer-events-none opacity-0' : ''"
              @click="emit('select', c.id)"
            >
              <span
                v-if="c.id === streamingId"
                class="mt-1.5 h-2 w-2 shrink-0 animate-pulse rounded-full bg-primary-500 motion-reduce:animate-none"
                aria-label="streaming"
              />
              <span
                v-else
                class="mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-md"
                :class="
                  c.id === activeId
                    ? 'text-primary-600 dark:text-primary-300'
                    : 'text-surface-400'
                "
              >
                <AppIcon
                  :icon="{ type: 'lucide', value: 'message-square' }"
                  :size="13"
                />
              </span>
              <span class="grid min-w-0 flex-1 gap-0.5">
                <span
                  class="truncate text-xs font-medium"
                  :class="
                    c.id === activeId
                      ? 'text-primary-800 dark:text-primary-100'
                      : 'text-surface-700 dark:text-surface-200'
                  "
                  :title="c.title || 'New chat'"
                >
                  {{ c.title || "New chat" }}
                </span>
                <span
                  v-if="timeLabel(c)"
                  class="truncate text-[11px] text-surface-400 dark:text-surface-500"
                >
                  {{ timeLabel(c) }}
                </span>
              </span>
            </Button>

            <div
              v-if="editingId !== c.id"
              class="absolute inset-y-0 right-1.5 flex items-center gap-0.5 opacity-100 transition-opacity group-focus-within:opacity-100 [@media(hover:hover)]:opacity-0 [@media(hover:hover)]:group-hover:opacity-100"
            >
              <Button
                type="button"
                text
                rounded
                severity="secondary"
                size="small"
                class="text-surface-400 hover:text-surface-700 dark:hover:text-surface-100"
                aria-label="Rename"
                @click.stop="startRename(c)"
              >
                <AppIcon
                  :icon="{ type: 'lucide', value: 'pencil' }"
                  :size="13"
                />
              </Button>
              <Button
                type="button"
                text
                rounded
                severity="danger"
                size="small"
                class="text-surface-400 hover:text-red-500"
                aria-label="Delete"
                @click.stop="remove(c)"
              >
                <AppIcon
                  :icon="{ type: 'lucide', value: 'trash' }"
                  :size="13"
                />
              </Button>
            </div>

            <form
              v-if="editingId === c.id"
              class="absolute inset-x-1.5 top-1/2 z-10 flex -translate-y-1/2 items-center gap-1"
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
                <AppIcon
                  :icon="{ type: 'lucide', value: 'check' }"
                  :size="13"
                />
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
                <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="13" />
              </Button>
            </form>
          </li>
        </ul>
      </template>

      <div
        v-if="conversations.length === 0"
        class="mt-6 grid gap-2 px-3 text-center text-xs text-surface-400"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'message-circle' }"
          :size="22"
          class="mx-auto text-surface-300 dark:text-surface-600"
        />
        <span>No conversations yet.</span>
        <span class="text-surface-400 dark:text-surface-500">
          Start a new chat to see it here.
        </span>
      </div>
    </div>
  </section>
</template>
