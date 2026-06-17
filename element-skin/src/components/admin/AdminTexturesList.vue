<template>
  <div class="w-full mx-auto py-5 animate-fade-in">
    <PageHeader title="材质管理" subtitle="浏览和管理皮肤库中所有上传的材质">
      <template #icon><Picture /></template>
      <template #actions>
        <el-button
          type="primary"
          :icon="Refresh"
          @click="refreshTexturesFromFirst"
          plain
          class="hover-lift"
        >
          刷新列表
        </el-button>
      </template>
    </PageHeader>

    <div class="flex items-center gap-4 mb-6 flex-wrap">
      <SearchBar
        class="flex-1 min-w-[260px]"
        v-model="searchQuery"
        placeholder="搜索哈希、材质名或上传者"
        @clear="handleClearSearch"
        @search="handleSearch"
      />
      <div class="shrink-0">
        <UiSegmented v-model="typeFilter" @change="handleTypeFilterChange">
          <el-radio-button :value="null">全部</el-radio-button>
          <el-radio-button value="skin">皮肤</el-radio-button>
          <el-radio-button value="cape">披风</el-radio-button>
        </UiSegmented>
      </div>
    </div>

    <!-- Skeleton loading -->
    <div v-if="isLoading" class="grid grid-cols-[repeat(auto-fill,240px)] justify-center gap-6">
      <div v-for="n in 8" :key="n" class="skeleton-card">
        <div class="skeleton-preview"></div>
        <div class="skeleton-info">
          <div class="skeleton-line"></div>
          <div class="skeleton-line short"></div>
        </div>
      </div>
    </div>

    <!-- Card grid -->
    <div v-else-if="textures.length > 0" class="grid grid-cols-[repeat(auto-fill,240px)] justify-center gap-6">
      <TextureCard
        v-for="(item, index) in textures"
        :key="item.hash"
        :texture="item"
        :delay-index="index % limit"
        :is-dark="isDark"
        :textures-url="texturesUrl"
        :title="item.name || '未命名'"
        show-type
        @preview="openPreview"
      >
        <template #info="{ texture }">
          <div
            class="inline-flex rounded-lg px-2.5 py-1 text-xs font-bold uppercase leading-none tracking-[0.5px]"
            :class="
              texture.type === 'skin'
                ? 'bg-[rgba(64,158,255,0.1)] text-[#409eff]'
                : 'bg-[rgba(103,194,58,0.1)] text-[#67c23a]'
            "
          >
            {{ texture.type === 'skin' ? '皮肤' : '披风' }}
          </div>
          <div
            class="max-w-full overflow-hidden text-ellipsis whitespace-nowrap text-[15px] font-semibold text-[var(--color-heading)]"
          >
            {{ texture.name || '未命名' }}
          </div>
          <div
            class="texture-uploader"
            v-if="texture.uploader_display_name || texture.uploader_email"
          >
            {{ texture.uploader_display_name || texture.uploader_email }}
          </div>
        </template>
        <template #actions="{ texture }">
          <UiButton variant="gradient-primary" @click="openPreview(texture)"
            ><el-icon><Edit /></el-icon><span>编辑</span></UiButton
          >
        </template>
      </TextureCard>
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
    <UiDialog v-model="showPreview" destroy-on-close variant="viewer">
      <UiViewerLayout v-if="selectedItem">
        <template #stage>
          <TexturePreviewStage :texture="selectedItem" :textures-url="texturesUrl" />
        </template>
        <div class="flex min-h-0 flex-1 flex-col">
          <!-- name (editable) -->
          <section class="border-b border-[var(--color-border)] py-3.5">
            <el-input v-model="previewNote" placeholder="未命名纹理" @blur="updatePreviewNote" />
          </section>
          <!-- hash (readonly) -->
          <section class="border-b border-[var(--color-border)] py-3.5">
            <span
              class="inline-flex h-7 max-w-60 items-center overflow-hidden text-ellipsis whitespace-nowrap rounded-full border border-[var(--color-border)] bg-[var(--color-background-soft)] px-3 font-mono text-xs text-[var(--color-text)] transition"
            >
              {{ selectedItem.hash }}
            </span>
          </section>
          <!-- uploader -->
          <section
            class="border-b border-[var(--color-border)] py-3.5"
            v-if="selectedItem.uploader_display_name || selectedItem.uploader_email"
          >
            <div
              class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
            >
              上传者
            </div>
            <div
              class="flex items-center gap-2 text-[15px] text-[var(--color-heading)] font-medium"
            >
              <span>{{
                selectedItem.uploader_display_name || selectedItem.uploader_email || '未知'
              }}</span>
            </div>
          </section>
          <!-- model (skin only) -->
          <section
            class="border-b border-[var(--color-border)] py-3.5"
            v-if="selectedItem.type === 'skin'"
          >
            <div
              class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
            >
              模型选择
            </div>
            <UiSegmented
              :model-value="selectedItem.model"
              @change="updateModel"
            >
              <el-radio-button value="default">Default</el-radio-button>
              <el-radio-button value="slim">Slim</el-radio-button>
            </UiSegmented>
          </section>
          <!-- public toggle -->
          <section class="border-b border-[var(--color-border)] py-3.5">
            <div
              class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
            >
              公开状态
            </div>
            <el-switch :model-value="selectedItem.is_public" @change="updateIsPublic" />
          </section>
          <!-- force delete -->
          <section class="border-b-0 py-3.5">
            <el-button type="danger" plain @click="confirmForceDelete">强制下架</el-button>
          </section>
        </div>
      </UiViewerLayout>
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import { ref, inject, onMounted, type Ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Picture, Edit } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import SearchBar from '@/components/common/SearchBar.vue'
import TextureCard from '@/components/textures/TextureCard.vue'
import TexturePreviewStage from '@/components/textures/TexturePreviewStage.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiSegmented from '@/components/ui/UiSegmented.vue'
import UiViewerLayout from '@/components/ui/UiViewerLayout.vue'
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
    const res = await getAdminTextures(
      buildSearchParams({ cursor: pagination.currentCursor.value }),
    )
    textures.value = res.data.items
    pagination.setPageData(res.data)
  } catch {
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
    await patchAdminTexture(selectedItem.value.hash, {
      type: selectedItem.value.type,
      note: newName,
    })
    selectedItem.value.name = newName
    ElMessage.success('名称已更新')
  } catch {
    ElMessage.error('更新名称失败')
  }
}

