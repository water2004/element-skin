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
  background: rgba(64, 158, 255, 0.12);
  border-radius: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: var(--transition-base);
}

.page-header-text {
  flex: 1;
  min-width: 0;
}

.page-header-text h2 {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  color: var(--color-heading);
  letter-spacing: -0.5px;
}

.page-header-text .subtitle {
  margin: 4px 0 0 0;
  color: var(--color-text-light);
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.page-header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
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
}
</style>
