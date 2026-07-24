<script setup lang="ts">
import AppIcon from "./AppIcon.vue";
import TerminalDemo from "./TerminalDemo.vue";
import { REPO } from "../data";
</script>

<template>
  <section
    class="mx-auto grid max-w-[1120px] items-center gap-12 px-6 pt-16 pb-10 md:grid-cols-[1.05fr_1fr]"
  >
    <div v-reveal>
      <div
        class="mb-6 inline-flex items-center gap-2 rounded-full border border-line bg-white/[0.02] px-3 py-1.5 text-xs font-semibold tracking-wide text-muted uppercase"
      >
        <span class="pulse h-[7px] w-[7px] rounded-full bg-ok shadow-[0_0_0_3px_rgba(52,211,153,0.2)]"></span>
        Self-hosted · single binary · GPL-3.0
      </div>
      <h1 class="text-[clamp(38px,5.4vw,60px)] font-extrabold">
        One gateway for <span class="text-accent">everything</span> you log into.
      </h1>
      <p class="mt-5 mb-7 max-w-[46ch] text-lg text-muted">
        SSH, containers, Kubernetes, databases and remote desktops — behind one
        secure, audited, browser-based control plane. Every protocol is a
        compiled-in Go plugin; the core owns rendering, sessions, and audit.
      </p>
      <div class="flex flex-wrap gap-3.5">
        <a
          href="#start"
          class="inline-flex items-center gap-2 rounded-xl bg-accent px-5 py-3 font-semibold text-[#04121f] transition hover:brightness-110"
        >
          Get started <AppIcon name="arrow" />
        </a>
        <a
          :href="REPO"
          target="_blank"
          rel="noreferrer"
          class="inline-flex items-center gap-2 rounded-xl border border-line bg-white/[0.03] px-5 py-3 font-semibold transition hover:border-accent"
        >
          <AppIcon name="github" /> Star on GitHub
        </a>
      </div>
      <div class="mt-10 flex flex-wrap gap-8">
        <div class="flex flex-col">
          <b class="font-mono text-2xl font-extrabold">20+</b>
          <span class="text-[13px] text-muted">built-in protocols</span>
        </div>
        <div class="flex flex-col">
          <b class="font-mono text-2xl font-extrabold">1</b>
          <span class="text-[13px] text-muted">Go binary, embedded UI</span>
        </div>
        <div class="flex flex-col">
          <b class="font-mono text-2xl font-extrabold">0</b>
          <span class="text-[13px] text-muted">frontend code per plugin</span>
        </div>
      </div>
    </div>

    <div v-reveal class="max-md:-order-1">
      <TerminalDemo />
    </div>
  </section>
</template>
