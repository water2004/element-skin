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
      <div
        class="grid grid-cols-[repeat(auto-fill,240px)] justify-center gap-6"
        v-if="profiles.length > 0"
      >
        <RoleCard
          v-for="(profile, index) in profiles"
          :key="profile.id"
          :profile="profile"
          :delay-index="index % limit"
          :is-dark="isDark"
          :textures-url="texturesUrl"
          :official-binding="officialBindingFor(profile.id)"
          :can-create-official-binding="canCreateOfficialBinding"
          :can-sync-official-binding="canSyncOfficialBinding"
          :can-delete-official-binding="canDeleteOfficialBinding"
          @preview="openPreviewDialog"
          @delete="deleteRole"
          @clear-skin="clearRoleSkin"
          @clear-cape="clearRoleCape"
          @bind-official="openOfficialBindingDialog"
          @sync-official="syncOfficialBinding"
          @unbind-official="unbindOfficialBinding"
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

    <OfficialBindingDialog
      v-model:visible="showOfficialBindingDialog"
      :profile="bindingProfile"
      :identities="externalIdentities"
      :loading="bindingLoading"
      @confirm="bindOfficialProfile"
      @manage-identities="goToIdentities"
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
import { computed, ref, onMounted, inject } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { Ref } from 'vue'
import { Plus, Download } from '@element-plus/icons-vue'
import ActionBar from '@/components/common/ActionBar.vue'
import CursorPager from '@/components/common/CursorPager.vue'
import UiButton from '@/components/ui/UiButton.vue'
import RoleCard from '@/components/dashboard/roles/RoleCard.vue'
import RolePreviewDialog from '@/components/dashboard/roles/RolePreviewDialog.vue'
import CreateRoleDialog from '@/components/dashboard/roles/CreateRoleDialog.vue'
import OfficialBindingDialog from '@/components/dashboard/roles/OfficialBindingDialog.vue'
import RemoteYggImportDialog from '@/components/dashboard/roles/RemoteYggImportDialog.vue'
import { textureAssetUrl as texturesUrl } from '@/components/textures/textureAssets'
import { useRemoteYggProfileImport } from '@/components/dashboard/roles/useRemoteYggProfileImport'
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
import type { ExternalIdentity, OfficialProfileBinding, Profile, User } from '@/api/types'
import { getExternalIdentities } from '@/api/identity'
import {
  createOfficialProfileBinding,
  deleteOfficialProfileBinding,
  getOfficialProfileBindings,
  syncOfficialProfileBinding,
} from '@/api/official-profiles'
import { getErrorMessage, isExternalIdentityReauthorizationRequired } from '@/utils/error'

const { setAvatar } = useAvatar()

// Inject shared state from AppLayout
const fetchMe = inject<() => Promise<void>>('fetchMe')
const isDark = inject<Ref<boolean>>('isDark', ref(false))
const user = inject<Ref<User | null>>('user', ref(null))

const router = useRouter()

const profiles = ref<Profile[]>([])
const limit = 12
const loading = ref(false)

// 游标分页 composable
const pagination = useCursorPagination<Profile>(limit)

const showCreateRoleDialog = ref(false)
const newRoleName = ref('')

const showPreviewDialog = ref(false)
const selectedProfile = ref<Profile | null>(null)
const externalIdentities = ref<ExternalIdentity[]>([])
const officialBindings = ref<OfficialProfileBinding[]>([])
const showOfficialBindingDialog = ref(false)
const bindingProfile = ref<Profile | null>(null)
const bindingLoading = ref(false)

const permissionSet = computed(() => new Set(user.value?.permissions || []))
const canCreateOfficialBinding = computed(
  () =>
    permissionSet.value.has('official_profile.create.owned') &&
    permissionSet.value.has('external_identity.read.owned'),
)
const canSyncOfficialBinding = computed(() =>
  permissionSet.value.has('official_profile.refresh.owned'),
)
const canDeleteOfficialBinding = computed(() =>
  permissionSet.value.has('official_profile.delete.owned'),
)

