<script setup lang="ts">
import AppIcon from "./AppIcon.vue";
import { REPO } from "../data";
</script>

<template>
  <header
    class="sticky top-0 z-40 mx-auto flex max-w-[1120px] items-center justify-between gap-4 px-6 py-4 backdrop-blur-md"
  >
    <div
      class="pointer-events-none absolute inset-0 -z-10 bg-gradient-to-b from-bg/85 to-transparent"
    ></div>
    <a href="#top" class="flex items-center gap-2.5 text-lg font-bold">
      <img src="/logo.svg" width="30" height="30" alt="" />
      <span>Shell<span class="text-accent">CN</span></span>
    </a>
    <nav class="flex items-center gap-6 text-sm text-muted">
      <a href="#concept" class="hidden transition-colors hover:text-ink sm:inline">Concept</a>
      <a href="#protocols" class="hidden transition-colors hover:text-ink sm:inline">Protocols</a>
      <a href="#features" class="hidden transition-colors hover:text-ink sm:inline">Features</a>
      <a href="#start" class="hidden transition-colors hover:text-ink sm:inline">Get started</a>
      <a
        :href="REPO"
        target="_blank"
        rel="noreferrer"
        class="inline-flex items-center gap-2 rounded-lg border border-line px-3.5 py-1.5 text-ink transition-colors hover:border-accent"
      >
        <AppIcon name="github" /><span>GitHub</span>
      </a>
    </nav>
  </header>
</template>
