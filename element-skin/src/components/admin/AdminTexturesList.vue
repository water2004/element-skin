<template>
  <div class="textures-section animate-fade-in">
    <PageHeader title="材质管理" subtitle="浏览和管理皮肤库中所有上传的材质">
      <template #icon><Picture /></template>
      <template #actions>
        <el-button type="primary" :icon="Refresh" @click="refreshTexturesFromFirst" plain class="hover-lift">
          刷新列表
        </el-button>
      </template>
    </PageHeader>

    <div class="filter-bar">
      <div class="search-bar-container">
        <el-input
          v-model="searchQuery"
          placeholder="搜索哈希、材质名或上传者"
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

    <!-- Skeleton loading -->
    <div v-if="isLoading" class="auto-grid">
      <div v-for="n in 8" :key="n" class="surface-card skeleton-card">
        <div class="skeleton-preview"></div>
        <div class="skeleton-info">
          <div class="skeleton-line"></div>
          <div class="skeleton-line short"></div>
        </div>
      </div>
    </div>

    <!-- Card grid -->
    <div v-else-if="textures.length > 0" class="auto-grid">
      <div
        class="surface-card hoverable animate-card-slide clickable-card"
        v-for="(item, index) in textures"
        :key="item.hash"
        @click="openPreview(item)"
        :style="{ '--delay-index': index % limit }"
      >
        <div class="texture-preview" :style="{ background: isDark ? 'var(--color-background-hero-dark)' : 'var(--color-background-hero-light)' }">
          <SkinViewer
            v-if="item.type === 'skin'"
            :skinUrl="texturesUrl(item.hash)"
            :model="item.model || 'default'"
            :width="200"
            :height="280"
            is-static
          />
          <CapeViewer
            v-else
            :capeUrl="texturesUrl(item.hash)"
            :width="200"
            :height="280"
            is-static
          />
        </div>
        <div class="texture-info">
          <div class="type-tag" :class="item.type">{{ item.type === 'skin' ? '皮肤' : '披风' }}</div>
          <div class="texture-title">{{ item.name || '未命名' }}</div>
          <div class="texture-uploader" v-if="item.uploader_display_name || item.uploader_email">
            {{ item.uploader_display_name || item.uploader_email }}
          </div>
        </div>
        <div class="texture-actions" @click.stop>
          <el-button class="btn-gradient btn-gradient-primary" @click="openPreview(item)"><el-icon><Edit /></el-icon><span>编辑</span></el-button>
        </div>
      </div>
    </div>

    <el-empty v-else-if="!loading" description="暂无材质数据" :image-size="80" />

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

    <!-- Preview dialog -->
    <el-dialog v-model="showPreview" destroy-on-close class="dialog-viewer" append-to-body>
      <div class="viewer-layout" v-if="selectedItem">
        <div class="viewer-stage">
          <SkinViewer
            v-if="selectedItem.type === 'skin'"
            :skinUrl="texturesUrl(selectedItem.hash)"
            :model="selectedItem.model || 'default'"
            :width="320"
            :height="430"
          />
          <CapeViewer
            v-else
            :capeUrl="texturesUrl(selectedItem.hash)"
            :width="320"
            :height="430"
          />
        </div>
        <div class="viewer-info-panel">
          <!-- name (editable) -->
          <section class="viewer-section title-section">
            <el-input v-model="previewNote" placeholder="未命名纹理" @blur="updatePreviewNote" />
          </section>
          <!-- hash (readonly) -->
          <section class="viewer-section meta-section">
            <span class="meta-chip hash">{{ selectedItem.hash }}</span>
          </section>
          <!-- uploader -->
          <section class="viewer-section" v-if="selectedItem.uploader_display_name || selectedItem.uploader_email">
            <div class="viewer-section-label">上传者</div>
            <div class="uploader-info">
              <span>{{ selectedItem.uploader_display_name || selectedItem.uploader_email || '未知' }}</span>
            </div>
          </section>
          <!-- model (skin only) -->
          <section class="viewer-section" v-if="selectedItem.type === 'skin'">
            <div class="viewer-section-label">模型选择</div>
            <el-radio-group :model-value="selectedItem.model" @change="updateModel" class="capsule-radio">
              <el-radio-button value="default">Default</el-radio-button>
              <el-radio-button value="slim">Slim</el-radio-button>
            </el-radio-group>
          </section>
          <!-- public toggle -->
          <section class="viewer-section">
            <div class="viewer-section-label">公开状态</div>
            <el-switch
              :model-value="selectedItem.is_public"
              @change="updateIsPublic"
            />
          </section>
          <!-- force delete -->
          <section class="viewer-section footer-section">
            <el-button type="danger" plain @click="confirmForceDelete">强制下架</el-button>
          </section>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, inject, onMounted, type Ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Picture, Search, Edit } from '@element-plus/icons-vue'
import SkinViewer from '@/components/SkinViewer.vue'
import CapeViewer from '@/components/CapeViewer.vue'
import CursorPager from '@/components/common/CursorPager.vue'
import { useCursorPagination } from '@/composables/useCursorPagination'
import { getAdminTextures, patchAdminTexture, deleteAdminTexture } from '@/api/admin/textures'
import type { Texture } from '@/api/types'
import PageHeader from '@/components/common/PageHeader.vue'

type TextureQueryParams = { cursor?: string | null; limit?: number; q?: string; type?: string }

const isDark = inject<Ref<boolean>>('isDark', ref(false))

const textures = ref<Texture[]>([])
const limit = 20
const pagination = useCursorPagination<Texture>(limit)
const loading = ref(false)
const isLoading = ref(true)
const searchQuery = ref('')
const activeSearchQuery = ref('')
const typeFilter = ref<string | null>(null)

