<template>
  <header class="ui-page-header">
    <div class="ui-page-header-content">
      <div v-if="$slots.icon" class="ui-page-header-icon" aria-hidden="true">
        <slot name="icon" />
      </div>
      <div class="ui-page-header-text">
        <component :is="'h' + level" class="ui-page-header-title">{{ title }}</component>
        <p v-if="subtitle" class="ui-page-header-subtitle">{{ subtitle }}</p>
      </div>
    </div>
    <ActionBar v-if="$slots.actions" class="ui-page-header-actions" :label="actionsLabel">
      <slot name="actions" />
    </ActionBar>
  </header>
</template>

<script setup lang="ts">
import ActionBar from './ActionBar.vue'

withDefaults(
  defineProps<{
    title: string
    subtitle?: string
    level?: 1 | 2 | 3
    actionsLabel?: string
  }>(),
  { subtitle: undefined, level: 1, actionsLabel: undefined },
)
</script>

<style scoped>
.ui-page-header {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 30px;
  gap: 20px;
  width: 100%;
}

.ui-page-header-content {
  display: flex;
  align-items: flex-start;
  gap: var(--space-content-padding);
  flex: 1 1 240px;
  min-width: 0;
}

.ui-page-header-icon {
  width: 56px;
  height: 56px;
  font-size: 26px;
  color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
  border-radius: var(--radius-card);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.ui-page-header-text {
  min-width: 0;
  overflow-wrap: anywhere;
}

.ui-page-header-title {
  margin: 0;
  font-size: var(--font-size-title);
  font-weight: 700;
  line-height: 1.4;
  color: var(--color-heading);
}

.ui-page-header-subtitle {
  margin: 4px 0 0;
  color: var(--color-text-light);
  font-size: var(--font-size-description);
  line-height: 1.6;
}

.ui-page-header-actions {
  flex: 0 1 auto;
  max-width: 100%;
}

@media (max-width: 768px) {
  .ui-page-header {
    flex-direction: column;
    gap: var(--space-content-padding);
  }

  .ui-page-header-content {
    flex-basis: auto;
    width: 100%;
  }

  .ui-page-header-actions {
    width: 100%;
  }
}
</style>
