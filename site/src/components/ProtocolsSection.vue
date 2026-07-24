<script setup lang="ts">
import { PROTOCOLS, CAT_COLOR } from "../data";
</script>

<template>
  <section id="protocols" class="mx-auto max-w-[1120px] px-6 py-24">
    <div v-reveal class="mx-auto mb-12 max-w-[680px] text-center">
      <div class="mb-3.5 text-[13px] font-bold tracking-wider text-accent uppercase">
        One connection, many capabilities
      </div>
      <h2 class="text-[clamp(28px,4vw,40px)] font-extrabold">Reach it all from one place.</h2>
      <p class="mt-4 text-[17px] text-muted">
        Twenty protocols ship in the box. Add more as out-of-tree gRPC plugins or from the marketplace.
      </p>
    </div>

    <div v-reveal class="grid grid-cols-[repeat(auto-fill,minmax(150px,1fr))] gap-3">
      <div
        v-for="[name, cat] in PROTOCOLS"
        :key="name"
        class="flex items-center gap-3 rounded-xl border border-line bg-panel px-4 py-3.5 transition hover:-translate-y-0.5 hover:border-accent hover:bg-panel-2"
      >
        <span class="h-2.5 w-2.5 flex-none rounded-full" :style="{ background: CAT_COLOR[cat] }"></span>
        <span>
          <b class="text-[14.5px] font-semibold">{{ name }}</b>
          <small class="block font-mono text-[11.5px] text-dim">{{ cat }}</small>
        </span>
      </div>
      <div
        class="flex items-center justify-center rounded-xl border border-dashed border-line bg-panel px-4 py-3.5 font-semibold text-accent"
      >
        + external plugins
      </div>
    </div>
  </section>
</template>
