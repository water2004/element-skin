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
      <div class="common-card" v-for="(tex, index) in textures" :key="tex.hash + tex.type" :style="{ '--delay-index': index }">
        <div class="texture-preview" :style="{ background: isDark ? 'var(--color-background-hero-dark)' : 'var(--color-background-hero-light)' }">
          <SkinViewer
            v-if="tex.type === 'skin'"
            :skinUrl="texturesUrl(tex.hash)"
            :width="200"
            :height="280"
            @load="handleTextureLoad(tex.hash)"
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
        <div class="texture-info">
          <div class="texture-type-badge" :class="tex.type">
            {{ tex.type === 'skin' ? '皮肤' : '披风' }}
          </div>
          <div class="texture-note" @click="startEditNote(tex)" v-if="editingNoteHash !== tex.hash">
            {{ tex.note || '无备注' }}
          </div>
          <el-input
            v-else
            v-model="editingNoteValue"
            placeholder="输入备注，最多200字"
            size="default"
            class="texture-note-input"
            autofocus
            @blur="finishEditNote(tex)"
            @keyup.enter="finishEditNote(tex)"
          />
        </div>
        <div class="texture-actions">
          <el-button class="action-btn action-btn-primary" @click="openApplyDialog(tex)">
            <el-icon><Check /></el-icon>
            <span>使用</span>
          </el-button>
          <el-button class="action-btn action-btn-danger" @click="deleteMyTexture(tex.hash, tex.type)">
            <span class="btn-content">
              <el-icon class="btn-icon"><Delete /></el-icon>
              <span class="btn-label">删除</span>
            </span>
          </el-button>
        </div>
      </div>
    </div>

    <el-empty v-else description="还没有纹理，快去上传吧！" />

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
      </el-form>
      <template #footer>
        <el-button @click="showUploadDialog = false">取消</el-button>
        <el-button type="primary" @click="doUpload">
          <el-icon><Upload /></el-icon>
          确认上传
        </el-button>
      </template>
    </el-dialog>

    <!-- 应用纹理对话框 -->
    <el-dialog v-model="showApplyDialog" title="应用纹理到角色" width="450px">
      <el-form label-width="100px" :model="applyForm">
        <el-form-item label="选择角色">
          <el-select v-model="applyForm.profile_id" placeholder="选择要应用的角色" style="width:100%">
            <el-option
              v-for="p in userProfiles || []"
              :key="p.id"
              :label="p.name"
              :value="p.id"
            >
              <span>{{ p.name }}</span>
              <span style="float:right; color: #8492a6; font-size: 13px">{{ p.model || 'default' }}</span>
            </el-option>
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showApplyDialog = false">取消</el-button>
        <el-button type="primary" @click="doApply">
          <el-icon><Check /></el-icon>
          确认应用
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, inject, computed } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'
import { Upload, UploadFilled, Check, Delete } from '@element-plus/icons-vue'
import SkinViewer from '@/components/SkinViewer.vue'
import CapeViewer from '@/components/CapeViewer.vue'

// Inject shared state from AppLayout
const user = inject('user')
const fetchMe = inject('fetchMe')
const userProfiles = computed(() => user.value?.profiles || [])
const isDark = inject('isDark')

const textures = ref([])
const textureResolutions = ref(new Map())
const editingNoteHash = ref('')
const editingNoteValue = ref('')
const showUploadDialog = ref(false)
const uploadForm = ref({ texture_type: 'skin', model: 'default', note: '', file: null })
const uploadRef = ref(null)
const showApplyDialog = ref(false)
const applyForm = ref({ profile_id: '', texture_type: '', hash: '' })

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

function texturesUrl(hash) {
  if (!hash) return ''
  return (import.meta.env.VITE_API_BASE || '') + '/static/textures/' + hash + '.png'
}

function startEditNote(tex){
  editingNoteHash.value = tex.hash
  editingNoteValue.value = tex.note || ''
}

