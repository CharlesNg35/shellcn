<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import Button from "primevue/button";
import AiMessageList from "./AiMessageList.vue";
import AiComposer from "./AiComposer.vue";
import AiConversationList from "./AiConversationList.vue";
import AppIcon from "../../components/AppIcon.vue";
import { useAiChatStore } from "../../stores/aiChat";

// This component (and everything it imports — the chat store, markdown stack,
// highlight.js) rides the lazy AI chunk. It is never in the main bundle.
const props = defineProps<{ connectionId: string }>();

const store = useAiChatStore();
const st = computed(() => store.state(props.connectionId));
const busy = computed(() => st.value.runState !== "idle");
const showHistory = ref(false);

function send(text: string): void {
  store.send(props.connectionId, text);
}
function stop(): void {
  store.stop(props.connectionId);
}
function selectConversation(id: string): void {
  void store.selectConversation(props.connectionId, id);
  showHistory.value = false;
}
function newChat(): void {
  store.newChat(props.connectionId);
  showHistory.value = false;
}

onMounted(() => {
  void store.connect(props.connectionId);
});
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div
      class="flex items-center gap-2 border-b border-surface-200 px-4 py-2.5 dark:border-surface-800"
    >
      <Button
        text
        rounded
        severity="secondary"
        size="small"
        aria-label="Conversation history"
        :class="showHistory ? 'text-primary-500' : ''"
        @click="showHistory = !showHistory"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'panel-left' }" :size="15" />
      </Button>
      <AppIcon
        :icon="{ type: 'lucide', value: 'sparkles' }"
        :size="16"
        class="text-primary-500"
      />
      <span
        class="flex-1 truncate text-sm font-semibold text-surface-800 dark:text-surface-100"
      >
        Assistant
      </span>
      <span
        v-if="!st.connected"
        class="text-xs text-surface-400"
        aria-live="polite"
      >
        connecting…
      </span>
      <Button
        text
        rounded
        severity="secondary"
        size="small"
        aria-label="New chat"
        :disabled="busy"
        @click="newChat"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'plus' }" :size="15" />
      </Button>
    </div>

    <div class="flex min-h-0 flex-1">
      <AiConversationList
        v-if="showHistory"
        :conversations="st.conversations"
        :active-id="st.activeId"
        :streaming-id="busy ? st.activeId : null"
        :busy="busy"
        @select="selectConversation"
        @create="newChat"
        @rename="
          (id, title) => store.renameConversation(connectionId, id, title)
        "
        @remove="(id) => store.deleteConversation(connectionId, id)"
      />

      <div class="flex min-h-0 flex-1 flex-col">
        <AiMessageList
          :messages="st.messages"
          :current-id="st.current?.id ?? null"
          :streaming="busy"
          @quick-start="send"
        />
        <AiComposer
          :busy="busy"
          :disabled="!st.connected"
          @send="send"
          @stop="stop"
        />
      </div>
    </div>
  </div>
</template>
