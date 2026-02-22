<template>
  <div class="wardrobe-section">
    <div class="section-header">
      <h2>我的衣柜</h2>
      <el-button type="primary" @click="showUploadDialog = true" size="large">
        <el-icon><Upload /></el-icon>
        <span style="margin-left:8px">上传纹理</span>
      </el-button>
    </div>

    <div class="common-grid" v-if="textures.length > 0">
      <div
        class="common-card clickable-card"
        v-for="(tex, index) in textures"
        :key="tex.hash + tex.type"
        :style="{ '--delay-index': index }"
        @click="openDetailDialog(tex)"
      >
        <div class="texture-preview" :style="{ background: isDark ? 'var(--color-background-hero-dark)' : 'var(--color-background-hero-light)' }">
          <SkinViewer
            v-if="tex.type === 'skin'"
            :skinUrl="texturesUrl(tex.hash)"
            :model="tex.model || 'default'"
            :width="200"
            :height="280"
          />
          <CapeViewer
            v-else
            :capeUrl="texturesUrl(tex.hash)"
            :width="200"
            :height="280"
          />
          <!-- 皮肤分辨率标签 -->
          <div
            v-if="tex.type === 'skin' && textureResolutions.get(tex.hash)"
            class="resolution-badge"
            :style="getResolutionBadgeStyle(textureResolutions.get(tex.hash))"
          >
            {{ textureResolutions.get(tex.hash) }}x
          </div>
        </div>
        <div class="texture-info-simple">
          <div class="texture-type-tag" :class="tex.type">
            {{ tex.type === 'skin' ? '皮肤' : '披风' }}
          </div>
          <div class="texture-note-simple">{{ tex.note || '未命名纹理' }}</div>
        </div>
      </div>
    </div>

    <el-empty v-else description="还没有纹理，快去上传吧！" />

    <el-dialog
      v-model="showDetailDialog"
      width="800px"
      destroy-on-close
      class="gallery-dialog"
    >
      <div class="gallery-container" v-if="selectedTexture">
        <div class="gallery-stage">
          <SkinViewer
            v-if="selectedTexture.type === 'skin'"
            :skinUrl="texturesUrl(selectedTexture.hash)"
            :model="selectedTexture.model || 'default'"
            :width="320"
            :height="430"
          />
          <CapeViewer
            v-else  
            :capeUrl="texturesUrl(selectedTexture.hash)"
            :width="320"
            :height="430"
          />
        </div>

        <div class="gallery-info" v-loading="isDetailLoading">
          <template v-if="selectedTexture">
            <section class="info-section title-section">
            <div class="title-row">
              <el-button text circle class="title-edit-btn" @click="focusNoteInput">
                <el-icon><Edit /></el-icon>
              </el-button>
              <el-input
                ref="noteInputRef"
                v-model="editingNoteValue"
                placeholder="未命名纹理"
                class="gallery-note-input"
                @blur="updateNote"
                @keyup.enter="updateNote"
              />
            </div>
          </section>

          <section class="info-section meta-section">
            <span class="meta-chip">{{ textureResolutions.get(selectedTexture.hash) || '--' }}px</span>
            <span class="meta-chip hash-chip">{{ selectedTexture.hash }}</span>
          </section>

          <section class="info-section" v-if="selectedTexture.type === 'skin'">
            <div class="section-label">模型选择</div>
            <el-radio-group v-model="selectedTexture.model" @change="updateModel" class="capsule-radio">
              <el-radio-button value="default">Default</el-radio-button>
              <el-radio-button value="slim">Slim</el-radio-button>
            </el-radio-group>
          </section>

          <section class="info-section" v-if="!isDetailLoading && selectedTexture.is_public !== 2">
            <div class="section-label">公开状态</div>
            <div class="public-toggle-row">
              <el-switch
                v-model="selectedTexture.is_public"
                :active-value="1"
                :inactive-value="0"
                @change="updateIsPublic"
              />
              <span class="public-status-text">
                {{ selectedTexture.is_public === 1 ? '公开（其他用户可在皮肤库看到）' : '私有（仅自己可见）' }}
              </span>
            </div>
          </section>

          <section class="info-section">
            <div class="section-label">应用到角色</div>
            <div class="apply-row">
              <el-select v-model="applyForm.profile_id" placeholder="选择目标" class="gallery-select">
                <el-option
                  v-for="p in userProfiles || []"
                  :key="p.id"
                  :label="p.name"
                  :value="p.id"
                />
              </el-select>
              <el-button type="primary" class="gallery-apply-btn" @click="doApply" :loading="isApplying">
                确定
              </el-button>
            </div>
          </section>

          <section class="info-section footer-section">
            <el-button type="danger" plain class="gallery-delete-btn" @click="confirmDelete">
              删除纹理
            </el-button>
          </section>
          </template>
        </div>
      </div>
    </el-dialog>

    <!-- 上传对话框 -->
    <el-dialog v-model="showUploadDialog" title="上传纹理" width="500px" class="upload-dialog">
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
            <div class="el-upload__text">
              将 PNG 文件拖到此处，或<em>点击上传</em>
            </div>
            <template #tip>
              <div class="el-upload__tip">
                仅支持 PNG 格式的皮肤文件
              </div>
            </template>
          </el-upload>
        </el-form-item>
        <el-form-item label="纹理类型">
          <el-select v-model="uploadForm.texture_type" placeholder="选择类型" style="width:100%">
            <el-option label="皮肤 (Skin)" value="skin" />
            <el-option label="披风 (Cape)" value="cape" />
          </el-select>
        </el-form-item>
        <el-form-item label="皮肤模型" v-if="uploadForm.texture_type === 'skin'">
          <el-select v-model="uploadForm.model" placeholder="选择模型" style="width:100%">
            <el-option label="普通 (4px 手臂)" value="default" />
            <el-option label="纤细 (3px 手臂)" value="slim" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="uploadForm.note" placeholder="给这个纹理添加备注（可选）" />
        </el-form-item>
        <el-form-item label="是否公开">
          <el-switch v-model="uploadForm.is_public" />
          <el-text size="small" type="info" style="margin-left: 12px">
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

