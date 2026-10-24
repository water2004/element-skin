<template>
  <ElCard
    v-bind="cardAttrs()"
    :shadow="shadow"
    :class="['ui-card', `ui-card--padding-${padding}`, attrs.class, { 'is-hoverable': hoverable }]"
  >
    <template v-if="$slots.header" #header>
      <slot name="header" />
    </template>
    <slot />
  </ElCard>
</template>

<script setup lang="ts">
import { useAttrs } from 'vue'
import { ElCard } from 'element-plus'

defineOptions({ inheritAttrs: false })

withDefaults(
  defineProps<{
    hoverable?: boolean
    shadow?: 'always' | 'hover' | 'never'
    padding?: 'none' | 'compact' | 'default' | 'spacious'
  }>(),
  {
    hoverable: false,
    shadow: 'never',
    padding: 'default',
  },
)

const attrs = useAttrs()

function cardAttrs() {
  const rest = { ...attrs }
  delete rest.class
  return rest
}
</script>

<style scoped>
.ui-card {
  --el-card-padding: var(--space-card-padding);
  border-radius: var(--radius-card);
  border: 1px solid var(--color-border);
  box-shadow: none;
  background: var(--color-card-background);
  transition:
    background-color var(--motion-duration-base) ease,
    border-color var(--motion-duration-base) ease,
    transform var(--motion-duration-base) ease,
    box-shadow var(--motion-duration-base) ease;
  overflow: hidden;
}

.ui-card.is-hoverable:hover {
  transform: translateY(calc(-1 * var(--motion-offset-hover)));
}

.ui-card.is-always-shadow {
  box-shadow: var(--shadow-card);
}

.ui-card.is-hover-shadow:hover,
.ui-card.is-always-shadow.is-hoverable:hover {
  box-shadow: var(--shadow-card-hover);
}

.ui-card.is-never-shadow,
.ui-card.is-never-shadow:hover {
  box-shadow: none;
}

.ui-card--padding-none {
  --el-card-padding: 0px;
}

.ui-card--padding-compact {
  --el-card-padding: var(--space-content-padding);
}

.ui-card--padding-spacious {
  --el-card-padding: var(--space-panel-padding);
}
</style>
