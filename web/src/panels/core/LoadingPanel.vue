<script setup lang="ts">
import SkeletonList from "../../components/SkeletonList.vue";
</script>

<template>
  <div class="h-full min-h-0">
    <SkeletonList :rows="8" />
  </div>
</template>