<script setup>
import { ref, onMounted, inject, computed } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Upload, UploadFilled, Edit } from '@element-plus/icons-vue'
import SkinViewer from '@/components/SkinViewer.vue'
import CapeViewer from '@/components/CapeViewer.vue'

// Inject shared state from AppLayout
const user = inject('user')
const fetchMe = inject('fetchMe')
const userProfiles = computed(() => user.value?.profiles || [])
const isDark = inject('isDark')

const textures = ref([])
const textureResolutions = ref(new Map())
const showDetailDialog = ref(false)
const selectedTexture = ref(null)
const isDetailLoading = ref(false)
const editingNoteValue = ref('')
const isApplying = ref(false)
const noteInputRef = ref(null)

const showUploadDialog = ref(false)
const uploadForm = ref({ texture_type: 'skin', model: 'default', note: '', is_public: false, file: null })
const uploadRef = ref(null)
const applyForm = ref({ profile_id: '', texture_type: '', hash: '' })

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

function texturesUrl(hash) {
  if (!hash) return ''
  return (import.meta.env.VITE_API_BASE || '') + '/static/textures/' + hash + '.png'
}

async function openDetailDialog(tex) {
  // 先设置基础信息用于预览展示
  // 显式设置 is_public 为 2 (隐藏)，直到详情加载完成，防止 el-switch 状态错误
  selectedTexture.value = { ...tex, is_public: 2 }
  editingNoteValue.value = tex.note || ''
  applyForm.value.hash = tex.hash
  applyForm.value.texture_type = tex.type
  applyForm.value.profile_id = ''
  
  showDetailDialog.value = true
  isDetailLoading.value = true
  
  try {
    const res = await axios.get(`/me/textures/${tex.hash}/${tex.type}`, { headers: authHeaders() })
    // 更新为完整信息（包含 is_public 等）
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
    await axios.patch(`/me/textures/${tex.hash}/${tex.type}`, { note: updated }, { headers: authHeaders() })
    tex.note = updated
    // 同步更新本地列表，避免重新获取
    const localTex = textures.value.find(t => t.hash === tex.hash && t.type === tex.type)
    if (localTex) localTex.note = updated
    ElMessage.success('备注已更新')
  } catch (e) {
    ElMessage.error('更新备注失败')
  }
}

async function updateModel(val) {
  if (!selectedTexture.value || isDetailLoading.value) return
  const tex = selectedTexture.value
  try {
    await axios.patch(`/me/textures/${tex.hash}/${tex.type}`, { model: val }, { headers: authHeaders() })
    tex.model = val
    // 同步更新本地列表
    const localTex = textures.value.find(t => t.hash === tex.hash && t.type === tex.type)
    if (localTex) localTex.model = val
    ElMessage.success(`模型已切换为 ${val === 'slim' ? '纤细' : '普通'}`)
  } catch (e) {
    ElMessage.error('切换模型失败')
  }
}

