<template>
  <PageContainer variant="wide" class="animate-fade-in">
    <PageHeader
      title="首页图片"
      subtitle="管理静态图与 Panorama 的播放顺序、时长和镜头轨迹"
      actions-full
    >
      <template #icon><PictureFilled /></template>
      <template #actions>
        <DirtyTag :visible="hasChanges" :spaced="false" />
        <el-upload
          action="#"
          :http-request="uploadImage"
          :show-file-list="false"
          accept=".png,.jpg,.jpeg,.webp"
          :disabled="busy"
        >
          <el-button :icon="Upload" size="large" :disabled="busy">上传图片</el-button>
        </el-upload>
        <el-upload
          action="#"
          :http-request="uploadPanorama"
          :show-file-list="false"
          accept=".zip"
          :disabled="busy"
        >
          <el-button :icon="Box" size="large" :disabled="busy">上传 Panorama</el-button>
        </el-upload>
        <el-button size="large" :disabled="busy || !hasChanges" @click="discardChanges">
          放弃更改
        </el-button>
        <el-button
          type="primary"
          :icon="Check"
          size="large"
          :loading="saving"
          :disabled="busy || !hasChanges"
          @click="saveChanges"
        >
          保存配置
        </el-button>
      </template>
    </PageHeader>

    <p class="mt-0 mb-5 text-sm text-[var(--color-text-light)]">
      编辑、启用和排序需保存配置后生效；上传与删除立即生效。有未保存更改时，请先保存或放弃，再上传或删除。
    </p>
    <p v-if="saveIncomplete" role="status" class="mb-5 text-sm text-[var(--el-color-warning)]">
      保存未完成，已成功的部分不会回滚。可重试剩余更改；“放弃更改”只丢弃未保存部分。
    </p>

    <TransitionGroup name="media-grid" tag="div" class="media-list" v-loading="loading">
      <HomepageMediaCard
        v-for="item in items"
        :key="item.id"
        :item="item"
        :preview-url="previewUrl(item)"
        :dirty="isItemDirty(item.id)"
        :disabled="busy"
        :dragging="draggingId === item.id"
        :drag-over="dragOverId === item.id && draggingId !== item.id"
        @open="openDetails(item)"
        @toggle-enabled="toggleEnabled(item)"
        @remove="remove(item)"
        @pressstart="startLongPress(item.id, $event)"
        @dragstart="startDrag(item.id, $event)"
        @dragenter="setDragOver(item.id)"
        @dragover="moveDraggedTo(item.id, $event)"
        @drop="endDrag"
        @dragend="endDrag"
      />

      <div v-if="items.length === 0 && !loading" class="py-10">
        <el-empty description="暂无首页媒体" />
      </div>
    </TransitionGroup>

    <HomepageMediaDialog
      v-model:visible="detailVisible"
      :item="selectedItem"
      :busy="busy"
      :preview-url="selectedItem ? previewUrl(selectedItem) : ''"
      @update:item="updateSelectedItem"
    />
  </PageContainer>
</template>

<script setup lang="ts">
import PageContainer from '@/components/common/PageContainer.vue'
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { UploadRequestOptions } from 'element-plus'
import { Box, Check, PictureFilled, Upload } from '@element-plus/icons-vue'
import type { HomepageMedia } from '@/api/types'
import {
  deleteHomepageMedia,
  listHomepageMedia,
  uploadHomepageImage,
  uploadHomepagePanorama,
} from '@/api/admin/homepage-media'
import HomepageMediaCard from '@/components/admin/homepage/HomepageMediaCard.vue'
import HomepageMediaDialog from '@/components/admin/homepage/HomepageMediaDialog.vue'
import {
  cloneHomepageMediaItems,
  homepageMediaPreviewUrl,
  homepageMediaSnapshot,
  isHomepageMediaDirty,
  normalizeHomepageMedia,
} from '@/components/admin/homepage/homepageMediaState'
import { saveHomepageMediaChanges } from '@/components/admin/homepage/homepageMediaSave'
import { useHomepageMediaDrag } from '@/components/admin/homepage/useHomepageMediaDrag'
import PageHeader from '@/components/common/PageHeader.vue'
import DirtyTag from '@/components/common/DirtyTag.vue'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import { getErrorMessage } from '@/utils/error'

const items = ref<HomepageMedia[]>([])
const savedItems = ref<HomepageMedia[]>([])
const loading = ref(false)
const saving = ref(false)
const resourceBusy = ref(false)
const saveIncomplete = ref(false)
const writeBusy = computed(() => saving.value || resourceBusy.value)
const busy = computed(() => loading.value || writeBusy.value)
const detailVisible = ref(false)
const selectedId = ref<string | null>(null)
const {
  draggingId,
  dragOverId,
  consumeSuppressedClick,
  endDrag,
  moveDraggedTo,
  setDragOver,
  startDrag,
  startLongPress,
} = useHomepageMediaDrag(items)

