<template>
  <PageContainer variant="gallery" class="roles-section">
    <PageHeader
      title="角色管理"
      subtitle="创建并管理您的 Minecraft 角色身份"
      class="animate-fade-in"
      ><template #actions>
        <UiButton
          v-if="canCreateOfficialBinding"
          :disabled="viewer.blocked.value"
          size="large"
          variant="secondary"
          tone="success"
          :icon="Link"
          class="role-header-button"
          @click="openOfficialBindingDialog"
        >
          绑定正版
        </UiButton>
        <UiButton
          v-if="canCreateRole"
          :disabled="viewer.blocked.value"
          size="large"
          @click="showYggImportDialog = true"
          variant="secondary"
          tone="warning"
          :icon="Download"
          class="role-header-button"
        >
          导入皮肤站角色
        </UiButton>
        <UiButton
          v-if="canCreateRole"
          :disabled="viewer.blocked.value"
          size="large"
          @click="showCreateRoleDialog = true"
          variant="primary"
          :icon="Plus"
          class="role-header-button"
        >
          新建角色
        </UiButton>
      </template></PageHeader
    >

    <div
      class="min-h-[400px]"
      v-loading="loading || sessionPending"
      element-loading-background="transparent"
    >
      <div
        class="grid grid-cols-[repeat(auto-fill,240px)] justify-center gap-6"
        :style="{ '--motion-card-count': profiles.length }"
        v-if="profiles.length > 0"
      >
        <ProfileCard
          v-for="(profile, index) in profiles"
          :key="profile.id"
          :profile="profile"
          :delay-index="index % limit"
          :textures-url="texturesUrl"
          :official="officialState === 'ready' && !!officialBindingFor(profile.id)"
          @preview="openPreviewDialog"
        >
          <template
            v-if="
              (profile.skin_hash && (canSetAvatar || canClearTexture)) ||
              (profile.cape_hash && canClearTexture)
            "
            #actions
          >
            <div
              class="grid w-full auto-cols-fr grid-flow-col gap-2"
              role="group"
              aria-label="角色快捷操作"
            >
              <el-tooltip
                v-if="canSetAvatar && profile.skin_hash"
                content="设为头像"
                placement="top"
                :trigger="['hover', 'focus']"
                :trigger-keys="[]"
              >
                <UiButton
                  variant="secondary"
                  aria-label="设为头像"
                  :swap-icon="Setting"
                  :disabled="viewer.blocked.value"
                  @click.stop="setAsAvatar(profile)"
                  >头像</UiButton
                >
              </el-tooltip>
              <el-tooltip
                v-if="canClearTexture && profile.skin_hash"
                content="清除皮肤"
                placement="top"
                :trigger="['hover', 'focus']"
                :trigger-keys="[]"
              >
                <UiButton
                  variant="secondary"
                  tone="warning"
                  aria-label="清除皮肤"
                  :swap-icon="Close"
                  :disabled="viewer.blocked.value"
                  @click.stop="clearRoleSkin(profile.id)"
                  >皮肤</UiButton
                >
              </el-tooltip>
              <el-tooltip
                v-if="canClearTexture && profile.cape_hash"
                content="清除披风"
                placement="top"
                :trigger="['hover', 'focus']"
                :trigger-keys="[]"
              >
                <UiButton
                  variant="secondary"
                  tone="warning"
                  aria-label="清除披风"
                  :swap-icon="Close"
                  :disabled="viewer.blocked.value"
                  @click.stop="clearRoleCape(profile.id)"
                  >披风</UiButton
                >
              </el-tooltip>
            </div>
          </template>
        </ProfileCard>
      </div>

      <el-empty
        v-else-if="!loading && !sessionPending"
        :description="listError || (canRead ? '还没有角色，快去创建吧！' : '无权读取角色列表')"
      />
    </div>

    <div class="pagination-container" v-if="profiles.length > 0">
      <CursorPager
        :count="profiles.length"
        :loading="pagination.isLoading.value"
        :disabled-prev="!pagination.canGoPrev.value"
        :disabled-next="!pagination.canGoNext.value"
        :page-size="pagination.limit.value"
        @prev="handlePrevPage"
        @next="handleNextPage"
        @page-size-change="handlePageSizeChange"
      />
    </div>

    <RolePreviewDialog
      v-model:visible="showPreviewDialog"
      :profile="selectedProfile"
      :textures-url="texturesUrl"
      :can-rename="canRenameRole"
      :can-set-avatar="canSetAvatar"
      :can-clear-texture="canClearTexture"
      :can-delete="canDeleteRole"
      :official-binding="selectedProfile ? officialBindingFor(selectedProfile.id) : null"
      :can-sync-official-binding="canSyncOfficialBinding"
      :can-delete-official-binding="canDeleteOfficialBinding"
      v-model:dirty="viewer.dirty.value"
      :busy="viewer.busy.value"
      :revision="viewer.revision.value"
      :save-name="updateRoleName"
      :official-state="officialState"
      @set-avatar="setAsAvatar"
      @clear-skin="clearRoleSkin"
      @clear-cape="clearRoleCape"
      @sync-official="syncOfficialBinding"
      @unbind-official="unbindOfficialBinding"
      @delete="deleteRole"
    />

    <CreateRoleDialog
      v-model:visible="showCreateRoleDialog"
      v-model:name="newRoleName"
      :creating="creatingRole"
      @create="createRole"
    />

    <OfficialBindingDialog
      v-model:visible="showOfficialBindingDialog"
      :identities="externalIdentities"
      :bindings="officialBindings"
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
  </PageContainer>