async function finishEditNote(tex){
  const original = tex.note || ''
  const updated = editingNoteValue.value || ''
  editingNoteHash.value = ''
  editingNoteValue.value = ''
  if (updated === original) return
  try {
    await axios.patch(`/me/textures/${tex.hash}/${tex.type}`, { note: updated }, { headers: authHeaders() })
    tex.note = updated
    ElMessage.success('备注已更新')
  } catch (e) {
    console.error('update note error:', e)
    ElMessage.error('更新备注失败')
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

function handleTextureLoad(hash) {
  // Callback placeholder
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

  try {
    await axios.post('/me/textures', formData, { headers: { ...authHeaders(), 'Content-Type': 'multipart/form-data' } })
    ElMessage.success('上传成功')
    showUploadDialog.value = false
    uploadForm.value = { texture_type: 'skin', model: 'default', note: '', file: null }
    if (uploadRef.value) {
      uploadRef.value.clearFiles()
    }
    fetchTextures()
  } catch (e) {
    ElMessage.error('上传失败: ' + (e.response?.data?.detail || e.message))
  }
}

async function deleteMyTexture(hash, type) {
  try {
    await axios.delete(`/me/textures/${hash}/${type}`, { headers: authHeaders() })
    ElMessage.success('已删除')
    fetchTextures()
  } catch (e) {
    ElMessage.error('删除失败')
  }
}

function openApplyDialog(tex) {
  applyForm.value.hash = tex.hash
  applyForm.value.texture_type = tex.type
  applyForm.value.profile_id = ''
  showApplyDialog.value = true
}

async function doApply() {
  if (!applyForm.value.profile_id) return ElMessage.error('请选择角色')
  try {
    await axios.post(`/me/textures/${applyForm.value.hash}/apply`, {
      profile_id: applyForm.value.profile_id,
      texture_type: applyForm.value.texture_type
    }, { headers: authHeaders() })
    ElMessage.success('已应用')
    showApplyDialog.value = false
    fetchMe() // Refresh parent (user profiles)
    fetchTextures()
  } catch (e) {
    ElMessage.error('应用失败: ' + (e.response?.data?.detail || e.message))
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

.texture-preview {
  width: 100%;
  height: 280px;
  display: flex;
  justify-content: center;
  align-items: center;
  position: relative;
  overflow: hidden;
  transition: background 0.3s ease; /* Add transition for smooth theme change */
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

.texture-info {
  padding: 16px;
  text-align: center;
  background: var(--color-card-background);
}

.texture-type-badge {
  display: inline-block;
  padding: 6px 14px;
  border-radius: 14px;
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 10px;
  letter-spacing: 0.5px;
}

.texture-type-badge.skin {
  background: rgba(64, 158, 255, 0.1);
  color: #409eff;
}

.texture-type-badge.cape {
  background: rgba(103, 194, 58, 0.1);
  color: #67c23a;
}

.texture-note {
  font-size: 14px;
  color: var(--color-text);
  min-height: 22px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 4px;
  transition: all 0.2s ease;
}

.texture-note:hover {
  background: var(--color-background-soft);
  color: #409eff;
}

.texture-note-input {
  margin-top: 4px;
}

.texture-actions {
  display: flex;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid var(--color-border);
  background: var(--color-background-soft);
}

.texture-actions .el-button {
  flex: 1;
}

.action-btn {
  border: none;
  font-weight: 500;
  transition: all 0.3s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.action-btn span {
  font-size: 14px;
}

.action-btn-primary {
  background: linear-gradient(135deg, #409eff 0%, #5cadff 100%);
  color: #fff;
}

.action-btn-primary:hover {
  background: linear-gradient(135deg, #66b1ff 0%, #79bbff 100%);
  transform: translateY(-2px) scale(1.02);
  box-shadow: 0 6px 20px rgba(64, 158, 255, 0.4);
}

.action-btn-danger {
  background: linear-gradient(135deg, #f56c6c 0%, #f78989 100%);
  color: #fff;
  position: relative;
  overflow: hidden;
}

.action-btn-danger .btn-content {
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
}

.action-btn-danger .btn-label {
  margin: 0;
  grid-area: 1 / 1;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.action-btn-danger .btn-icon {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%) scale(0.6) rotate(-90deg);
  opacity: 0;
  transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
  font-size: 16px;
  pointer-events: none;
}

.action-btn-danger:hover .btn-label {
  opacity: 0;
  transform: translateY(8px) scale(0.8);
}

.action-btn-danger:hover .btn-icon {
  opacity: 1;
  transform: translate(-50%, -50%) scale(1) rotate(0deg);
}

.action-btn-danger:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(245, 108, 108, 0.25);
}

/* Upload Dialog Styles */
.upload-dialog :deep(.el-upload-dragger) {
  width: 100%;
}
.upload-wrapper {
  width: 100%;
}
</style>