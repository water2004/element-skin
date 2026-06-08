<template>
  <div class="skin-library-container animate-fade-in">
    <div v-if="isDisabled" class="disabled-container">
      <el-empty description="皮肤库已关闭">
        <template #extra>
          <el-button type="primary" @click="$router.push('/')">返回首页</el-button>
        </template>
      </el-empty>
    </div>
    <template v-else>
      <div class="page-header">
        <div class="page-header-content">
          <div>
            <h1>皮肤库</h1>
            <p>探索并收藏精美材质</p>
          </div>
        </div>
        <div class="page-header-actions">
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
          <el-radio-group v-model="filterType" @change="handleFilterChange" size="large" class="capsule-radio">
            <el-radio-button value="">全部</el-radio-button>
            <el-radio-button value="skin">皮肤</el-radio-button>
            <el-radio-button value="cape">披风</el-radio-button>
          </el-radio-group>
          <el-select v-model="sortBy" @change="handleSortChange" size="large" class="sort-select">
            <el-option label="最新上传" value="latest" />
            <el-option label="最多使用" value="most_used" />
          </el-select>
        </div>
      </div>

    <div class="library-grid-container" v-loading="loading" element-loading-background="transparent">
      <div class="auto-grid" v-if="items.length > 0">
        <div 
          class="surface-card hoverable animate-card-slide clickable-card" 
          v-for="(item, index) in items" 
          :key="item.hash"
          :style="{ '--delay-index': index % 20 }"
          @click="openPreviewDialog(item)"
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
            <div
              v-if="item.type === 'skin' && textureResolutions.get(item.hash)"
              class="floating-badge"
              :style="getResolutionBadgeStyle(textureResolutions.get(item.hash))"
            >
              {{ textureResolutions.get(item.hash) }}x
            </div>
          </div>
          <div class="texture-info">
            <div class="texture-title">{{ item.name || '未命名材质' }}</div>
            <div class="texture-meta-info">
              <span class="uploader-name" v-if="item.uploader_name">
                <el-icon><User /></el-icon>
                {{ item.uploader_name }}
              </span>
              <span class="meta-separator" v-if="item.uploader_name">·</span>
              <span class="texture-date">
                {{ formatDate(item.created_at) }}
              </span>
              <span class="meta-separator">·</span>
              <span class="texture-usage">{{ item.usage_count || 0 }} 次使用</span>
            </div>
          </div>
          <div class="texture-actions" @click.stop>
            <el-button 
              class="btn-gradient btn-gradient-primary" 
              @click="addToWardrobe(item)"
              :disabled="!isLogged"
            >
              <el-icon><Plus /></el-icon>
              <span>添加到衣柜</span>
            </el-button>
          </div>
        </div>
      </div>
      
      <el-empty v-else-if="!loading" description="库中暂无公开材质" />

      <!-- 预览对话框 -->
      <el-dialog
        v-model="showPreviewDialog"
        destroy-on-close
        class="dialog-viewer"
        append-to-body
      >
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
            <section class="viewer-section title-section">
              <div class="viewer-title-row">
                <h2 class="viewer-display-title">{{ selectedItem.name || '未命名纹理' }}</h2>
              </div>
            </section>

            <section class="viewer-section meta-section">
              <div class="viewer-title-row">
                <span class="meta-chip">{{ textureResolutions.get(selectedItem.hash) || '--' }}px</span>
                <span class="meta-chip" :class="selectedItem.type">
                  {{ selectedItem.type === 'skin' ? '皮肤' : '披风' }}
                </span>
              </div>
              <div class="hash-label">HASH: {{ selectedItem.hash }}</div>
            </section>

            <section class="viewer-section" v-if="selectedItem.uploader_name">
              <div class="viewer-section-label">上传者</div>
              <div class="uploader-info">
                <el-icon><User /></el-icon>
                <span>{{ selectedItem.uploader_name }}</span>
              </div>
            </section>

            <section class="viewer-section footer-section" style="margin-top: auto;">
              <el-button 
                type="primary" 
                size="large" 
                class="btn-gradient btn-gradient-primary" 
                style="width: 100%; border-radius: 12px; height: 50px;"
                @click="addToWardrobe(selectedItem)"
                :disabled="!isLogged"
              >
                <el-icon><Plus /></el-icon>
                <span style="margin-left: 8px;">添加到我的衣柜</span>
              </el-button>
              <p v-if="!isLogged" class="login-hint">登录后即可收藏此纹理</p>
            </section>
          </div>
        </div>
      </el-dialog>

      <div class="pagination-container">
        <CursorPager
          v-if="items.length > 0"
          :count="items.length"
          :loading="pagination.isLoading.value"
          :disabled-prev="!pagination.canGoPrev.value"
          :disabled-next="!pagination.canGoNext.value"
          @prev="handlePrevPage"
          @next="handleNextPage"
        />
      </div>
    </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, inject, computed, type Ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus, Search, User } from '@element-plus/icons-vue'
