<script setup lang="ts">
import AppIcon from "./AppIcon.vue";
import { FEATURES } from "../data";
</script>

<template>
  <section id="features" class="mx-auto max-w-[1120px] px-6 py-24">
    <div v-reveal class="mx-auto mb-12 max-w-[680px] text-center">
      <div class="mb-3.5 text-[13px] font-bold tracking-wider text-accent uppercase">
        Secure & auditable from day one
      </div>
      <h2 class="text-[clamp(28px,4vw,40px)] font-extrabold">Built for real infrastructure.</h2>
    </div>

    <div class="grid gap-[18px] sm:grid-cols-2 lg:grid-cols-3">
      <article
        v-for="f in FEATURES"
        :key="f.title"
        v-reveal
        class="group relative overflow-hidden rounded-2xl border border-line bg-panel p-6 transition hover:-translate-y-1 hover:border-accent/40"
      >
        <span
          class="absolute inset-x-0 top-0 h-0.5 bg-accent opacity-0 transition-opacity group-hover:opacity-100"
        ></span>
        <div class="mb-4 grid h-[46px] w-[46px] place-items-center rounded-xl bg-accent/10 text-[23px] text-accent">
          <AppIcon :name="(f.icon as any)" />
        </div>
        <h3 class="mb-2 text-lg font-bold">{{ f.title }}</h3>
        <p class="text-[14.5px] text-muted">{{ f.body }}</p>
      </article>
    </div>
  </section>
</template>
