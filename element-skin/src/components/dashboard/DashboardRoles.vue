<template>
  <div class="roles-section">
    <div class="section-header">
      <h2>角色管理</h2>
      <div class="header-actions">
        <el-button type="success" size="large" @click="startMicrosoftAuth">
          <el-icon><Connection /></el-icon>
          <span style="margin-left:8px">绑定正版角色</span>
        </el-button>
        <el-button type="primary" size="large" @click="showCreateRoleDialog = true">
          <el-icon><Plus /></el-icon>
          <span style="margin-left:8px">新建角色</span>
        </el-button>
      </div>
    </div>

    <div class="common-grid">
      <div v-for="(profile, index) in user?.profiles || []" :key="profile.id" class="common-card" :style="{ '--delay-index': index }">
        <div class="role-preview" :style="{ background: isDark ? 'var(--color-background-hero-dark)' : 'var(--color-background-hero-light)' }">
          <SkinViewer
            v-if="profile.skin_hash"
            :skinUrl="texturesUrl(profile.skin_hash)"
            :capeUrl="profile.cape_hash ? texturesUrl(profile.cape_hash) : null"
            :width="200"
            :height="280"
          />
          <el-empty v-else description="未设置皮肤" :image-size="120" />
        </div>
        <div class="role-info">
          <div class="role-name">{{ profile.name }}</div>
          <div class="role-model">模型: {{ profile.model || 'default' }}</div>
        </div>
        <div class="role-actions">
          <el-button
            class="action-btn action-btn-danger"
            @click="deleteRole(profile.id)"
            size="default"
          >
            <span class="btn-content">
              <el-icon class="btn-icon"><Delete /></el-icon>
              <span class="btn-label">删除</span>
            </span>
          </el-button>

          <el-button
            v-if="profile.skin_hash"
            class="action-btn action-btn-warning"
            @click="clearRoleSkin(profile.id)"
            size="default"
          >
            <span class="btn-content">
              <el-icon class="btn-icon"><Close /></el-icon>
              <span class="btn-label">皮肤</span>
            </span>
          </el-button>

          <el-button
            v-if="profile.cape_hash"
            class="action-btn action-btn-warning"
            @click="clearRoleCape(profile.id)"
            size="default"
          >
            <span class="btn-content">
              <el-icon class="btn-icon"><Close /></el-icon>
              <span class="btn-label">披风</span>
            </span>
          </el-button>
        </div>
      </div>
    </div>

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

    <!-- 微软正版登录对话框 -->
    <el-dialog
      v-model="showMicrosoftLoginDialog"
      title="绑定正版角色"
      width="400px"
      :close-on-click-modal="false"
      :destroy-on-close="true"
      :before-close="handleMicrosoftDialogClose"
    >
      <div class="microsoft-login-content">
        <!-- 步骤2: 选择角色 (已找到) -->
        <div v-if="microsoftStep === 'select-profile' && microsoftProfile" class="step-content">
          <div class="simple-profile-info">
            <div class="info-text">
              <h3 class="profile-name">{{ microsoftProfile?.name }}</h3>
              <p class="profile-uuid">{{ formatUUID(microsoftProfile?.id || '') }}</p>
            </div>
            <div class="info-status">
               <el-tag v-if="microsoftProfile?.has_game" type="success" effect="dark">
                  拥有游戏
               </el-tag>
               <el-tag v-else type="danger" effect="dark">
                  无游戏权限
               </el-tag>
            </div>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="cancelMicrosoftLogin" :disabled="importing">取消</el-button>
          <el-button
            v-if="microsoftStep === 'select-profile'"
            type="primary"
            @click="importMicrosoftProfile"
            :loading="importing"
            :disabled="!microsoftProfile?.has_game"
          >
            确认导入
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, inject, computed } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Connection, Plus, Delete, Close, Check, Select, Warning, Download } from '@element-plus/icons-vue'
import SkinViewer from '@/components/SkinViewer.vue'

// Inject shared state from AppLayout
const user = inject('user')
const fetchMe = inject('fetchMe')
const isDark = inject('isDark')

const router = useRouter()

const showCreateRoleDialog = ref(false)
const newRoleName = ref('')
const showMicrosoftLoginDialog = ref(false)
const microsoftStep = ref('select-profile')
const microsoftProfile = ref(null)
const importing = ref(false)

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

function texturesUrl(hash) {
  if (!hash) return ''
  return (import.meta.env.VITE_API_BASE || '') + '/static/textures/' + hash + '.png'
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

async function clearRoleSkin(pid) {
  try {
    await ElMessageBox.confirm(
      '确定要清除该角色的皮肤吗？',
      '确认清除',
      { type: 'warning', confirmButtonText: '确定清除', cancelButtonText: '取消' }
    )
    await axios.delete(`/me/profiles/${pid}/skin`, { headers: authHeaders() })
    ElMessage.success('皮肤已清除')
    fetchMe()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('清除失败: ' + (e.response?.data?.detail || e.message))
    }
  }
}

async function clearRoleCape(pid) {
  try {
    await ElMessageBox.confirm(
      '确定要清除该角色的披风吗？',
      '确认清除',
      { type: 'warning', confirmButtonText: '确定清除', cancelButtonText: '取消' }
    )
    await axios.delete(`/me/profiles/${pid}/cape`, { headers: authHeaders() })
    ElMessage.success('披风已清除')
    fetchMe()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('清除失败: ' + (e.response?.data?.detail || e.message))
    }
  }
}

// 微软正版登录相关函数
function formatUUID(uuid) {
  if (uuid.length === 32) {
    return `${uuid.slice(0, 8)}-${uuid.slice(8, 12)}-${uuid.slice(12, 16)}-${uuid.slice(16, 20)}-${uuid.slice(20)}`
  }
  return uuid
}