async function updateIsPublic(val) {
  // 数据加载中或对象为空时，拒绝任何更新操作
  if (!selectedTexture.value || isDetailLoading.value) return
  
  const tex = selectedTexture.value
  try {
    await axios.patch(`/me/textures/${tex.hash}/${tex.type}`, { is_public: val === 1 }, { headers: authHeaders() })
    ElMessage.success(val === 1 ? '材质已公开' : '材质已设为私有')
  } catch (e) {
    ElMessage.error('更新公开状态失败')
    // 恢复原状态
    tex.is_public = val === 1 ? 0 : 1
  }
}

async function fetchTextures() {
  try {
    const res = await axios.get('/me/textures', { headers: authHeaders() })
    textures.value = res.data
    textures.value.forEach(tex => {
      if (tex.type === 'skin') {
        loadTextureResolution(tex.hash)
      }
    })
  } catch (e) {
    console.error(e)
  }
}

function loadTextureResolution(hash) {
  const img = new Image()
  img.crossOrigin = 'anonymous'
  img.onload = () => {
    const resolution = img.width
    textureResolutions.value.set(hash, resolution)
  }
  img.src = texturesUrl(hash)
}

function getResolutionBadgeStyle(resolution) {
  let hue = 0
  if (resolution <= 64) {
    hue = 120
  } else if (resolution <= 128) {
    hue = 120 - ((resolution - 64) / 64) * 60
  } else if (resolution <= 256) {
    hue = 60 - ((resolution - 128) / 128) * 30
  } else if (resolution <= 512) {
    hue = 30 - ((resolution - 256) / 256) * 30
  } else {
    hue = 330
  }

  const saturation = 58
  const lightness = 65

  return {
    background: `linear-gradient(135deg, hsl(${hue}, ${saturation}%, ${lightness}%), hsl(${hue + 15}, ${saturation - 5}%, ${lightness - 3}%))`,
    boxShadow: `0 2px 6px hsla(${hue}, ${saturation}%, ${lightness - 15}%, 0.25)`
  }
}

function handleFileChange(file) {
  uploadForm.value.file = file.raw
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
    await axios.post('/me/textures', formData, { headers: { ...authHeaders(), 'Content-Type': 'multipart/form-data' } })
    ElMessage.success('上传成功')
    showUploadDialog.value = false
    uploadForm.value = { texture_type: 'skin', model: 'default', note: '', is_public: false, file: null }
    if (uploadRef.value) {
      uploadRef.value.clearFiles()
    }
    fetchTextures()
  } catch (e) {
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
      confirmButtonClass: 'el-button--danger'
    })

    await axios.delete(`/me/textures/${selectedTexture.value.hash}/${selectedTexture.value.type}`, { headers: authHeaders() })
    ElMessage.success('已删除')
    showDetailDialog.value = false
    fetchTextures()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('删除失败')
  }
}