async function updateModel(newModel: string | number | boolean | undefined) {
  if (!selectedItem.value) return
  try {
    await patchAdminTexture(selectedItem.value.hash, {
      type: selectedItem.value.type,
      model: String(newModel),
    })
    selectedItem.value.model = String(newModel)
    ElMessage.success('模型已更新')
  } catch {
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
        { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' },
      )
    } catch {
      return
    }
  }

  try {
    await patchAdminTexture(item.hash, { type: item.type, is_public: Boolean(newValue) })
    item.is_public = Boolean(newValue)
    ElMessage.success(newValue ? '材质已公开' : '已取消公开')
  } catch {
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
      { confirmButtonText: '确认强制删除', cancelButtonText: '取消', type: 'error' },
    )
    await deleteAdminTexture(item.hash, { force: true, type: item.type })
    ElMessage.success('材质已强制下架')
    showPreview.value = false
    await fetchTextures()
  } catch {
    // User cancelled or error
  }
}

onMounted(refreshTexturesFromFirst)
</script>

<style scoped>
.texture-uploader {
  font-size: 12px;
  color: var(--color-text-light);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Skeleton loading */
.skeleton-card {
  pointer-events: none;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-card-background);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  overflow: hidden;
}
.skeleton-preview {
  height: 280px;
  background: var(--el-skeleton-color, #e0e0e0);
  border-radius: 8px 8px 0 0;
}
.skeleton-info {
  padding: 12px;
}
.skeleton-line {
  height: 14px;
  background: var(--el-skeleton-color, #e0e0e0);
  border-radius: 4px;
  margin-bottom: 8px;
}
.skeleton-line.short {
  width: 60%;
}
</style>
