<template>
  <div class="wardrobe-section animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <div>
          <h1>我的衣柜</h1>
          <p>管理并应用您的皮肤与披风纹理</p>
        </div>
      </div>
      <UiButton @click="showUploadDialog = true" size="large" variant="gradient-primary">
        <el-icon><Upload /></el-icon>
        <span class="ml-2">上传纹理</span>
      </UiButton>
    </div>

    <div class="min-h-[400px]" v-loading="loading" element-loading-background="transparent">
      <div
        class="grid grid-cols-[repeat(auto-fill,240px)] justify-center gap-6"
        v-if="textures.length > 0"
      >
        <TextureCard
          v-for="(tex, index) in textures"
          :key="tex.hash + tex.type"
          :texture="tex"
          :delay-index="index % limit"
          :is-dark="isDark"
          :textures-url="texturesUrl"
          :resolution="textureResolutions.get(tex.hash)"
          :title="tex.note || '未命名纹理'"
          show-type
          @preview="openDetailDialog"
        >
        </TextureCard>
      </div>

      <el-empty v-else-if="!loading" description="还没有纹理，快去上传吧！" />
    </div>

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

    <TextureDetailDialog
      v-model="showDetailDialog"
      v-model:texture="selectedTexture"
      v-model:note="editingNoteValue"
      v-model:profile-id="applyForm.profile_id"
      :is-loading="isDetailLoading"
      :is-applying="isApplying"
      :profiles="userProfiles"
      :resolution="selectedTexture ? textureResolutions.get(selectedTexture.hash) : undefined"
      :textures-url="texturesUrl"
      @update-note="updateNote"
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
      @file-change="handleFileChange"
      @file-remove="handleFileRemove"
      @submit="doUpload"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, inject } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { UploadFile } from 'element-plus'
import type { Ref } from 'vue'
import { Upload } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import TextureDetailDialog from '@/components/dashboard/wardrobe/TextureDetailDialog.vue'
import TextureUploadDialog from '@/components/dashboard/wardrobe/TextureUploadDialog.vue'
import { createDefaultUploadForm, disposeLocalTextureUrl, replaceLocalTextureUrl } from '@/components/dashboard/wardrobe/uploadForm'
import TextureCard from '@/components/textures/TextureCard.vue'
import {
  cacheSkinTextureWidths,
  textureAssetUrl as texturesUrl,
} from '@/components/textures/textureAssets'
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
import { getErrorMessage } from '@/utils/error'

// Inject shared state from AppLayout
const fetchMe = inject<() => Promise<void>>('fetchMe')
const isDark = inject<Ref<boolean>>('isDark', ref(false))

const userProfiles = ref<Profile[]>([])
const fetchUserProfiles = async () => {
  try {
    // Fetch all profiles for the dropdown (use a large limit if needed, or implement search)
    const res = await getProfiles({ limit: 100 })
    userProfiles.value = res.data.items
  } catch (e) {
    console.error('Failed to fetch profiles for wardrobe:', e)
  }
}

const textures = ref<Texture[]>([])
const limit = 20
const pagination = useCursorPagination<Texture>(limit)
const loading = ref(false)
const textureResolutions = ref(new Map<string, number>())
const showDetailDialog = ref(false)
const selectedTexture = ref<Texture | null>(null)
const isDetailLoading = ref(false)
const editingNoteValue = ref('')
const isApplying = ref(false)

const showUploadDialog = ref(false)
const uploadForm = ref(createDefaultUploadForm())
const uploadDialogRef = ref<InstanceType<typeof TextureUploadDialog> | null>(null)
const previewUrl = ref<string | null>(null)
const applyForm = ref({ profile_id: '', texture_type: '', hash: '' })

async function openDetailDialog(tex: Texture) {
  selectedTexture.value = { ...tex, is_public: 2 }
  editingNoteValue.value = tex.note || ''
  applyForm.value.hash = tex.hash
  applyForm.value.texture_type = tex.type
  applyForm.value.profile_id = ''

  showDetailDialog.value = true
  isDetailLoading.value = true

  try {
    const res = await getTextureDetail(tex.hash, tex.type)
    selectedTexture.value = res.data
    editingNoteValue.value = res.data.note || ''
  } catch (e) {
    console.error('Fetch texture detail error:', e)
    ElMessage.error('获取详情失败')
  } finally {
    isDetailLoading.value = false
  }
}

