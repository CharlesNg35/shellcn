<script setup lang="ts">
import { computed } from "vue";
import { useAuthStore } from "../stores/auth";

const props = defineProps<{ admin?: boolean }>();
const auth = useAuthStore();
const allowed = computed(() => (props.admin ? auth.isAdmin : true));
</script>

<template>
  <slot v-if="allowed" />
  <slot v-else name="denied" />
</template>