import SkinViewer from '@/components/SkinViewer.vue'
import CapeViewer from '@/components/CapeViewer.vue'
import CursorPager from '@/components/common/CursorPager.vue'
import { useCursorPagination } from '@/composables/useCursorPagination'
import { getPublicSkinLibrary } from '@/api/public'
import { addToWardrobe as apiAddToWardrobe } from '@/api/textures'
import type { Texture, User as UserType } from '@/api/types'

const isDark = inject<Ref<boolean>>('isDark', ref(false))
const user = inject<Ref<UserType | null>>('user', ref(null))
const isLogged = computed(() => !!user.value)

const items = ref<Texture[]>([])
const limit = 20
const pagination = useCursorPagination<Texture>(limit)
const loading = ref(false)
const isDisabled = ref(false)
const filterType = ref('')
const sortBy = ref<'latest' | 'most_used'>('latest')
const searchQuery = ref('')
const activeSearchQuery = ref('')
const textureResolutions = ref(new Map<string, number>())
const showPreviewDialog = ref(false)
const selectedItem = ref<Texture | null>(null)

function openPreviewDialog(item: Texture) {
  selectedItem.value = item
  showPreviewDialog.value = true
}

function texturesUrl(hash: string | null | undefined) {
  if (!hash) return ''
  const base = import.meta.env.BASE_URL
  return `${base}static/textures/${hash}.png`.replace(/\/+/g, '/')
}

function formatDate(ts: number | undefined) {
  if (!ts) return ''
  const date = new Date(ts)
  return date.toLocaleDateString()
}

async function fetchLibrary() {
  loading.value = true
  pagination.isLoading.value = true
  try {
    const params = {
      cursor: pagination.currentCursor.value,
      limit: limit,
      texture_type: filterType.value || undefined,
      q: activeSearchQuery.value || undefined,
      sort: sortBy.value,
    }
    const res = await getPublicSkinLibrary(params)
    items.value = res.data.items
    pagination.setPageData(res.data)

    items.value.forEach(item => {
      if (item.type === 'skin') {
        loadTextureResolution(item.hash)
      }
    })
  } catch (e: any) {
    console.error('Fetch library error:', e)
    if (e.response?.status === 403) {
      isDisabled.value = true
    } else {
      ElMessage.error('加载皮肤库失败')
    }
  } finally {
    loading.value = false
    pagination.isLoading.value = false
  }
}

function loadTextureResolution(hash: string) {
  if (textureResolutions.value.has(hash)) return
  const img = new Image()
  img.crossOrigin = 'anonymous'
  img.onload = () => {
    textureResolutions.value.set(hash, img.width)
  }
  img.src = texturesUrl(hash)
}

function getResolutionBadgeStyle(resolution: number | undefined) {
  if (!resolution) return {}
  let hue = 0
  if (resolution <= 64) hue = 120
  else if (resolution <= 128) hue = 120 - ((resolution - 64) / 64) * 60
  else if (resolution <= 256) hue = 60 - ((resolution - 128) / 128) * 30
  else if (resolution <= 512) hue = 30 - ((resolution - 256) / 256) * 30
  else hue = 330

  return {
    background: `linear-gradient(135deg, hsl(${hue}, 58%, 65%), hsl(${hue + 15}, 53%, 62%))`,
    boxShadow: `0 2px 6px hsla(${hue}, 58%, 50%, 0.25)`
  }
}