</template>

<script setup lang="ts">
import PageContainer from '@/components/common/PageContainer.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import { computed, ref, watch, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Download, Link, Setting, Close } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import UiButton from '@/components/ui/UiButton.vue'
import ProfileCard from '@/components/common/ProfileCard.vue'
import RolePreviewDialog from '@/components/dashboard/roles/RolePreviewDialog.vue'
import CreateRoleDialog from '@/components/dashboard/roles/CreateRoleDialog.vue'
import OfficialBindingDialog from '@/components/dashboard/roles/OfficialBindingDialog.vue'
import RemoteYggImportDialog from '@/components/dashboard/roles/RemoteYggImportDialog.vue'
import { textureAssetUrl as texturesUrl } from '@/components/textures/textureAssets'
import { useRemoteYggProfileImport } from '@/components/dashboard/roles/useRemoteYggProfileImport'
import { useCursorPagination } from '@/composables/useCursorPagination'
import { useViewerSession } from '@/composables/useViewerSession'
import { useAvatar } from '@/composables/useAvatar'
import {
  getProfiles,
  createProfile,
  patchProfile,
  deleteProfile,
  clearProfileSkin,
  clearProfileCape,
} from '@/api/profiles'
import type { ExternalIdentity, OfficialProfileBinding, Profile } from '@/api/types'
import { getExternalIdentities } from '@/api/identity'
import { useSessionStore } from '@/stores/session'
import { storeToRefs } from 'pinia'
import {
  createOfficialProfileBinding,
  deleteOfficialProfileBinding,
  getOfficialProfileBindings,
  syncOfficialProfileBinding,
} from '@/api/official-profiles'
import { getErrorMessage, isExternalIdentityReauthorizationRequired } from '@/utils/error'

const { setAvatar } = useAvatar()

// Inject shared state from AppLayout
const sessionStore = useSessionStore()
const { fetchMe } = sessionStore
const { user } = storeToRefs(sessionStore)

const router = useRouter()

const profiles = ref<Profile[]>([])
const pagination = useCursorPagination<Profile>(20)
const limit = pagination.limit
const loading = ref(false)
const listError = ref('')
let listRequest = 0

const showCreateRoleDialog = ref(false)
const creatingRole = ref(false)
const newRoleName = ref('')

