<template>
  <div class="skin-library-container">
    <div v-if="isDisabled" class="disabled-container animate-fade-in">
      <el-empty description="皮肤库已关闭">
        <template #extra>
          <el-button type="primary" @click="$router.push('/')">返回首页</el-button>
        </template>
      </el-empty>
    </div>
    <template v-else>
      <div class="page-header animate-fade-in">
        <div class="page-header-content">
          <div>
            <h1>皮肤库</h1>
            <p>探索并收藏精美材质</p>
          </div>
        </div>
        <ActionBar full align="center">
          <SearchBar
            v-model="searchQuery"
            :loading="loading"
            placeholder="搜索哈希、材质名或上传者"
            @clear="handleClearSearch"
            @search="handleSearch"
          />
          <UiSegmented
            v-model="filterType"
            @change="handleFilterChange"
            size="large"
            class="skin-library-filter"
          >
            <el-radio-button value="">全部</el-radio-button>
            <el-radio-button value="skin">皮肤</el-radio-button>
            <el-radio-button value="cape">披风</el-radio-button>
          </UiSegmented>
          <el-select v-model="sortBy" @change="handleSortChange" size="large" class="sort-select">
            <el-option label="最新上传" value="latest" />
            <el-option label="最多使用" value="most_used" />
          </el-select>
        </ActionBar>
      </div>

      <div
        class="library-grid-container"
        v-loading="loading || sessionPending"
        element-loading-background="transparent"
      >
        <div
          class="grid grid-cols-[repeat(auto-fill,240px)] justify-center gap-6"
          :style="{ '--motion-card-count': items.length }"
          v-if="items.length > 0"
        >
          <TextureCard
            v-for="(item, index) in items"
            :key="item.hash + item.type"
            :texture="item"
            :delay-index="index % 20"
            :textures-url="texturesUrl"
            :title="item.name || '未命名材质'"
            @preview="openPreviewDialog"
          >
            <template #info="{ texture }">
              <div
                class="flex w-full min-w-0 items-center justify-center gap-2 text-[var(--color-text-light)]"
              >
                <ViewerIdentity :name="texture.uploader_name" />
                <TextureModelTag :type="texture.type" :model="texture.model" />
                <TextureFavorites :count="texture.usage_count" />
              </div>
            </template>
            <template #actions="{ texture }">
              <UiButton
                v-if="canAddToWardrobe"
                variant="primary"
                :icon="Plus"
                :disabled="viewer.busy.value"
                @click.stop="addToWardrobe(texture)"
              >
                添加到衣柜
              </UiButton>
            </template>
          </TextureCard>
        </div>

        <el-empty
          v-else-if="!loading && !sessionPending"
          :description="listError || '库中暂无公开材质'"
        />

        <UiDialog
          v-model="showPreviewDialog"
          destroy-on-close
          variant="viewer"
          :busy="viewer.busy.value"
          title="材质详情"
        >
          <UiViewerLayout v-if="selectedItem">
            <template #stage
              ><TexturePreviewStage :texture="selectedItem" :textures-url="texturesUrl"
            /></template>
            <h2 class="m-0 break-all pr-12 text-2xl font-bold text-[var(--color-heading)]">
              {{ selectedItem.name || '未命名材质' }}
            </h2>
            <section
              class="flex flex-col items-start gap-4 border-b border-[var(--color-border)] py-5"
            >
              <div class="flex flex-wrap gap-2">
                <TextureResolutionTag :hash="selectedItem.hash" /><TextureModelTag
                  :type="selectedItem.type"
                  :model="selectedItem.model"
                />
              </div>
              <div class="flex flex-wrap items-center gap-3">
                <span class="text-sm text-[var(--color-text-light)]">上传者</span
                ><ViewerIdentity :name="selectedItem.uploader_name" />
              </div>
              <span class="text-sm text-[var(--color-text-light)]"
                >上传日期：{{ formatDate(selectedItem.created_at) || '未知' }}</span
              >
              <TextureFavorites :count="selectedItem.usage_count" />
            </section>
            <ActionBar v-if="canAddToWardrobe" align="start" class="py-5">
              <UiButton
                variant="primary"
                :icon="Plus"
                :disabled="viewer.busy.value"
                @click="addToWardrobe(selectedItem)"
                >添加到衣柜</UiButton
              >
            </ActionBar>
          </UiViewerLayout>
        </UiDialog>

        <div class="pagination-container">
          <CursorPager
            v-if="items.length > 0"
            :count="items.length"
            :loading="pagination.isLoading.value"
            :disabled-prev="!pagination.canGoPrev.value"
            :disabled-next="!pagination.canGoNext.value"
            :page-size="pagination.limit.value"
            @prev="handlePrevPage"
            @next="handleNextPage"
            @page-size-change="handlePageSizeChange"
          />
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import ActionBar from '@/components/common/ActionBar.vue'
import CursorPager from '@/components/common/CursorPager.vue'
import SearchBar from '@/components/common/SearchBar.vue'
import TextureCard from '@/components/textures/TextureCard.vue'
import TexturePreviewStage from '@/components/textures/TexturePreviewStage.vue'
import { textureAssetUrl as texturesUrl } from '@/components/textures/textureAssets'
import UiButton from '@/components/ui/UiButton.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiSegmented from '@/components/ui/UiSegmented.vue'
import UiViewerLayout from '@/components/ui/UiViewerLayout.vue'
import ViewerIdentity from '@/components/common/ViewerIdentity.vue'
import TextureModelTag from '@/components/textures/TextureModelTag.vue'
import TextureFavorites from '@/components/textures/TextureFavorites.vue'
import TextureResolutionTag from '@/components/textures/TextureResolutionTag.vue'
import { useViewerSession } from '@/composables/useViewerSession'
import { useCursorPagination } from '@/composables/useCursorPagination'
import { getPublicSkinLibrary } from '@/api/public'
import { addToWardrobe as apiAddToWardrobe } from '@/api/textures'
import type { Texture } from '@/api/types'
import { useSessionStore } from '@/stores/session'
import { storeToRefs } from 'pinia'
import { getErrorMessage, getApiError } from '@/utils/error'

