<template>
  <div class="roles-section animate-fade-in">
    <PageHeader title="角色管理" subtitle="浏览和管理全站所有用户的游戏角色与材质">
      <template #icon><UserFilled /></template>
      <template #actions>
        <el-button type="primary" :icon="Refresh" @click="refreshFromFirst" plain class="hover-lift">
          刷新列表
        </el-button>
      </template>
    </PageHeader>

    <div class="search-bar-container">
      <el-input
        v-model="searchQuery"
        placeholder="搜索角色名、邮箱或用户名"
        clearable
        @clear="handleClearSearch"
        @keyup.enter="handleSearch"
        size="large"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
        <template #append>
          <el-button :icon="Search" @click="handleSearch">搜索</el-button>
        </template>
      </el-input>
    </div>

    <div class="roles-grid-container" v-loading="loading" element-loading-background="transparent">
      <div class="auto-grid" v-if="profiles.length > 0">
        <div
        class="surface-card hoverable animate-card-slide clickable-card"
        v-for="(profile, index) in profiles"
        :key="profile.id"
        @click="openPreview(profile)"
        :style="{ '--delay-index': index % limit }"
      >
          <div
            class="role-preview"
            :style="{ background: isDark ? 'var(--color-background-hero-dark)' : 'var(--color-background-hero-light)' }"
          >
            <SkinViewer
              v-if="profile.skin_hash"
              :skinUrl="texturesUrl(profile.skin_hash)"
              :capeUrl="profile.cape_hash ? texturesUrl(profile.cape_hash) : null"
              :model="profile.texture_model || 'default'"
              :width="200"
              :height="280"
              is-static
            />
            <el-empty v-else description="未设置皮肤" :image-size="120" />
          </div>
          <div class="role-info">
            <div class="role-name">{{ profile.name }}</div>
            <div class="role-owner">所属: {{ profile.owner_display_name || profile.owner_email || '-' }}</div>
            <div class="role-model">模型: {{ profile.texture_model || 'default' }}</div>
          </div>
          <div class="role-actions" @click.stop>
            <el-button class="btn-gradient btn-gradient-primary" @click="openPreview(profile)"><el-icon><Edit /></el-icon><span>编辑</span></el-button>
          </div>
        </div>
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

    <!-- Preview Dialog -->
    <el-dialog
      v-model="showPreview"
      destroy-on-close
      class="dialog-viewer"
      append-to-body
    >
      <div class="viewer-layout" v-if="selectedProfile">
        <div class="viewer-stage">
          <SkinViewer
            v-if="selectedProfile.skin_hash"
            :skinUrl="texturesUrl(selectedProfile.skin_hash)"
            :capeUrl="selectedProfile.cape_hash ? texturesUrl(selectedProfile.cape_hash) : null"
            :model="selectedProfile.texture_model || 'default'"
            :width="320"
            :height="430"
          />
          <el-empty v-else description="未设置皮肤" />
        </div>
        <div class="viewer-info-panel">
          <section class="viewer-section title-section">
            <el-input
              v-model="previewName"
              @blur="updateProfileName"
              placeholder="角色名称"
            />
          </section>
          <section class="viewer-section">
            <div class="viewer-section-label">皮肤绑定</div>
            <el-input :model-value="selectedProfile.skin_hash || '未绑定'" disabled>
              <template #append>
                <el-button :disabled="!selectedProfile.skin_hash" @click="clearProfileSkin">清除</el-button>
              </template>
            </el-input>
          </section>
          <section class="viewer-section">
            <div class="viewer-section-label">披风绑定</div>
            <el-input :model-value="selectedProfile.cape_hash || '未绑定'" disabled>
              <template #append>
                <el-button :disabled="!selectedProfile.cape_hash" @click="clearProfileCape">清除</el-button>
              </template>
            </el-input>
          </section>
          <section class="viewer-section footer-section">
            <el-button type="danger" plain @click="confirmDeleteRole">删除角色</el-button>
          </section>
        </div>
      </div>
    </el-dialog>

    <!-- Edit Dialog -->
    <el-dialog
      v-model="editDialogVisible"
      title="编辑角色"
      class="dialog-form"
      destroy-on-close
      align-center
      append-to-body
    >
      <el-form label-position="top">
        <el-form-item label="角色名">
          <el-input
            v-model="editForm.name"
            maxlength="16"
            show-word-limit
            placeholder="输入角色名"
          />
        </el-form-item>
        <el-form-item label="皮肤绑定">
          <div class="binding-row">
            <el-input :model-value="editForm.skin_hash || '未绑定'" disabled class="binding-input" />
            <el-button
              size="small"
              :disabled="!editForm.skin_hash"
              :loading="clearingSkin"
              @click="clearBinding('skin')"
            >
              清除
            </el-button>
          </div>
        </el-form-item>
        <el-form-item label="披风绑定">
          <div class="binding-row">
            <el-input :model-value="editForm.cape_hash || '未绑定'" disabled class="binding-input" />
            <el-button
              size="small"
              :disabled="!editForm.cape_hash"
              :loading="clearingCape"
              @click="clearBinding('cape')"
            >
              清除
            </el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveProfile" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, inject } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, UserFilled, Search, Edit } from '@element-plus/icons-vue'