async function startMicrosoftAuth() {
  try {
    const response = await axios.get('/microsoft/auth-url', { headers: authHeaders() })
    const authUrl = response.data.auth_url
    sessionStorage.setItem('ms_auth_state', response.data.state)
    window.location.href = authUrl
  } catch (error) {
    ElMessage.error('启动微软登录失败: ' + (error.response?.data?.detail || error.message))
  }
}

async function importMicrosoftProfile() {
  if (!microsoftProfile.value) return

  try {
    importing.value = true
    // Do NOT switch step, just show loading on button

    const skinData = microsoftProfile.value.skins?.[0]
    const capeData = microsoftProfile.value.capes?.[0]

    const importData = {
      profile_id: microsoftProfile.value.id,
      profile_name: microsoftProfile.value.name,
      skin_url: skinData?.url || null,
      skin_variant: skinData?.variant || 'classic',
      cape_url: capeData?.url || null
    }

    await axios.post('/microsoft/import-profile', importData, { headers: authHeaders() })

    ElMessage.success('正版角色导入成功！')

    showMicrosoftLoginDialog.value = false
    // Delay clearing the profile slightly to allow transition, or just leave it since dialog is destroying anyway
    // But safely clearing it prevents state leak if reopened somehow without reload (unlikely but possible)
    setTimeout(() => {
        microsoftProfile.value = null
        microsoftStep.value = 'select-profile'
    }, 300)

    // Refresh data in background
    try {
      if (fetchMe) await fetchMe()
    } catch (e) {
      console.warn('Failed to refresh user profile:', e)
    }

  } catch (error) {
    ElMessage.error('导入失败: ' + (error.response?.data?.detail || error.message))
  } finally {
    importing.value = false
  }
}

function cancelMicrosoftLogin() {
  showMicrosoftLoginDialog.value = false
  microsoftStep.value = 'select-profile'
  microsoftProfile.value = null
  importing.value = false
}

function handleMicrosoftDialogClose(done) {
  if (importing.value) {
    return; // Prevent closing while importing
  }
  cancelMicrosoftLogin()
  done()
}

onMounted(async () => {
  const urlParams = new URLSearchParams(window.location.search)
  const msToken = urlParams.get('ms_token')
  const error = urlParams.get('error')

  if (error) {
    ElMessage.error('微软登录失败: ' + error)
    router.replace({ query: {} })
  } else if (msToken) {
    try {
      const response = await axios.post('/microsoft/get-profile',
        { ms_token: msToken },
        { headers: authHeaders() }
      )

      microsoftProfile.value = response.data.profile
      microsoftProfile.value.has_game = response.data.has_game
      microsoftStep.value = 'select-profile'
      showMicrosoftLoginDialog.value = true

      ElMessage.success('授权成功！')
    } catch (e) {
      ElMessage.error('获取角色信息失败: ' + (e.response?.data?.detail || e.message))
    }
    router.replace({ query: {} })
  }
})
</script>

<style scoped>
/* Previous Styles ... */
.roles-section {
  animation: fadeIn 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

.common-grid {
  justify-content: center;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.role-preview {
  width: 100%;
  height: 280px;
  display: flex;
  justify-content: center;
  align-items: center;
  transition: background 0.3s ease; /* Add transition for smooth theme change */
}

.role-info {
  padding: 16px;
  text-align: center;
  background: var(--color-card-background);
}

.role-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-heading);
  margin-bottom: 8px;
}

.role-model {
  font-size: 13px;
  color: var(--color-text-light);
  font-weight: 500;
}

.role-actions {
  display: flex;
  flex-direction: row;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid var(--color-border);
  background: var(--color-background-soft);
  align-items: center;
}

.role-actions .el-button {
  flex: 1;
  min-width: 0;
}

/* Action Buttons Styles */
.action-btn {
  border: none;
  font-weight: 500;
  transition: all 0.3s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
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

.action-btn-warning {
  color: var(--color-text); /* User requested dark text color */
  border-color: #f3d19e;
  background: rgba(230, 162, 60, 0.1);
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
}

.action-btn-warning .btn-content {
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
}

.action-btn-warning .btn-label {
  margin: 0;
  grid-area: 1 / 1;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.action-btn-warning .btn-icon {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%) scale(0.6) rotate(-90deg);
  opacity: 0;
  transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
  font-size: 16px;
  pointer-events: none;
}

.action-btn-warning:hover .btn-label {
  opacity: 0;
  transform: translateY(8px) scale(0.8);
}

.action-btn-warning:hover .btn-icon {
  opacity: 1;
  transform: translate(-50%, -50%) scale(1) rotate(0deg);
}

.action-btn-warning:hover {
  color: #fff;
  background: linear-gradient(135deg, #ffa726 0%, #fb8c00 100%);
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(251,140,0,0.18);
}

/* Microsoft Login New Styles */
.microsoft-login-content {
  padding: 10px 0;
}

.step-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.simple-profile-info {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 20px;
  background: var(--color-background-soft);
  border-radius: 8px;
  width: 100%;
}

.info-text {
  text-align: left;
}

.profile-name {
  margin: 0 0 4px 0;
  font-size: 20px;
  color: var(--color-heading);
}

.profile-uuid {
  margin: 0;
  font-family: monospace;
  font-size: 13px;
  color: var(--color-text-light);
}

.centered-step {
  padding: 40px 0;
}

.loading-spinner {
  position: relative;
  width: 48px;
  height: 48px;
  margin-bottom: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.spinner-ring {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  border: 3px solid #EBEEF5;
  border-top-color: #409EFF;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

.spinner-icon {
  font-size: 20px;
  color: #409EFF;
}

.loading-text {
  color: #606266;
  font-size: 14px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
