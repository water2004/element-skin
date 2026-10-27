<template>
  <span class="inline-flex min-w-0 items-center gap-2 text-sm">
    <span class="shrink-0 text-[var(--color-text-light)]">{{ label }}</span>
    <el-popover
      v-if="value"
      :visible="visible"
      placement="top"
      :width="320"
      :teleported="true"
      popper-class="max-w-[calc(100vw-2rem)]"
    >
      <template #reference>
        <span
          @mouseenter="show"
          @mouseleave="scheduleHide"
          @focusin="show"
          @focusout="scheduleHide"
        >
          <UiButton
            ref="trigger"
            variant="tertiary"
            size="small"
            class="font-mono"
            :aria-label="`查看完整${label}，点击可展开`"
            :aria-expanded="visible"
            @click.stop="pin"
            @keydown.esc.stop.prevent="close"
          >
            {{ value.length > 8 ? `${value.slice(0, 8)}...` : value }}
          </UiButton>
        </span>
      </template>
      <div
        ref="popup"
        @mouseenter="show"
        @mouseleave="scheduleHide"
        @focusin="show"
        @focusout="scheduleHide"
        @keydown.esc.stop.prevent="close"
      >
        <UiButton
          variant="tertiary"
          class="h-auto! max-w-full text-left"
          :aria-label="`复制完整${label}`"
          @click.stop="copy"
          ><span class="flex min-w-0 flex-col items-start gap-1">
            <code class="whitespace-normal break-all font-mono select-text">{{ value }}</code>
            <span class="text-xs text-[var(--color-text-light)]">点击复制</span>
          </span></UiButton
        >
      </div>
    </el-popover>
    <span v-else class="text-[var(--color-text-light)]">未绑定</span>
  </span>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import UiButton from '@/components/ui/UiButton.vue'
const props = defineProps<{ label: string; value?: string | null }>()
const visible = ref(false)
const trigger = ref<{ $el: HTMLElement } | null>(null)
const popup = ref<HTMLElement | null>(null)
let pinned = false
let timer: ReturnType<typeof setTimeout> | undefined
function show() {
  clearTimeout(timer)
  visible.value = true
}
function scheduleHide() {
  clearTimeout(timer)
  timer = setTimeout(() => {
    if (!pinned && !popup.value?.contains(document.activeElement)) visible.value = false
  }, 150)
}
function pin() {
  pinned = true
  show()
}
function close() {
  clearTimeout(timer)
  trigger.value?.$el?.focus()
  pinned = false
  visible.value = false
}
function higherDialogOpen() {
  return [...document.querySelectorAll<HTMLElement>('.el-message-box__wrapper')].some(
    (element) => element.style.display !== 'none',
  )
}
function outside(event: MouseEvent) {
  if (higherDialogOpen()) return
  const node = event.target as Node
  if (!popup.value?.contains(node) && !trigger.value?.$el?.contains(node)) {
    event.preventDefault()
    event.stopPropagation()
    close()
  }
}
function escape(event: KeyboardEvent) {
  if (event.key !== 'Escape' || event.isComposing) return
  // Message boxes are a higher layer and own their cancellation key.
  if (higherDialogOpen()) return
  event.preventDefault()
  event.stopImmediatePropagation()
  close()
}
watch(visible, (value, _, cleanup) => {
  if (!value) return
  document.addEventListener('click', outside, true)
  document.addEventListener('keydown', escape, true)
  cleanup(() => {
    document.removeEventListener('click', outside, true)
    document.removeEventListener('keydown', escape, true)
  })
})
watch(
  () => props.value,
  () => {
    pinned = false
    visible.value = false
  },
)
onBeforeUnmount(() => clearTimeout(timer))
async function copy() {
  if (!props.value) return
  try {
    await navigator.clipboard.writeText(props.value)
    ElMessage.success('已复制')
  } catch {
    ElMessage.error('复制失败，请选择完整值手动复制')
  }
}
</script>
