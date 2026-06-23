<template>
  <div class="roles-section animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <div>
          <h1>角色管理</h1>
          <p>创建并管理您的 Minecraft 角色身份</p>
        </div>
      </div>
      <ActionBar full>
        <UiButton
          size="large"
          @click="showYggImportDialog = true"
          variant="gradient-warning"
          class="role-header-button"
        >
          <el-icon><Download /></el-icon>
          <span class="ml-2">导入皮肤站角色</span>
        </UiButton>
        <UiButton
          size="large"
          @click="startMicrosoftAuth"
          variant="gradient-success"
          class="role-header-button"
        >
          <el-icon><Connection /></el-icon>
          <span class="ml-2">绑定正版角色</span>
        </UiButton>
        <UiButton
          size="large"
          @click="showCreateRoleDialog = true"
          variant="gradient-primary"
          class="role-header-button"
        >
          <el-icon><Plus /></el-icon>
          <span class="ml-2">新建角色</span>
        </UiButton>
      </ActionBar>
    </div>

    <div class="min-h-[400px]" v-loading="loading" element-loading-background="transparent">
      <div class="grid grid-cols-[repeat(auto-fill,240px)] justify-center gap-6" v-if="profiles.length > 0">
        <RoleCard
          v-for="(profile, index) in profiles"
          :key="profile.id"
          :profile="profile"
          :delay-index="index % limit"
          :is-dark="isDark"
          :textures-url="texturesUrl"
          @preview="openPreviewDialog"
          @delete="deleteRole"
          @clear-skin="clearRoleSkin"
          @clear-cape="clearRoleCape"
        />
      </div>

      <el-empty v-else-if="!loading" description="还没有角色，快去创建吧！" />
    </div>

    <div class="pagination-container" v-if="profiles.length > 0">
      <CursorPager
        :count="profiles.length"
        :loading="pagination.isLoading.value"
        :disabled-prev="!pagination.canGoPrev.value"
        :disabled-next="!pagination.canGoNext.value"
        @prev="handlePrevPage"
        @next="handleNextPage"
      />
    </div>

    <RolePreviewDialog
      v-model:visible="showPreviewDialog"
      :profile="selectedProfile"
      :textures-url="texturesUrl"
      @rename="updateRoleName"
      @set-avatar="setAsAvatar"
      @clear-skin="clearRoleSkin"
      @clear-cape="clearRoleCape"
      @delete="deleteRole"
    />

    <CreateRoleDialog
      v-model:visible="showCreateRoleDialog"
      v-model:name="newRoleName"
      @create="createRole"
    />

    <MicrosoftImportDialog
      v-model:visible="showMicrosoftLoginDialog"
      :profile="microsoftProfile"
      :importing="importing"
      @cancel="cancelMicrosoftLogin"
      @confirm="importMicrosoftProfile"
    />

    <RemoteYggImportDialog
      v-model:visible="showYggImportDialog"
      v-model:api-url="yggApiUrl"
      v-model:username="yggUsername"
      v-model:password="yggPassword"
      v-model:selected-profiles="selectedYggProfiles"
      :step="yggStep"
      :profiles="yggProfiles"
      :loading="yggLoading"
      @cancel="handleYggDialogClose"
      @next="getYggProfiles"
      @confirm="importYggProfile"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, inject } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { Ref } from 'vue'
import { Connection, Plus, Download } from '@element-plus/icons-vue'
import ActionBar from '@/components/common/ActionBar.vue'
import CursorPager from '@/components/common/CursorPager.vue'
import UiButton from '@/components/ui/UiButton.vue'
import RoleCard from '@/components/dashboard/roles/RoleCard.vue'
import RolePreviewDialog from '@/components/dashboard/roles/RolePreviewDialog.vue'
import CreateRoleDialog from '@/components/dashboard/roles/CreateRoleDialog.vue'
import MicrosoftImportDialog from '@/components/dashboard/roles/MicrosoftImportDialog.vue'
import RemoteYggImportDialog from '@/components/dashboard/roles/RemoteYggImportDialog.vue'
import { useCursorPagination } from '@/composables/useCursorPagination'
import { useAvatar } from '@/composables/useAvatar'
import {
  getProfiles,
  createProfile,
  patchProfile,
  deleteProfile,
  clearProfileSkin,
  clearProfileCape,
} from '@/api/profiles'
import {
  getMicrosoftAuthUrl,
  getMicrosoftProfile,
  importMicrosoftProfile as apiImportMicrosoftProfile,
} from '@/api/microsoft'
import { getRemoteYggProfiles, importRemoteYggProfiles } from '@/api/remote-ygg'
import type { MicrosoftGameProfile, Profile } from '@/api/types'
import { getErrorMessage } from '@/utils/error'

const { setAvatar } = useAvatar()

// Inject shared state from AppLayout
const fetchMe = inject<() => Promise<void>>('fetchMe')
const isDark = inject<Ref<boolean>>('isDark', ref(false))

const router = useRouter()

const profiles = ref<Profile[]>([])
const limit = 12
const loading = ref(false)

