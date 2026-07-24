<script setup lang="ts">
import AppIcon from "./AppIcon.vue";
</script>

<template>
  <section id="concept" class="mx-auto max-w-[1120px] px-6 py-24">
    <div v-reveal class="mx-auto mb-12 max-w-[680px] text-center">
      <div class="mb-3.5 text-[13px] font-bold tracking-wider text-accent uppercase">The model</div>
      <h2 class="text-[clamp(28px,4vw,40px)] font-extrabold">Plugins declare. The core owns.</h2>
      <p class="mt-4 text-[17px] text-muted">
        A plugin ships one versioned <b class="text-ink">manifest</b> (config schema, views,
        actions, streams, routes) and a set of typed route handlers. The core handles auth,
        sessions, transport, audit, and rendering, then sends the browser a projection of the
        manifest. The frontend renders that projection, so
        <b class="text-ink">adding a protocol takes no frontend changes.</b>
      </p>
    </div>

    <div v-reveal class="flex flex-wrap items-stretch justify-center gap-3">
      <div class="flex min-w-[220px] flex-1 flex-col gap-2 rounded-2xl border border-line bg-panel p-6">
        <div class="mb-1.5 grid h-[42px] w-[42px] place-items-center rounded-xl bg-accent/10 text-[22px] text-accent">
          <AppIcon name="globe" />
        </div>
        <b class="text-base">Browser</b>
        <span class="text-sm text-muted">Universal renderer with ~20 panel types: terminals, tables, trees, desktops.</span>
      </div>

      <div class="grid place-items-center text-[22px] text-dim max-md:rotate-90">
        <AppIcon name="arrow" />
      </div>

      <div
        class="flex min-w-[220px] flex-1 flex-col gap-2 rounded-2xl border border-accent/35 bg-panel p-6 shadow-[0_0_0_1px_rgba(56,189,248,0.1)]"
      >
        <div class="mb-1.5 grid h-[42px] w-[42px] place-items-center rounded-xl bg-accent text-[22px] text-[#04121f]">
          <AppIcon name="shield" />
        </div>
        <b class="text-base">Core gateway</b>
        <span class="text-sm text-muted">AuthN/Z · RBAC · secrets · sessions · transport · audit · projection.</span>
      </div>

      <div class="grid place-items-center text-[22px] text-dim max-md:rotate-90">
        <AppIcon name="arrow" />
      </div>

      <div class="flex min-w-[220px] flex-1 flex-col gap-2 rounded-2xl border border-line bg-panel p-6">
        <div class="mb-1.5 grid h-[42px] w-[42px] place-items-center rounded-xl bg-accent/10 text-[22px] text-accent">
          <AppIcon name="plug" />
        </div>
        <b class="text-base">Plugins → targets</b>
        <span class="text-sm text-muted">Manifest + route handlers. Direct or reverse-agent transport.</span>
      </div>
    </div>
  </section>
</template>
