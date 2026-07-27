<script setup lang="ts">
import { computed, onMounted } from "vue";
import { useMarketStore } from "../stores/market";
import AppIcon from "./AppIcon.vue";

// PluginUpdateBadge shows a managed-plugin update for a connection's protocol and
// deep-links to the filtered marketplace. Admin gating is the caller's job (wrap
// in <RoleGate admin>): the marketplace fetch is admin-only, so a non-admin must
// never mount this.
const props = defineProps<{ protocol?: string }>();

const market = useMarketStore();
const update = computed(() => market.updateFor(props.protocol));

onMounted(() => void market.ensureLoaded());
</script>

<template>
  <RouterLink
    v-if="update"
    v-tooltip.bottom="
      `Plugin update available${update.latest ? ` — ${update.latest.version}` : ''}`
    "
    :to="{ name: 'protocols', query: { tab: 'market', q: protocol } }"
    class="inline-flex shrink-0 items-center gap-1 rounded-full border border-amber-300 bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 transition-colors hover:bg-amber-100 dark:border-amber-500/40 dark:bg-amber-950/30 dark:text-amber-300 dark:hover:bg-amber-950/50"
  >
    <AppIcon :icon="{ type: 'lucide', value: 'arrow-up-circle' }" :size="13" />
    Update
  </RouterLink>
</template>
