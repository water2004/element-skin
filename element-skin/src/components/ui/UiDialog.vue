<template>
  <ElDialog
    v-model="model"
    v-bind="dialogAttrs()"
    :append-to-body="appendToBody"
    :title="title"
    :show-close="false"
    :header-class="[headerClass, showClose ? 'show-close' : ''].join(' ')"
    :header-aria-level="headerAriaLevel"
    :class="['ui-dialog', `ui-dialog--${props.variant}`, attrs.class]"
    :before-close="requestDismiss"
    :aria-busy="busy || undefined"
    @closed="handleClosed"
  >
    <template #header="scope">
      <slot name="header" v-bind="scope" :close="requestClose">
        <span
          :id="scope.titleId"
          :class="scope.titleClass"
          role="heading"
          :aria-level="headerAriaLevel"
        >
          {{ title }}
        </span>
      </slot>
      <button
        v-if="showClose"
        type="button"
        class="el-dialog__headerbtn"
        aria-label="关闭此对话框"
        @click="requestClose"
      >
        <ElIcon class="el-dialog__close"><component :is="closeIcon || Close" /></ElIcon>
      </button>
    </template>
    <slot :request-close="requestClose" />
    <template v-if="$slots.footer" #footer>
      <slot name="footer" :request-close="requestClose" :close-pending="closePending" />
    </template>
  </ElDialog>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, useAttrs, watch } from 'vue'
import { ElDialog, ElIcon, ElMessageBox, type DialogProps } from 'element-plus'
import { Close } from '@element-plus/icons-vue'

defineOptions({ inheritAttrs: false })

const model = defineModel<boolean>({ required: true })

const props = withDefaults(
  defineProps<{
    variant?: 'form' | 'wide-form' | 'viewer'
    title?: string
    showClose?: boolean
    headerClass?: string
    headerAriaLevel?: string
    closeIcon?: DialogProps['closeIcon']
    appendToBody?: boolean
    busy?: boolean
    dirty?: boolean
    beforeClose?: DialogProps['beforeClose']
  }>(),
  {
    variant: 'form',
    title: '',
    showClose: true,
    headerClass: '',
    headerAriaLevel: '2',
    closeIcon: undefined,
    appendToBody: true,
    busy: false,
    dirty: false,
    beforeClose: undefined,
  },
)

const attrs = useAttrs()
const emit = defineEmits<{ closed: [] }>()
const closePending = ref(false)
let lifecycle = 0

function dialogAttrs() {
  const rest = { ...attrs }
  delete rest.class
  return rest
}

// 取消和关闭按钮明确放弃；遮罩和 Esc 防止误丢修改。两者共用提交保护。
function requestClose() {
  return closeDialog(false)
}

function requestDismiss() {
  return closeDialog(true)
}

async function closeDialog(confirmDiscard: boolean) {
  if (!model.value || props.busy || closePending.value) return
  const currentLifecycle = lifecycle
  let closeCommitted = false
  closePending.value = true
  try {
    if (confirmDiscard && props.dirty) {
      try {
        await ElMessageBox.confirm('当前修改尚未保存，确定放弃吗？', '放弃修改', {
          type: 'warning',
          confirmButtonText: '放弃修改',
          cancelButtonText: '继续编辑',
          closeOnClickModal: false,
          distinguishCancelAndClose: true,
        })
      } catch {
        return
      }
    }
    const close = (cancel?: boolean) => {
      if (cancel || currentLifecycle !== lifecycle || !model.value || props.busy) return
      closeCommitted = true
      model.value = false
    }
    if (currentLifecycle !== lifecycle || !model.value || props.busy) return
    if (props.beforeClose) props.beforeClose(close)
    else close()
  } finally {
    if (currentLifecycle === lifecycle && !closeCommitted) closePending.value = false
  }
}

function handleClosed() {
  // 快速重开时，旧离场动画的 closed 不能清理新一轮表单。
  if (!model.value) emit('closed')
}

watch(
  model,
  () => {
    lifecycle++
    closePending.value = false
  },
  { flush: 'sync' },
)
onBeforeUnmount(() => {
  lifecycle++
})

defineExpose({ requestClose, requestDismiss })
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
  transition:
    color var(--motion-duration-fast) var(--motion-easing-standard),
    background-color var(--motion-duration-fast) var(--motion-easing-standard),
    border-color var(--motion-duration-fast) var(--motion-easing-standard),
    box-shadow var(--motion-duration-fast) var(--motion-easing-standard),
    opacity var(--motion-duration-fast) var(--motion-easing-standard);
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
