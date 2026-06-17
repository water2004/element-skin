<template>
  <div class="wardrobe-section animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <div>
          <h1>我的衣柜</h1>
          <p>管理并应用您的皮肤与披风纹理</p>
        </div>
      </div>
      <el-button
        @click="showUploadDialog = true"
        size="large"
        class="btn-gradient btn-gradient-primary"
      >
        <el-icon><Upload /></el-icon>
        <span class="ml-2">上传纹理</span>
      </el-button>
    </div>

    <div class="min-h-400" v-loading="loading" element-loading-background="transparent">
      <div class="auto-grid" v-if="textures.length > 0">
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

    <el-dialog v-model="showDetailDialog" destroy-on-close class="dialog-viewer" append-to-body>
      <div class="viewer-layout" v-if="selectedTexture">
        <TexturePreviewStage :texture="selectedTexture" :textures-url="texturesUrl" />

        <div class="viewer-info-panel" v-loading="isDetailLoading">
          <template v-if="selectedTexture">
            <section class="viewer-section title-section">
              <div class="viewer-title-row">
                <el-button text circle class="title-edit-btn" @click="focusNoteInput">
                  <el-icon><Edit /></el-icon>
                </el-button>
                <el-input
                  ref="noteInputRef"
                  v-model="editingNoteValue"
                  placeholder="未命名纹理"
                  class="viewer-title-input"
                  @blur="updateNote"
                  @keyup.enter="updateNote"
                />
              </div>
            </section>

            <section class="viewer-section meta-section">
              <div class="viewer-title-row">
                <span class="meta-chip"
                  >{{ textureResolutions.get(selectedTexture.hash) || '--' }}px</span
                >
                <span class="meta-chip hash">{{ selectedTexture.hash }}</span>
              </div>
            </section>

            <section class="viewer-section" v-if="selectedTexture.type === 'skin'">
              <div class="viewer-section-label">模型选择</div>
              <el-radio-group
                v-model="selectedTexture.model"
                @change="updateModel"
                class="capsule-radio"
              >
                <el-radio-button value="default">Default</el-radio-button>
                <el-radio-button value="slim">Slim</el-radio-button>
              </el-radio-group>
            </section>

            <section
              class="viewer-section"
              v-if="!isDetailLoading && selectedTexture.is_public !== 2"
            >
              <div class="viewer-section-label">公开状态</div>
              <div class="flex items-center gap-3">
                <el-switch
                  v-model="selectedTexture.is_public"
                  :active-value="1"
                  :inactive-value="0"
                  @change="updateIsPublic"
                />
                <span class="text-13 text-info">
                  {{
                    selectedTexture.is_public === 1
                      ? '公开（其他用户可在皮肤库看到）'
                      : '私有（仅自己可见）'
                  }}
                </span>
              </div>
            </section>

            <section class="viewer-section">
              <div class="viewer-section-label">应用到角色</div>
              <div class="flex gap-2">
                <el-select
                  v-model="applyForm.profile_id"
                  placeholder="选择目标"
                  class="gallery-select"
                >
                  <el-option
                    v-for="p in userProfiles || []"
                    :key="p.id"
                    :label="p.name"
                    :value="p.id"
                  />
                </el-select>
                <el-button
                  type="primary"
                  class="gallery-apply-btn"
                  @click="doApply"
                  :loading="isApplying"
                >
                  确定
                </el-button>
              </div>
            </section>

            <section class="viewer-section mt-auto pb-0 border-b-0">
              <el-button type="danger" plain class="w-full rounded-lg" @click="confirmDelete">
                删除纹理
              </el-button>
            </section>
          </template>
        </div>
      </div>
    </el-dialog>

    <!-- 上传对话框 -->
    <el-dialog
      v-model="showUploadDialog"
      title="上传纹理"
      class="upload-dialog dialog-form"
      append-to-body
    >
      <el-form label-width="100px" :model="uploadForm" class="upload-form">
        <el-form-item label="选择文件" class="upload-form-item">
          <el-upload
            ref="uploadRef"
            :auto-upload="false"
            :limit="1"
            accept=".png"
            :on-change="handleFileChange"
            drag
            class="upload-wrapper"
          >
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">将 PNG 文件拖到此处，或<em>点击上传</em></div>
            <template #tip>
              <div class="el-upload__tip">仅支持 PNG 格式的皮肤文件</div>
            </template>
          </el-upload>
        </el-form-item>
        <el-form-item label="纹理类型">
          <el-select v-model="uploadForm.texture_type" placeholder="选择类型" class="w-full">
            <el-option label="皮肤 (Skin)" value="skin" />
            <el-option label="披风 (Cape)" value="cape" />
          </el-select>
        </el-form-item>
        <el-form-item label="皮肤模型" v-if="uploadForm.texture_type === 'skin'">
          <el-select v-model="uploadForm.model" placeholder="选择模型" class="w-full">
            <el-option label="普通 (4px 手臂)" value="default" />
            <el-option label="纤细 (3px 手臂)" value="slim" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="uploadForm.note" placeholder="给这个纹理添加备注（可选）" />
        </el-form-item>
        <el-form-item label="是否公开">
          <el-switch v-model="uploadForm.is_public" />
          <el-text size="small" type="info" class="ml-3">
            公开后其他用户可以在皮肤库中看到并使用
          </el-text>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showUploadDialog = false">取消</el-button>
        <el-button type="primary" @click="doUpload">
          <el-icon><Upload /></el-icon>
          确认上传
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, inject } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { UploadInstance, UploadFile } from 'element-plus'
import type { Ref } from 'vue'
import { Upload, UploadFilled, Edit } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import TextureCard from '@/components/textures/TextureCard.vue'
import TexturePreviewStage from '@/components/textures/TexturePreviewStage.vue'
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
const noteInputRef = ref<{ focus: () => void } | null>(null)

