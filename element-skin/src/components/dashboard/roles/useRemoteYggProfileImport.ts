import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getRemoteYggProfiles, importRemoteYggProfiles } from '@/api/remote-ygg'
import type { YggdrasilImportResult } from '@/api/types'
import { getErrorMessage } from '@/utils/error'

export interface RemoteYggProfile {
  id: string
  name: string
}

export interface RemoteYggImportDeps {
  getProfiles?: typeof getRemoteYggProfiles
  importProfiles?: typeof importRemoteYggProfiles
  onImported?: () => Promise<void> | void
  success?: (message: string) => void
  warning?: (message: string) => void
  error?: (message: string) => void
}

export function selectedRemoteYggProfiles(
  profiles: RemoteYggProfile[],
  selectedProfileIds: string[],
) {
  const selected = new Set(selectedProfileIds)
  return profiles.filter((profile) => selected.has(profile.id))
}

export function remoteYggImportPayload(profiles: RemoteYggProfile[]) {
  return profiles.map((profile) => ({
    profile_id: profile.id,
    profile_name: profile.name,
  }))
}

export function remoteYggImportResultMessage(result: Partial<YggdrasilImportResult>) {
  const successCount = result.success_count ?? 0
  const failureCount = result.failure_count ?? 0
  return {
    type: failureCount > 0 ? 'warning' : 'success',
    message:
      failureCount > 0
        ? `已导入 ${successCount} 个角色，${failureCount} 个失败`
        : `成功导入 ${successCount} 个角色`,
  } as const
}

export function useRemoteYggProfileImport(deps: RemoteYggImportDeps = {}) {
  const showYggImportDialog = ref(false)
  const yggStep = ref<'input' | 'select'>('input')
  const yggApiUrl = ref('')
  const yggUsername = ref('')
  const yggPassword = ref('')
  const yggProfiles = ref<RemoteYggProfile[]>([])
  const selectedYggProfiles = ref<string[]>([])
  const yggLoading = ref(false)

  const previewProfiles = deps.getProfiles ?? getRemoteYggProfiles
  const importProfiles = deps.importProfiles ?? importRemoteYggProfiles
  const notifySuccess = deps.success ?? ElMessage.success
  const notifyWarning = deps.warning ?? ElMessage.warning
  const notifyError = deps.error ?? ElMessage.error

  async function getYggProfiles() {
    if (!yggApiUrl.value || !yggUsername.value || !yggPassword.value) {
      notifyWarning('请填写完整信息')
      return
    }
    try {
      yggLoading.value = true
      const res = await previewProfiles({
        api_url: yggApiUrl.value,
        username: yggUsername.value,
        password: yggPassword.value,
      })

      yggProfiles.value = res.data.items
      if (yggProfiles.value.length === 0) {
        notifyWarning('该账户下没有角色')
      } else {
        yggStep.value = 'select'
        selectedYggProfiles.value = yggProfiles.value.map((profile) => profile.id)
      }
    } catch (e: unknown) {
      notifyError('获取失败: ' + getErrorMessage(e, '获取失败'))
    } finally {
      yggLoading.value = false
    }
  }

  async function importYggProfile() {
    const selectedProfiles = selectedRemoteYggProfiles(yggProfiles.value, selectedYggProfiles.value)
    if (selectedProfiles.length === 0) return

    try {
      yggLoading.value = true
      const res = await importProfiles({
        api_url: yggApiUrl.value,
        profiles: remoteYggImportPayload(selectedProfiles),
      })

      const notice = remoteYggImportResultMessage(res.data)
      if (notice.type === 'warning') {
        notifyWarning(notice.message)
      } else {
        notifySuccess(notice.message)
      }
      showYggImportDialog.value = false
      await deps.onImported?.()
      resetYggImport()
    } catch (e: unknown) {
      notifyError('导入失败: ' + getErrorMessage(e, '导入失败'))
    } finally {
      yggLoading.value = false
    }
  }

  function resetYggImport() {
    yggStep.value = 'input'
    yggApiUrl.value = ''
    yggUsername.value = ''
    yggPassword.value = ''
    yggProfiles.value = []
    selectedYggProfiles.value = []
  }

  function handleYggDialogClose(done?: () => void) {
    if (yggLoading.value) return
    resetYggImport()
    showYggImportDialog.value = false
    if (done && typeof done === 'function') done()
  }

  return {
    showYggImportDialog,
    yggStep,
    yggApiUrl,
    yggUsername,
    yggPassword,
    yggProfiles,
    selectedYggProfiles,
    yggLoading,
    getYggProfiles,
    importYggProfile,
    resetYggImport,
    handleYggDialogClose,
  }
}
