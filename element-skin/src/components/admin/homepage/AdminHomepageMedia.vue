<template>
  <div class="max-w-[1040px] mx-auto py-5 animate-fade-in">
    <PageHeader title="首页图片" subtitle="管理静态图与 Panorama 的播放顺序、时长和镜头轨迹">
      <template #icon><PictureFilled /></template>
      <template #actions>
        <ActionBar>
          <el-upload
            action="#"
            :http-request="uploadImage"
            :show-file-list="false"
            accept=".png,.jpg,.jpeg,.webp"
          >
            <el-button :icon="Upload" size="large" class="hover-lift">上传图片</el-button>
          </el-upload>
          <el-upload
            action="#"
            :http-request="uploadPanorama"
            :show-file-list="false"
            accept=".zip"
          >
            <el-button :icon="Box" size="large" class="hover-lift">上传 Panorama</el-button>
          </el-upload>
          <el-button
            type="primary"
            :icon="Check"
            size="large"
            :loading="saving"
            :disabled="loading || !hasChanges"
            class="hover-lift"
            @click="saveChanges"
          >
            保存配置
          </el-button>
        </ActionBar>
      </template>
    </PageHeader>

    <TransitionGroup name="media-grid" tag="div" class="media-list" v-loading="loading">
      <HomepageMediaCard
        v-for="item in items"
        :key="item.id"
        :item="item"
        :preview-url="previewUrl(item)"
        :dirty="isItemDirty(item.id)"
        :dragging="draggingId === item.id"
        :drag-over="dragOverId === item.id && draggingId !== item.id"
        @open="openDetails(item)"
        @toggle-enabled="item.enabled = !item.enabled"
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
      :preview-url="selectedItem ? previewUrl(selectedItem) : ''"
      @update:item="updateSelectedItem"
    />
  </div>
</template>

<script setup lang="ts">
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
import ActionBar from '@/components/common/ActionBar.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import { getErrorMessage } from '@/utils/error'

const items = ref<HomepageMedia[]>([])
const savedItems = ref<HomepageMedia[]>([])
const loading = ref(false)
const saving = ref(false)
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
  if (consumeSuppressedClick()) return
  selectedId.value = item.id
  detailVisible.value = true
}

function updateSelectedItem(updated: HomepageMedia) {
  const item = items.value.find((candidate) => candidate.id === updated.id)
  if (item) Object.assign(item, normalizeHomepageMedia(updated))
}

async function uploadImage({ file }: UploadRequestOptions) {
  if (!canRunResourceAction()) return
  const formData = new FormData()
  formData.append('file', file)
  try {
    await uploadHomepageImage(formData)
    ElMessage.success('图片已上传')
    fetchItems()
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, '上传失败'))
  }
}

async function uploadPanorama({ file }: UploadRequestOptions) {
  if (!canRunResourceAction()) return
  const formData = new FormData()
  formData.append('file', file)
  try {
    await uploadHomepagePanorama(formData)
    ElMessage.success('Panorama 已上传')
    fetchItems()
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, '上传失败'))
  }
}

function canRunResourceAction() {
  if (!hasChanges.value) return true
  ElMessage.warning('请先保存当前配置')
  return false
}

async function saveChanges() {
  if (!hasChanges.value) return
  saving.value = true
  try {
    savedItems.value = await saveHomepageMediaChanges(items.value, savedItems.value)
    ElMessage.success('配置已保存')
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, '保存失败'))
    await fetchItems()
  } finally {
    saving.value = false
  }
}

async function remove(item: HomepageMedia) {
  if (!canRunResourceAction()) return
  try {
    await ElMessageBox.confirm('确定要删除这个首页媒体吗？', '确认删除', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
    await deleteHomepageMedia(item.id)
    ElMessage.success('已删除')
    fetchItems()
  } catch {}
}

onMounted(fetchItems)
</script>

<style scoped>
.media-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, 240px);
  gap: 22px;
  justify-content: center;
}
.media-grid-move,
:deep(.media-grid-move) {
  transition: transform 0.22s cubic-bezier(0.4, 0, 0.2, 1);
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
