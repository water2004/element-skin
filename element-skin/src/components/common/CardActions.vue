<template>
  <div class="card-actions" @click.stop>
    <slot />
  </div>
</template>

<style scoped>
.card-actions {
  display: flex;
  flex-direction: row;
  gap: var(--space-control-gap);
  padding: var(--space-action-gap) var(--space-content-padding);
  border-top: 1px solid var(--color-border);
  background: var(--color-background-soft);
  align-items: center;
}

.card-actions :deep(.el-button) {
  flex: 1;
  min-width: 0;
  margin-left: 0 !important;
}

@media (max-width: 520px) {
  .card-actions {
    flex-wrap: wrap;
  }

  .card-actions :deep(.el-button) {
    flex: 1 1 92px;
  }
}
</style>