// Preview dialog
const showPreview = ref(false)
const selectedItem = ref<Texture | null>(null)
const previewNote = ref('')

function texturesUrl(hash: string | null | undefined) {
  if (!hash) return ''
  const base = import.meta.env.BASE_URL
  return `${base}static/textures/${hash}.png`.replace(/\/+/g, '/')
}

function buildSearchParams(extraParams: TextureQueryParams = {}): TextureQueryParams {
  const params: TextureQueryParams = { limit, ...extraParams }
  if (activeSearchQuery.value) params.q = activeSearchQuery.value
  if (typeFilter.value) params.type = typeFilter.value
  return params
}

async function fetchTextures() {
  loading.value = true
  pagination.isLoading.value = true
  try {
    const res = await getAdminTextures(buildSearchParams({ cursor: pagination.currentCursor.value }))
    textures.value = res.data.items
    pagination.setPageData(res.data)
  } catch (e) {
    ElMessage.error('加载材质列表失败')
  } finally {
    loading.value = false
    pagination.isLoading.value = false
    isLoading.value = false
  }
}

async function refreshTexturesFromFirst() {
  pagination.reset()
  await fetchTextures()
}

async function handleNextPage() {
  await pagination.goToNextPage(async (cursor, pageLimit) => {
    const res = await getAdminTextures(buildSearchParams({ cursor, limit: pageLimit }))
    textures.value = res.data.items
    return res.data
  })
}

async function handlePrevPage() {
  await pagination.goToPrevPage(async (cursor, pageLimit) => {
    const res = await getAdminTextures(buildSearchParams({ cursor, limit: pageLimit }))
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

// Preview dialog functions
function openPreview(item: Texture) {
  selectedItem.value = item
  previewNote.value = item.name || ''
  showPreview.value = true
}

async function updatePreviewNote() {
  if (!selectedItem.value) return
  const newName = previewNote.value.trim()
  if (newName === selectedItem.value.name) return
  try {
    await patchAdminTexture(selectedItem.value.hash, { type: selectedItem.value.type, note: newName })
    selectedItem.value.name = newName
    ElMessage.success('名称已更新')
  } catch (e) {
    ElMessage.error('更新名称失败')
  }
}

async function updateModel(newModel: string | number | boolean | undefined) {
  if (!selectedItem.value) return
  try {
    await patchAdminTexture(selectedItem.value.hash, { type: selectedItem.value.type, model: String(newModel) })
    selectedItem.value.model = String(newModel)
    ElMessage.success('模型已更新')
  } catch (e) {
    ElMessage.error('更新模型失败')
  }
}

async function updateIsPublic(newValue: string | number | boolean) {
  if (!selectedItem.value) return
  const item = selectedItem.value

  // Guard: ignore if unchanged (== handles true==1, false==0)
  if (item.is_public == newValue) return

  // Confirmation only when turning private
  if (!newValue) {
    try {
      await ElMessageBox.confirm(
        '取消公开后，该材质将不会出现在公共皮肤库中，已绑定此材质的角色不受影响。确定取消公开？',
        '确认操作',
        { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
      )
    } catch {
      return
    }
  }

  try {
    await patchAdminTexture(item.hash, { type: item.type, is_public: Boolean(newValue) })
    item.is_public = Boolean(newValue)
    ElMessage.success(newValue ? '材质已公开' : '已取消公开')
  } catch (e) {
    ElMessage.error('操作失败')
  }
}

async function confirmForceDelete() {
  if (!selectedItem.value) return
  const item = selectedItem.value
  try {
    await ElMessageBox.confirm(
      '强制下架将从所有用户的衣柜中移除该材质，并从皮肤库中彻底删除。此操作不可撤销！确定继续？',
      '危险操作',
      { confirmButtonText: '确认强制删除', cancelButtonText: '取消', type: 'error' }
    )
    await deleteAdminTexture(item.hash, { force: true, type: item.type })
    ElMessage.success('材质已强制下架')
    showPreview.value = false
    await fetchTextures()
  } catch (e) {
    // User cancelled or error
  }
}

onMounted(refreshTexturesFromFirst)
</script>

<style scoped>
.textures-section {
  /* max-width: 1500px; */
  width: 100%;
  margin: 0 auto;
  padding: 20px 0;
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
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

/* Card grid */
.texture-preview {
  width: 100%;
  height: 280px;
  display: flex;
  justify-content: center;
  align-items: center;
  position: relative;
  overflow: hidden;
}

.texture-info {
  padding: 12px 16px;
  text-align: center;
  background: var(--color-card-background);
}

.texture-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-heading);
  margin: 8px 0 4px 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.texture-uploader {
  font-size: 12px;
  color: var(--color-text-light);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.texture-actions {
  display: flex;
  align-items: center;
  flex-direction: row;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid var(--color-border);
  background: var(--color-background-soft);
}

.texture-actions .el-button {
  flex: 1;
  min-width: 0;
  /* font-size: 12px; */
}

.clickable-card {
  cursor: pointer;
}

/* Skeleton loading */
.skeleton-card { pointer-events: none; }
.skeleton-preview { height: 280px; background: var(--el-skeleton-color, #e0e0e0); border-radius: 8px 8px 0 0; }
.skeleton-info { padding: 12px; }
.skeleton-line { height: 14px; background: var(--el-skeleton-color, #e0e0e0); border-radius: 4px; margin-bottom: 8px; }
.skeleton-line.short { width: 60%; }

/* Preview dialog uploader */
.uploader-info {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  color: var(--color-heading);
  font-weight: 500;
}

/* Footer section pushed to bottom */
.viewer-section.footer-section {
  border-bottom: none;
}
</style>