async function doApply() {
  if (!applyForm.value.profile_id) return ElMessage.error('请选择角色')
  isApplying.value = true
  try {
    await axios.post(`/me/textures/${applyForm.value.hash}/apply`, {
      profile_id: applyForm.value.profile_id,
      texture_type: applyForm.value.texture_type
    }, { headers: authHeaders() })
    ElMessage.success('已应用')
    fetchMe() // Refresh parent (user profiles)
    fetchTextures()
  } catch (e) {
    ElMessage.error('应用失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    isApplying.value = false
  }
}

onMounted(() => {
  fetchTextures()
})
</script>

<style scoped>
.wardrobe-section {
  animation: fadeIn 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

.common-grid {
  justify-content: center;
}

.clickable-card {
  cursor: pointer;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.clickable-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--el-box-shadow-light);
}

.texture-preview {
  width: 100%;
  height: 280px;
  display: flex;
  justify-content: center;
  align-items: center;
  position: relative;
  overflow: hidden;
}

.resolution-badge {
  position: absolute;
  top: 8px;
  right: 8px;
  padding: 4px 10px;
  border-radius: 6px;
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  backdrop-filter: blur(4px);
  animation: badgeFadeIn 0.5s cubic-bezier(0.4, 0, 0.2, 1) 0.3s backwards;
  z-index: 10;
}

.texture-info-simple {
  padding: 12px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  background: var(--color-card-background);
}

.texture-note-simple {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
  max-width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  text-align: center;
}

.texture-type-tag {
  font-size: 10px;
  padding: 1px 8px;
  border-radius: 10px;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.texture-type-tag.skin {
  background: rgba(64, 158, 255, 0.1);
  color: #409eff;
}

.texture-type-tag.cape {
  background: rgba(103, 194, 58, 0.1);
  color: #67c23a;
}

.gallery-dialog :deep(.el-dialog__headerbtn) {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 10;
  width: 32px;
  height: 32px;
}

.gallery-container {
  display: grid;
  grid-template-columns: 1fr 1fr;
  min-height: 560px;
}

.gallery-stage {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 560px;
  background: var(--color-background-hero-light);
  transition: background 0.3s ease;
}

:global(html.dark) .gallery-stage {
  background: var(--color-background-hero-dark);
}

.gallery-info {
  background: var(--color-card-background);
  padding: 24px;
  display: flex;
  flex-direction: column;
  min-height: 560px;
  border-left: 1px solid var(--el-border-color-lighter);
}

.info-section {
  padding: 14px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.title-section {
  padding-top: 0;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.gallery-note-input {
  flex: 1;
}

.gallery-note-input :deep(.el-input__wrapper) {
  box-shadow: none !important;
  background: transparent !important;
  padding: 0;
}

.gallery-note-input :deep(.el-input__inner) {
  height: 44px;
  font-size: 28px;
  font-weight: 700;
  color: var(--color-text);
}

.title-edit-btn {
  color: var(--el-text-color-secondary);
}

.meta-section {
  display: flex;
  align-items: center;
  gap: 8px;
}

.meta-chip {
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  height: 28px;
  padding: 0 12px;
  border-radius: 999px;
  border: 1px solid var(--el-border-color-lighter);
  background: var(--color-background-soft);
  font-size: 12px;
  color: var(--el-text-color-regular);
}

.hash-chip {
  max-width: 240px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.section-label {
  font-size: 12px;
  font-weight: 700;
  color: var(--el-text-color-regular);
  margin-bottom: 10px;
}

.capsule-radio :deep(.el-radio-button__inner) {
  border-radius: 8px !important;
  margin-right: 6px;
  border: 1px solid var(--el-border-color-lighter) !important;
  background: transparent;
  font-weight: 500;
}

.capsule-radio :deep(.el-radio-button__orig-radio:checked + .el-radio-button__inner) {
  background: var(--el-color-primary) !important;
  border-color: var(--el-color-primary) !important;
  color: #fff !important;
}

.apply-row {
  display: flex;
  gap: 8px;
}

.public-toggle-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.public-status-text {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.gallery-select {
  flex: 1;
}

.gallery-select :deep(.el-input__wrapper) {
  border-radius: 8px;
  border: 1px solid var(--el-border-color-lighter);
  background: var(--color-background-soft);
  box-shadow: none !important;
}

.gallery-apply-btn {
  min-width: 90px;
  border-radius: 8px;
}

.footer-section {
  margin-top: auto;
  border-bottom: 0;
  padding-bottom: 0;
}

.gallery-delete-btn {
  width: 100%;
  border-radius: 8px;
}

@media (max-width: 900px) {
  .gallery-dialog {
    --el-dialog-width: 94vw;
  }

  .gallery-container {
    grid-template-columns: 1fr;
  }

  .gallery-stage {
    min-height: 340px;
  }

  .gallery-info {
    min-height: auto;
    border-left: 0;
    border-top: 1px solid var(--el-border-color-lighter);
    padding: 16px;
  }

  .hash-chip {
    max-width: 100%;
  }
}

/* Upload Dialog Styles */
.upload-dialog :deep(.el-upload-dragger) {
  width: 100%;
}
.upload-wrapper {
  width: 100%;
}
</style>

<!-- 全局样式：穿透 Teleport 渲染的 el-dialog -->
<style>
/* 针对 gallery-dialog 的全局覆盖，解决 Teleport 导致 scoped 样式失效问题 */
.gallery-dialog.el-dialog {
  padding: 0 !important;
  --el-dialog-padding-primary: 0;
  border-radius: 14px !important;
  overflow: hidden !important;
  border: 1px solid var(--el-border-color-lighter) !important;
}

.gallery-dialog.el-dialog .el-dialog__header {
  padding: 0 !important;
  margin: 0 !important;
  height: 0 !important;
  min-height: 0 !important;
  overflow: visible !important;
}

.gallery-dialog.el-dialog .el-dialog__body {
  padding: 0 !important;
  margin: 0 !important;
}

.gallery-dialog.el-dialog .el-dialog__headerbtn {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 10;
}
</style>