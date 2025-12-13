<template>
  <div class="dashboard-container">
    <el-container style="height:100%">
      <el-aside width="220px" class="dashboard-sidebar">
        <div class="user-info">
          <el-avatar :size="60" class="user-avatar">{{ emailInitial }}</el-avatar>
          <div class="user-name">{{ user?.display_name || user?.email || '用户' }}</div>
        </div>
        <el-menu :default-active="activeRoute" mode="vertical" router class="sidebar-menu">
          <el-menu-item index="/dashboard/wardrobe">
            <el-icon><Box /></el-icon>
            <span>我的衣柜</span>
          </el-menu-item>
          <el-menu-item index="/dashboard/roles">
            <el-icon><User /></el-icon>
            <span>角色管理</span>
          </el-menu-item>
          <el-menu-item index="/dashboard/profile">
            <el-icon><Setting /></el-icon>
            <span>个人资料</span>
          </el-menu-item>
          <el-menu-item v-if="user?.is_admin" index="/admin" class="admin-menu-item">
            <el-icon><Tools /></el-icon>
            <span>管理面板</span>
          </el-menu-item>
        </el-menu>
      </el-aside>

      <el-main class="dashboard-main">
        <div v-if="active === 'wardrobe'" class="wardrobe-section">
          <div class="section-header">
            <h2>我的衣柜</h2>
            <el-button type="primary" @click="showUploadDialog = true" size="large">
              <el-icon><Upload /></el-icon>
              <span style="margin-left:8px">上传纹理</span>
            </el-button>
          </div>

          <div class="textures-grid" v-if="textures.length > 0">
            <div class="texture-card" v-for="tex in textures" :key="tex.hash + tex.type">
              <div class="texture-preview">
                <SkinViewer
                  v-if="tex.type === 'skin'"
                  :skinUrl="texturesUrl(tex.hash)"
                  :width="200"
                  :height="280"
                />
                <img v-else :src="texturesUrl(tex.hash)" class="cape-preview" />
              </div>
              <div class="texture-info">
                <div class="texture-type-badge" :class="tex.type">
                  {{ tex.type === 'skin' ? '皮肤' : '披风' }}
                </div>
                <div class="texture-note">{{ tex.note || '无备注' }}</div>
              </div>
              <div class="texture-actions">
                <el-button type="primary" size="small" @click="openApplyDialog(tex)">
                  <el-icon><Check /></el-icon>
                  使用
                </el-button>
                <el-button type="danger" size="small" @click="deleteMyTexture(tex.hash, tex.type)">
                  <el-icon><Delete /></el-icon>
                  删除
                </el-button>
              </div>
            </div>
          </div>

          <el-empty v-else description="还没有纹理，快去上传吧！" />
        </div>

        <div v-if="active === 'roles' && user" class="roles-section">
          <div class="section-header">
            <h2>角色管理</h2>
            <el-button type="primary" size="large" @click="showCreateRoleDialog = true">
              <el-icon><Plus /></el-icon>
              <span style="margin-left:8px">新建角色</span>
            </el-button>
          </div>

          <div class="roles-grid">
            <el-card v-for="profile in user.profiles || []" :key="profile.id" class="role-card">
              <div class="role-preview">
                <SkinViewer
                  v-if="profile.skin_hash"
                  :skinUrl="texturesUrl(profile.skin_hash)"
                  :capeUrl="profile.cape_hash ? texturesUrl(profile.cape_hash) : null"
                  :width="180"
                  :height="240"
                />
                <el-empty v-else description="未设置皮肤" :image-size="120" />
              </div>
              <div class="role-info">
                <h3>{{ profile.name }}</h3>
                <div class="role-model">模型: {{ profile.model || 'default' }}</div>
              </div>
              <div class="role-actions">
                <el-button type="danger" size="small" @click="deleteRole(profile.id)">
                  <el-icon><Delete /></el-icon>
                  删除
                </el-button>
              </div>
            </el-card>
          </div>
        </div>

        <div v-if="active === 'profile' && user" class="profile-section">
          <div class="section-header">
            <h2>个人资料</h2>
          </div>

          <el-card class="profile-form-card">
            <div class="profile-header">
              <el-avatar :size="72" class="profile-avatar">{{ emailInitial }}</el-avatar>
              <div class="profile-meta">
                <h3>{{ user.display_name || '未设置显示名' }}</h3>
                <p>{{ user.email }}</p>
              </div>
            </div>

            <el-divider />

            <el-form label-width="120px" :model="form" label-position="left">
              <el-form-item label="邮箱">
                <el-input v-model="form.email" placeholder="请输入邮箱" />
              </el-form-item>
              <el-form-item label="显示名">
                <el-input v-model="form.display_name" placeholder="显示名称（可选）" />
              </el-form-item>
              <el-form-item label="新密码">
                <el-input type="password" v-model="form.password" placeholder="留空则不修改密码" show-password />
              </el-form-item>
              <div class="profile-actions">
                <el-button type="primary" @click="updateProfile" size="large">
                  <el-icon><Check /></el-icon>
                  保存修改
                </el-button>
                <el-button type="danger" @click="deleteAccount" size="large" v-if="!user.is_admin">
                  <el-icon><Delete /></el-icon>
                  注销账号
                </el-button>
              </div>
            </el-form>
          </el-card>
        </div>
      </el-main>
    </el-container>

    <!-- 上传对话框 -->
    <el-dialog v-model="showUploadDialog" title="上传纹理" width="500px" class="upload-dialog">
      <el-form label-width="100px" :model="uploadForm">
        <el-form-item label="选择文件">
          <el-upload
            ref="uploadRef"
            :auto-upload="false"
            :limit="1"
            accept=".png"
            :on-change="handleFileChange"
            drag
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
              v-for="p in user?.profiles || []"
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

    <!-- 新建角色对话框 -->
    <el-dialog v-model="showCreateRoleDialog" title="新建角色" width="420px">
      <el-form label-width="100px">
        <el-form-item label="角色名称">
          <el-input v-model="newRoleName" placeholder="请输入角色名称" maxlength="32" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateRoleDialog = false">取消</el-button>
        <el-button type="primary" @click="createRole">
          <el-icon><Check /></el-icon>
          创建
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Box, User, Setting, Upload, UploadFilled, Check, Delete, Plus, Tools
} from '@element-plus/icons-vue'
import SkinViewer from '@/components/SkinViewer.vue'

