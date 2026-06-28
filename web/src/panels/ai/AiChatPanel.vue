<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import Button from "primevue/button";
import AiMessageList from "./AiMessageList.vue";
import AiComposer from "./AiComposer.vue";
import AiConversationList from "./AiConversationList.vue";
import AiActionConfirm from "./AiActionConfirm.vue";
import AiModelSwitcher from "./AiModelSwitcher.vue";
import AiQueuedMessages from "./AiQueuedMessages.vue";
import AppIcon from "@/components/AppIcon.vue";
import { useAiChatStore } from "@/stores/aiChat";

const props = defineProps<{ connectionId: string }>();
const emit = defineEmits<{ close: [] }>();

const store = useAiChatStore();
const st = computed(() => store.state(props.connectionId));
const busy = computed(() => st.value.runState !== "idle");
const providerReady = computed(
  () =>
    store.providersReady &&
    (store.providers.length > 0 || Boolean(store.global?.configured)),
);
const composerDisabled = computed(() => !providerReady.value);
const disabledReason = computed(() => {
  if (!store.providersReady) return "Loading AI settings...";
  if (!providerReady.value) return "No AI provider configured";
  return "";
});
const statusLabel = computed(() => {
  if (st.value.runState === "stopping") return "stopping...";
  if (st.value.runState !== "idle") return "streaming...";
  return "";
});
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
  void store.loadProviders();
  void store.loadConversations(props.connectionId);
});
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div
      class="flex min-w-0 items-center gap-2 border-b border-surface-200 px-3 py-2.5 dark:border-surface-800"
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
        <AppIcon :icon="{ type: 'lucide', value: 'history' }" :size="15" />
      </Button>
      <AppIcon
        :icon="{ type: 'lucide', value: 'sparkles' }"
        :size="16"
        class="hidden shrink-0 text-primary-500 min-[380px]:block"
      />
      <div class="grid min-w-0 flex-1 gap-0.5">
        <span
          class="truncate text-sm font-semibold text-surface-800 dark:text-surface-100"
        >
          Assistant
        </span>
        <span
          v-if="statusLabel"
          class="truncate text-xs text-surface-400"
          aria-live="polite"
        >
          {{ statusLabel }}
        </span>
      </div>
      <AiModelSwitcher
        v-if="store.providers.length || store.global?.configured"
        class="max-w-34 shrink min-[420px]:max-w-44"
        :providers="store.providers"
        :global="store.global"
        :provider-id="st.providerId"
        :disabled="busy"
        @select="(p) => store.setProvider(connectionId, p)"
      />
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
      <Button
        text
        rounded
        severity="secondary"
        size="small"
        aria-label="Close assistant"
        @click="emit('close')"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="15" />
      </Button>
    </div>

    <div class="relative flex min-h-0 flex-1 overflow-hidden">
      <Transition name="ai-history">
        <AiConversationList
          v-if="showHistory"
          class="absolute inset-0 z-20"
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
          @close="showHistory = false"
        />
      </Transition>

      <div class="flex min-h-0 min-w-0 flex-1 flex-col">
        <AiMessageList
          :messages="st.messages"
          :current-id="st.current?.id ?? null"
          :streaming="busy"
          :has-more="st.hasMore"
          :loading-older="st.loadingOlder"
          :disabled="composerDisabled"
          @quick-start="send"
          @load-older="store.loadOlder(connectionId)"
        />
        <AiActionConfirm
          v-if="st.pendingConfirm"
          :pending="st.pendingConfirm"
          @approve="
            (remember) => store.resolveConfirm(connectionId, true, { remember })
          "
          @reject="store.resolveConfirm(connectionId, false)"
        />
        <AiQueuedMessages
          :messages="st.queue"
          @remove="(index) => store.dequeue(connectionId, index)"
        />
        <AiComposer
          :run-state="st.runState"
          :disabled="composerDisabled"
          :disabled-reason="disabledReason"
          @send="send"
          @stop="stop"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.ai-history-enter-active,
.ai-history-leave-active {
  transition:
    opacity 0.18s ease,
    transform 0.18s ease;
}

.ai-history-enter-from,
.ai-history-leave-to {
  opacity: 0;
  transform: translateX(-1.25rem);
}

@media (prefers-reduced-motion: reduce) {
  .ai-history-enter-active,
  .ai-history-leave-active {
    transition: none;
  }

  .ai-history-enter-from,
  .ai-history-leave-to {
    transform: none;
  }
}
</style>