async function handleNextPage() {
  await pagination.goToNextPage(async (cursor, pageLimit) => {
    const params = {
      cursor,
      limit: pageLimit,
      texture_type: filterType.value || undefined,
      q: activeSearchQuery.value || undefined,
      sort: sortBy.value,
    }
    const res = await getPublicSkinLibrary(params)
    items.value = res.data.items
    return res.data
  })
  items.value.forEach(item => {
    if (item.type === 'skin') {
      loadTextureResolution(item.hash)
    }
  })
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function handlePrevPage() {
  await pagination.goToPrevPage(async (cursor, pageLimit) => {
    const params = {
      cursor,
      limit: pageLimit,
      texture_type: filterType.value || undefined,
      q: activeSearchQuery.value || undefined,
      sort: sortBy.value,
    }
    const res = await getPublicSkinLibrary(params)
    items.value = res.data.items
    return res.data
  })
  items.value.forEach(item => {
    if (item.type === 'skin') {
      loadTextureResolution(item.hash)
    }
  })
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function handleFilterChange() {
  pagination.reset()
  await fetchLibrary()
}

async function handleSortChange() {
  pagination.reset()
  await fetchLibrary()
}

function handleSearch() {
  activeSearchQuery.value = searchQuery.value.trim()
  pagination.reset()
  fetchLibrary()
}

function handleClearSearch() {
  searchQuery.value = ''
  activeSearchQuery.value = ''
  pagination.reset()
  fetchLibrary()
}

async function addToWardrobe(item: Texture) {
  try {
    await apiAddToWardrobe(item.hash, item.type)
    ElMessage.success('已成功添加到我的衣柜')
  } catch (e: any) {
    ElMessage.error('添加失败: ' + (e.response?.data?.detail || e.message))
  }
}

onMounted(() => {
  fetchLibrary()
})
</script>

<style scoped>
.skin-library-container {
  margin: 0 0;
  padding: 0;
}

.disabled-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 60vh;
}

.library-grid-container {
  min-height: 400px;
}

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
  margin-bottom: 6px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.texture-meta-info {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: var(--color-text-light);
}

.meta-separator {
  opacity: 0.5;
}

.uploader-name {
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 3px;
  max-width: 100px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.texture-date {
  font-size: 12px;
}

.texture-actions {
  display: flex;
  padding: 12px 16px;
  border-top: 1px solid var(--color-border);
  background: var(--color-background-soft);
}

.texture-actions .el-button {
  flex: 1;
}

.clickable-card {
  cursor: pointer;
}

.skin-library-container .page-header {
  align-items: flex-start;
  flex-direction: column;
}

.skin-library-container .page-header-content {
  width: 100%;
}

.skin-library-container .page-header-actions {
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-start;
  min-width: 0;
  max-width: 100%;
  width: 100%;
}

.search-bar-container {
  flex: 1 1 420px;
  max-width: 640px;
  min-width: 320px;
}

.sort-select {
  flex: 0 0 180px;
}

.skin-library-container .capsule-radio {
  flex: 0 1 auto;
  min-width: 0;
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
}

@media (max-width: 900px) {
  .search-bar-container {
    max-width: none;
  }
}

@media (max-width: 640px) {
  .search-bar-container {
    flex-basis: 100%;
    min-width: 0;
  }

  .skin-library-container .capsule-radio {
    flex: 1 1 260px;
  }

  .skin-library-container .capsule-radio :deep(.el-radio-button) {
    flex: 1 1 0;
  }

  .skin-library-container .capsule-radio :deep(.el-radio-button__inner) {
    width: 100%;
  }

  .sort-select {
    flex: 1 1 150px;
    min-width: 150px;
  }
}
</style>
