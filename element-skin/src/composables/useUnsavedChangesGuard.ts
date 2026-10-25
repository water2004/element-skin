import { onBeforeUnmount, watch, type Ref } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'

interface UnsavedChangesOptions {
  dirty: Readonly<Ref<boolean>>
  busy?: Readonly<Ref<boolean>>
}

export function useUnsavedChangesGuard({ dirty, busy }: UnsavedChangesOptions) {
  let disposed = false
  let pending: Promise<boolean> | null = null

  function warnBeforeUnload(event: BeforeUnloadEvent) {
    if (dirty.value || busy?.value) event.preventDefault()
  }

  watch(
    () => dirty.value || !!busy?.value,
    (protectedPage, _previous, onCleanup) => {
      if (!protectedPage) return
      window.addEventListener('beforeunload', warnBeforeUnload)
      onCleanup(() => window.removeEventListener('beforeunload', warnBeforeUnload))
    },
    { immediate: true, flush: 'sync' },
  )

  async function confirmDiscard(action: 'leave' | 'reload') {
    if (disposed) return false
    if (busy?.value) {
      ElMessage.warning('操作正在进行，请完成后再离开或重新加载')
      return false
    }
    if (!dirty.value) return true
    if (pending) return pending
    const reloading = action === 'reload'
    pending = ElMessageBox.confirm(
      reloading
        ? '重新加载会丢弃当前页面未保存的更改，确定继续吗？'
        : '当前页面有未保存的更改，离开后将丢失。确定放弃并离开吗？',
      reloading ? '重新加载配置' : '离开页面',
      {
        type: 'warning',
        confirmButtonText: reloading ? '放弃并重新加载' : '放弃并离开',
        cancelButtonText: '继续编辑',
        closeOnClickModal: false,
        distinguishCancelAndClose: true,
      },
    )
      .then(() => !disposed && !busy?.value)
      .catch(() => false)
      .finally(() => {
        pending = null
      })
    return pending
  }

  onBeforeRouteLeave(() => confirmDiscard('leave'))
  onBeforeRouteUpdate((to, from) => {
    if (JSON.stringify(to.params) !== JSON.stringify(from.params)) return confirmDiscard('leave')
    return true
  })
  onBeforeUnmount(() => {
    disposed = true
  })

  return { confirmReload: () => confirmDiscard('reload') }
}
