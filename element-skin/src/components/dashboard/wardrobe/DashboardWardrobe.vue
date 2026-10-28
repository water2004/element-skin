<template>
  <PageContainer variant="gallery" class="wardrobe-section">
    <PageHeader title="我的衣柜" subtitle="管理并应用您的皮肤与披风纹理" class="animate-fade-in"
      ><template #actions
        ><UiButton
          v-if="canUpload"
          @click="showUploadDialog = true"
          size="large"
          variant="primary"
          :icon="Upload"
        >
          上传纹理
        </UiButton></template
      ></PageHeader
    >

    <div
      class="min-h-[400px]"
      v-loading="loading || sessionPending"
      element-loading-background="transparent"
    >
      <div
        class="grid grid-cols-[repeat(auto-fill,240px)] justify-center gap-6"
        :style="{ '--motion-card-count': textures.length }"
        v-if="textures.length > 0"
      >
        <TextureCard
          v-for="(tex, index) in textures"
          :key="tex.hash + tex.type"
          :texture="tex"
          :delay-index="index % limit"
          :textures-url="texturesUrl"
          :title="tex.note || '未命名材质'"
          @preview="openDetailDialog"
        >
        </TextureCard>
      </div>

      <el-empty
        v-else-if="!loading && !sessionPending"
        :description="listError || (canRead ? '还没有纹理，快去上传吧！' : '无权读取衣柜')"
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

    <TextureDetailDialog
      v-model="showDetailDialog"
      :texture="selectedTexture"
      v-model:dirty="viewer.dirty.value"
      :busy="viewer.busy.value"
      :revision="viewer.revision.value"
      :save-name="updateNote"
      :ready="detailReady"
      :error="detailError"
      v-model:profile-id="applyForm.profile_id"
      :is-loading="isDetailLoading"
      :can-edit-metadata="canEditMetadata"
      :can-edit-visibility="canEditVisibility"
      :can-apply="canApply"
      :can-delete="canDelete"
      :profiles="userProfiles"
      :profiles-loading="profilesLoading"
      :profiles-error="profilesError"
      :textures-url="texturesUrl"
      @update-model="updateModel"
      @update-public="updateIsPublic"
      @apply="doApply"
      @delete="confirmDelete"
    />

    <TextureUploadDialog
      ref="uploadDialogRef"
      v-model="showUploadDialog"
      v-model:form="uploadForm"
      :preview-url="previewUrl"
      :uploading="uploading"
      @file-change="handleFileChange"
      @file-remove="handleFileRemove"
      @file-exceed="handleFileExceed"
      @submit="doUpload"
    />
  </PageContainer>
</template>

<script setup lang="ts">
import PageContainer from '@/components/common/PageContainer.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import { computed, ref, watch, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { UploadFile } from 'element-plus'
import { Upload } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import TextureDetailDialog from '@/components/dashboard/wardrobe/TextureDetailDialog.vue'
import TextureUploadDialog from '@/components/dashboard/wardrobe/TextureUploadDialog.vue'
import {
  createDefaultUploadForm,
  disposeLocalTextureUrl,
  replaceLocalTextureUrl,
} from '@/components/dashboard/wardrobe/uploadForm'
import TextureCard from '@/components/textures/TextureCard.vue'
import { textureAssetUrl as texturesUrl } from '@/components/textures/textureAssets'
import { useViewerSession } from '@/composables/useViewerSession'
import UiButton from '@/components/ui/UiButton.vue'
import { useCursorPagination } from '@/composables/useCursorPagination'
import { getProfiles } from '@/api/profiles'
import {
  getTextures,
  uploadTexture,
  getTextureDetail,
  patchTexture,
  deleteTexture,
  applyTexture,
} from '@/api/textures'
import type { Profile, Texture } from '@/api/types'
import { useSessionStore } from '@/stores/session'
import { getErrorMessage } from '@/utils/error'

const session = useSessionStore()
const { fetchMe } = session
const permissions = computed(() => new Set(session.user?.permissions || []))
const sessionPending = computed(() => !session.authReady && !session.user)
const canRead = computed(() => permissions.value.has('texture.read.owned'))
const canUpload = computed(() => permissions.value.has('texture.create.owned'))
const canEditMetadata = computed(() => permissions.value.has('texture.update_metadata.owned'))
const canEditVisibility = computed(() => permissions.value.has('texture.update_visibility.owned'))
const canDelete = computed(() => permissions.value.has('texture.delete.owned'))
const canApply = computed(
  () => permissions.value.has('texture.apply.owned') && permissions.value.has('profile.read.owned'),
)