const route = useRoute()
const user = ref(null)
const newRoleName = ref('')
const showCreateRoleDialog = ref(false)
const form = ref({ email: '', password: '', display_name: '' })
const textures = ref([])
const showUploadDialog = ref(false)
const uploadForm = ref({ texture_type: 'skin', note: '', file: null })
const uploadRef = ref(null)
const showApplyDialog = ref(false)
const applyForm = ref({ profile_id: '', texture_type: '', hash: '' })

const emailInitial = computed(() => {
  const email = user.value?.email || user.value?.display_name || 'U'
  return email.charAt(0).toUpperCase()
})

// 根据路由计算当前激活的标签
const activeRoute = computed(() => route.path)
const active = computed(() => {
  if (route.path.includes('/roles')) return 'roles'
  if (route.path.includes('/profile')) return 'profile'
  return 'wardrobe'
})

// 监听路由变化，加载对应数据
watch(() => route.path, (newPath) => {
  if (newPath.includes('/wardrobe')) {
    fetchTextures()
  } else if (newPath.includes('/roles') || newPath.includes('/profile')) {
    if (!user.value) fetchMe()
  }
}, { immediate: true })

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

function texturesUrl(hash) {
  if (!hash) return ''
  return (import.meta.env.VITE_API_BASE || '') + '/static/textures/' + hash + '.png'
}

async function fetchMe() {
  try {
    const res = await axios.get('/me', { headers: authHeaders() })
    user.value = res.data
    form.value.email = user.value.email
    form.value.display_name = user.value.display_name || ''
  } catch (e) {
    console.error('fetchMe error:', e)
    if (e.response?.status === 401 || e.response?.status === 403) {
      ElMessage.error('登录已过期，请重新登录')
      localStorage.removeItem('jwt')
      localStorage.removeItem('accessToken')
      setTimeout(() => {
        window.location.href = '/login'
      }, 1000)
    } else {
      ElMessage.error('获取用户信息失败')
    }
  }
}

onMounted(async () => {
  // 先刷新 token 获取最新的管理员状态
  try {
    const res = await axios.post('/me/refresh-token', {}, { headers: authHeaders() })
    if (res.data.token) {
      localStorage.setItem('token', res.data.token)
    }
  } catch (e) {
    console.warn('Failed to refresh token:', e)
  }

  await fetchMe()
  if (route.path.includes('/wardrobe') || route.path === '/dashboard' || route.path === '/dashboard/') {
    fetchTextures()
  }
})

