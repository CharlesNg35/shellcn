<script setup lang="ts">
import { computed, onBeforeUnmount } from "vue";
import Button from "primevue/button";
import { useDockStore } from "@/stores/dock";
import { KEEP_ALIVE_DOCK_PANELS_MAX } from "@/stores/sessionLimits";
import PanelHost from "../core/PanelHost.vue";
import AppIcon from "@/components/AppIcon.vue";

const props = defineProps<{ connectionId: string }>();
const dock = useDockStore();
const state = computed(() => dock.state(props.connectionId));
const active = computed(() =>
  state.value.items.find((i) => i.id === state.value.activeId),
);

let startY = 0;
let startHeight = 0;
let capture: { el: HTMLElement; pointerId: number } | null = null;

function onResizeMove(e: PointerEvent): void {
  dock.setHeight(props.connectionId, startHeight + (startY - e.clientY));
}
function onResizeEnd(): void {
  if (capture?.el.hasPointerCapture?.(capture.pointerId)) {
    capture.el.releasePointerCapture(capture.pointerId);
  }
  capture = null;
  window.removeEventListener("pointermove", onResizeMove);
  window.removeEventListener("pointerup", onResizeEnd);
}
function onResizeStart(e: PointerEvent): void {
  startY = e.clientY;
  startHeight = state.value.height;
  const el = e.currentTarget as HTMLElement | null;
  if (el?.setPointerCapture) {
    el.setPointerCapture(e.pointerId);
    capture = { el, pointerId: e.pointerId };
  }
  window.addEventListener("pointermove", onResizeMove);
  window.addEventListener("pointerup", onResizeEnd);
}

onBeforeUnmount(onResizeEnd);
</script>

<template>
  <div
    class="flex max-h-[70%] shrink-0 flex-col border-t border-surface-200 dark:border-surface-800"
    :style="state.collapsed ? undefined : { height: state.height + 'px' }"
  >
    <div
      v-if="!state.collapsed"
      class="-my-1 box-content h-1 shrink-0 cursor-row-resize bg-clip-content py-1 transition-colors hover:bg-primary-500/40 focus-visible:bg-primary-500/50 focus-visible:outline-none"
      role="separator"
      aria-orientation="horizontal"
      aria-label="Resize dock"
      tabindex="0"
      :aria-valuenow="state.height"
      aria-valuemin="120"
      aria-valuemax="800"
      @pointerdown="onResizeStart"
      @keydown.up.prevent="dock.setHeight(connectionId, state.height + 16)"
      @keydown.down.prevent="dock.setHeight(connectionId, state.height - 16)"
    />
    <div
      class="flex shrink-0 items-center gap-1 bg-surface-50 px-2 py-1 dark:bg-surface-900"
    >
      <div class="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto">
        <div
          v-for="item in state.items"
          :key="item.id"
          class="group flex shrink-0 items-center overflow-hidden rounded text-xs transition-colors"
          :class="
            item.id === state.activeId
              ? 'bg-surface-0 text-surface-900 shadow-sm dark:bg-surface-800 dark:text-surface-0'
              : 'text-surface-500 hover:text-surface-800 dark:hover:text-surface-200'
          "
        >
          <button
            type="button"
            :title="item.title"
            class="flex max-w-48 min-w-0 flex-1 items-center gap-1.5 overflow-hidden px-2 py-1 text-left focus-visible:ring-2 focus-visible:ring-primary-500/35 focus-visible:outline-none focus-visible:ring-inset"
            @click="dock.activate(connectionId, item.id)"
          >
            <AppIcon v-if="item.icon" :icon="item.icon" :size="13" />
            <span class="min-w-0 flex-1 truncate">{{ item.title }}</span>
          </button>
          <Button
            type="button"
            text
            rounded
            severity="secondary"
            size="small"
            :aria-label="`Close ${item.title}`"
            :pt="{ root: 'h-4 w-4 p-0 opacity-60 hover:opacity-100' }"
            @click.stop="dock.close(connectionId, item.id)"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="12" />
          </Button>
        </div>
      </div>
      <Button
        type="button"
        text
        rounded
        severity="secondary"
        size="small"
        class="shrink-0"
        :aria-label="state.collapsed ? 'Expand dock' : 'Collapse dock'"
        @click="dock.toggleCollapsed(connectionId)"
      >
        <AppIcon
          :icon="{
            type: 'lucide',
            value: state.collapsed ? 'chevron-up' : 'chevron-down',
          }"
          :size="14"
        />
      </Button>
    </div>
    <div v-show="!state.collapsed" class="min-h-0 flex-1 overflow-hidden">
      <KeepAlive :max="KEEP_ALIVE_DOCK_PANELS_MAX">
        <PanelHost
          v-if="active"
          :key="active.id"
          :panel="active.panel"
          :connection-id="connectionId"
          :source="active.source"
          :config="active.config"
          :resource="active.resource"
          :record="active.record"
        />
      </KeepAlive>
    </div>
  </div>
</template>
