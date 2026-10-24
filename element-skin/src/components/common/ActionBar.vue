<template>
  <div class="action-bar" :class="[`align-${align}`, { 'is-full': full }]">
    <slot />
  </div>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    align?: 'start' | 'center' | 'end'
    full?: boolean
  }>(),
  {
    align: 'end',
    full: false,
  },
)
</script>

<style scoped>
.action-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--space-action-gap);
  min-width: 0;
}

.action-bar.is-full {
  flex: 1 1 100%;
  width: 100%;
}

.align-start {
  justify-content: flex-start;
}

.align-center {
  justify-content: center;
}

.align-end {
  justify-content: flex-end;
}

.action-bar :deep(.el-button) {
  margin-left: 0 !important;
}

@media (max-width: 900px) {
  .action-bar {
    justify-content: center;
  }
}
</style>
