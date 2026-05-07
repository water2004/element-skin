<template>
  <div class="textures-section animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <div class="page-header-icon"><Picture /></div>
        <div class="page-header-text">
          <h2>材质管理</h2>
          <p class="subtitle">浏览和管理皮肤库中所有上传的材质</p>
        </div>
      </div>
      <div class="page-header-actions">
        <el-button type="primary" :icon="Refresh" @click="refreshTexturesFromFirst" plain class="hover-lift">
          刷新列表
        </el-button>
      </div>
    </div>

    <div class="filter-bar">
      <div class="search-bar-container">
        <el-input
          v-model="searchQuery"
          placeholder="搜索哈希或上传者邮箱"
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
      <div class="type-filter">
        <el-radio-group v-model="typeFilter" @change="handleTypeFilterChange" class="capsule-radio">
          <el-radio-button :value="null">全部</el-radio-button>
          <el-radio-button value="skin">皮肤</el-radio-button>
          <el-radio-button value="cape">披风</el-radio-button>
        </el-radio-group>
      </div>
    </div>

    <el-card class="surface-card" shadow="never">
      <el-table :data="textures" style="width: 100%" class="modern-table" v-loading="loading">
        <el-table-column label="预览" width="80" align="center">
          <template #default="{ row }">
            <img :src="textureUrl(row.hash)" class="preview-thumb" :alt="row.hash" />
          </template>
        </el-table-column>
        <el-table-column label="哈希" min-width="130">
          <template #default="{ row }">
            <span class="mono-text">{{ row.hash.substring(0, 12) }}...</span>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="名称" min-width="120">
          <template #default="{ row }">
            <span>{{ row.name || '未命名' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.type === 'skin' ? '' : 'success'" effect="light" size="small">
              {{ row.type === 'skin' ? '皮肤' : '披风' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="上传者" min-width="140">
          <template #default="{ row }">
            <span>{{ row.uploader_display_name || row.uploader_email || '未知' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="模型" width="80" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.model" size="small" :type="row.model === 'slim' ? 'success' : 'info'">
              {{ row.model }}
            </el-tag>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>
        <el-table-column label="公开" width="80" align="center">
          <template #default="{ row }">
            <el-switch
              :model-value="row.is_public"
              :loading="togglingHash === row.hash"
              @change="togglePublic(row, $event)"
              size="small"
            />
          </template>
        </el-table-column>
        <el-table-column label="上传时间" width="160">
          <template #default="{ row }">
            <span class="timestamp-text">{{ formatDate(row.created_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" align="center">
          <template #default="{ row }">
            <div class="action-btns">
              <el-button
                size="small"
                type="danger"
                plain
                @click="removeTexture(row)"
              >
                移除
              </el-button>
              <el-button
                size="small"
                type="danger"
                @click="forceDeleteTexture(row)"
              >
                强制下架
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && textures.length === 0" description="暂无材质数据" :image-size="80" />

      <div class="pagination-container">
        <CursorPager
          v-if="textures.length > 0"
          :count="textures.length"
          :loading="pagination.isLoading.value"
          :disabled-prev="!pagination.canGoPrev.value"
          :disabled-next="!pagination.canGoNext.value"
          @prev="handlePrevPage"
          @next="handleNextPage"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Picture, Search } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import { useCursorPagination } from '@/composables/useCursorPagination'

const textures = ref([])
const limit = 20
const pagination = useCursorPagination(limit)
const loading = ref(false)
const searchQuery = ref('')
const activeSearchQuery = ref('')
const typeFilter = ref(null)
const togglingHash = ref(null)

const authHeaders = () => ({ Authorization: 'Bearer ' + localStorage.getItem('jwt') })

function textureUrl(hash) {
  return `/static/textures/${hash}.png`
}

function buildSearchParams(extraParams = {}) {
  const params = { limit, ...extraParams }
  if (activeSearchQuery.value) params.q = activeSearchQuery.value
  if (typeFilter.value) params.type = typeFilter.value
  return params
}

async function fetchTextures() {
  loading.value = true
  pagination.isLoading.value = true
  try {
    const res = await axios.get('/admin/textures', {
      headers: authHeaders(),
      params: buildSearchParams({ cursor: pagination.currentCursor.value })
    })
    textures.value = res.data.items
    pagination.setPageData(res.data)
  } catch (e) {
    ElMessage.error('加载材质列表失败')
  } finally {
    loading.value = false
    pagination.isLoading.value = false
  }
}

async function refreshTexturesFromFirst() {
  pagination.reset()
  await fetchTextures()
}

async function handleNextPage() {
  await pagination.goToNextPage(async (cursor, pageLimit) => {
    const res = await axios.get('/admin/textures', {
      headers: authHeaders(),
      params: buildSearchParams({ cursor, limit: pageLimit })
    })
    textures.value = res.data.items
    return res.data
  })
}

async function handlePrevPage() {
  await pagination.goToPrevPage(async (cursor, pageLimit) => {
    const res = await axios.get('/admin/textures', {
      headers: authHeaders(),
      params: buildSearchParams({ cursor, limit: pageLimit })
    })
    textures.value = res.data.items
    return res.data
  })
}

function handleSearch() {
  activeSearchQuery.value = searchQuery.value.trim()
  pagination.reset()
  fetchTextures()
}

function handleClearSearch() {
  searchQuery.value = ''
  activeSearchQuery.value = ''
  pagination.reset()
  fetchTextures()
}

function handleTypeFilterChange() {
  pagination.reset()
  fetchTextures()
}

async function togglePublic(item, newValue) {
  togglingHash.value = item.hash
  const previousValue = item.is_public
  try {
    await axios.patch(`/admin/textures/${item.hash}`,
      { is_public: newValue ? 1 : 0 },
      { headers: authHeaders() }
    )
    item.is_public = newValue
    ElMessage.success(newValue ? '已设为公开' : '已设为私有')
  } catch (e) {
    item.is_public = previousValue
    ElMessage.error('切换公开状态失败')
  } finally {
    togglingHash.value = null
  }
}

async function removeTexture(item) {
  try {
    await ElMessageBox.confirm(
      '确定从该用户移除此材质？',
      '确认移除',
      { type: 'warning' }
    )
    await axios.delete(`/admin/textures/${item.hash}`, {
      headers: authHeaders(),
      params: {
        user_id: item.uploader_user_id,
        type: item.type
      }
    })
    ElMessage.success('材质已移除')
    await fetchTextures()
  } catch (e) {
    // User cancelled or error
  }
}

async function forceDeleteTexture(item) {
  try {
    await ElMessageBox.confirm(
      '危险操作：将从所有用户移除该材质并下架！',
      '极端警告',
      { type: 'error', confirmButtonText: '确认强制下架', cancelButtonText: '取消' }
    )
    await axios.delete(`/admin/textures/${item.hash}`, {
      headers: authHeaders(),
      params: {
        force: 'true',
        type: item.type
      }
    })
    ElMessage.success('材质已强制下架')
    await fetchTextures()
  } catch (e) {
    // User cancelled or error
  }
}

function formatDate(dateStr) {
  if (!dateStr) return '-'
  try {
    const d = new Date(dateStr)
    return d.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit'
    })
  } catch {
    return dateStr
  }
}

onMounted(refreshTexturesFromFirst)
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

.textures-section { max-width: 1200px; margin: 0 auto; padding: 20px 0; }

.filter-bar {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.search-bar-container {
  flex: 1;
  min-width: 260px;
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

.type-filter {
  flex-shrink: 0;
}

.preview-thumb {
  width: 48px;
  height: 48px;
  object-fit: contain;
  border-radius: 4px;
  background: var(--color-background-hero-dark, #1a1a2e);
}

.mono-text {
  font-family: var(--el-font-family-mono);
  font-size: 13px;
  color: var(--color-text);
  background: var(--color-background-soft);
  padding: 2px 6px;
  border-radius: 4px;
}

.timestamp-text {
  font-size: 13px;
  color: var(--color-text-light);
}

.text-muted {
  color: var(--color-text-light);
}

.action-btns {
  display: flex;
  gap: 6px;
  justify-content: center;
}

.modern-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.modern-table :deep(.el-table__row) {
  transition: background-color 0.3s ease;
}
</style>
