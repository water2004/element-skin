<template>
  <el-card v-bind="forwardedAttrs" :shadow="shadow" :class="rootClass">
    <template v-if="$slots.header" #header>
      <slot name="header" />
    </template>
    <slot />
  </el-card>
</template>

<script setup lang="ts">
import { computed, useAttrs } from 'vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    hoverable?: boolean
    shadow?: 'always' | 'hover' | 'never'
  }>(),
  {
    hoverable: false,
    shadow: 'never',
  },
)

const attrs = useAttrs()

const forwardedAttrs = computed(() => {
  const rest = { ...attrs }
  delete rest.class
  return rest
})

const rootClass = computed(() => ['ui-card', attrs.class, { 'is-hoverable': props.hoverable }])
</script>

<style scoped>
.ui-card {
  --el-card-padding: var(--space-card-padding);
  border-radius: var(--radius-card);
  border: 1px solid var(--color-border);
  box-shadow: var(--shadow-card);
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
  box-shadow: var(--shadow-card-hover);
}
</style>
