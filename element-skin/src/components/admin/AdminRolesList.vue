<template>
  <div class="roles-section animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <div class="page-header-icon"><UserFilled /></div>
        <div class="page-header-text">
          <h2>角色管理</h2>
          <p class="subtitle">浏览和管理全站所有用户的游戏角色与材质</p>
        </div>
      </div>
      <div class="page-header-actions">
        <el-button type="primary" :icon="Refresh" @click="refreshFromFirst" plain class="hover-lift">
          刷新列表
        </el-button>
      </div>
    </div>

    <div class="search-bar-container">
      <el-input
        v-model="searchQuery"
        placeholder="搜索角色名或邮箱"
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

    <el-card class="surface-card" shadow="never">
      <el-table :data="profiles" style="width: 100%" class="modern-table" v-loading="loading">
        <el-table-column label="角色名" min-width="160">
          <template #default="{ row }">
            <span class="profile-name">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column label="所属用户" min-width="200">
          <template #default="{ row }">
            <span>{{ row.owner_display_name || row.owner_email || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="模型" width="100" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="row.texture_model === 'slim' ? 'success' : ''">
              {{ row.texture_model || 'default' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="皮肤预览" width="100" align="center">
          <template #default="{ row }">
            <img
              v-if="row.skin_hash"
              :src="textureUrl(row.skin_hash)"
              class="preview-thumb"
              alt="皮肤预览"
            />
            <span v-else class="no-preview">-</span>
          </template>
        </el-table-column>
        <el-table-column label="披风预览" width="100" align="center">
          <template #default="{ row }">
            <img
              v-if="row.cape_hash"
              :src="textureUrl(row.cape_hash)"
              class="preview-thumb"
              alt="披风预览"
            />
            <span v-else class="no-preview">-</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140" align="center">
          <template #default="{ row }">
            <el-button size="small" @click="openEditDialog(row)">编辑</el-button>
            <el-button size="small" type="danger" @click="deleteProfile(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-container">
        <CursorPager
          v-if="profiles.length > 0"
          :count="profiles.length"
          :loading="profilesPagination.isLoading.value"
          :disabled-prev="!profilesPagination.canGoPrev.value"
          :disabled-next="!profilesPagination.canGoNext.value"
          @prev="handlePrevPage"
          @next="handleNextPage"
        />
      </div>
    </el-card>

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
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, UserFilled, Search } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import { useCursorPagination } from '@/composables/useCursorPagination'

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

const profiles = ref<Profile[]>([])
const limit = 20
const profilesPagination = useCursorPagination<Profile>(limit)
const loading = ref(false)
const searchQuery = ref('')
const activeSearchQuery = ref('')

const editDialogVisible = ref(false)
const editingProfile = ref<Profile | null>(null)
const editForm = ref<EditForm>({ name: '', skin_hash: null, cape_hash: null })
const saving = ref(false)

const clearingSkin = ref(false)
const clearingCape = ref(false)

async function clearBinding(type: 'skin' | 'cape') {
  const id = editingProfile.value?.id
  if (!id) return
  const endpoint = type === 'skin' ? `/admin/profiles/${id}/skin` : `/admin/profiles/${id}/cape`

  try {
    if (type === 'skin') clearingSkin.value = true
    else clearingCape.value = true

    await axios.patch(endpoint, { hash: null }, { headers: authHeaders() })
    ElMessage.success(`${type === 'skin' ? '皮肤' : '披风'}绑定已清除`)
    await refreshFromFirst()
  } catch (e) {
    ElMessage.error('清除失败')
  } finally {
    clearingSkin.value = false
    clearingCape.value = false
  }
}

const authHeaders = () => ({ Authorization: 'Bearer ' + localStorage.getItem('jwt') })

const textureUrl = (hash: string | null | undefined) => (hash ? `/static/textures/${hash}.png` : '')

function buildSearchParams(extraParams: Record<string, unknown> = {}) {
  const params: Record<string, unknown> = { limit, ...extraParams }
  if (activeSearchQuery.value) params.q = activeSearchQuery.value
  return params
}

async function refreshProfiles() {
  loading.value = true
  profilesPagination.isLoading.value = true
  try {
    const res = await axios.get('/admin/profiles', {
      headers: authHeaders(),
      params: buildSearchParams({ cursor: profilesPagination.currentCursor.value }),
    })
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
    const res = await axios.get('/admin/profiles', {
      headers: authHeaders(),
      params: buildSearchParams({ cursor, limit: pageLimit }),
    })
    profiles.value = res.data.items
    return res.data
  })
}

async function handlePrevPage() {
  await profilesPagination.goToPrevPage(async (cursor, pageLimit) => {
    const res = await axios.get('/admin/profiles', {
      headers: authHeaders(),
      params: buildSearchParams({ cursor, limit: pageLimit }),
    })
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
    await axios.patch(
      `/admin/profiles/${editingProfile.value.id}`,
      {
        name: editForm.value.name.trim(),
      },
      { headers: authHeaders() },
    )
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
    await axios.delete(`/admin/profiles/${profile.id}`, { headers: authHeaders() })
    ElMessage.success('角色已删除')
    await refreshFromFirst()
  } catch (e) {
    // User cancelled or error — ElMessageBox.confirm rejection is normal cancellation
  }
}

onMounted(refreshFromFirst)
</script>

<style>
@import "@/assets/styles/dialogs.css";
</style>

<style scoped>
@import "@/assets/styles/animations.css";
@import "@/assets/styles/layout.css";
@import "@/assets/styles/cards.css";
@import "@/assets/styles/tags.css";
@import "@/assets/styles/buttons.css";
@import "@/assets/styles/headers.css";

.roles-section {
  max-width: 1100px;
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

.profile-name {
  font-weight: 600;
  color: var(--color-heading);
}

.preview-thumb {
  width: 48px;
  height: 48px;
  object-fit: contain;
  border-radius: 4px;
  background: var(--color-background-hero-dark, #1a1a2e);
}

.no-preview {
  color: var(--color-text-light);
  font-size: 13px;
}

.modern-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.modern-table :deep(.el-table__row) {
  transition: background-color 0.3s ease;
}

@media (max-width: 768px) {
  .roles-section {
    padding: 10px 0;
  }
}

.binding-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
.binding-input {
  flex: 1;
}
</style>