// 游标分页 composable
const pagination = useCursorPagination<Profile>(limit)

const showCreateRoleDialog = ref(false)
const newRoleName = ref('')
const showMicrosoftLoginDialog = ref(false)
const microsoftProfile = ref<MicrosoftGameProfile | null>(null)
const microsoftImportToken = ref<string | null>(null)
const importing = ref(false)

const showPreviewDialog = ref(false)
const selectedProfile = ref<Profile | null>(null)

const showYggImportDialog = ref(false)
const yggStep = ref<'input' | 'select'>('input')
const yggApiUrl = ref('')
const yggUsername = ref('')
const yggPassword = ref('')
const yggProfiles = ref<Array<{ id: string; name: string }>>([])
const selectedYggProfiles = ref<string[]>([])
const yggLoading = ref(false)

function openPreviewDialog(profile: Profile) {
  selectedProfile.value = profile
  showPreviewDialog.value = true
}

function texturesUrl(hash: string | null | undefined) {
  if (!hash) return ''
  const base = import.meta.env.BASE_URL
  return `${base}static/textures/${hash}.png`.replace(/\/+/g, '/')
}

async function fetchProfiles() {
  loading.value = true
  try {
    const params = {
      cursor: pagination.currentCursor.value,
      limit: limit,
    }
    const res = await getProfiles(params)
    profiles.value = res.data.items
    pagination.setPageData(res.data)
  } catch {
    ElMessage.error('加载角色失败')
  } finally {
    loading.value = false
  }
}

