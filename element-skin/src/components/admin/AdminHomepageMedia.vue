<template>
  <div class="admin-homepage-media animate-fade-in">
    <PageHeader title="首页图片" subtitle="管理静态图与 Panorama 的播放顺序、时长和镜头轨迹">
      <template #icon><PictureFilled /></template>
      <template #actions>
        <div class="upload-actions">
          <el-button
            type="primary"
            :icon="Check"
            size="large"
            :loading="saving"
            :disabled="loading || !hasChanges"
            @click="saveChanges"
          >
            保存配置
          </el-button>
          <el-upload
            action="#"
            :http-request="uploadImage"
            :show-file-list="false"
            accept=".png,.jpg,.jpeg,.webp"
          >
            <el-button :icon="Upload" size="large">上传图片</el-button>
          </el-upload>
          <el-upload
            action="#"
            :http-request="uploadPanorama"
            :show-file-list="false"
            accept=".zip"
          >
            <el-button :icon="Box" size="large">上传 Panorama</el-button>
          </el-upload>
        </div>
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
        @dragenter="dragOverId = item.id"
        @dragover="moveDraggedTo(item.id, $event)"
        @drop="endDrag"
        @dragend="endDrag"
      />

      <div v-if="items.length === 0 && !loading" class="empty-placeholder">
        <el-empty description="暂无首页媒体" />
      </div>
    </TransitionGroup>

    <HomepageMediaDialog
      v-model:visible="detailVisible"
      :item="selectedItem"
      :preview-url="selectedItem ? previewUrl(selectedItem) : ''"
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
  patchHomepageMedia,
  reorderHomepageMedia,
  uploadHomepageImage,
  uploadHomepagePanorama,
} from '@/api/admin/homepage-media'
import HomepageMediaCard from '@/components/admin/homepage/HomepageMediaCard.vue'
import HomepageMediaDialog from '@/components/admin/homepage/HomepageMediaDialog.vue'
import PageHeader from '@/components/common/PageHeader.vue'

type HomepageMediaPatch = Parameters<typeof patchHomepageMedia>[1]

const items = ref<HomepageMedia[]>([])
const savedItems = ref<HomepageMedia[]>([])
const loading = ref(false)
const saving = ref(false)
const detailVisible = ref(false)
const selectedId = ref<string | null>(null)
const draggingId = ref<string | null>(null)
const dragOverId = ref<string | null>(null)
const suppressNextClick = ref(false)
const longPressDelay = 350
let longPressTimer = 0
let pendingPress: {
  id: string
  pointerId: number
  startX: number
  startY: number
  card: HTMLElement
} | null = null
let pointerDragging = false
let dragGhost: HTMLElement | null = null
let ghostOffsetX = 0
let ghostOffsetY = 0
let lastReorderKey: string | null = null

const selectedItem = computed(() => items.value.find((item) => item.id === selectedId.value))
const hasChanges = computed(() => snapshot(items.value) !== snapshot(savedItems.value))
const savedById = computed(() => new Map(savedItems.value.map((item) => [item.id, item])))

function mediaUrl(item: HomepageMedia, face?: string) {
  const base = import.meta.env.BASE_URL
  const suffix = face ? `${item.storage_path}/${face}` : item.storage_path
  return `${base}static/carousel/${suffix}`.replace(/\/+/g, '/')
}

function previewUrl(item: HomepageMedia) {
  return item.type === 'panorama' ? mediaUrl(item, 'panorama_0.png') : mediaUrl(item)
}

async function fetchItems() {
  loading.value = true
  try {
    const res = await listHomepageMedia()
    const normalized = res.data.map(normalizeItem)
    items.value = cloneItems(normalized)
    savedItems.value = cloneItems(normalized)
  } catch (e) {
    ElMessage.error('获取首页媒体失败')
  } finally {
    loading.value = false
  }
}

function normalizeItem(item: HomepageMedia): HomepageMedia {
  return {
    ...item,
    duration_ms: Number(item.duration_ms),
    enabled: Boolean(item.enabled),
    overlay_opacity_light: Number(item.overlay_opacity_light),
    overlay_opacity_dark: Number(item.overlay_opacity_dark),
    start_yaw: Number(item.start_yaw),
    start_pitch: Number(item.start_pitch),
    yaw_speed_dps: Number(item.yaw_speed_dps),
    pitch_speed_dps: Number(item.pitch_speed_dps),
  }
}

function cloneItems(source: HomepageMedia[]) {
  return source.map((item) => ({ ...item }))
}

function snapshot(source: HomepageMedia[]) {
  return JSON.stringify(source.map(snapshotItem))
}

