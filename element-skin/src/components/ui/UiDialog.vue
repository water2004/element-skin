<template>
  <el-dialog
    v-model="model"
    v-bind="forwardedAttrs"
    :append-to-body="appendToBody"
    :class="rootClass"
  >
    <template v-if="$slots.header" #header>
      <slot name="header" />
    </template>
    <slot />
    <template v-if="$slots.footer" #footer>
      <slot name="footer" />
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, useAttrs } from 'vue'

defineOptions({ inheritAttrs: false })

const model = defineModel<boolean>({ required: true })

const props = withDefaults(
  defineProps<{
    variant?: 'form' | 'wide-form' | 'viewer'
    appendToBody?: boolean
  }>(),
  {
    variant: 'form',
    appendToBody: true,
  },
)

const attrs = useAttrs()

const forwardedAttrs = computed(() => {
  const rest = { ...attrs }
  delete rest.class
  return rest
})

const rootClass = computed(() => ['ui-dialog', `ui-dialog--${props.variant}`, attrs.class])
</script>

<style>
.ui-dialog {
  border-radius: var(--radius-dialog) !important;
  overflow: hidden !important;
  border: 1px solid var(--color-border);
  background: var(--color-card-background);
}

.ui-dialog--form {
  width: min(calc(100vw - var(--space-panel-padding)), var(--dialog-width-form)) !important;
}

.ui-dialog--wide-form {
  width: min(calc(100vw - var(--space-panel-padding)), var(--dialog-width-wide-form)) !important;
}

.ui-dialog--wide-form .el-dialog__body {
  padding: 0 !important;
}

.ui-dialog--viewer {
  width: min(calc(100vw - var(--space-panel-padding)), var(--dialog-width-viewer)) !important;
  padding: 0 !important;
}

.ui-dialog--viewer .el-dialog__header {
  padding: 0 !important;
  margin: 0 !important;
  height: 0 !important;
}

.ui-dialog--viewer .el-dialog__body {
  padding: 0 !important;
}

.ui-dialog--viewer .el-dialog__headerbtn {
  position: absolute;
  top: var(--space-content-padding);
  right: var(--space-content-padding);
  z-index: 1000;
  width: var(--control-height-default);
  height: var(--control-height-default);
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-background-soft);
  border-radius: 50%;
  border: 1px solid var(--color-border);
  transition: all var(--motion-duration-fast) var(--motion-easing-standard);
}

.ui-dialog--viewer .el-dialog__headerbtn:hover {
  background: var(--color-primary);
  border-color: var(--color-primary);
}

.ui-dialog--viewer .el-dialog__headerbtn:hover .el-dialog__close {
  color: var(--color-on-accent) !important;
}

.ui-dialog--viewer .el-dialog__headerbtn .el-icon {
  font-size: 16px;
  color: var(--color-text-light);
}
</style>
