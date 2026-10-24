<template>
  <div
    class="action-bar"
    :class="[`align-${align}`, `mobile-align-${mobileAlign}`, { 'is-full': full }]"
    :role="label ? 'group' : undefined"
    :aria-label="label"
  >
    <slot />
  </div>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    align?: 'start' | 'center' | 'end'
    mobileAlign?: 'start' | 'center' | 'end'
    label?: string
    full?: boolean
  }>(),
  {
    align: 'end',
    mobileAlign: 'start',
    label: undefined,
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

@media (max-width: 768px) {
  .mobile-align-start {
    justify-content: flex-start;
  }

  .mobile-align-center {
    justify-content: center;
  }

  .mobile-align-end {
    justify-content: flex-end;
  }
}
</style>
