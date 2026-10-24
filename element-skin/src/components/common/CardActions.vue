<template>
  <div class="card-actions" :role="label ? 'group' : undefined" :aria-label="label" @click.stop>
    <slot />
  </div>
</template>

<script setup lang="ts">
defineProps<{ label?: string }>()
</script>

<style scoped>
.card-actions {
  display: flex;
  flex-direction: row;
  flex-wrap: wrap;
  gap: var(--space-control-gap);
  padding: var(--space-action-gap) var(--space-content-padding);
  border-top: 1px solid var(--color-border);
  background: var(--color-background-soft);
  align-items: center;
}

.card-actions :deep(.el-button) {
  flex: 1 1 92px;
  min-width: 0;
  margin-left: 0 !important;
}
</style>