function snapshotItem(item: HomepageMedia) {
  return {
    id: item.id,
    title: item.title,
    enabled: Boolean(item.enabled),
    duration_ms: Number(item.duration_ms),
    overlay_opacity_light: Number(item.overlay_opacity_light),
    overlay_opacity_dark: Number(item.overlay_opacity_dark),
    start_yaw: Number(item.start_yaw),
    start_pitch: Number(item.start_pitch),
    yaw_speed_dps: Number(item.yaw_speed_dps),
    pitch_speed_dps: Number(item.pitch_speed_dps),
  }
}

function isItemDirty(id: string) {
  const current = items.value.find((item) => item.id === id)
  const saved = savedById.value.get(id)
  return current && saved
    ? JSON.stringify(snapshotItem(current)) !== JSON.stringify(snapshotItem(saved))
    : false
}

function openDetails(item: HomepageMedia) {
  if (suppressNextClick.value) {
    suppressNextClick.value = false
    return
  }
  selectedId.value = item.id
  detailVisible.value = true
}

async function uploadImage({ file }: UploadRequestOptions) {
  if (!canRunResourceAction()) return
  const formData = new FormData()
  formData.append('file', file)
  try {
    await uploadHomepageImage(formData)
    ElMessage.success('图片已上传')
    fetchItems()
  } catch (e: any) {
    ElMessage.error(e.response?.data?.detail || '上传失败')
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
  } catch (e: any) {
    ElMessage.error(e.response?.data?.detail || '上传失败')
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
    const savedMap = savedById.value
    const changedItems = items.value.filter((item) => {
      const saved = savedMap.get(item.id)
      return !saved || JSON.stringify(snapshotItem(item)) !== JSON.stringify(snapshotItem(saved))
    })
    const orderChanged =
      items.value.map((item) => item.id).join(',') !==
      savedItems.value.map((item) => item.id).join(',')

    for (const item of changedItems) {
      const res = await patchHomepageMedia(item.id, buildPatch(item))
      Object.assign(item, normalizeItem(res.data))
    }
    if (orderChanged) {
      await reorderHomepageMedia(items.value.map((item) => item.id))
    }

    savedItems.value = cloneItems(items.value.map(normalizeItem))
    ElMessage.success('配置已保存')
  } catch (e: any) {
    ElMessage.error(e.response?.data?.detail || '保存失败')
    await fetchItems()
  } finally {
    saving.value = false
  }
}

function buildPatch(item: HomepageMedia): HomepageMediaPatch {
  const body: HomepageMediaPatch = {
    title: item.title,
    enabled: item.enabled,
    duration_ms: Number(item.duration_ms),
    overlay_opacity_light: Number(item.overlay_opacity_light),
    overlay_opacity_dark: Number(item.overlay_opacity_dark),
  }
  if (item.type === 'panorama') {
    body.start_yaw = Number(item.start_yaw)
    body.start_pitch = Number(item.start_pitch)
    body.yaw_speed_dps = Number(item.yaw_speed_dps)
    body.pitch_speed_dps = Number(item.pitch_speed_dps)
  }
  return body
}

function startDrag(id: string, event: DragEvent) {
  draggingId.value = id
  dragOverId.value = id
  suppressNextClick.value = true
  event.dataTransfer?.setData('text/plain', id)
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    const card = event.currentTarget instanceof HTMLElement ? event.currentTarget : null
    if (card) event.dataTransfer.setDragImage(card, card.offsetWidth / 2, card.offsetHeight / 2)
  }
}

function startLongPress(id: string, event: PointerEvent) {
  if (event.pointerType === 'mouse' || event.button !== 0) return
  const card = event.currentTarget instanceof HTMLElement ? event.currentTarget : null
  if (!card) return
  clearLongPressTimer()
  pendingPress = {
    id,
    pointerId: event.pointerId,
    startX: event.clientX,
    startY: event.clientY,
    card,
  }
  longPressTimer = window.setTimeout(() => {
    beginPointerDrag(event.clientX, event.clientY)
  }, longPressDelay)
  document.addEventListener('pointermove', handlePointerMove, { passive: false })
  document.addEventListener('pointerup', finishPointerDrag, { passive: false })
  document.addEventListener('pointercancel', finishPointerDrag, { passive: false })
}

function beginPointerDrag(clientX: number, clientY: number) {
  if (!pendingPress || pointerDragging) return
  pointerDragging = true
  draggingId.value = pendingPress.id
  dragOverId.value = pendingPress.id
  suppressNextClick.value = true

  const rect = pendingPress.card.getBoundingClientRect()
  ghostOffsetX = clientX - rect.left
  ghostOffsetY = clientY - rect.top
  dragGhost = pendingPress.card.cloneNode(true) as HTMLElement
  dragGhost.classList.add('is-touch-ghost')
  dragGhost.style.position = 'fixed'
  dragGhost.style.left = '0'
  dragGhost.style.top = '0'
  dragGhost.style.width = `${rect.width}px`
  dragGhost.style.height = `${rect.height}px`
  dragGhost.style.margin = '0'
  dragGhost.style.pointerEvents = 'none'
  dragGhost.style.zIndex = '2147483001'
  dragGhost.style.opacity = '0.92'
  dragGhost.style.transition = 'none'
  document.body.appendChild(dragGhost)
  moveGhost(clientX, clientY)
}

