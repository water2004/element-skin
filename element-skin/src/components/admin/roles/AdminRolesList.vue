<template>
  <div class="w-full mx-auto py-5">
    <div class="animate-fade-in">
      <PageHeader title="角色管理" subtitle="浏览和管理全站所有用户的游戏角色与材质">
        <template #icon><UserFilled /></template>
        <template #actions>
          <UiButton
            loading-mode="icon"
            :icon="Refresh"
            :loading="loading"
            @click="refreshFromFirst"
          >
            刷新
          </UiButton>
        </template>
      </PageHeader>

      <SearchBar
        v-model="searchQuery"
        :loading="loading"
        placeholder="搜索角色名、邮箱或用户名"
        class="mb-6"
        @clear="handleClearSearch"
        @search="handleSearch"
      />
    </div>

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
          admin
          :textures-url="texturesUrl"
          @preview="openPreview"
        />
      </div>

      <el-empty
        v-else-if="!loading && !sessionPending"
        :description="listError || (canRead ? '暂无角色数据' : '无权读取角色列表')"
        :image-size="80"
      />
    </div>

    <div class="pagination-container" v-if="profiles.length > 0">
      <CursorPager
        :count="profiles.length"
        :loading="profilesPagination.isLoading.value"
        :disabled-prev="!profilesPagination.canGoPrev.value"
        :disabled-next="!profilesPagination.canGoNext.value"
        :page-size="profilesPagination.limit.value"
        @prev="handlePrevPage"
        @next="handleNextPage"
        @page-size-change="handlePageSizeChange"
      />
    </div>

    <AdminRolePreviewDialog
      v-model:visible="showPreview"
      v-model:dirty="viewer.dirty.value"
      :busy="viewer.busy.value"
      :revision="viewer.revision.value"
      :save-name="updateProfileName"
      :profile="selectedProfile"
      :textures-url="texturesUrl"
      :can-update="canUpdate"
      :can-delete="canDelete"
      @clear-skin="clearProfileTexture('skin')"
      @clear-cape="clearProfileTexture('cape')"
      @delete="confirmDeleteRole"
    />
  </div>
</template>

<script setup lang="ts">
import UiButton from '@/components/ui/UiButton.vue'
import { computed, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, UserFilled } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import SearchBar from '@/components/common/SearchBar.vue'
import ProfileCard from '@/components/common/ProfileCard.vue'
import AdminRolePreviewDialog from '@/components/admin/roles/AdminRolePreviewDialog.vue'
import { textureAssetUrl as texturesUrl } from '@/components/textures/textureAssets'
import { useCursorPagination } from '@/composables/useCursorPagination'
import {
  getAdminProfiles,
  patchAdminProfile,
  deleteAdminProfile,
  patchProfileSkin,
  patchProfileCape,
} from '@/api/admin/profiles'
import PageHeader from '@/components/common/PageHeader.vue'
import type { Profile } from '@/api/types'
import { useViewerSession } from '@/composables/useViewerSession'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const sessionPending = computed(() => !session.authReady && !session.user)
const canRead = computed(() => session.user?.permissions?.includes('profile.read.any') ?? false)
const canUpdate = computed(() => session.user?.permissions?.includes('profile.update.any') ?? false)
const canDelete = computed(() => session.user?.permissions?.includes('profile.delete.any') ?? false)

const profiles = ref<Profile[]>([])
const profilesPagination = useCursorPagination<Profile>(20)
const limit = profilesPagination.limit
const loading = ref(false)
const listError = ref('')
let listRequest = 0
const searchQuery = ref('')
const activeSearchQuery = ref('')

// Preview dialog
const showPreview = ref(false)
const selectedProfile = ref<Profile | null>(null)
const viewer = useViewerSession(
  showPreview,
  computed(() => selectedProfile.value?.id || ''),
)

function buildSearchParams(extraParams: Record<string, unknown> = {}) {
  const params: Record<string, unknown> = { limit: limit.value, ...extraParams }
  if (activeSearchQuery.value) params.q = activeSearchQuery.value
  return params
}

async function refreshProfiles() {
  if (!canRead.value) {
    loading.value = false
    return
  }
  listError.value = ''
  const request = ++listRequest
  const actorId = session.user?.id
  const active = viewer.snapshot()
  const current = () => active() && request === listRequest && actorId === session.user?.id
  loading.value = true
  profilesPagination.isLoading.value = true
  try {
    const res = await getAdminProfiles(
      buildSearchParams({ cursor: profilesPagination.currentCursor.value }),
    )
    if (!current()) return
    profiles.value = res.data.items
    profilesPagination.setPageData(res.data)
  } catch {
    if (!current()) return
    listError.value = '加载角色列表失败'
    ElMessage.error('加载角色列表失败')
  } finally {
    if (request === listRequest) loading.value = false
    if (request === listRequest) profilesPagination.isLoading.value = false
  }
}

