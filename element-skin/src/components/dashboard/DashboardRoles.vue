<template>
  <div class="roles-section animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <h1>角色管理</h1>
        <p>创建并管理您的 Minecraft 角色身份</p>
      </div>
      <div class="page-header-actions">
        <el-button size="large" @click="startMicrosoftAuth" class="btn-gradient btn-gradient-success">
          <el-icon><Connection /></el-icon>
          <span style="margin-left:8px">绑定正版角色</span>
        </el-button>
        <el-button size="large" @click="showCreateRoleDialog = true" class="btn-gradient btn-gradient-primary">
          <el-icon><Plus /></el-icon>
          <span style="margin-left:8px">新建角色</span>
        </el-button>
      </div>
    </div>

    <div class="auto-grid">
      <div v-for="(profile, index) in user?.profiles || []" :key="profile.id" class="surface-card hoverable animate-card-slide" :style="{ '--delay-index': index }">
        <div
          class="role-preview"
          :style="{ background: isDark ? 'var(--color-background-hero-dark)' : 'var(--color-background-hero-light)' }"
        >
          <SkinViewer
            v-if="profile.skin_hash"
            :skinUrl="texturesUrl(profile.skin_hash)"
            :capeUrl="profile.cape_hash ? texturesUrl(profile.cape_hash) : null"
            :model="profile.model || 'default'"
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
            class="btn-gradient btn-gradient-danger btn-icon-swap"
            @click="deleteRole(profile.id)"
            size="default"
          >
            <span class="btn-label">删除</span>
            <el-icon class="btn-icon"><Delete /></el-icon>
          </el-button>

          <el-button
            v-if="profile.skin_hash"
            class="btn-soft-warning btn-icon-swap"
            @click="clearRoleSkin(profile.id)"
            size="default"
          >
            <span class="btn-label">皮肤</span>
            <el-icon class="btn-icon"><Close /></el-icon>
          </el-button>

          <el-button
            v-if="profile.cape_hash"
            class="btn-soft-warning btn-icon-swap"
            @click="clearRoleCape(profile.id)"
            size="default"
          >
            <span class="btn-label">披风</span>
            <el-icon class="btn-icon"><Close /></el-icon>
          </el-button>
        </div>
      </div>
    </div>

    <!-- 新建角色对话框 -->
    <el-dialog v-model="showCreateRoleDialog" title="新建角色" width="420px" append-to-body>
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
      append-to-body
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
  const base = import.meta.env.BASE_URL
  return `${base}static/textures/${hash}.png`.replace(/\/+/g, '/')
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
@import "@/assets/styles/animations.css";
@import "@/assets/styles/layout.css";
@import "@/assets/styles/buttons.css";
@import "@/assets/styles/cards.css";

.roles-section {
}

.role-preview {
  width: 100%;
  height: 280px;
  display: flex;
  justify-content: center;
  align-items: center;
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

/* Microsoft Login Specific Styles */
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
</style>