async function fetchTextures() {
  try {
    const res = await axios.get('/me/textures', { headers: authHeaders() })
    textures.value = res.data
  } catch (e) {
    console.error(e)
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
  formData.append('note', uploadForm.value.note || '')

  try {
    await axios.post('/me/textures', formData, { headers: { ...authHeaders(), 'Content-Type': 'multipart/form-data' } })
    ElMessage.success('上传成功')
    showUploadDialog.value = false
    uploadForm.value = { texture_type: 'skin', note: '', file: null }
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
    fetchMe()
    fetchTextures()
  } catch (e) {
    ElMessage.error('应用失败: ' + (e.response?.data?.detail || e.message))
  }
}

async function createRole() {
  const name = (newRoleName.value || '').trim()
  if (!name) return ElMessage.error('请输入角色名称')
  try {
    await axios.post('/me/profiles', { name }, { headers: authHeaders() })
    newRoleName.value = ''
    showCreateRoleDialog.value = false
    ElMessage.success('创建成功')
    fetchMe()
  } catch (e) {
    ElMessage.error('创建失败: ' + (e.response?.data?.detail || e.message))
  }
}

async function deleteRole(pid) {
  try {
    await axios.delete(`/me/profiles/${pid}`, { headers: authHeaders() })
    ElMessage.success('已删除')
    fetchMe()
  } catch (e) {
    ElMessage.error('删除失败')
  }
}

async function updateProfile() {
  try {
    const payload = { ...form.value }
    if (!payload.password) delete payload.password
    await axios.patch('/me', payload, { headers: authHeaders() })
    ElMessage.success('保存成功')
    fetchMe()
  } catch (e) {
    ElMessage.error('保存失败: ' + (e.response?.data?.detail || e.message))
  }
}

async function deleteAccount() {
  try {
    await ElMessageBox.confirm(
      '确定要注销账号吗？此操作将删除您的所有数据！',
      '危险操作',
      { type: 'error', confirmButtonText: '确定注销' }
    )
    await axios.delete('/me', { headers: authHeaders() })
    ElMessage.success('账号已注销')
    localStorage.removeItem('jwt')
    localStorage.removeItem('accessToken')
    setTimeout(() => {
      window.location.href = '/'
    }, 1000)
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('注销失败: ' + (e.response?.data?.detail || e.message))
    }
  }
}
</script>

<style scoped>
.dashboard-container {
  min-height: 100vh;
  background: #f5f7fa;
}

.dashboard-container :deep(.el-container) {
  min-height: 100vh;
}

.dashboard-sidebar {
  background: #fff;
  border-right: 1px solid #e4e7ed;
  padding: 20px 0;
  min-height: 100vh;
}

.user-info {
  text-align: center;
  padding: 20px;
  margin-bottom: 20px;
}

.user-avatar {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
  font-weight: bold;
  font-size: 24px;
  margin-bottom: 12px;
}

.user-name {
  font-size: 16px;
  font-weight: 500;
  color: #303133;
  margin-top: 12px;
}

.sidebar-menu {
  border: none;
}

.sidebar-menu .el-menu-item {
  height: 50px;
  line-height: 50px;
  margin: 4px 12px;
  border-radius: 8px;
  transition: all 0.3s ease;
}

.sidebar-menu .el-menu-item:hover {
  background-color: #ecf5ff;
  transform: translateX(4px);
}

.sidebar-menu .el-menu-item.is-active {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
  transform: translateX(0);
}

.sidebar-menu .admin-menu-item {
  margin-top: 20px;
  border-top: 1px solid #ebeef5;
  padding-top: 8px;
}

.sidebar-menu .admin-menu-item:hover {
  background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
  color: #fff;
}

.dashboard-main {
  padding: 30px;
  background: #f5f7fa;
  min-height: 100vh;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
  animation: fadeIn 0.4s ease-out;
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.section-header h2 {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
  margin: 0;
}

/* 衣柜样式 */
.textures-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 24px;
}

.texture-card {
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
  transition: all 0.3s ease;
}

.texture-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.12);
}

.texture-preview {
  width: 100%;
  height: 280px;
  display: flex;
  justify-content: center;
  align-items: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.cape-preview {
  max-width: 80%;
  max-height: 80%;
  object-fit: contain;
}

.texture-info {
  padding: 16px;
  text-align: center;
}

.texture-type-badge {
  display: inline-block;
  padding: 4px 12px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
  margin-bottom: 8px;
}

.texture-type-badge.skin {
  background: #ecf5ff;
  color: #409eff;
}

.texture-type-badge.cape {
  background: #f0f9ff;
  color: #67c23a;
}

.texture-note {
  font-size: 14px;
  color: #606266;
}

.texture-actions {
  display: flex;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid #ebeef5;
}

.texture-actions .el-button {
  flex: 1;
}

/* 角色管理样式 */
.create-role-card {
  margin-bottom: 24px;
}

/* 移除单独的工具栏，按钮与区块标题右侧对齐 */

.roles-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 24px;
}

.role-card {
  border-radius: 12px;
  overflow: hidden;
  transition: all 0.3s ease;
}

.role-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.12);
}

.role-preview {
  width: 100%;
  height: 240px;
  display: flex;
  justify-content: center;
  align-items: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.role-info {
  padding: 16px;
  text-align: center;
}

.role-info h3 {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin: 0 0 8px 0;
}

.role-model {
  font-size: 13px;
  color: #909399;
}

.role-actions {
  padding: 12px 16px;
  border-top: 1px solid #ebeef5;
  text-align: center;
}

/* 个人资料样式 */
.profile-form-card {
  max-width: 600px;
  margin: 0 auto;
  padding: 30px;
}

.profile-header {
  display: flex;
  align-items: center;
  gap: 16px;
}

.profile-meta h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: #303133;
}

.profile-meta p {
  margin: 6px 0 0;
  color: #909399;
  font-size: 13px;
}

.profile-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
}

/* 上传对话框样式 */
.upload-dialog :deep(.el-upload-dragger) {
  width: 100%;
}
</style>
