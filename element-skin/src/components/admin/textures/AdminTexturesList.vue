<template>
  <PageContainer variant="gallery">
    <div class="animate-fade-in">
      <PageHeader title="材质管理" subtitle="浏览和管理皮肤库中所有上传的材质">
        <template #icon><Picture /></template>
        <template #actions>
          <UiButton
            loading-mode="icon"
            :icon="Refresh"
            :loading="loading"
            @click="refreshTexturesFromFirst"
          >
            刷新
          </UiButton>
        </template>
      </PageHeader>

      <div class="flex items-center gap-4 mb-6 flex-wrap">
        <SearchBar
          class="flex-1 min-w-[260px]"
          v-model="searchQuery"
          :loading="loading"
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
    </div>

    <div
      class="min-h-[400px]"
      role="region"
      aria-label="材质列表"
      :aria-busy="loading"
      v-loading="loading || sessionPending"
      element-loading-background="transparent"
    >
      <div
        v-if="textures.length > 0"
        class="grid grid-cols-[repeat(auto-fill,240px)] justify-center gap-6"
        :style="{ '--motion-card-count': textures.length }"
      >
        <TextureCard
          v-for="(item, index) in textures"
          :key="item.hash + item.type"
          :texture="item"
          :delay-index="index % limit"
          :textures-url="texturesUrl"
          :title="item.name || '未命名'"
          @preview="openPreview"
        >
          <template #info="{ texture }"
            ><ViewerIdentity :name="texture.uploader_display_name"
          /></template>
        </TextureCard>
      </div>
      <el-empty
        v-else-if="!loading && !sessionPending"
        :description="listError || (canRead ? '暂无材质数据' : '无权读取材质列表')"
        :image-size="80"
      />
    </div>

    <div class="pagination-container">
      <CursorPager
        v-if="textures.length > 0"
        :count="textures.length"
        :loading="pagination.isLoading.value"
        :disabled-prev="!pagination.canGoPrev.value"
        :disabled-next="!pagination.canGoNext.value"
        :page-size="pagination.limit.value"
        @prev="handlePrevPage"
        @next="handleNextPage"
        @page-size-change="handlePageSizeChange"
      />
    </div>

    <UiDialog
      v-model="showPreview"
      destroy-on-close
      variant="viewer"
      :busy="viewer.busy.value"
      :dirty="viewer.dirty.value"
      title="管理材质详情"
    >
      <UiViewerLayout v-if="selectedItem">
        <template #stage
          ><TexturePreviewStage :texture="selectedItem" :textures-url="texturesUrl"
        /></template>
        <ViewerNameEditor
          :key="selectedItem.hash + selectedItem.type + String(viewer.revision.value)"
          :value="selectedItem.name || ''"
          :can-edit="canEditMetadata"
          :busy="viewer.busy.value"
          :save="updatePreviewNote"
          label="材质名称"
          @dirty="viewer.dirty.value = $event"
        />
        <section class="flex flex-col items-start gap-4 border-b border-[var(--color-border)] py-5">
          <div class="flex flex-wrap items-center gap-3">
            <span class="text-sm text-[var(--color-text-light)]">上传者</span
            ><ViewerIdentity :name="selectedItem.uploader_display_name" />
          </div>
          <IdentifierValue label="Hash" :value="selectedItem.hash" />
        </section>
        <div class="flex flex-col items-start gap-5 py-5">
          <div v-if="selectedItem.type === 'skin' && canEditMetadata">
            <div class="mb-2 text-sm text-[var(--color-text-light)]">修改模型</div>
            <TextureModelSelect
              :model="selectedItem.model"
              :busy="viewer.busy.value"
              :disabled="viewer.blocked.value"
              @change="updateModel"
            />
          </div>
          <div class="flex flex-wrap items-center gap-3">
            <span class="text-sm text-[var(--color-text-light)]">公开状态</span>
            <el-switch
              v-if="canEditVisibility"
              :model-value="!!selectedItem.is_public"
              :disabled="viewer.blocked.value"
              aria-label="公开状态"
              @change="updateIsPublic"
            />
            <span>{{ selectedItem.is_public ? '公开' : '私有' }}</span>
          </div>
          <UiButton
            v-if="canDelete"
            variant="secondary"
            tone="danger"
            :disabled="viewer.blocked.value"
            @click="confirmForceDelete"
            >强制下架</UiButton
          >
        </div>
      </UiViewerLayout>
    </UiDialog>
  </PageContainer>