const userProfiles = ref<Profile[]>([])
const profilesLoading = ref(false)
const profilesError = ref('')
let profilesGeneration = 0
const fetchUserProfiles = async () => {
  if (!canApply.value) return
  const generation = ++profilesGeneration
  profilesLoading.value = true
  profilesError.value = ''
  try {
    const all: Profile[] = []
    const seen = new Set<string>()
    let cursor: string | undefined
    do {
      const res = await getProfiles({ limit: 100, ...(cursor ? { cursor } : {}) })
      all.push(...res.data.items)
      if (!res.data.has_next) break
      const next = res.data.next_cursor
      if (!next || seen.has(next)) throw new Error('角色分页游标无效')
      seen.add(next)
      cursor = next
    } while (true)
    if (generation === profilesGeneration && canApply.value) userProfiles.value = all
  } catch {
    if (generation === profilesGeneration) {
      userProfiles.value = []
      profilesError.value = '角色列表加载失败，请重新打开详情重试'
    }
  } finally {
    if (generation === profilesGeneration) profilesLoading.value = false
  }
}

const textures = ref<Texture[]>([])
const pagination = useCursorPagination<Texture>(20)
const limit = pagination.limit
const loading = ref(false)
const listError = ref('')
let listRequest = 0
const showDetailDialog = ref(false)
const selectedTexture = ref<Texture | null>(null)
const isDetailLoading = ref(false)
const detailReady = ref(false)
const detailError = ref('')
const viewer = useViewerSession(
  showDetailDialog,
  computed(() =>
    selectedTexture.value ? selectedTexture.value.hash + selectedTexture.value.type : '',
  ),
)

const showUploadDialog = ref(false)
const uploading = ref(false)
const uploadForm = ref(createDefaultUploadForm())
const uploadDialogRef = ref<InstanceType<typeof TextureUploadDialog> | null>(null)
const previewUrl = ref<string | null>(null)
const applyForm = ref({ profile_id: '', texture_type: '', hash: '' })

async function openDetailDialog(tex: Texture) {
  if (!canRead.value || !(await viewer.allowChange())) return
  selectedTexture.value = { ...tex }
  applyForm.value = { hash: tex.hash, texture_type: tex.type, profile_id: '' }
  showDetailDialog.value = true
  detailReady.value = false
  detailError.value = ''
  isDetailLoading.value = true
  const current = viewer.snapshot()
  void fetchUserProfiles()
  try {
    const res = await getTextureDetail(tex.hash, tex.type)
    if (!current() || !showDetailDialog.value || !canRead.value) return
    selectedTexture.value = res.data
    detailReady.value = true
  } catch {
    if (current()) detailError.value = '获取详情失败，请关闭后重试'
  } finally {
    if (current()) isDetailLoading.value = false
  }
}

function updateLocalTexture(target: Texture, patch: Partial<Texture>) {
  Object.assign(target, patch)
  const row = textures.value.find((t) => t.hash === target.hash && t.type === target.type)
  if (row) Object.assign(row, patch)
}

async function updateNote(value: string): Promise<boolean> {
  const target = selectedTexture.value
  const note = value.trim()
  if (
    !canRead.value ||
    !canEditMetadata.value ||
    !detailReady.value ||
    !target ||
    note === (target.note || '')
  )
    return false
  return viewer.run(
    async (current) => {
      const result = await patchTexture(target.hash, target.type, { note })
      if (!current()) return false
      updateLocalTexture(target, { note: result.data.note ?? note })
      ElMessage.success('名称已更新')
    },
    '更新名称失败',
    true,
  )
}

async function updateModel(value: string | number | boolean | undefined) {
  const target = selectedTexture.value
  if (
    !canRead.value ||
    !canEditMetadata.value ||
    !detailReady.value ||
    !target ||
    target.type !== 'skin' ||
    !['default', 'slim'].includes(String(value)) ||
    target.model === value
  )
    return
  await viewer.run(async (current) => {
    const result = await patchTexture(target.hash, target.type, { model: String(value) })
    if (!current()) return false
    updateLocalTexture(target, { model: result.data.model || String(value) })
    ElMessage.success('模型已更新')
  }, '切换模型失败')
}

async function updateIsPublic(value: string | number | boolean) {
  const target = selectedTexture.value
  const isPublic = value === 1 || value === true
  if (
    !canRead.value ||
    !canEditVisibility.value ||
    !detailReady.value ||
    !target ||
    target.is_public === 2 ||
    !!target.is_public === isPublic
  )
    return
  await viewer.run(async (current) => {
    const result = await patchTexture(target.hash, target.type, { is_public: isPublic })
    if (!current()) return false
    updateLocalTexture(target, { is_public: result.data.is_public ?? (isPublic ? 1 : 0) })
    ElMessage.success(isPublic ? '材质已公开' : '材质已设为私有')
  }, '更新公开状态失败')
}

async function fetchTextures() {
  if (!canRead.value) return
  listError.value = ''
  const request = ++listRequest
  const actorId = session.user?.id
  const active = viewer.snapshot()
  const current = () => active() && request === listRequest && actorId === session.user?.id
  loading.value = true
  try {
    const params = {
      cursor: pagination.currentCursor.value,
      limit: limit.value,
    }
    const res = await getTextures(params)
    if (!current()) return
    textures.value = res.data.items
    pagination.setPageData(res.data)
  } catch {
    if (!current()) return
    listError.value = '加载衣柜失败'
    ElMessage.error(listError.value)
  } finally {
    if (request === listRequest) loading.value = false
  }
}

