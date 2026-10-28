<template>
  <el-radio-group v-model="model" v-bind="forwardedAttrs" :class="rootClass">
    <slot />
  </el-radio-group>
</template>

<script setup lang="ts">
import { computed, useAttrs } from 'vue'

defineOptions({ inheritAttrs: false })

const model = defineModel<string | number | boolean | null | undefined>({ required: true })

const props = withDefaults(
  defineProps<{
    variant?: 'capsule' | 'modern'
  }>(),
  {
    variant: 'capsule',
  },
)

const attrs = useAttrs()

const forwardedAttrs = computed(() => {
  const rest = { ...attrs }
  delete rest.class
  return rest
})

const rootClass = computed(() => ['ui-segmented', `ui-segmented--${props.variant}`, attrs.class])
</script>

<style>
.ui-segmented {
  max-width: 100%;
  min-width: 0;
  row-gap: var(--space-control-gap);
}

.ui-segmented--capsule .el-radio-button__inner {
  background: var(--color-segmented-background);
  color: var(--color-text);
  border: 1px solid var(--color-border) !important;
  font-weight: 500;
  transition:
    background-color var(--motion-duration-base) var(--motion-easing-standard),
    border-color var(--motion-duration-base) var(--motion-easing-standard),
    color var(--motion-duration-base) var(--motion-easing-standard);
}

.ui-segmented--capsule .el-radio-button.is-active .el-radio-button__inner {
  background-color: var(--color-primary) !important;
  border-color: var(--color-primary) !important;
  color: var(--color-on-accent) !important;
}

.ui-segmented--modern .el-radio-button__inner {
  height: 48px;
  display: flex;
  align-items: center;
  padding: 0 30px;
  border-radius: var(--radius-control) !important;
  margin-right: var(--space-action-gap);
  border: 1px solid var(--color-border) !important;
  background: var(--color-segmented-background);
  color: var(--color-text);
  box-shadow: none !important;
  transition:
    background-color var(--motion-duration-base) var(--motion-easing-standard),
    border-color var(--motion-duration-base) var(--motion-easing-standard),
    color var(--motion-duration-base) var(--motion-easing-standard);
}

.ui-segmented--modern .el-radio-button.is-active .el-radio-button__inner {
  background-color: var(--color-selection-background) !important;
  color: var(--color-primary) !important;
  border-color: var(--color-primary) !important;
}

html.dark .ui-segmented .el-radio-button__inner {
  background: var(--color-segmented-background) !important;
  color: var(--color-text);
  border-color: var(--color-border) !important;
}

html.dark .ui-segmented--modern .el-radio-button.is-active .el-radio-button__inner {
  background-color: var(--color-selection-background) !important;
  color: var(--color-primary) !important;
}
</style>