</template>

<script setup lang="ts">
import PageContainer from '@/components/common/PageContainer.vue'
import { computed, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Picture } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import SearchBar from '@/components/common/SearchBar.vue'
import TextureCard from '@/components/textures/TextureCard.vue'
import TextureModelSelect from '@/components/textures/TextureModelSelect.vue'
import TexturePreviewStage from '@/components/textures/TexturePreviewStage.vue'
import { textureAssetUrl as texturesUrl } from '@/components/textures/textureAssets'
import UiButton from '@/components/ui/UiButton.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiSegmented from '@/components/ui/UiSegmented.vue'
import UiViewerLayout from '@/components/ui/UiViewerLayout.vue'
import ViewerNameEditor from '@/components/common/ViewerNameEditor.vue'
import ViewerIdentity from '@/components/common/ViewerIdentity.vue'
import IdentifierValue from '@/components/common/IdentifierValue.vue'
import { useViewerSession } from '@/composables/useViewerSession'
import { useCursorPagination } from '@/composables/useCursorPagination'
import { getAdminTextures, patchAdminTexture, deleteAdminTexture } from '@/api/admin/textures'
import type { Texture } from '@/api/types'
import PageHeader from '@/components/common/PageHeader.vue'
import { useSessionStore } from '@/stores/session'

type TextureQueryParams = { cursor?: string | null; limit?: number; q?: string; type?: string }

const session = useSessionStore()
const permissions = computed(() => new Set(session.user?.permissions || []))
const sessionPending = computed(() => !session.authReady && !session.user)
const canRead = computed(() => permissions.value.has('texture.read.any'))
const canEditMetadata = computed(() => permissions.value.has('texture.update_metadata.any'))
const canEditVisibility = computed(() => permissions.value.has('texture.update_visibility.any'))
const canDelete = computed(() => permissions.value.has('texture.delete.any'))

const textures = ref<Texture[]>([])
const pagination = useCursorPagination<Texture>(20)
const limit = pagination.limit
const loading = ref(true)
const listError = ref('')
let listRequest = 0
const searchQuery = ref('')
const activeSearchQuery = ref('')
const typeFilter = ref<string | null>(null)

// Preview dialog
const showPreview = ref(false)
const selectedItem = ref<Texture | null>(null)
const viewer = useViewerSession(
  showPreview,
  computed(() => (selectedItem.value ? selectedItem.value.hash + selectedItem.value.type : '')),
)

function buildSearchParams(extraParams: TextureQueryParams = {}): TextureQueryParams {
  const params: TextureQueryParams = { limit: limit.value, ...extraParams }
  if (activeSearchQuery.value) params.q = activeSearchQuery.value
  if (typeFilter.value) params.type = typeFilter.value
  return params
}

async function fetchTextures() {
  if (!canRead.value) {
    loading.value = false
    return
  }
  listError.value = ''
  const request = ++listRequest
  const actorId = session.user?.id
  const active = viewer.snapshot()
  const current = () => active() && request === listRequest && actorId === session.user?.id
  loading.value = true
  pagination.isLoading.value = true
  try {
    const res = await getAdminTextures(
      buildSearchParams({ cursor: pagination.currentCursor.value }),
    )
    if (!current()) return
    textures.value = res.data.items
    pagination.setPageData(res.data)
  } catch {
    if (!current()) return
    listError.value = '加载材质列表失败'
    ElMessage.error('加载材质列表失败')
  } finally {
    if (request === listRequest) loading.value = false
    if (request === listRequest) pagination.isLoading.value = false
  }
}

async function refreshTexturesFromFirst() {
  if (!(await viewer.allowChange())) return
  pagination.reset()
  await fetchTextures()
}

async function handleNextPage() {
  if (!(await viewer.allowChange())) return
  await pagination.goToNextPage(async (cursor, pageLimit) => {
    const res = await getAdminTextures(buildSearchParams({ cursor, limit: pageLimit }))
    textures.value = res.data.items
    return res.data
  })
}

async function handlePrevPage() {
  if (!(await viewer.allowChange())) return
  await pagination.goToPrevPage(async (cursor, pageLimit) => {
    const res = await getAdminTextures(buildSearchParams({ cursor, limit: pageLimit }))
    textures.value = res.data.items
    return res.data
  })
}

