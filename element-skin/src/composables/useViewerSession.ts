import { computed, onBeforeUnmount, ref, watch, type Ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import { getErrorMessage, getErrorStatus } from '@/utils/error'

/** A detail's draft and write lock. API calls and object updates stay in its page. */
export function useViewerSession(open: Ref<boolean>, key: Readonly<Ref<string>>) {
  const busy = ref(false)
  const dirty = ref(false)
  const blocked = computed(() => busy.value || dirty.value)
  const revision = ref(0)
  let writeRevision = 0
  let disposed = false

  watch(
    [open, key],
    () => {
      revision.value++
      dirty.value = false
    },
    { flush: 'sync' },
  )
  onBeforeUnmount(() => {
    disposed = true
    revision.value++
  })
  useUnsavedChangesGuard({ dirty, busy })

  function snapshot() {
    const version = revision.value
    const mutation = writeRevision
    return () => !disposed && version === revision.value && mutation === writeRevision
  }

  async function allowChange() {
    if (disposed || busy.value) return false
    if (!dirty.value) return true
    const current = snapshot()
    try {
      await ElMessageBox.confirm('放弃尚未保存的名称修改？', '放弃修改', {
        type: 'warning',
        confirmButtonText: '放弃修改',
        cancelButtonText: '继续编辑',
        closeOnClickModal: false,
        distinguishCancelAndClose: true,
      })
      if (!current() || busy.value) return false
      open.value = false
      dirty.value = false
      return true
    } catch {
      return false
    }
  }

  async function run(
    action: (current: () => boolean) => Promise<void | boolean>,
    failure: string,
    savingName = false,
  ): Promise<boolean> {
    if (disposed || busy.value || (dirty.value && !savingName)) return false
    busy.value = true
    writeRevision++
    const current = snapshot()
    try {
      const result = await action(current)
      return result !== false && !disposed
    } catch (error) {
      if (current() && error !== 'cancel' && error !== 'close') {
        const uncertain = !getErrorStatus(error)
        ElMessage.error(
          getErrorMessage(error, failure) + (uncertain ? '；未能确认写入结果，请核对后再重试' : ''),
        )
      }
      return false
    } finally {
      busy.value = false
    }
  }

  return { busy, dirty, blocked, revision, snapshot, allowChange, run }
}