import SkinViewer from '@/components/SkinViewer.vue'
import CursorPager from '@/components/common/CursorPager.vue'
import { useCursorPagination } from '@/composables/useCursorPagination'
import { getAdminProfiles, patchAdminProfile, deleteAdminProfile, patchProfileSkin, patchProfileCape } from '@/api/admin/profiles'
import PageHeader from '@/components/common/PageHeader.vue'

interface Profile {
  id: string
  name: string
  owner_email?: string
  owner_display_name?: string
  texture_model?: string
  skin_hash?: string | null
  cape_hash?: string | null
}

interface EditForm {
  name: string
  skin_hash: string | null
  cape_hash: string | null
}

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

// Edit dialog
const editDialogVisible = ref(false)
const editingProfile = ref<Profile | null>(null)
const editForm = ref<EditForm>({ name: '', skin_hash: null, cape_hash: null })
const saving = ref(false)

const clearingSkin = ref(false)
const clearingCape = ref(false)

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
    const res = await getAdminProfiles(buildSearchParams({ cursor: profilesPagination.currentCursor.value }))
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

// ---- Card grid delete ----

function confirmDelete(profile: Profile) {
  deleteProfile(profile)
}

// ---- Existing functions (unchanged) ----

async function clearBinding(type: 'skin' | 'cape') {
  const id = editingProfile.value?.id
  if (!id) return
  const endpoint = type === 'skin' ? `/admin/profiles/${id}/skin` : `/admin/profiles/${id}/cape`

  try {
    if (type === 'skin') clearingSkin.value = true
    else clearingCape.value = true

    if (type === 'skin') await patchProfileSkin(id, { hash: null })
    else await patchProfileCape(id, { hash: null })
    ElMessage.success(`${type === 'skin' ? '皮肤' : '披风'}绑定已清除`)
    await refreshFromFirst()
  } catch (e) {
    ElMessage.error('清除失败')
  } finally {
    clearingSkin.value = false
    clearingCape.value = false
  }
}

function openEditDialog(profile: Profile) {
  editingProfile.value = profile
  editForm.value = {
    name: profile.name || '',
    skin_hash: profile.skin_hash ?? null,
    cape_hash: profile.cape_hash ?? null,
  }
  editDialogVisible.value = true
}

async function saveProfile() {
  if (!editingProfile.value) return
  if (!editForm.value.name.trim()) {
    ElMessage.error('角色名不能为空')
    return
  }
  saving.value = true
  try {
    await patchAdminProfile(editingProfile.value.id, { name: editForm.value.name.trim() })
    ElMessage.success('角色已更新')
    editDialogVisible.value = false
    await refreshFromFirst()
  } catch (e: any) {
    if (e?.response?.status === 409) {
      ElMessage.error('角色名已存在，请使用其他名称')
    } else {
      ElMessage.error('更新角色失败')
    }
  } finally {
    saving.value = false
  }
}

async function deleteProfile(profile: Profile) {
  try {
    await ElMessageBox.confirm('确定删除此角色？此操作不可撤销。', '确认删除', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
    await deleteAdminProfile(profile.id)
    ElMessage.success('角色已删除')
    await refreshFromFirst()
  } catch (e) {
    // User cancelled or error — ElMessageBox.confirm rejection is normal cancellation
  }
}

onMounted(refreshFromFirst)
</script>

<style scoped>
.roles-section {
  /* max-width: 1500px; */
  width: 100%;
  margin: 0 auto;
  padding: 20px 0;
}

.search-bar-container {
  margin-bottom: 16px;
  display: flex;
}

.search-bar-container :deep(.el-input-group) {
  display: flex;
  align-items: stretch;
}

.search-bar-container :deep(.el-input-group__append) {
  background: var(--el-color-primary);
  color: #fff;
  border-color: var(--el-color-primary);
  cursor: pointer;
  padding: 0 20px;
  display: flex;
  align-items: center;
  transition: all 0.3s ease;
  border-top-left-radius: 0;
  border-bottom-left-radius: 0;
}

.search-bar-container :deep(.el-input-group__append:hover) {
  background: var(--el-color-primary-light-3);
  border-color: var(--el-color-primary-light-3);
  opacity: 0.9;
}

.search-bar-container :deep(.el-input-group__append .el-button) {
  border: none;
  background: transparent;
  color: inherit;
  padding: 0;
  margin: 0;
  height: 100%;
}

.roles-grid-container {
  min-height: 400px;
}

.role-preview {
  width: 100%;
  height: 280px;
  display: flex;
  justify-content: center;
  align-items: center;
}

.role-info {
  padding: 16px;
  text-align: center;
  background: var(--color-card-background);
}

.role-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-heading);
  margin-bottom: 4px;
}

.role-owner {
  font-size: 13px;
  color: var(--color-text-light);
  margin-bottom: 4px;
}

.role-model {
  font-size: 13px;
  color: var(--color-text-light);
  font-weight: 500;
}

.role-actions {
  display: flex;
  flex-direction: row;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid var(--color-border);
  background: var(--color-background-soft);
  align-items: center;
}

.role-actions .el-button {
  flex: 1;
  min-width: 0;
}

.clickable-card {
  cursor: pointer;
}

.binding-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.binding-input {
  flex: 1;
}

@media (max-width: 768px) {
  .roles-section {
    padding: 10px 0;
  }
}
</style>