async function handleNextPage() {
  if (!(await viewer.allowChange())) return
  await pagination.goToNextPage(async (cursor, pageLimit) => {
    const params = { cursor, limit: pageLimit }
    const res = await getTextures(params)
    textures.value = res.data.items
    return res.data
  })
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function handlePrevPage() {
  if (!(await viewer.allowChange())) return
  await pagination.goToPrevPage(async (cursor, pageLimit) => {
    const params = { cursor, limit: pageLimit }
    const res = await getTextures(params)
    textures.value = res.data.items
    return res.data
  })
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function handlePageSizeChange(value: number) {
  if (!(await viewer.allowChange())) return
  if (!pagination.setLimit(value)) return
  await fetchTextures()
}

async function refreshFirstPage() {
  pagination.reset()
  await fetchTextures()
}

function handleFileChange(file: UploadFile) {
  if (uploading.value) return
  uploadForm.value.file = file.raw ?? null
  previewUrl.value = replaceLocalTextureUrl(previewUrl.value, uploadForm.value.file)
}

function handleFileRemove() {
  if (uploading.value) return
  uploadForm.value.file = null
  previewUrl.value = replaceLocalTextureUrl(previewUrl.value, null)
}

function handleFileExceed() {
  ElMessage.warning('最多只能上传 1 个文件，请先移除已选文件')
}

function resetUploadState() {
  uploadForm.value = createDefaultUploadForm()
  disposeLocalTextureUrl(previewUrl.value)
  previewUrl.value = null
}

// Closing the upload dialog without submitting must discard the pending file
// and preview; otherwise reopening shows a stale preview and a working upload
// while the file picker no longer lists the selected file.
watch(showUploadDialog, (isOpen) => {
  if (!isOpen) {
    resetUploadState()
    uploadDialogRef.value?.clearFiles()
  }
})

async function doUpload() {
  if (!canUpload.value) return
  if (uploading.value) return
  const file = uploadForm.value.file
  if (!file) return ElMessage.error('请选择文件')
  if (!uploadForm.value.texture_type) return ElMessage.error('请选择纹理类型')

  const formData = new FormData()
  formData.append('file', file)
  formData.append('texture_type', uploadForm.value.texture_type)
  if (uploadForm.value.texture_type === 'skin') {
    formData.append('model', uploadForm.value.model || 'default')
  }
  formData.append('note', uploadForm.value.note || '')
  formData.append('is_public', uploadForm.value.is_public ? 'true' : 'false')

  uploading.value = true
  try {
    await uploadTexture(formData)
    ElMessage.success('上传成功')
    showUploadDialog.value = false
    await refreshFirstPage()
  } catch (e: unknown) {
    ElMessage.error('上传失败: ' + getErrorMessage(e, '上传失败'))
  } finally {
    uploading.value = false
  }
}

async function confirmDelete() {
  const target = selectedTexture.value
  if (!target || !canRead.value || !canDelete.value || !detailReady.value) return
  await viewer.run(async (current) => {
    await ElMessageBox.confirm(
      '当前删除规则：若你是皮肤库记录的原上传者，将删除该库条目及所有用户对应的衣柜记录；否则只移除你自己的衣柜记录。角色绑定、头像和材质文件会保留。当前页面无法确认原上传者身份，请确认可能的影响后再继续。',
      '删除纹理',
      {
        confirmButtonText: '确定删除',
        cancelButtonText: '取消',
        type: 'warning',
        confirmButtonClass: 'el-button--danger',
        closeOnClickModal: false,
        distinguishCancelAndClose: true,
      },
    )
    if (!current() || !canDelete.value || !canRead.value) return false
    await deleteTexture(target.hash, target.type)
    if (!current()) return false
    textures.value = textures.value.filter((t) => t.hash !== target.hash || t.type !== target.type)
    showDetailDialog.value = false
    ElMessage.success('已删除')
  }, '删除失败')
}

async function doApply() {
  const target = selectedTexture.value
  const profileId = applyForm.value.profile_id
  if (
    !canRead.value ||
    !canApply.value ||
    !detailReady.value ||
    !target ||
    profilesLoading.value ||
    profilesError.value
  )
    return
  if (!userProfiles.value.some((p) => p.id === profileId)) return ElMessage.error('请选择有效角色')
  await viewer.run(async (current) => {
    await applyTexture(target.hash, { profile_id: profileId, texture_type: target.type })
    if (!current()) return false
    ElMessage.success('已应用')
    try {
      await fetchMe()
    } catch {
      ElMessage.warning('材质已应用，但账号信息刷新失败')
    }
  }, '应用失败')
}

onUnmounted(() => {
  profilesGeneration++
  disposeLocalTextureUrl(previewUrl.value)
})
watch(
  [() => session.user?.id, canRead],
  ([id, allowed], previous) => {
    if (previous && (previous[0] !== id || !allowed)) {
      profilesGeneration++
      userProfiles.value = []
      profilesLoading.value = false
      profilesError.value = ''
      showDetailDialog.value = false
      selectedTexture.value = null
      textures.value = []
    }
    if (allowed) void refreshFirstPage()
    else loading.value = false
  },
  { immediate: true },
)
</script>
