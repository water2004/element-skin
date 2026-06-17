<template>
  <div class="w-full mx-auto py-5 animate-fade-in">
    <PageHeader title="角色管理" subtitle="浏览和管理全站所有用户的游戏角色与材质">
      <template #icon><UserFilled /></template>
      <template #actions>
        <el-button
          type="primary"
          :icon="Refresh"
          @click="refreshFromFirst"
          plain
          class="hover-lift"
        >
          刷新列表
        </el-button>
      </template>
    </PageHeader>

    <SearchBar
      v-model="searchQuery"
      placeholder="搜索角色名、邮箱或用户名"
      @clear="handleClearSearch"
      @search="handleSearch"
    />

    <div class="min-h-400" v-loading="loading" element-loading-background="transparent">
      <div class="auto-grid" v-if="profiles.length > 0">
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
        @prev="handlePrevPage"
        @next="handleNextPage"
      />
    </div>

    <AdminRolePreviewDialog
      v-model:visible="showPreview"
      v-model:name="previewName"
      :profile="selectedProfile"
      :textures-url="texturesUrl"
      @rename="updateProfileName"
      @clear-skin="clearProfileSkin"
      @clear-cape="clearProfileCape"
      @delete="confirmDeleteRole"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, inject } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, UserFilled } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import SearchBar from '@/components/common/SearchBar.vue'
import AdminRoleCard from '@/components/admin/roles/AdminRoleCard.vue'
import AdminRolePreviewDialog from '@/components/admin/roles/AdminRolePreviewDialog.vue'
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

const isDark = inject('isDark', ref(false))

const profiles = ref<Profile[]>([])
const limit = 20
const profilesPagination = useCursorPagination<Profile>(limit)
const loading = ref(false)
const searchQuery = ref('')
const activeSearchQuery = ref('')

// Preview dialog
const showPreview = ref(false)
const selectedProfile = ref<Profile | null>(null)
const previewName = ref('')

function texturesUrl(hash: string | null | undefined) {
  if (!hash) return ''
  const base = import.meta.env.BASE_URL
  return `${base}static/textures/${hash}.png`.replace(/\/+/g, '/')
}

function buildSearchParams(extraParams: Record<string, unknown> = {}) {
  const params: Record<string, unknown> = { limit, ...extraParams }
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
  } catch (e) {
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
  } catch (e: any) {
    if (e?.response?.status === 409) {
      ElMessage.error('角色名已存在，请使用其他名称')
    } else {
      ElMessage.error('更新角色名失败')
    }
    previewName.value = selectedProfile.value.name || ''
  }
}

async function clearProfileSkin() {
  const id = selectedProfile.value?.id
  if (!id) return
  try {
    await patchProfileSkin(id, { hash: null })
    ElMessage.success('皮肤绑定已清除')
    await refreshFromFirst()
  } catch (e) {
    ElMessage.error('清除失败')
  }
}

async function clearProfileCape() {
  const id = selectedProfile.value?.id
  if (!id) return
  try {
    await patchProfileCape(id, { hash: null })
    ElMessage.success('披风绑定已清除')
    await refreshFromFirst()
  } catch (e) {
    ElMessage.error('清除失败')
  }
}

async function confirmDeleteRole() {
  if (!selectedProfile.value) return
  try {
    await ElMessageBox.confirm('确定删除此角色？此操作不可撤销。', '确认删除', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
    await deleteAdminProfile(selectedProfile.value.id)
    ElMessage.success('角色已删除')
    showPreview.value = false
    await refreshFromFirst()
  } catch (e) {
    // User cancelled or error
  }
}

onMounted(refreshFromFirst)
</script>