async function refreshFromFirst() {
  if (!(await viewer.allowChange())) return
  profilesPagination.reset()
  await refreshProfiles()
}

async function handleNextPage() {
  if (!(await viewer.allowChange())) return
  await profilesPagination.goToNextPage(async (cursor, pageLimit) => {
    const res = await getAdminProfiles(buildSearchParams({ cursor, limit: pageLimit }))
    profiles.value = res.data.items
    return res.data
  })
}

async function handlePrevPage() {
  if (!(await viewer.allowChange())) return
  await profilesPagination.goToPrevPage(async (cursor, pageLimit) => {
    const res = await getAdminProfiles(buildSearchParams({ cursor, limit: pageLimit }))
    profiles.value = res.data.items
    return res.data
  })
}

async function handlePageSizeChange(value: number) {
  if (!(await viewer.allowChange())) return
  if (!profilesPagination.setLimit(value)) return
  await refreshFromFirst()
}

async function handleSearch() {
  if (!(await viewer.allowChange())) return
  activeSearchQuery.value = searchQuery.value.trim()
  profilesPagination.reset()
  refreshProfiles()
}

async function handleClearSearch() {
  if (!(await viewer.allowChange())) return
  searchQuery.value = ''
  activeSearchQuery.value = ''
  profilesPagination.reset()
  refreshProfiles()
}

// ---- Preview dialog functions ----

async function openPreview(profile: Profile) {
  if (!canRead.value || !(await viewer.allowChange())) return
  selectedProfile.value = profile
  showPreview.value = true
}

function updateLocalProfile(target: Profile, patch: Partial<Profile>) {
  Object.assign(target, patch)
  const row = profiles.value.find((item) => item.id === target.id)
  if (row) Object.assign(row, patch)
}

async function updateProfileName(value: string): Promise<boolean> {
  const target = selectedProfile.value
  const name = value.trim()
  if (!canUpdate.value || !canRead.value || !target || !name || name === target.name) return false
  return viewer.run(
    async (current) => {
      await patchAdminProfile(target.id, { name })
      if (!current()) return false
      updateLocalProfile(target, { name })
      ElMessage.success('角色名称已更新')
    },
    '更新角色名失败',
    true,
  )
}

async function clearProfileTexture(type: 'skin' | 'cape') {
  const target = selectedProfile.value
  const field = type === 'skin' ? 'skin_hash' : 'cape_hash'
  if (!canRead.value || !canUpdate.value || !target?.[field]) return
  const label = type === 'skin' ? '皮肤' : '披风'
  await viewer.run(async (current) => {
    await ElMessageBox.confirm(
      `确定清除角色「${target.name}」的${label}绑定吗？此操作只解除绑定，不删除材质。`,
      `清除${label}`,
      {
        type: 'warning',
        confirmButtonText: `清除${label}`,
        cancelButtonText: '取消',
        closeOnClickModal: false,
        distinguishCancelAndClose: true,
      },
    )
    if (!current() || !canUpdate.value || !canRead.value) return false
    const patch = type === 'skin' ? patchProfileSkin : patchProfileCape
    await patch(target.id, { hash: null })
    if (!current()) return false
    updateLocalProfile(target, { [field]: null })
    ElMessage.success(`${label}绑定已清除`)
  }, `清除${label}绑定失败`)
}

async function confirmDeleteRole() {
  const target = selectedProfile.value
  if (!target || !canDelete.value || !canRead.value) return
  await viewer.run(async (current) => {
    await ElMessageBox.confirm(
      `确定删除角色「${target.name}」？角色记录将永久删除，该用户衣柜中的材质仍会保留。`,
      '删除角色',
      {
        type: 'warning',
        confirmButtonText: '删除',
        cancelButtonText: '取消',
        closeOnClickModal: false,
        distinguishCancelAndClose: true,
      },
    )
    if (!current() || !canDelete.value || !canRead.value) return false
    await deleteAdminProfile(target.id)
    if (!current()) return false
    profiles.value = profiles.value.filter((row) => row.id !== target.id)
    showPreview.value = false
    ElMessage.success('角色已删除')
  }, '删除角色失败')
}

watch(
  [() => session.user?.id, canRead],
  ([id, allowed], previous) => {
    if (previous && (previous[0] !== id || !allowed)) {
      showPreview.value = false
      selectedProfile.value = null
      profiles.value = []
    }
    if (allowed) void refreshFromFirst()
    else loading.value = false
  },
  { immediate: true },
)
</script>
