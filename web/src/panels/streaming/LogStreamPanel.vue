<script setup lang="ts">
import { computed, nextTick, ref } from "vue";
import Button from "primevue/button";
import InputText from "primevue/inputtext";
import { useStream } from "../../composables/useStream";
import type { PanelProps } from "../types";
import StubBanner from "./StubBanner.vue";

const props = defineProps<PanelProps>();

const MAX = 1000;
const lines = ref<string[]>([]);
const paused = ref(false);
const follow = ref(true);
const filterText = ref("");
const viewport = ref<HTMLElement | null>(null);

function append(frame: string): void {
  let text = frame;
  try {
    const parsed = JSON.parse(frame) as { ts?: string; line?: string };
    if (parsed.line) text = `${parsed.ts ? `${parsed.ts} ` : ""}${parsed.line}`;
  } catch {
    /* plain text frame */
  }
  if (paused.value) return;
  lines.value.push(text);
  if (lines.value.length > MAX) lines.value.splice(0, lines.value.length - MAX);
  void nextTick(() => {
    if (viewport.value && follow.value)
      viewport.value.scrollTop = viewport.value.scrollHeight;
  });
}

const { status } = useStream(
  props.connectionId,
  props.source,
  { resource: props.resource },
  append,
);

const visibleLines = computed(() => {
  const q = filterText.value.trim().toLowerCase();
  if (!q) return lines.value;
  return lines.value.filter((line) => line.toLowerCase().includes(q));
});

const downloadHref = computed(
  () =>
    `data:text/plain;charset=utf-8,${encodeURIComponent(lines.value.join("\n"))}`,
);
</script>

<template>
  <div class="flex h-full flex-col bg-[#0b0f17]">
    <StubBanner :status="status" />
    <div
      class="flex flex-wrap items-center gap-2 border-b border-surface-800 bg-surface-950 px-3 py-2"
    >
      <InputText
        v-model="filterText"
        placeholder="Filter logs"
        class="max-w-64"
      />
      <Button
        type="button"
        severity="secondary"
        :label="paused ? 'Resume' : 'Pause'"
        @click="paused = !paused"
      />
      <Button
        type="button"
        severity="secondary"
        :label="follow ? 'Following' : 'Follow'"
        @click="follow = !follow"
      />
      <Button
        type="button"
        severity="secondary"
        label="Clear"
        @click="lines = []"
      />
      <Button
        as="a"
        severity="secondary"
        :href="downloadHref"
        download="logs.txt"
        label="Download"
      />
    </div>
    <div
      ref="viewport"
      class="min-h-0 flex-1 overflow-auto p-3 font-mono text-xs leading-relaxed text-surface-200"
    >
      <div
        v-for="(line, i) in visibleLines"
        :key="i"
        class="whitespace-pre-wrap"
      >
        {{ line }}
      </div>
      <div v-if="!visibleLines.length" class="text-surface-500">
        Waiting for log frames…
      </div>
    </div>
  </div>
</template>
