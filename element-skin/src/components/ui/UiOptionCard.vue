<template>
  <component :is="as" v-bind="$attrs" class="ui-option-card">
    <slot />
  </component>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    as?: string
  }>(),
  {
    as: 'div',
  },
)
</script>

<style scoped>
.ui-option-card {
  width: 100%;
  margin-right: 0 !important;
  height: auto !important;
  padding: var(--space-action-gap) var(--space-content-padding) !important;
  display: flex !important;
  align-items: center;
  border-radius: var(--radius-control) !important;
  transition:
    background-color var(--motion-duration-base) var(--motion-easing-standard),
    border-color var(--motion-duration-base) var(--motion-easing-standard) !important;
  border: 1px solid var(--color-border) !important;
  background: var(--color-card-background) !important;
}

.ui-option-card:hover {
  border-color: var(--color-primary) !important;
  background: var(--color-background-soft) !important;
}

.ui-option-card.is-checked {
  background: var(--color-option-selected-background) !important;
  border-color: var(--color-primary) !important;
}

:deep(.ui-option-card__info) {
  display: flex;
  flex-direction: column;
  text-align: left;
  gap: 2px;
}

:deep(.ui-option-card__title) {
  font-weight: 600;
  font-size: var(--font-size-body);
  color: var(--color-heading);
}

:deep(.ui-option-card__subtitle) {
  font-size: var(--font-size-caption);
  color: var(--color-text-light);
  font-family: monospace;
}
</style>
