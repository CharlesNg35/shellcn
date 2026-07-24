<script setup lang="ts">
import { computed, ref } from "vue";
import AppIcon from "./AppIcon.vue";
import { TABS } from "../data";

const active = ref(TABS[0].id);
const copied = ref(false);
const activeCode = computed(() => TABS.find((t) => t.id === active.value)?.code ?? "");

async function copy() {
  try {
    await navigator.clipboard.writeText(activeCode.value);
  } catch {
    const ta = document.createElement("textarea");
    ta.value = activeCode.value;
    document.body.appendChild(ta);
    ta.select();
    document.execCommand("copy");
    ta.remove();
  }
  copied.value = true;
  setTimeout(() => (copied.value = false), 1600);
}
</script>

<template>
  <section id="start" class="mx-auto max-w-[1120px] px-6 py-24">
    <div v-reveal class="mx-auto mb-12 max-w-[680px] text-center">
      <div class="mb-3.5 text-[13px] font-bold tracking-wider text-accent uppercase">Run it</div>
      <h2 class="text-[clamp(28px,4vw,40px)] font-extrabold">Up in one command.</h2>
      <p class="mt-4 text-[17px] text-muted">
        Each needs a master key (it encrypts stored credentials) and a first admin login. Then open
        <code class="rounded bg-white/[0.06] px-1.5 py-0.5 font-mono text-[0.92em] text-ink">http://localhost:8081</code>
        and sign in.
      </p>
    </div>

    <div v-reveal class="mx-auto max-w-[820px]">
      <div class="flex gap-1.5">
        <button
          v-for="t in TABS"
          :key="t.id"
          type="button"
          class="cursor-pointer rounded-t-xl border border-b-0 border-line px-[18px] py-2.5 text-sm font-semibold"
          :class="active === t.id ? 'bg-panel text-ink' : 'bg-transparent text-muted'"
          @click="active = t.id"
        >
          {{ t.label }}
        </button>
      </div>
      <div
        class="relative rounded-b-2xl rounded-tr-2xl border border-line bg-[linear-gradient(180deg,#0c1626,#0a1220)] px-5 py-5 shadow-[0_24px_60px_rgba(0,0,0,0.45)]"
      >
        <button
          type="button"
          class="absolute top-3.5 right-3.5 inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs font-semibold transition"
          :class="copied ? 'border-ok text-ok' : 'border-line bg-panel-2 text-muted hover:border-accent hover:text-ink'"
          aria-label="Copy command"
          @click="copy"
        >
          <AppIcon :name="copied ? 'check' : 'copy'" />
          <span>{{ copied ? "Copied" : "Copy" }}</span>
        </button>
        <pre class="m-0 overflow-x-auto font-mono text-[13.5px] leading-[1.75] text-[#d7e6fb]"><code>{{ activeCode }}</code></pre>
      </div>
      <p class="mt-4.5 text-center text-sm text-muted">
        Reuse the same master key on restart, or stored credentials can't be decrypted. Data and
        recordings live in <code class="rounded bg-white/[0.06] px-1.5 py-0.5 font-mono text-[0.92em] text-ink">/data</code>, so mount a volume there.
      </p>
    </div>
  </section>
</template>