const {
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
  handleYggDialogClose,
} = useRemoteYggProfileImport({
  async onImported() {
    await refreshFirstPage()
    if (fetchMe) await fetchMe()
  },
})

function openPreviewDialog(profile: Profile) {
  selectedProfile.value = profile
  showPreviewDialog.value = true
}

function officialBindingFor(profileId: string) {
  return officialBindings.value.find((binding) => binding.profile_id === profileId) || null
}

function openOfficialBindingDialog(profile: Profile) {
  bindingProfile.value = profile
  showOfficialBindingDialog.value = true
}

function goToIdentities() {
  showOfficialBindingDialog.value = false
  void router.push('/dashboard/identities')
}

async function fetchOfficialResources() {
  try {
    const [identityResponse, bindingResponse] = await Promise.all([
      permissionSet.value.has('external_identity.read.owned')
        ? getExternalIdentities()
        : Promise.resolve(null),
      permissionSet.value.has('official_profile.read.owned')
        ? getOfficialProfileBindings()
        : Promise.resolve(null),
    ])
    externalIdentities.value = identityResponse?.data.items || []
    officialBindings.value = bindingResponse?.data.items || []
  } catch (e: unknown) {
    ElMessage.error('加载正版绑定失败: ' + getErrorMessage(e, '加载失败'))
  }
}

async function bindOfficialProfile(payload: { identity_id: string; profile_id: string }) {
  try {
    bindingLoading.value = true
    await createOfficialProfileBinding(payload)
    showOfficialBindingDialog.value = false
    bindingProfile.value = null
    ElMessage.success('正版角色已绑定；需要时请点击“同步”更新角色数据')
    await fetchOfficialResources()
  } catch (e: unknown) {
    if (isExternalIdentityReauthorizationRequired(e)) {
      ElMessage.error('该 Microsoft 身份授权已失效，请前往身份管理页重新登录')
      await fetchOfficialResources()
    } else {
      ElMessage.error('绑定失败: ' + getErrorMessage(e, '绑定失败'))
    }
  } finally {
    bindingLoading.value = false
  }
}

async function syncOfficialBinding(binding: OfficialProfileBinding) {
  try {
    await ElMessageBox.confirm(
      `将用远端正版角色 ${binding.remote_name} 覆盖本站角色的名称、皮肤和披风，是否继续？`,
      '同步正版角色',
      { type: 'warning', confirmButtonText: '开始同步', cancelButtonText: '取消' },
    )
    await syncOfficialProfileBinding(binding.id)
    ElMessage.success('正版角色同步完成')
    await Promise.all([fetchProfiles(), fetchOfficialResources()])
    if (fetchMe) await fetchMe()
  } catch (e: unknown) {
    if (e !== 'cancel' && e !== 'close') {
      if (isExternalIdentityReauthorizationRequired(e)) {
        ElMessage.error('该 Microsoft 身份授权已失效，请前往身份管理页重新登录')
        await fetchOfficialResources()
      } else {
        ElMessage.error('同步失败: ' + getErrorMessage(e, '同步失败'))
      }
    }
  }
}

async function unbindOfficialBinding(binding: OfficialProfileBinding) {
  try {
    await ElMessageBox.confirm('解除绑定不会删除本站角色、皮肤或外部身份。', '解除正版绑定', {
      type: 'warning',
      confirmButtonText: '解除绑定',
      cancelButtonText: '取消',
    })
    await deleteOfficialProfileBinding(binding.id)
    ElMessage.success('已解除正版绑定')
    await fetchOfficialResources()
  } catch (e: unknown) {
    if (e !== 'cancel' && e !== 'close') {
      ElMessage.error('解除失败: ' + getErrorMessage(e, '解除失败'))
    }
  }
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
    await Promise.all([refreshFirstPage(), fetchOfficialResources()])
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

onMounted(async () => {
  await Promise.all([refreshFirstPage(), fetchOfficialResources()])
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
