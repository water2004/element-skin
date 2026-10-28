<template>
  <header class="ui-page-header">
    <div class="ui-page-header-content">
      <div v-if="$slots.leading" class="shrink-0"><slot name="leading" /></div>
      <div v-if="$slots.icon" class="ui-page-header-icon" aria-hidden="true">
        <slot name="icon" />
      </div>
      <div class="ui-page-header-text">
        <component :is="'h' + level" class="ui-page-header-title">{{ title }}</component>
        <p v-if="subtitle" class="ui-page-header-subtitle">{{ subtitle }}</p>
        <div v-if="$slots.meta" class="mt-2 flex min-w-0 flex-wrap items-center gap-2">
          <slot name="meta" />
        </div>
      </div>
    </div>
    <ActionBar
      v-if="$slots.actions"
      class="ui-page-header-actions"
      :class="{ 'is-full': actionsFull }"
      :align="actionsFull ? 'start' : 'end'"
      :label="actionsLabel"
    >
      <slot name="actions" />
    </ActionBar>
    <div v-if="$slots.toolbar" class="ui-page-header-toolbar"><slot name="toolbar" /></div>
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
    actionsFull?: boolean
  }>(),
  { subtitle: undefined, level: 1, actionsLabel: undefined, actionsFull: false },
)
</script>

<style scoped>
.ui-page-header {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: var(--space-panel-padding);
  gap: var(--space-content-padding);
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
  font-weight: 400;
  line-height: 1.6;
  color: var(--color-heading);
  background: linear-gradient(135deg, var(--color-heading) 0%, var(--color-primary) 100%);
  background-clip: text;
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.ui-page-header-subtitle {
  margin: var(--space-control-gap) 0 0;
  color: var(--color-text-light);
  font-size: var(--font-size-description);
  line-height: 1.6;
}

.ui-page-header-actions {
  flex: 0 1 auto;
  max-width: 100%;
}

.ui-page-header-toolbar {
  flex: 1 1 100%;
  width: 100%;
  min-width: 0;
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
