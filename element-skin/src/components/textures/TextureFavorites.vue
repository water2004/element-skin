<template>
  <span
    class="inline-flex shrink-0 items-center gap-1 text-sm"
    :aria-label="`收藏数 ${count || 0}`"
  >
    <span aria-hidden="true">♡</span><span class="tabular-nums">{{ count || 0 }}</span>
  </span>
</template>
<script setup lang="ts">
defineProps<{ count?: number }>()
</script>