async function updateNote() {
  if (!selectedTexture.value || isDetailLoading.value) return
  const tex = selectedTexture.value
  const updated = editingNoteValue.value.trim()
  if (updated === (tex.note || '')) return

  try {
    await patchTexture(tex.hash, tex.type, { note: updated })
    tex.note = updated
    const localTex = textures.value.find((t) => t.hash === tex.hash && t.type === tex.type)
    if (localTex) localTex.note = updated
    ElMessage.success('备注已更新')
  } catch {
    ElMessage.error('更新备注失败')
  }
}

async function updateModel(val: string | number | boolean | null | undefined) {
  if (!selectedTexture.value || isDetailLoading.value) return
  const tex = selectedTexture.value
  try {
    await patchTexture(tex.hash, tex.type, { model: String(val) })
    tex.model = String(val)
    const localTex = textures.value.find((t) => t.hash === tex.hash && t.type === tex.type)
    if (localTex) localTex.model = String(val)
    ElMessage.success(`模型已切换为 ${val === 'slim' ? '纤细' : '普通'}`)
  } catch {
    ElMessage.error('切换模型失败')
  }
}

async function updateIsPublic(val: string | number | boolean) {
  if (!selectedTexture.value || isDetailLoading.value) return
  const tex = selectedTexture.value
  const isPublic = val === 1 || val === true
  try {
    await patchTexture(tex.hash, tex.type, { is_public: isPublic })
    const visibility = isPublic ? 1 : 0
    tex.is_public = visibility
    const localTex = textures.value.find((item) => item.hash === tex.hash && item.type === tex.type)
    if (localTex) localTex.is_public = visibility
    ElMessage.success(isPublic ? '材质已公开' : '材质已设为私有')
  } catch {
    ElMessage.error('更新公开状态失败')
  }
}

async function fetchTextures() {
  loading.value = true
  try {
    const params = {
      cursor: pagination.currentCursor.value,
      limit: limit,
    }
    const res = await getTextures(params)
    textures.value = res.data.items
    pagination.setPageData(res.data)
    void cacheSkinTextureWidths(textures.value, textureResolutions.value)
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

async function handleNextPage() {
  await pagination.goToNextPage(async (cursor, pageLimit) => {
    const params = { cursor, limit: pageLimit }
    const res = await getTextures(params)
    textures.value = res.data.items
    return res.data
  })
  void cacheSkinTextureWidths(textures.value, textureResolutions.value)
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function handlePrevPage() {
  await pagination.goToPrevPage(async (cursor, pageLimit) => {
    const params = { cursor, limit: pageLimit }
    const res = await getTextures(params)
    textures.value = res.data.items
    return res.data
  })
  void cacheSkinTextureWidths(textures.value, textureResolutions.value)
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function refreshFirstPage() {
  pagination.reset()
  await fetchTextures()
}

function handleFileChange(file: UploadFile) {
  uploadForm.value.file = file.raw ?? null
  previewUrl.value = replaceLocalTextureUrl(previewUrl.value, uploadForm.value.file)
}

function handleFileRemove() {
  uploadForm.value.file = null
  previewUrl.value = replaceLocalTextureUrl(previewUrl.value, null)
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

  try {
    await uploadTexture(formData)
    ElMessage.success('上传成功')
    showUploadDialog.value = false
    await refreshFirstPage()
  } catch (e: unknown) {
    ElMessage.error('上传失败: ' + getErrorMessage(e, '上传失败'))
  }
}

async function confirmDelete() {
  if (!selectedTexture.value) return
  try {
    await ElMessageBox.confirm('确定要从衣柜中删除此纹理吗？此操作不可撤销。', '警告', {
      confirmButtonText: '确定删除',
      cancelButtonText: '取消',
      type: 'warning',
      confirmButtonClass: 'el-button--danger',
    })

    await deleteTexture(selectedTexture.value.hash, selectedTexture.value.type)
    ElMessage.success('已删除')
    showDetailDialog.value = false
    await refreshFirstPage()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('删除失败')
  }
}

async function doApply() {
  if (!applyForm.value.profile_id) return ElMessage.error('请选择角色')
  isApplying.value = true
  try {
    await applyTexture(applyForm.value.hash, {
      profile_id: applyForm.value.profile_id,
      texture_type: applyForm.value.texture_type,
    })
    ElMessage.success('已应用')
    if (fetchMe) fetchMe()
    fetchUserProfiles()
    fetchTextures()
  } catch (e: unknown) {
    ElMessage.error('应用失败: ' + getErrorMessage(e, '应用失败'))
  } finally {
    isApplying.value = false
  }
}

onMounted(() => {
  refreshFirstPage()
  fetchUserProfiles()
})

onUnmounted(() => {
  disposeLocalTextureUrl(previewUrl.value)
})
</script>