const session = useSessionStore()
const { user } = storeToRefs(session)
const sessionPending = computed(() => !session.authReady && !user.value)
const canAddToWardrobe = computed(() =>
  user.value?.permissions?.includes('wardrobe_entry.add.owned'),
)

const items = ref<Texture[]>([])
const pagination = useCursorPagination<Texture>(20)
const limit = pagination.limit
const loading = ref(false)
const listError = ref('')
let listRequest = 0
const isDisabled = ref(false)
const filterType = ref('')
const sortBy = ref<'latest' | 'most_used'>('latest')
const searchQuery = ref('')
const activeSearchQuery = ref('')
const showPreviewDialog = ref(false)
const selectedItem = ref<Texture | null>(null)

const viewer = useViewerSession(
  showPreviewDialog,
  computed(() => (selectedItem.value ? selectedItem.value.hash + selectedItem.value.type : '')),
)

async function openPreviewDialog(item: Texture) {
  if (!(await viewer.allowChange())) return
  selectedItem.value = item
  showPreviewDialog.value = true
}

function formatDate(ts: number | undefined) {
  if (!ts) return ''
  const date = new Date(ts)
  return date.toLocaleDateString()
}

async function fetchLibrary() {
  listError.value = ''
  const request = ++listRequest
  const actorId = session.user?.id
  const active = viewer.snapshot()
  const current = () => active() && request === listRequest && actorId === session.user?.id
  loading.value = true
  pagination.isLoading.value = true
  try {
    const params = {
      cursor: pagination.currentCursor.value,
      limit: limit.value,
      texture_type: filterType.value || undefined,
      q: activeSearchQuery.value || undefined,
      sort: sortBy.value,
    }
    const res = await getPublicSkinLibrary(params)
    if (!current()) return
    items.value = res.data.items
    pagination.setPageData(res.data)
  } catch (e: unknown) {
    if (!current()) return
    console.error('Fetch library error:', e)
    const descriptor = getApiError(e)
    if (descriptor?.object === 'texture_library' && descriptor.reason === 'disabled') {
      isDisabled.value = true
    } else {
      listError.value = getErrorMessage(e, '加载皮肤库失败')
      ElMessage.error(listError.value)
    }
  } finally {
    if (request === listRequest) loading.value = false
    if (request === listRequest) pagination.isLoading.value = false
  }
}

async function handleNextPage() {
  if (!(await viewer.allowChange())) return
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
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function handlePrevPage() {
  if (!(await viewer.allowChange())) return
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
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function handleFilterChange() {
  if (!(await viewer.allowChange())) return
  pagination.reset()
  await fetchLibrary()
}

async function handleSortChange() {
  if (!(await viewer.allowChange())) return
  pagination.reset()
  await fetchLibrary()
}

async function handleSearch() {
  if (!(await viewer.allowChange())) return
  activeSearchQuery.value = searchQuery.value.trim()
  pagination.reset()
  fetchLibrary()
}

async function handleClearSearch() {
  if (!(await viewer.allowChange())) return
  searchQuery.value = ''
  activeSearchQuery.value = ''
  pagination.reset()
  fetchLibrary()
}

async function handlePageSizeChange(value: number) {
  if (!(await viewer.allowChange())) return
  if (!pagination.setLimit(value)) return
  await fetchLibrary()
}

async function addToWardrobe(item: Texture) {
  if (!canAddToWardrobe.value) return
  await viewer.run(async (current) => {
    await apiAddToWardrobe(item.hash, item.type)
    if (current()) ElMessage.success('已加入我的衣柜')
  }, '添加失败')
}

watch(
  [() => user.value?.id, () => session.authReady],
  () => {
    if (!sessionPending.value) void fetchLibrary()
  },
  { immediate: true },
)
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

.skin-library-filter {
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

  .skin-library-filter {
    flex: 1 1 260px;
  }

  .skin-library-filter :deep(.el-radio-button) {
    flex: 1 1 0;
  }

  .skin-library-filter :deep(.el-radio-button__inner) {
    width: 100%;
  }

  .sort-select {
    flex: 1 1 150px;
    min-width: 150px;
  }
}
</style>
