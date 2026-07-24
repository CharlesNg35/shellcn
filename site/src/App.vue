<script setup lang="ts">
import TheNav from "./components/TheNav.vue";
import HeroSection from "./components/HeroSection.vue";
import ConceptSection from "./components/ConceptSection.vue";
import ProtocolsSection from "./components/ProtocolsSection.vue";
import FeaturesSection from "./components/FeaturesSection.vue";
import StartSection from "./components/StartSection.vue";
import TheFooter from "./components/TheFooter.vue";
</script>

<template>
  <div class="bg-grid pointer-events-none fixed inset-0 -z-20" aria-hidden="true"></div>
  <div
    class="pointer-events-none fixed top-[-300px] left-1/2 -z-10 h-[620px] w-[820px] -translate-x-1/2 rounded-full blur-3xl"
    style="background: radial-gradient(closest-side, rgba(37, 99, 235, 0.12), transparent 70%)"
    aria-hidden="true"
  ></div>

  <TheNav />
  <main id="top">
    <HeroSection />
    <ConceptSection />
    <ProtocolsSection />
    <FeaturesSection />
    <StartSection />
  </main>
  <TheFooter />
</template>