const showPreviewDialog = ref(false)
const selectedProfile = ref<Profile | null>(null)
const viewer = useViewerSession(
  showPreviewDialog,
  computed(() => selectedProfile.value?.id || ''),
)
const officialState = ref<'ready' | 'loading' | 'error' | 'forbidden'>('forbidden')
let officialGeneration = 0
onBeforeUnmount(() => {
  officialGeneration++
})
const externalIdentities = ref<ExternalIdentity[]>([])
const officialBindings = ref<OfficialProfileBinding[]>([])
const showOfficialBindingDialog = ref(false)
const bindingLoading = ref(false)

const permissionSet = computed(() => new Set(user.value?.permissions || []))
const sessionPending = computed(() => !sessionStore.authReady && !sessionStore.user)
const canRead = computed(() => permissionSet.value.has('profile.read.owned'))
const canCreateRole = computed(() => permissionSet.value.has('profile.create.owned'))
const canRenameRole = computed(() => permissionSet.value.has('profile.update.owned'))
const canDeleteRole = computed(() => permissionSet.value.has('profile.delete.owned'))
const canClearTexture = computed(() => permissionSet.value.has('texture.clear.owned'))
const canSetAvatar = computed(() => permissionSet.value.has('account.update.self'))
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
    await fetchMe()
  },
})

async function openPreviewDialog(profile: Profile) {
  if (!canRead.value || !(await viewer.allowChange())) return
  selectedProfile.value = profile
  showPreviewDialog.value = true
}

function officialBindingFor(profileId: string) {
  return officialBindings.value.find((binding) => binding.profile_id === profileId) || null
}

function openOfficialBindingDialog() {
  if (!canCreateOfficialBinding.value) return
  showOfficialBindingDialog.value = true
}

function goToIdentities() {
  showOfficialBindingDialog.value = false
  void router.push('/dashboard/identities')
}

async function fetchOfficialResources() {
  const generation = ++officialGeneration
  const canReadBindings = permissionSet.value.has('official_profile.read.owned')
  officialState.value = canReadBindings ? 'loading' : 'forbidden'
  const [identities, bindings] = await Promise.allSettled([
    permissionSet.value.has('external_identity.read.owned')
      ? getExternalIdentities()
      : Promise.resolve(null),
    canReadBindings ? getOfficialProfileBindings() : Promise.resolve(null),
  ])
  if (generation !== officialGeneration) return
  if (identities.status === 'fulfilled')
    externalIdentities.value = identities.value?.data.items || []
  else ElMessage.error('外部身份加载失败')
  if (bindings.status === 'fulfilled') {
    officialBindings.value = bindings.value?.data.items || []
    officialState.value = canReadBindings ? 'ready' : 'forbidden'
  } else {
    officialState.value = 'error'
    ElMessage.error('正版绑定加载失败')
  }
}