const selectedItem = computed(() => items.value.find((item) => item.id === selectedId.value))
const hasChanges = computed(
  () => homepageMediaSnapshot(items.value) !== homepageMediaSnapshot(savedItems.value),
)
const savedById = computed(() => new Map(savedItems.value.map((item) => [item.id, item])))
useUnsavedChangesGuard({ dirty: hasChanges, busy: writeBusy })

function discardChanges() {
  if (busy.value) return
  endDrag()
  items.value = cloneHomepageMediaItems(savedItems.value)
  saveIncomplete.value = false
}

function toggleEnabled(item: HomepageMedia) {
  if (!busy.value) item.enabled = !item.enabled
}

function previewUrl(item: HomepageMedia) {
  return homepageMediaPreviewUrl(item, import.meta.env.BASE_URL)
}

async function fetchItems() {
  loading.value = true
  try {
    const res = await listHomepageMedia()
    const normalized = res.data.items.map(normalizeHomepageMedia)
    items.value = cloneHomepageMediaItems(normalized)
    savedItems.value = cloneHomepageMediaItems(normalized)
  } catch {
    ElMessage.error('获取首页媒体失败')
  } finally {
    loading.value = false
  }
}

function isItemDirty(id: string) {
  const current = items.value.find((item) => item.id === id)
  const saved = savedById.value.get(id)
  return isHomepageMediaDirty(current, saved)
}

function openDetails(item: HomepageMedia) {
  if (busy.value || consumeSuppressedClick()) return
  selectedId.value = item.id
  detailVisible.value = true
}

function updateSelectedItem(updated: HomepageMedia) {
  if (busy.value) return
  const item = items.value.find((candidate) => candidate.id === updated.id)
  if (item) Object.assign(item, normalizeHomepageMedia(updated))
}

async function uploadImage({ file }: UploadRequestOptions) {
  if (!canRunResourceAction()) return
  resourceBusy.value = true
  const formData = new FormData()
  formData.append('file', file)
  try {
    await uploadHomepageImage(formData)
    ElMessage.success('图片已上传')
    await fetchItems()
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, '上传失败'))
  } finally {
    resourceBusy.value = false
  }
}

async function uploadPanorama({ file }: UploadRequestOptions) {
  if (!canRunResourceAction()) return
  resourceBusy.value = true
  const formData = new FormData()
  formData.append('file', file)
  try {
    await uploadHomepagePanorama(formData)
    ElMessage.success('Panorama 已上传')
    await fetchItems()
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, '上传失败'))
  } finally {
    resourceBusy.value = false
  }
}

function canRunResourceAction() {
  if (busy.value) return false
  if (!hasChanges.value) return true
  ElMessage.warning('请先保存或放弃当前配置，再上传或删除媒体')
  return false
}

async function saveChanges() {
  if (busy.value || !hasChanges.value) return
  endDrag()
  saving.value = true
  try {
    savedItems.value = await saveHomepageMediaChanges(items.value, savedItems.value, {
      onProgress: (acknowledged) => {
        savedItems.value = acknowledged
      },
    })
    saveIncomplete.value = false
    ElMessage.success('配置已保存')
  } catch (e: unknown) {
    saveIncomplete.value = true
    ElMessage.error('保存未完成：' + getErrorMessage(e, '请重试'))
  } finally {
    saving.value = false
  }
}

async function remove(item: HomepageMedia) {
  if (!canRunResourceAction()) return
  resourceBusy.value = true
  try {
    try {
      await ElMessageBox.confirm(
        `确定删除首页媒体“${item.title}”吗？此操作立即生效，无法通过放弃配置恢复。`,
        '删除媒体',
        {
          type: 'warning',
          confirmButtonText: '删除媒体',
          cancelButtonText: '取消',
          closeOnClickModal: false,
        },
      )
    } catch {
      return
    }
    await deleteHomepageMedia(item.id)
    ElMessage.success('已删除')
    await fetchItems()
  } catch (error: unknown) {
    ElMessage.error(getErrorMessage(error, '删除媒体失败'))
  } finally {
    resourceBusy.value = false
  }
}

onMounted(fetchItems)
</script>

<style scoped>
.media-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, 240px);
  gap: var(--space-panel-padding);
  justify-content: center;
}
.media-grid-move,
:deep(.media-grid-move) {
  transition: transform var(--motion-duration-base) var(--motion-easing-standard);
}
.empty-placeholder {
  padding: 40px 0;
}

@media (max-width: 768px) {
  .media-list {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