async function handleNextPage() {
  await pagination.goToNextPage(async (cursor, pageLimit) => {
    const params = { cursor, limit: pageLimit }
    const res = await getProfiles(params)
    profiles.value = res.data.items
    return res.data
  })
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function handlePrevPage() {
  await pagination.goToPrevPage(async (cursor, pageLimit) => {
    const params = { cursor, limit: pageLimit }
    const res = await getProfiles(params)
    profiles.value = res.data.items
    return res.data
  })
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function refreshFirstPage() {
  pagination.reset()
  await fetchProfiles()
}

async function createRole() {
  const name = (newRoleName.value || '').trim()
  if (!name) return ElMessage.error('请输入角色名称')
  try {
    await createProfile({ name })
    newRoleName.value = ''
    showCreateRoleDialog.value = false
    ElMessage.success('创建成功')
    await refreshFirstPage()
    if (fetchMe) fetchMe()
  } catch (e: unknown) {
    ElMessage.error('创建失败: ' + getErrorMessage(e, '创建失败'))
  }
}

async function deleteRole(pid: string) {
  try {
    await deleteProfile(pid)
    ElMessage.success('已删除')
    showPreviewDialog.value = false
    await refreshFirstPage()
    if (fetchMe) fetchMe()
  } catch {
    ElMessage.error('删除失败')
  }
}

async function updateRoleName(name: string) {
  if (!selectedProfile.value) return
  const pid = selectedProfile.value.id
  const newName = (name || '').trim()

  if (!newName) {
    ElMessage.error('角色名不能为空')
    return
  }

  try {
    await patchProfile(pid, { name: newName })
    selectedProfile.value.name = newName
    ElMessage.success('名称已修改')
    await fetchProfiles()
    if (fetchMe) fetchMe()
  } catch (e: unknown) {
    ElMessage.error('修改失败: ' + getErrorMessage(e, '修改失败'))
  }
}

async function clearRoleSkin(pid: string) {
  try {
    await ElMessageBox.confirm('确定要清除该角色的皮肤吗？', '确认清除', {
      type: 'warning',
      confirmButtonText: '确定清除',
      cancelButtonText: '取消',
    })
    await clearProfileSkin(pid)
    ElMessage.success('皮肤已清除')
    showPreviewDialog.value = false
    await fetchProfiles()
    if (fetchMe) fetchMe()
  } catch (e: unknown) {
    if (e !== 'cancel') {
      ElMessage.error('清除失败: ' + getErrorMessage(e, '清除失败'))
    }
  }
}

async function clearRoleCape(pid: string) {
  try {
    await ElMessageBox.confirm('确定要清除该角色的披风吗？', '确认清除', {
      type: 'warning',
      confirmButtonText: '确定清除',
      cancelButtonText: '取消',
    })
    await clearProfileCape(pid)
    ElMessage.success('披风已清除')
    showPreviewDialog.value = false
    await fetchProfiles()
    if (fetchMe) fetchMe()
  } catch (e: unknown) {
    if (e !== 'cancel') {
      ElMessage.error('清除失败: ' + getErrorMessage(e, '清除失败'))
    }
  }
}

async function setAsAvatar(profile: Profile) {
  if (!profile.skin_hash) return

  const loadingMsg = ElMessage({
    message: '正在设置头像...',
    type: 'info',
    duration: 0,
  })

  try {
    await setAvatar(profile.skin_hash, profile.model === 'slim' ? 'slim' : 'default')
    loadingMsg.close()
    ElMessage.success('已设为头像')
  } catch (error) {
    loadingMsg.close()
    ElMessage.error('设置头像失败')
    console.error('Failed to set avatar:', error)
  }
}

async function startMicrosoftAuth() {
  try {
    const response = await getMicrosoftAuthUrl()
    const authUrl = response.data.auth_url
    window.location.href = authUrl
  } catch (error: unknown) {
    ElMessage.error('启动微软登录失败: ' + getErrorMessage(error, '启动微软登录失败'))
  }
}

async function importMicrosoftProfile() {
  if (!microsoftProfile.value) return
  if (!microsoftImportToken.value) {
    ElMessage.error('导入凭证已失效，请重新授权')
    return
  }

  try {
    importing.value = true
    // Do NOT switch step, just show loading on button

    // 导入资料由服务端依据一次性 import_token 固化，前端只需回传该 token。
    await apiImportMicrosoftProfile({ ms_token: microsoftImportToken.value })

    ElMessage.success('正版角色导入成功！')

    showMicrosoftLoginDialog.value = false
    // Delay clearing the profile slightly to allow transition, or just leave it since dialog is destroying anyway
    // But safely clearing it prevents state leak if reopened somehow without reload (unlikely but possible)
    setTimeout(() => {
      microsoftProfile.value = null
      microsoftImportToken.value = null
    }, 300)

    // Refresh data in background
    try {
      fetchProfiles()
      if (fetchMe) await fetchMe()
    } catch (e) {
      console.warn('Failed to refresh user profile:', e)
    }
  } catch (error: unknown) {
    ElMessage.error('导入失败: ' + getErrorMessage(error, '导入失败'))
  } finally {
    importing.value = false
  }
}

function cancelMicrosoftLogin() {
  showMicrosoftLoginDialog.value = false
  microsoftProfile.value = null
  microsoftImportToken.value = null
  importing.value = false
}

// Yggdrasil 相关函数
async function getYggProfiles() {
  if (!yggApiUrl.value || !yggUsername.value || !yggPassword.value) {
    return ElMessage.warning('请填写完整信息')
  }
  try {
    yggLoading.value = true
    const res = await getRemoteYggProfiles({
      api_url: yggApiUrl.value,
      username: yggUsername.value,
      password: yggPassword.value,
    })

    yggProfiles.value = res.data.profiles
    if (yggProfiles.value.length === 0) {
      ElMessage.warning('该账户下没有角色')
    } else {
      yggStep.value = 'select'
      selectedYggProfiles.value = yggProfiles.value.map((profile) => profile.id)
    }
  } catch (e: unknown) {
    ElMessage.error('获取失败: ' + getErrorMessage(e, '获取失败'))
  } finally {
    yggLoading.value = false
  }
}

async function importYggProfile() {
  const selectedProfiles = yggProfiles.value.filter((profile) =>
    selectedYggProfiles.value.includes(profile.id),
  )
  if (selectedProfiles.length === 0) return

  try {
    yggLoading.value = true
    const res = await importRemoteYggProfiles({
      api_url: yggApiUrl.value,
      profiles: selectedProfiles.map((profile) => ({
        profile_id: profile.id,
        profile_name: profile.name,
      })),
    })

    const successCount = res.data?.success_count ?? 0
    const failureCount = res.data?.failure_count ?? 0
    if (failureCount > 0) {
      ElMessage.warning(`已导入 ${successCount} 个角色，${failureCount} 个失败`)
    } else {
      ElMessage.success(`成功导入 ${successCount} 个角色`)
    }
    showYggImportDialog.value = false
    await refreshFirstPage()
    if (fetchMe) fetchMe()
    resetYggImport()
  } catch (e: unknown) {
    ElMessage.error('导入失败: ' + getErrorMessage(e, '导入失败'))
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

onMounted(async () => {
  await refreshFirstPage()
  const urlParams = new URLSearchParams(window.location.search)
  const msToken = urlParams.get('ms_token')
  const error = urlParams.get('error')

  if (error) {
    ElMessage.error('微软登录失败: ' + error)
    router.replace({ query: {} })
  } else if (msToken) {
    try {
      const response = await getMicrosoftProfile({ ms_token: msToken })

      microsoftProfile.value = {
        ...response.data.profile,
        has_game: response.data.has_game,
      }
      // 服务端换发的一次性导入凭证：确认导入时回传，导入资料以服务端固化为准。
      microsoftImportToken.value = response.data.import_token
      showMicrosoftLoginDialog.value = true

      ElMessage.success('授权成功！')
    } catch (e: unknown) {
      ElMessage.error('获取角色信息失败: ' + getErrorMessage(e, '获取角色信息失败'))
    }
    router.replace({ query: {} })
  }
})
</script>

<style scoped>
.role-header-button {
  flex: 0 1 auto;
  margin-left: 0 !important;
}

@media (max-width: 900px) {
  .role-header-button {
    flex: 1 1 180px;
  }
}

@media (max-width: 520px) {
  .role-header-button {
    flex-basis: 100%;
  }
}
</style>