function handlePointerMove(event: PointerEvent) {
  if (!pendingPress || event.pointerId !== pendingPress.pointerId) return

  if (!pointerDragging) {
    const moved = Math.hypot(
      event.clientX - pendingPress.startX,
      event.clientY - pendingPress.startY,
    )
    if (moved > 8) cancelLongPress()
    return
  }

  event.preventDefault()
  moveGhost(event.clientX, event.clientY)

  const target = document
    .elementFromPoint(event.clientX, event.clientY)
    ?.closest<HTMLElement>('[data-homepage-media-id]')
  const targetId = target?.dataset.homepageMediaId
  if (targetId) moveDraggedTo(targetId, event)
  if (!targetId) dragOverId.value = null
}

function moveGhost(clientX: number, clientY: number) {
  if (!dragGhost) return
  dragGhost.style.transform = `translate3d(${clientX - ghostOffsetX}px, ${clientY - ghostOffsetY}px, 0)`
}

function finishPointerDrag(event: PointerEvent) {
  if (pendingPress && event.pointerId !== pendingPress.pointerId) return
  if (pointerDragging) event.preventDefault()
  cleanupPointerDrag()
}

function cancelLongPress() {
  cleanupPointerDrag(false)
}

function cleanupPointerDrag(finishActive = true) {
  clearLongPressTimer()
  document.removeEventListener('pointermove', handlePointerMove)
  document.removeEventListener('pointerup', finishPointerDrag)
  document.removeEventListener('pointercancel', finishPointerDrag)
  dragGhost?.remove()
  dragGhost = null
  pendingPress = null
  resetReorderGuard()
  if (pointerDragging && finishActive) endDrag()
  pointerDragging = false
}

function clearLongPressTimer() {
  if (!longPressTimer) return
  window.clearTimeout(longPressTimer)
  longPressTimer = 0
}

function moveDraggedTo(targetId: string, event?: DragEvent | PointerEvent) {
  const sourceId = draggingId.value
  if (!sourceId || sourceId === targetId) {
    dragOverId.value = targetId
    return
  }
  const copy = items.value.slice()
  const from = copy.findIndex((item) => item.id === sourceId)
  const to = copy.findIndex((item) => item.id === targetId)
  if (from < 0 || to < 0 || from === to) return
  if (!shouldReorder(from, to, targetId, event)) {
    dragOverId.value = targetId
    return
  }
  const [item] = copy.splice(from, 1)
  if (!item) return
  copy.splice(to, 0, item)
  items.value = copy
  dragOverId.value = targetId
  lastReorderKey = reorderKey(from, to, targetId)
}

function endDrag() {
  draggingId.value = null
  dragOverId.value = null
  resetReorderGuard()
  window.setTimeout(() => {
    suppressNextClick.value = false
  })
}

function resetReorderGuard() {
  lastReorderKey = null
}

function shouldReorder(
  from: number,
  to: number,
  targetId: string,
  event?: DragEvent | PointerEvent,
) {
  const key = reorderKey(from, to, targetId)
  if (key === lastReorderKey) return false

  const target = event ? mediaCardElement(targetId, event.clientX, event.clientY) : null
  if (!target) return true

  const source = draggingId.value ? mediaCardElement(draggingId.value) : null
  const rect = target.getBoundingClientRect()
  const sourceRect = source?.getBoundingClientRect()
  const sameRow = sourceRect
    ? Math.abs(sourceRect.top - rect.top) < Math.min(sourceRect.height, rect.height) / 2
    : true

  if (sameRow) {
    const pointerX = event!.clientX - rect.left
    return from < to ? pointerX > rect.width / 2 : pointerX < rect.width / 2
  }

  const pointerY = event!.clientY - rect.top
  return from < to ? pointerY > rect.height / 2 : pointerY < rect.height / 2
}

function reorderKey(from: number, to: number, targetId: string) {
  return `${targetId}:${from < to ? 'after' : 'before'}`
}

function mediaCardElement(id: string, x?: number, y?: number) {
  const selector = `[data-homepage-media-id="${id}"]`
  if (x !== undefined && y !== undefined) {
    return document.elementFromPoint(x, y)?.closest<HTMLElement>(selector) || null
  }
  return document.querySelector<HTMLElement>(selector)
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
  } catch (e) {}
}

onMounted(fetchItems)
</script>

<style scoped>
.admin-homepage-media {
  max-width: 1040px;
  margin: 0 auto;
  padding: 20px 0;
}
.upload-actions {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}
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
