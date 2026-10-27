<template>
  <div class="w-full mx-auto py-5">
    <div class="animate-fade-in">
      <PageHeader title="角色管理" subtitle="浏览和管理全站所有用户的游戏角色与材质">
        <template #icon><UserFilled /></template>
        <template #actions>
          <el-button :icon="Refresh" plain :loading="loading" @click="refreshFromFirst">
            刷新
          </el-button>
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

    <div class="min-h-[400px]" v-loading="loading" element-loading-background="transparent">
      <div
        class="grid grid-cols-[repeat(auto-fill,240px)] justify-center gap-6"
        :style="{ '--motion-card-count': profiles.length }"
        v-if="profiles.length > 0"
      >
        <AdminRoleCard
          v-for="(profile, index) in profiles"
          :key="profile.id"
          :profile="profile"
          :delay-index="index % limit"
          :is-dark="isDark"
          :textures-url="texturesUrl"
          @preview="openPreview"
        />
      </div>

      <el-empty v-else-if="!loading" description="暂无角色数据" :image-size="80" />
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
      v-model:name="previewName"
      :profile="selectedProfile"
      :textures-url="texturesUrl"
      :clearing-texture="clearingTexture"
      :can-update="canUpdate"
      :can-delete="canDelete"
      @rename="updateProfileName"
      @clear-skin="clearProfileTexture('skin')"
      @clear-cape="clearProfileTexture('cape')"
      @delete="confirmDeleteRole"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, inject } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, UserFilled } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import SearchBar from '@/components/common/SearchBar.vue'
import AdminRoleCard from '@/components/admin/roles/AdminRoleCard.vue'
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
import { getErrorStatus } from '@/utils/error'
import { useSessionStore } from '@/stores/session'

const isDark = inject('isDark', ref(false))
const session = useSessionStore()
const canUpdate = computed(() => session.user?.permissions?.includes('profile.update.any') ?? false)
const canDelete = computed(() => session.user?.permissions?.includes('profile.delete.any') ?? false)

const profiles = ref<Profile[]>([])
const profilesPagination = useCursorPagination<Profile>(20)
const limit = profilesPagination.limit
const loading = ref(false)
const searchQuery = ref('')
const activeSearchQuery = ref('')

// Preview dialog
const showPreview = ref(false)
const selectedProfile = ref<Profile | null>(null)
const previewName = ref('')
const clearingTexture = ref<'skin' | 'cape' | null>(null)

function buildSearchParams(extraParams: Record<string, unknown> = {}) {
  const params: Record<string, unknown> = { limit: limit.value, ...extraParams }
  if (activeSearchQuery.value) params.q = activeSearchQuery.value
  return params
}

async function refreshProfiles() {
  loading.value = true
  profilesPagination.isLoading.value = true
  try {
    const res = await getAdminProfiles(
      buildSearchParams({ cursor: profilesPagination.currentCursor.value }),
    )
    profiles.value = res.data.items
    profilesPagination.setPageData(res.data)
  } catch {
    ElMessage.error('加载角色列表失败')
  } finally {
    loading.value = false
    profilesPagination.isLoading.value = false
  }
}

async function refreshFromFirst() {
  profilesPagination.reset()
  await refreshProfiles()
}

async function handleNextPage() {
  await profilesPagination.goToNextPage(async (cursor, pageLimit) => {
    const res = await getAdminProfiles(buildSearchParams({ cursor, limit: pageLimit }))
    profiles.value = res.data.items
    return res.data
  })
}

async function handlePrevPage() {
  await profilesPagination.goToPrevPage(async (cursor, pageLimit) => {
    const res = await getAdminProfiles(buildSearchParams({ cursor, limit: pageLimit }))
    profiles.value = res.data.items
    return res.data
  })
}

async function handlePageSizeChange(value: number) {
  if (!profilesPagination.setLimit(value)) return
  await refreshFromFirst()
}

function handleSearch() {
  activeSearchQuery.value = searchQuery.value.trim()
  profilesPagination.reset()
  refreshProfiles()
}

function handleClearSearch() {
  searchQuery.value = ''
  activeSearchQuery.value = ''
  profilesPagination.reset()
  refreshProfiles()
}

// ---- Preview dialog functions ----

function openPreview(profile: Profile) {
  selectedProfile.value = profile
  previewName.value = profile.name || ''
  showPreview.value = true
}

async function updateProfileName() {
  if (!canUpdate.value) return
  if (!selectedProfile.value) return
  const newName = previewName.value.trim()
  if (!newName) {
    ElMessage.error('角色名不能为空')
    previewName.value = selectedProfile.value.name || ''
    return
  }
  if (newName === selectedProfile.value.name) return

  try {
    await patchAdminProfile(selectedProfile.value.id, { name: newName })
    selectedProfile.value.name = newName
    ElMessage.success('角色名称已更新')
    await refreshFromFirst()
  } catch (e: unknown) {
    if (getErrorStatus(e) === 409) {
      ElMessage.error('角色名已存在，请使用其他名称')
    } else {
      ElMessage.error('更新角色名失败')
    }
    previewName.value = selectedProfile.value.name || ''
  }
}

async function clearProfileTexture(type: 'skin' | 'cape') {
  if (!canUpdate.value) return
  const target = selectedProfile.value
  const field = type === 'skin' ? 'skin_hash' : 'cape_hash'
  if (!target?.[field] || clearingTexture.value) return
  const id = target.id
  const label = type === 'skin' ? '皮肤' : '披风'
  clearingTexture.value = type
  try {
    try {
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
    } catch {
      return
    }
    try {
      if (!canUpdate.value) return
      const patch = type === 'skin' ? patchProfileSkin : patchProfileCape
      await patch(id, { hash: null })
      if (selectedProfile.value?.id === id) selectedProfile.value[field] = null
      const row = profiles.value.find((profile) => profile.id === id)
      if (row) row[field] = null
      ElMessage.success(`${label}绑定已清除`)
      await refreshFromFirst()
    } catch {
      ElMessage.error(`清除${label}绑定失败`)
    }
  } finally {
    clearingTexture.value = null
  }
}

async function confirmDeleteRole() {
  if (!selectedProfile.value || !canDelete.value) return
  const target = selectedProfile.value
  try {
    await ElMessageBox.confirm(
      `确定删除角色「${target.name}」？角色记录将永久删除，该用户衣柜中的材质仍会保留。`,
      '删除角色',
      {
        type: 'warning',
        confirmButtonText: '删除',
        cancelButtonText: '取消',
      },
    )
    if (!canDelete.value) return
    await deleteAdminProfile(target.id)
    ElMessage.success('角色已删除')
    showPreview.value = false
    await refreshFromFirst()
  } catch {
    // User cancelled or error
  }
}

onMounted(refreshFromFirst)
</script>
