<script setup lang="ts">
import AppIcon from "./AppIcon.vue";
import { REPO } from "../data";
</script>

<template>
  <section v-reveal class="mx-auto mt-10 max-w-[1120px] px-6 py-16 text-center">
    <h2 class="mb-7 text-[clamp(26px,4vw,38px)] font-extrabold">
      Run it, open a browser, and you're in.
    </h2>
    <div class="flex flex-wrap justify-center gap-3.5">
      <a
        :href="REPO"
        target="_blank"
        rel="noreferrer"
        class="inline-flex items-center gap-2 rounded-xl bg-accent px-5 py-3 font-semibold text-[#04121f] transition hover:brightness-110"
      >
        <AppIcon name="github" /> Get ShellCN
      </a>
      <a
        :href="`${REPO}#readme`"
        target="_blank"
        rel="noreferrer"
        class="inline-flex items-center gap-2 rounded-xl border border-line bg-white/[0.03] px-5 py-3 font-semibold transition hover:border-accent"
      >
        Read the docs <AppIcon name="arrow" />
      </a>
    </div>
  </section>

  <footer class="mx-auto max-w-[1120px] border-t border-line px-6 py-10 text-center text-muted">
    <div class="mb-3 inline-flex items-center gap-2.5 font-bold">
      <img src="/logo.svg" width="24" height="24" alt="" />
      <span>Shell<span class="text-accent">CN</span></span>
    </div>
    <p class="mx-auto mb-4.5 max-w-[52ch] text-[14.5px]">
      An open-source infrastructure access gateway. It's still early, so take it for a spin and tell
      us what breaks.
    </p>
    <div class="flex items-center justify-center gap-5.5 text-sm">
      <a :href="REPO" target="_blank" rel="noreferrer" class="hover:text-ink">GitHub</a>
      <a :href="`${REPO}#readme`" target="_blank" rel="noreferrer" class="hover:text-ink">Docs</a>
      <a :href="`${REPO}/releases`" target="_blank" rel="noreferrer" class="hover:text-ink">Releases</a>
      <span class="rounded-md border border-line px-2 py-[3px] font-mono text-xs text-dim">GPL-3.0</span>
    </div>
  </footer>
</template>
