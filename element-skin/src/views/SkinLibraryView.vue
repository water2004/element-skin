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
        <ActionBar full align="center">
          <SearchBar
            v-model="searchQuery"
            placeholder="搜索哈希、材质名或上传者"
            @clear="handleClearSearch"
            @search="handleSearch"
          />
          <el-radio-group
            v-model="filterType"
            @change="handleFilterChange"
            size="large"
            class="capsule-radio"
          >
            <el-radio-button value="">全部</el-radio-button>
            <el-radio-button value="skin">皮肤</el-radio-button>
            <el-radio-button value="cape">披风</el-radio-button>
          </el-radio-group>
          <el-select v-model="sortBy" @change="handleSortChange" size="large" class="sort-select">
            <el-option label="最新上传" value="latest" />
            <el-option label="最多使用" value="most_used" />
          </el-select>
        </ActionBar>
      </div>

      <div
        class="library-grid-container"
        v-loading="loading"
        element-loading-background="transparent"
      >
        <div class="auto-grid" v-if="items.length > 0">
          <TextureCard
            v-for="(item, index) in items"
            :key="item.hash"
            :texture="item"
            :delay-index="index % 20"
            :is-dark="isDark"
            :textures-url="texturesUrl"
            :resolution="textureResolutions.get(item.hash)"
            :title="item.name || '未命名材质'"
            @preview="openPreviewDialog"
          >
            <template #info="{ texture }">
              <div class="texture-title">{{ item.name || '未命名材质' }}</div>
              <div class="texture-meta-info">
                <span class="uploader-name" v-if="texture.uploader_name">
                  <el-icon><User /></el-icon>
                  {{ texture.uploader_name }}
                </span>
                <span class="meta-separator" v-if="texture.uploader_name">·</span>
                <span class="texture-date">
                  {{ formatDate(texture.created_at) }}
                </span>
                <span class="meta-separator">·</span>
                <span class="texture-usage">{{ texture.usage_count || 0 }} 次使用</span>
              </div>
            </template>
            <template #actions="{ texture }">
              <el-button
                class="btn-gradient btn-gradient-primary"
                @click="addToWardrobe(texture)"
                :disabled="!isLogged"
              >
                <el-icon><Plus /></el-icon>
                <span>添加到衣柜</span>
              </el-button>
            </template>
          </TextureCard>
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
            <TexturePreviewStage :texture="selectedItem" :textures-url="texturesUrl" />

            <div class="viewer-info-panel">
              <section class="viewer-section title-section">
                <div class="viewer-title-row">
                  <h2 class="viewer-display-title">{{ selectedItem.name || '未命名纹理' }}</h2>
                </div>
              </section>

              <section class="viewer-section meta-section">
                <div class="viewer-title-row">
                  <span class="meta-chip"
                    >{{ textureResolutions.get(selectedItem.hash) || '--' }}px</span
                  >
                  <span class="meta-chip" :class="selectedItem.type">
                    {{ selectedItem.type === 'skin' ? '皮肤' : '披风' }}
                  </span>
                </div>
                <div class="hash-label">HASH: {{ selectedItem.hash }}</div>
              </section>

              <section class="viewer-section" v-if="selectedItem.uploader_name">
                <div class="viewer-section-label">上传者</div>
                <div class="flex items-center gap-2 text-15 text-heading font-medium">
                  <el-icon><User /></el-icon>
                  <span>{{ selectedItem.uploader_name }}</span>
                </div>
              </section>

              <section class="viewer-section mt-auto border-b-0">
                <el-button
                  type="primary"
                  size="large"
                  class="btn-gradient btn-gradient-primary w-full rounded-2xl h-12"
                  @click="addToWardrobe(selectedItem)"
                  :disabled="!isLogged"
                >
                  <el-icon><Plus /></el-icon>
                  <span class="ml-2">添加到我的衣柜</span>
                </el-button>
                <p v-if="!isLogged" class="text-center text-13 text-info mt-3">
                  登录后即可收藏此纹理
                </p>
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
import { Plus, User } from '@element-plus/icons-vue'
import ActionBar from '@/components/common/ActionBar.vue'
import CursorPager from '@/components/common/CursorPager.vue'
import SearchBar from '@/components/common/SearchBar.vue'
import TextureCard from '@/components/textures/TextureCard.vue'
import TexturePreviewStage from '@/components/textures/TexturePreviewStage.vue'
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

    items.value.forEach((item) => {
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
  items.value.forEach((item) => {
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
  items.value.forEach((item) => {
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

.skin-library-container .page-header {
  align-items: flex-start;
  flex-direction: column;
}

.skin-library-container .page-header-content {
  width: 100%;
}

.search-bar-container {
  flex: 0 1 560px;
  min-width: 320px;
}

.sort-select {
  flex: 0 0 180px;
}

.skin-library-container .capsule-radio {
  flex: 0 1 auto;
  min-width: 0;
}

@media (max-width: 900px) {
  .search-bar-container {
    flex: 1 1 420px;
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