async function bindOfficialProfile(payload: { identity_id: string }) {
  if (!canCreateOfficialBinding.value) return
  if (bindingLoading.value) return
  try {
    bindingLoading.value = true
    await createOfficialProfileBinding(payload)
    showOfficialBindingDialog.value = false
    ElMessage.success('正版角色已绑定；可在角色详情中同步正版数据')
    await Promise.all([refreshFirstPage(), fetchOfficialResources()])
    await fetchMe()
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

function updateLocalProfile(target: Profile, patch: Partial<Profile>) {
  Object.assign(target, patch)
  const row = profiles.value.find((item) => item.id === target.id)
  if (row) Object.assign(row, patch)
}

async function syncOfficialBinding(binding: OfficialProfileBinding) {
  const target = selectedProfile.value
  if (
    !canRead.value ||
    !canSyncOfficialBinding.value ||
    !target ||
    binding.profile_id !== target.id ||
    officialState.value !== 'ready'
  )
    return
  await viewer.run(async (current) => {
    await ElMessageBox.confirm(
      `将同步正版角色 ${binding.remote_name} 的名称、模型、皮肤和披风；远端没有的材质会清除，名称冲突时保留本站名称。是否继续？`,
      '同步正版角色',
      {
        type: 'warning',
        confirmButtonText: '开始同步',
        cancelButtonText: '取消',
        closeOnClickModal: false,
        distinguishCancelAndClose: true,
      },
    )
    if (!current() || !canRead.value || !canSyncOfficialBinding.value) return false
    const response = await syncOfficialProfileBinding(binding.id)
    if (!current()) return false
    updateLocalProfile(target, response.data.profile)
    officialBindings.value = officialBindings.value.map((item) =>
      item.id === binding.id ? response.data : item,
    )
    ElMessage.success('正版角色同步完成')
    await refreshAccountAfterWrite()
  }, '同步失败')
}

async function unbindOfficialBinding(binding: OfficialProfileBinding) {
  const target = selectedProfile.value
  if (
    !canRead.value ||
    !canDeleteOfficialBinding.value ||
    !target ||
    binding.profile_id !== target.id ||
    officialState.value !== 'ready'
  )
    return
  await viewer.run(async (current) => {
    await ElMessageBox.confirm('解除绑定不会删除本站角色、皮肤或外部身份。', '解除正版绑定', {
      type: 'warning',
      confirmButtonText: '解除绑定',
      cancelButtonText: '取消',
      closeOnClickModal: false,
      distinguishCancelAndClose: true,
    })
    if (!current() || !canRead.value || !canDeleteOfficialBinding.value) return false
    await deleteOfficialProfileBinding(binding.id)
    if (!current()) return false
    officialBindings.value = officialBindings.value.filter((item) => item.id !== binding.id)
    ElMessage.success('已解除正版绑定')
  }, '解除失败')
}

async function fetchProfiles() {
  if (!canRead.value) return
  listError.value = ''
  const request = ++listRequest
  const actorId = sessionStore.user?.id
  const active = viewer.snapshot()
  const current = () => active() && request === listRequest && actorId === sessionStore.user?.id
  loading.value = true
  try {
    const params = {
      cursor: pagination.currentCursor.value,
      limit: limit.value,
    }
    const res = await getProfiles(params)
    if (!current()) return
    profiles.value = res.data.items
    pagination.setPageData(res.data)
  } catch {
    if (!current()) return
    listError.value = '加载角色失败'
    ElMessage.error('加载角色失败')
  } finally {
    if (request === listRequest) loading.value = false
  }
}

async function handleNextPage() {
  if (!(await viewer.allowChange())) return
  await pagination.goToNextPage(async (cursor, pageLimit) => {
    const params = { cursor, limit: pageLimit }
    const res = await getProfiles(params)
    profiles.value = res.data.items
    return res.data
  })
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function handlePrevPage() {
  if (!(await viewer.allowChange())) return
  await pagination.goToPrevPage(async (cursor, pageLimit) => {
    const params = { cursor, limit: pageLimit }
    const res = await getProfiles(params)
    profiles.value = res.data.items
    return res.data
  })
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function handlePageSizeChange(value: number) {
  if (!(await viewer.allowChange())) return
  if (!pagination.setLimit(value)) return
  await refreshFirstPage()
}

async function refreshFirstPage() {
  pagination.reset()
  await fetchProfiles()
}

async function createRole() {
  if (!canCreateRole.value) return
  if (creatingRole.value) return
  const name = (newRoleName.value || '').trim()
  if (!name) return ElMessage.error('请输入角色名称')
  creatingRole.value = true
  try {
    await createProfile({ name })
    newRoleName.value = ''
    showCreateRoleDialog.value = false
    ElMessage.success('创建成功')
    await refreshFirstPage()
    fetchMe()
  } catch (e: unknown) {
    ElMessage.error('创建失败: ' + getErrorMessage(e, '创建失败'))
  } finally {
    creatingRole.value = false
  }
}

async function refreshAccountAfterWrite() {
  try {
    await fetchMe()
  } catch {
    ElMessage.warning('操作已保存，但账号信息刷新失败')
  }
}

async function deleteRole(pid: string) {
  const target = selectedProfile.value
  if (!canRead.value || !canDeleteRole.value || target?.id !== pid) return
  await viewer.run(async (current) => {
    await ElMessageBox.confirm(
      '确定删除此角色？角色记录将永久删除，衣柜中的材质仍会保留。',
      '删除角色',
      {
        type: 'error',
        confirmButtonText: '确认删除',
        cancelButtonText: '取消',
        closeOnClickModal: false,
        distinguishCancelAndClose: true,
      },
    )
    if (!current() || !canRead.value || !canDeleteRole.value) return false
    await deleteProfile(pid)
    if (!current()) return false
    profiles.value = profiles.value.filter((p) => p.id !== pid)
    officialBindings.value = officialBindings.value.filter((b) => b.profile_id !== pid)
    if (selectedProfile.value?.id === pid) showPreviewDialog.value = false
    ElMessage.success('已删除')
    await refreshAccountAfterWrite()
  }, '删除失败')
}

async function updateRoleName(value: string): Promise<boolean> {
  const target = selectedProfile.value
  const name = value.trim()
  if (!canRead.value || !canRenameRole.value || !target || !name || name === target.name)
    return false
  return viewer.run(
    async (current) => {
      await patchProfile(target.id, { name })
      if (!current()) return false
      updateLocalProfile(target, { name })
      ElMessage.success('名称已修改')
    },
    '修改名称失败',
    true,
  )
}

async function clearRoleTexture(pid: string, type: 'skin' | 'cape') {
  const target = profileForAction(pid)
  const field = type === 'skin' ? 'skin_hash' : 'cape_hash'
  if (!canRead.value || !canClearTexture.value || !target?.[field]) return
  const label = type === 'skin' ? '皮肤' : '披风'
  await viewer.run(async (current) => {
    await ElMessageBox.confirm(
      `确定清除该角色的${label}绑定？此操作只解除绑定，不删除衣柜或皮肤库中的材质。`,
      `清除${label}`,
      {
        type: 'warning',
        confirmButtonText: '确认清除',
        cancelButtonText: '取消',
        closeOnClickModal: false,
        distinguishCancelAndClose: true,
      },
    )
    if (!current() || !canRead.value || !canClearTexture.value) return false
    await (type === 'skin' ? clearProfileSkin(pid) : clearProfileCape(pid))
    if (!current()) return false
    updateLocalProfile(target, { [field]: null })
    ElMessage.success(`${label}已清除`)
    await refreshAccountAfterWrite()
  }, `清除${label}失败`)
}
const clearRoleSkin = (pid: string) => clearRoleTexture(pid, 'skin')
const clearRoleCape = (pid: string) => clearRoleTexture(pid, 'cape')

async function setAsAvatar(profile: Profile) {
  const target = profileForAction(profile.id)
  if (!canRead.value || !canSetAvatar.value || !target?.skin_hash) return
  const hash = target.skin_hash
  const model = target.model === 'slim' ? 'slim' : 'default'
  await viewer.run(async (current) => {
    const updated = await setAvatar(hash, model)
    if (current() && updated) ElMessage.success('已设为头像')
    return updated
  }, '设置头像失败')
}

function profileForAction(id: string) {
  return (
    profiles.value.find((profile) => profile.id === id) ||
    (selectedProfile.value?.id === id ? selectedProfile.value : null)
  )
}

watch(
  [
    () => user.value?.id,
    () => permissionSet.value.has('official_profile.read.owned'),
    () => permissionSet.value.has('external_identity.read.owned'),
  ],
  () => {
    void fetchOfficialResources()
  },
  { immediate: true },
)
watch(
  [() => sessionStore.user?.id, canRead],
  ([id, allowed], previous) => {
    if (previous && (previous[0] !== id || !allowed)) {
      showPreviewDialog.value = false
      selectedProfile.value = null
      profiles.value = []
    }
    if (allowed) void refreshFirstPage()
    else loading.value = false
  },
  { immediate: true },
)
</script>

<style scoped>
.role-header-button {
  flex: 0 1 auto;
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
