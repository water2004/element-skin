<template>
  <div class="page-header">
    <div class="page-header-content">
      <div v-if="$slots.icon" class="page-header-icon"><slot name="icon" /></div>
      <div class="page-header-text">
        <h2>{{ title }}</h2>
        <p v-if="subtitle" class="subtitle">{{ subtitle }}</p>
      </div>
    </div>
    <div v-if="$slots.actions" class="page-header-actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  title: string
  subtitle?: string
}>()
</script>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 30px;
  gap: 20px;
  width: 100%;
}

.page-header-content {
  display: flex;
  align-items: center;
  gap: 18px;
  flex: 1;
  min-width: 0;
}

.page-header-icon {
  width: 56px;
  height: 56px;
  font-size: 26px;
  color: var(--el-color-primary);
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
  border-radius: var(--radius-card);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition:
    background-color var(--motion-duration-base) var(--motion-easing-standard),
    color var(--motion-duration-base) var(--motion-easing-standard);
}

.page-header-text {
  flex: 1;
  min-width: 0;
}

.page-header-text h2 {
  margin: 0;
  font-size: var(--font-size-title);
  font-weight: 700;
  color: var(--color-heading);
  letter-spacing: -0.5px;
}

.page-header-text .subtitle {
  margin: 4px 0 0 0;
  color: var(--color-text-light);
  font-size: var(--font-size-description);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.page-header-actions {
  display: flex;
  align-items: center;
  gap: var(--space-action-gap);
  flex-shrink: 0;
  flex-wrap: wrap;
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
  }

  .page-header-actions {
    width: 100%;
    justify-content: flex-start;
  }

  .page-header-content {
    width: 100%;
  }
}
</style>
