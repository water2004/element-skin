<template>
  <ElButton
    v-bind="buttonAttrs()"
    :class="[
      'ui-button',
      attrs.class,
      {
        'ui-button--icon-only': iconOnly,
        'ui-button--loading-icon': loadingMode === 'icon',
        'ui-button--swap': swapIcon && !iconOnly,
        'ui-button--hover': hoverEffect,
      },
    ]"
    :type="buttonType"
    :plain="variant === 'secondary' && tone !== 'neutral'"
    :text="variant === 'tertiary'"
    :size="size"
    :icon="iconOnly ? icon : undefined"
    :loading="loading"
    :disabled="disabled"
    :aria-busy="loading || undefined"
    @click="emit('click', $event)"
  >
    <template #loading>
      <span v-if="loadingMode !== 'icon' || iconOnly" class="ui-button-spinner" aria-hidden="true">
        <ElIcon class="is-loading"><Loading /></ElIcon>
      </span>
    </template>
    <template v-if="!iconOnly && $slots.default" #default>
      <span class="ui-button-content">
        <ElIcon
          v-if="icon || (loading && loadingMode === 'icon')"
          aria-hidden="true"
          :class="{ 'is-loading': loading && loadingMode === 'icon' }"
          ><component :is="loading && loadingMode === 'icon' ? Loading : icon"
        /></ElIcon>
        <slot />
      </span>
      <ElIcon v-if="swapIcon && !iconOnly" class="ui-button-swap" aria-hidden="true">
        <component :is="swapIcon" />
      </ElIcon>
    </template>
  </ElButton>
</template>

<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { ElButton, ElIcon, type ButtonProps } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    variant?: 'primary' | 'secondary' | 'tertiary'
    tone?: 'neutral' | 'primary' | 'success' | 'warning' | 'danger'
    size?: ButtonProps['size']
    icon?: ButtonProps['icon']
    iconOnly?: boolean
    loading?: boolean
    disabled?: boolean
    loadingMode?: 'overlay' | 'icon'
    swapIcon?: ButtonProps['icon']
    hoverEffect?: boolean
  }>(),
  {
    variant: 'secondary',
    tone: 'neutral',
    size: 'default',
    icon: undefined,
    iconOnly: false,
    loading: false,
    disabled: false,
    loadingMode: 'overlay',
    swapIcon: undefined,
    hoverEffect: false,
  },
)

const emit = defineEmits<{ click: [event: MouseEvent] }>()
const attrs = useAttrs()
const buttonType = computed(() =>
  props.tone !== 'neutral' ? props.tone : props.variant === 'primary' ? 'primary' : 'default',
)

// useAttrs 的值会随渲染更新，但不是可供 computed 跟踪的响应式来源。
function buttonAttrs() {
  const rest = { ...attrs }
  delete rest.class
  if (rest.tag === 'a' && (props.disabled || props.loading)) {
    delete rest.href
    rest['aria-disabled'] = true
    rest.tabindex = -1
  }
  return rest
}
</script>

<style scoped>
.ui-button {
  --ui-button-height: var(--control-height-default);
  position: relative;
  height: var(--ui-button-height);
  border-radius: var(--radius-control);
  transition:
    color var(--motion-duration-fast) var(--motion-easing-standard),
    background-color var(--motion-duration-fast) var(--motion-easing-standard),
    border-color var(--motion-duration-fast) var(--motion-easing-standard),
    box-shadow var(--motion-duration-fast) var(--motion-easing-standard);
}

.ui-button.el-button--small {
  --ui-button-height: var(--control-height-small);
}

.ui-button.el-button--large {
  --ui-button-height: var(--control-height-large);
}

.ui-button:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}

.ui-button--icon-only {
  width: var(--ui-button-height);
  padding: 0;
  flex-shrink: 0;
}

.ui-button-content {
  opacity: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-control-gap);
}

.ui-button-content :deep(.el-icon + span) {
  margin-left: 0;
}

.ui-button.is-loading:not(.ui-button--loading-icon) .ui-button-content {
  opacity: 0;
}

.ui-button-swap {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  transform: translateY(100%) rotate(-90deg);
  opacity: 0;
  pointer-events: none;
}

.ui-button--swap {
  overflow: hidden;
}

.ui-button--swap .ui-button-content {
  transform: translateY(0) scale(1);
}

.ui-button--swap .ui-button-content,
.ui-button-swap {
  transition:
    transform var(--motion-duration-swap) var(--motion-easing-swap),
    opacity var(--motion-duration-swap) ease;
}

.ui-button--swap:not(.is-disabled):not(.is-loading):focus-visible .ui-button-content {
  transform: translateY(-100%) scale(0.8);
  opacity: 0;
}

.ui-button--swap:not(.is-disabled):not(.is-loading):focus-visible .ui-button-swap {
  transform: translateY(0) rotate(0deg);
  opacity: 1;
}

@media (hover: hover) {
  .ui-button--swap:not(.is-disabled):not(.is-loading):hover .ui-button-content {
    transform: translateY(-100%) scale(0.8);
    opacity: 0;
  }
  .ui-button--swap:not(.is-disabled):not(.is-loading):hover .ui-button-swap {
    transform: translateY(0) rotate(0deg);
    opacity: 1;
  }
  .ui-button--hover:not(.is-disabled):not(.is-loading):hover {
    box-shadow: var(--shadow-button-hover);
  }
}

.ui-button-spinner {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
  z-index: 1;
}
</style>