const showUploadDialog = ref(false)
const uploadForm = ref<{
  texture_type: string
  model: string
  note: string
  is_public: boolean
  file: File | null
}>({ texture_type: 'skin', model: 'default', note: '', is_public: false, file: null })
const uploadRef = ref<UploadInstance | null>(null)
const applyForm = ref({ profile_id: '', texture_type: '', hash: '' })

function texturesUrl(hash: string | null | undefined) {
  if (!hash) return ''
  const base = import.meta.env.BASE_URL
  return `${base}static/textures/${hash}.png`.replace(/\/+/g, '/')
}

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

function focusNoteInput() {
  noteInputRef.value?.focus()
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
  } catch (e) {
    ElMessage.error('更新备注失败')
  }
}

async function updateModel(val: string | number | boolean | undefined) {
  if (!selectedTexture.value || isDetailLoading.value) return
  const tex = selectedTexture.value
  try {
    await patchTexture(tex.hash, tex.type, { model: String(val) })
    tex.model = String(val)
    const localTex = textures.value.find((t) => t.hash === tex.hash && t.type === tex.type)
    if (localTex) localTex.model = String(val)
    ElMessage.success(`模型已切换为 ${val === 'slim' ? '纤细' : '普通'}`)
  } catch (e) {
    ElMessage.error('切换模型失败')
  }
}

async function updateIsPublic(val: string | number | boolean) {
  if (!selectedTexture.value || isDetailLoading.value) return
  const tex = selectedTexture.value
  try {
    await patchTexture(tex.hash, tex.type, { is_public: val === 1 })
    ElMessage.success(val === 1 ? '材质已公开' : '材质已设为私有')
  } catch (e) {
    ElMessage.error('更新公开状态失败')
    tex.is_public = val === 1 ? 0 : 1
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
    textures.value.forEach((tex) => {
      if (tex.type === 'skin') {
        loadTextureResolution(tex.hash)
      }
    })
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
  textures.value.forEach((tex) => {
    if (tex.type === 'skin') {
      loadTextureResolution(tex.hash)
    }
  })
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function handlePrevPage() {
  await pagination.goToPrevPage(async (cursor, pageLimit) => {
    const params = { cursor, limit: pageLimit }
    const res = await getTextures(params)
    textures.value = res.data.items
    return res.data
  })
  textures.value.forEach((tex) => {
    if (tex.type === 'skin') {
      loadTextureResolution(tex.hash)
    }
  })
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function refreshFirstPage() {
  pagination.reset()
  await fetchTextures()
}

function loadTextureResolution(hash: string) {
  const img = new Image()
  img.crossOrigin = 'anonymous'
  img.onload = () => {
    textureResolutions.value.set(hash, img.width)
  }
  img.src = texturesUrl(hash)
}

function handleFileChange(file: UploadFile) {
  uploadForm.value.file = file.raw ?? null
}

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
    uploadForm.value = {
      texture_type: 'skin',
      model: 'default',
      note: '',
      is_public: false,
      file: null,
    }
    if (uploadRef.value) {
      uploadRef.value.clearFiles()
    }
    await refreshFirstPage()
  } catch (e: any) {
    ElMessage.error('上传失败: ' + (e.response?.data?.detail || e.message))
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
  } catch (e: any) {
    ElMessage.error('应用失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    isApplying.value = false
  }
}

onMounted(() => {
  refreshFirstPage()
  fetchUserProfiles()
})
</script>

<style scoped>
.title-section {
  padding-top: 0;
}

.gallery-select {
  flex: 1;
}

.gallery-select :deep(.el-input__wrapper) {
  border-radius: 8px;
  border: 1px solid var(--color-border);
  background: var(--color-background-soft);
  box-shadow: none !important;
}

.gallery-apply-btn {
  min-width: 90px;
  border-radius: 8px;
}

/* Upload Dialog Styles */
.upload-dialog :deep(.el-upload-dragger) {
  width: 100%;
}
.upload-wrapper {
  width: 100%;
}
</style>
