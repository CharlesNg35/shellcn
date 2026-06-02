<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import Button from "primevue/button";
import AiMessageList from "./AiMessageList.vue";
import AiComposer from "./AiComposer.vue";
import AiConversationList from "./AiConversationList.vue";
import AiActionConfirm from "./AiActionConfirm.vue";
import AiModelSwitcher from "./AiModelSwitcher.vue";
import AiQueuedMessages from "./AiQueuedMessages.vue";
import AppAlert from "../../components/AppAlert.vue";
import AppIcon from "../../components/AppIcon.vue";
import { useAiChatStore } from "../../stores/aiChat";

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
const composerDisabled = computed(
  () => !st.value.connected || !providerReady.value,
);
const disabledReason = computed(() => {
  if (!store.providersReady) return "Loading AI settings...";
  if (!providerReady.value) return "No AI provider configured";
  if (!st.value.connected) return "Connecting assistant...";
  return "";
});
const statusLabel = computed(() => {
  if (st.value.runState === "stopping") return "stopping...";
  if (st.value.runState !== "idle") return "streaming...";
  if (!st.value.connected) return "connecting...";
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
function retryConnection(): void {
  void store.connect(props.connectionId);
}

onMounted(() => {
  void store.connect(props.connectionId);
  void store.loadProviders();
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
        <AppIcon :icon="{ type: 'lucide', value: 'panel-left' }" :size="15" />
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
        class="max-w-[8.5rem] shrink min-[420px]:max-w-[11rem]"
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
      <AiConversationList
        v-if="showHistory"
        class="absolute inset-y-0 left-0 z-20"
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
      <Button
        v-if="showHistory"
        type="button"
        text
        severity="secondary"
        class="absolute inset-0 z-10 h-full w-full rounded-none border-0 bg-surface-950/10 p-0 backdrop-blur-[1px] hover:bg-surface-950/10 dark:bg-surface-950/30 dark:hover:bg-surface-950/30"
        aria-label="Close conversation history"
        @click="showHistory = false"
      />

      <div class="flex min-h-0 flex-1 flex-col">
        <div v-if="st.error" class="px-3 pt-3">
          <AppAlert tone="danger" title="Assistant error">
            <div class="flex min-w-0 items-center gap-2">
              <span class="min-w-0 flex-1">{{ st.error }}</span>
              <Button
                v-if="!st.connected"
                type="button"
                size="small"
                severity="secondary"
                outlined
                @click="retryConnection"
              >
                Retry
              </Button>
            </div>
          </AppAlert>
        </div>
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
        <div v-if="st.pendingConfirm" class="px-3 pt-2">
          <AiActionConfirm
            :pending="st.pendingConfirm"
            @approve="store.resolveConfirm(connectionId, true)"
            @reject="store.resolveConfirm(connectionId, false)"
          />
        </div>
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