async function handleSearch() {
  if (!(await viewer.allowChange())) return
  activeSearchQuery.value = searchQuery.value.trim()
  pagination.reset()
  fetchTextures()
}

async function handleClearSearch() {
  if (!(await viewer.allowChange())) return
  searchQuery.value = ''
  activeSearchQuery.value = ''
  pagination.reset()
  fetchTextures()
}

async function handlePageSizeChange(value: number) {
  if (!(await viewer.allowChange())) return
  if (!pagination.setLimit(value)) return
  await refreshTexturesFromFirst()
}

async function handleTypeFilterChange() {
  if (!(await viewer.allowChange())) return
  pagination.reset()
  fetchTextures()
}

// Preview dialog functions
async function openPreview(item: Texture) {
  if (!canRead.value || !(await viewer.allowChange())) return
  selectedItem.value = item
  showPreview.value = true
}

function updateLocalTexture(target: Texture, patch: Partial<Texture>) {
  Object.assign(target, patch)
  const row = textures.value.find((item) => item.hash === target.hash && item.type === target.type)
  if (row) Object.assign(row, patch)
}

async function updatePreviewNote(value: string): Promise<boolean> {
  const item = selectedItem.value
  const name = value.trim()
  if (!canRead.value || !canEditMetadata.value || !item || name === (item.name || '')) return false
  return viewer.run(
    async (current) => {
      await patchAdminTexture(item.hash, { type: item.type, note: name })
      if (!current()) return false
      updateLocalTexture(item, { name })
      ElMessage.success('名称已更新')
    },
    '更新名称失败',
    true,
  )
}

async function updateModel(value: string | number | boolean | undefined) {
  const item = selectedItem.value
  if (
    !canRead.value ||
    !canEditMetadata.value ||
    !item ||
    item.type !== 'skin' ||
    !['default', 'slim'].includes(String(value)) ||
    item.model === value
  )
    return
  await viewer.run(async (current) => {
    await patchAdminTexture(item.hash, { type: item.type, model: String(value) })
    if (!current()) return false
    updateLocalTexture(item, { model: String(value) })
    ElMessage.success('模型已更新')
  }, '更新模型失败')
}

async function updateIsPublic(value: string | number | boolean) {
  const item = selectedItem.value
  const isPublic = value === true || value === 1
  if (!canRead.value || !canEditVisibility.value || !item || !!item.is_public === isPublic) return
  await viewer.run(async (current) => {
    if (!isPublic) {
      await ElMessageBox.confirm(
        '取消公开后，该材质将不会出现在公共皮肤库中，已绑定此材质的角色不受影响。确定取消公开？',
        '确认操作',
        {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning',
          closeOnClickModal: false,
          distinguishCancelAndClose: true,
        },
      )
    }
    if (!current() || !canRead.value || !canEditVisibility.value) return false
    await patchAdminTexture(item.hash, { type: item.type, is_public: isPublic })
    if (!current()) return false
    updateLocalTexture(item, { is_public: isPublic })
    ElMessage.success(isPublic ? '材质已公开' : '已取消公开')
  }, '更新公开状态失败')
}

async function confirmForceDelete() {
  const item = selectedItem.value
  if (!item || !canRead.value || !canDelete.value) return
  await viewer.run(async (current) => {
    await ElMessageBox.confirm(
      '强制下架将删除此材质的皮肤库条目及所有用户对应的衣柜记录。已有角色绑定、头像和材质文件会保留，此操作不会停用已应用的材质。确定继续？',
      '危险操作',
      {
        confirmButtonText: '确认下架',
        cancelButtonText: '取消',
        type: 'warning',
        closeOnClickModal: false,
        distinguishCancelAndClose: true,
      },
    )
    if (!current() || !canRead.value || !canDelete.value) return false
    await deleteAdminTexture(item.hash, { force: true, type: item.type })
    if (!current()) return false
    textures.value = textures.value.filter(
      (row) => row.hash !== item.hash || row.type !== item.type,
    )
    showPreview.value = false
    ElMessage.success('材质已强制下架')
  }, '强制下架失败')
}

watch(
  [() => session.user?.id, canRead],
  ([id, allowed], previous) => {
    if (previous && (previous[0] !== id || !allowed)) {
      showPreview.value = false
      selectedItem.value = null
      textures.value = []
    }
    if (allowed) void refreshTexturesFromFirst()
    else loading.value = false
  },
  { immediate: true },
)
</script>
