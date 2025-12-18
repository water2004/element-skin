<template>
  <div class="admin-container">
    <el-container style="height:100%">
      <el-aside width="220px" class="admin-sidebar">
        <div class="admin-title">
          <el-icon size="24"><Tools /></el-icon>
          <span>管理面板</span>
        </div>
        <div class="title-divider"></div>
        <el-menu :default-active="activeRoute" mode="vertical" router class="sidebar-menu">
          <el-menu-item index="/admin/settings">
            <el-icon><Setting /></el-icon>
            <span>站点设置</span>
          </el-menu-item>
          <el-menu-item index="/admin/users">
            <el-icon><User /></el-icon>
            <span>用户管理</span>
          </el-menu-item>
          <el-menu-item index="/admin/invites">
            <el-icon><Ticket /></el-icon>
            <span>邀请码管理</span>
          </el-menu-item>
          <el-menu-item index="/dashboard/wardrobe">
            <el-icon><Back /></el-icon>
            <span>返回用户面板</span>
          </el-menu-item>
        </el-menu>
      </el-aside>

      <el-main class="admin-main">
        <!-- 站点设置 -->
        <div v-if="active === 'settings'" class="settings-section">
          <div class="section-header">
            <h2>站点设置</h2>
            <el-button type="primary" @click="loadSettings">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
          </div>

          <el-card class="settings-card">
            <el-form label-width="140px" :model="siteSettings">
              <el-form-item label="站点名称">
                <el-input v-model="siteSettings.site_name" placeholder="皮肤站" />
              </el-form-item>
              <el-form-item label="后端 API 地址">
                <el-input v-model="siteSettings.site_url" placeholder="https://skin.example.com" />
              </el-form-item>
              <el-form-item label="需要邀请码注册">
                <el-switch v-model="siteSettings.require_invite" />
              </el-form-item>
              <el-form-item label="允许用户注册">
                <el-switch v-model="siteSettings.allow_register" />
              </el-form-item>
              <el-form-item label="最大纹理大小">
                <el-input v-model="siteSettings.max_texture_size" type="number">
                  <template #suffix>KB</template>
                </el-input>
              </el-form-item>
              <el-divider content-position="left">安全设置</el-divider>
              <el-form-item label="启用速率限制">
                <el-switch v-model="siteSettings.rate_limit_enabled" />
              </el-form-item>
              <el-form-item label="登录失败限制" v-if="siteSettings.rate_limit_enabled">
                <el-input v-model="siteSettings.rate_limit_auth_attempts" type="number">
                  <template #suffix>次</template>
                </el-input>
                <el-text size="small" type="info" style="margin-top:4px">每个时间窗口内允许的最大尝试次数</el-text>
              </el-form-item>
              <el-form-item label="时间窗口" v-if="siteSettings.rate_limit_enabled">
                <el-input v-model="siteSettings.rate_limit_auth_window" type="number">
                  <template #suffix>分钟</template>
                </el-input>
                <el-text size="small" type="info" style="margin-top:4px">超限后需等待的时间</el-text>
              </el-form-item>
              <el-divider content-position="left">JWT 认证设置</el-divider>
              <el-form-item label="JWT 过期时间">
                <el-input v-model="siteSettings.jwt_expire_days" type="number">
                  <template #suffix>天</template>
                </el-input>
                <el-text size="small" type="info" style="margin-top:4px">用户登录后 Token 的有效期</el-text>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" @click="saveSettings" size="large">
                  <el-icon><Check /></el-icon>
                  保存设置
                </el-button>
              </el-form-item>
            </el-form>
          </el-card>
        </div>

        <!-- 用户管理 -->
        <div v-if="active === 'users'" class="users-section">
          <div class="section-header">
            <h2>用户管理</h2>
            <el-button type="primary" @click="refreshUsers">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
          </div>

          <el-card>
            <el-table :data="users" style="width: 100%">
              <el-table-column prop="email" label="邮箱" min-width="200" />
              <el-table-column prop="display_name" label="显示名" min-width="150" />
              <el-table-column label="管理员" width="100">
                <template #default="{ row }">
                  <el-tag v-if="row.is_admin" type="danger">是</el-tag>
                  <el-tag v-else type="info">否</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="角色数" width="100">
                <template #default="{ row }">
                  {{ row.profile_count || 0 }}
                </template>
              </el-table-column>
              <el-table-column label="操作" width="300" fixed="right">
                <template #default="{ row }">
                  <el-button
                    size="small"
                    :type="row.is_admin ? 'warning' : 'primary'"
                    @click="toggleAdmin(row)"
                  >
                    {{ row.is_admin ? '取消管理员' : '设为管理员' }}
                  </el-button>
                  <el-button
                    size="small"
                    type="warning"
                    @click="showResetPasswordDialog(row)"
                  >
                    重置密码
                  </el-button>
                  <el-button
                    size="small"
                    type="danger"
                    @click="deleteUser(row)"
                    :disabled="row.is_admin"
                  >
                    删除
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-card>
        </div>

        <!-- 邀请码管理 -->
        <div v-if="active === 'invites'" class="invites-section">
          <div class="section-header">
            <h2>邀请码管理</h2>
            <div style="display: flex; gap: 12px;">
              <el-button type="primary" @click="loadInvites">
                <el-icon><Refresh /></el-icon>
                刷新
              </el-button>
              <el-button type="success" @click="showInviteDialog">
                <el-icon><Plus /></el-icon>
                创建邀请码
              </el-button>
            </div>
          </div>

          <el-card>
            <el-table :data="invites" style="width: 100%">
              <el-table-column prop="code" label="邀请码" min-width="300">
                <template #default="{ row }">
                  <el-text copyable>{{ row.code }}</el-text>
                </template>
              </el-table-column>
              <el-table-column label="使用次数" width="150">
                <template #default="{ row }">
                  <span :style="{ color: getRemainingColor(row) }">
                    {{ row.used_count || 0 }} / {{ row.total_uses || '∞' }}
                  </span>
                  <el-tag
                    v-if="row.total_uses && row.used_count >= row.total_uses"
                    type="danger"
                    size="small"
                    style="margin-left: 8px;"
                  >
                    已用完
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="used_by" label="最后使用者" min-width="200" />
              <el-table-column label="创建时间" width="180">
                <template #default="{ row }">
                  {{ formatDate(row.created_at) }}
                </template>
              </el-table-column>
              <el-table-column label="操作" width="100" fixed="right">
                <template #default="{ row }">
                  <el-button
                    size="small"
                    type="danger"
                    @click="deleteInvite(row)"
                  >
                    删除
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-card>
        </div>
      </el-main>
    </el-container>

    <!-- 邀请码创建弹窗 -->
    <el-dialog
      v-model="inviteDialogVisible"
      title="创建邀请码"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-form label-width="100px">
        <el-form-item label="生成方式">
          <el-radio-group v-model="inviteMode">
            <el-radio value="auto">自动生成</el-radio>
            <el-radio value="manual">手动输入</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item v-if="inviteMode === 'manual'" label="邀请码">
          <el-input
            v-model="customInviteCode"
            placeholder="请输入自定义邀请码（6-32个字符）"
            maxlength="32"
            show-word-limit
          />
          <el-text size="small" type="info" style="margin-top: 8px;">
            支持字母、数字和常见符号，建议使用易记的格式
          </el-text>
        </el-form-item>

        <el-form-item v-if="inviteMode === 'auto'" label="预览">
          <el-text type="success" size="large" style="font-family: monospace;">
            {{ previewInviteCode }}
          </el-text>
          <el-button
            link
            type="primary"
            @click="refreshPreview"
            style="margin-left: 12px;"
          >
            <el-icon><Refresh /></el-icon>
            换一个
          </el-button>
        </el-form-item>

        <el-form-item label="使用次数">
          <el-radio-group v-model="inviteUsesMode" style="margin-bottom: 12px;">
            <el-radio value="limited">限制次数</el-radio>
            <el-radio value="unlimited">无限使用</el-radio>
          </el-radio-group>
          <el-input-number
            v-if="inviteUsesMode === 'limited'"
            v-model="inviteUses"
            :min="1"
            :max="1000"
            controls-position="right"
            style="width: 100%;"
          />
          <el-text v-if="inviteUsesMode === 'limited'" size="small" type="info" style="margin-top: 8px; display: block;">
            设置该邀请码可以被使用的次数
          </el-text>
          <el-text v-else size="small" type="info" style="margin-top: 8px; display: block;">
            该邀请码可以被无限次使用
          </el-text>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="inviteDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmCreateInvite" :loading="creating">
          <el-icon><Check /></el-icon>
          创建
        </el-button>
      </template>
    </el-dialog>

    <!-- 重置用户密码对话框 -->
    <el-dialog
      v-model="resetPasswordDialogVisible"
      title="重置用户密码"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-form label-width="100px">
        <el-form-item label="用户邮箱">
          <el-input :value="currentUser?.email" disabled />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input
            v-model="resetPasswordForm.new_password"
            type="password"
            placeholder="请输入新密码（至少6位）"
            show-password
          />
        </el-form-item>
        <el-form-item label="确认密码">
          <el-input
            v-model="resetPasswordForm.confirm_password"
            type="password"
            placeholder="请再次输入新密码"
            show-password
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetPasswordDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmResetPassword" :loading="resetting">
          <el-icon><Check /></el-icon>
          确认重置
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
  Tools, Setting, User, Ticket, Back, Check, Refresh, Plus
} from '@element-plus/icons-vue'

const route = useRoute()
const users = ref([])
const invites = ref([])
const inviteDialogVisible = ref(false)
const inviteMode = ref('auto')
const customInviteCode = ref('')
const previewInviteCode = ref('')
const creating = ref(false)
const resetPasswordDialogVisible = ref(false)
const currentUser = ref(null)
const resetPasswordForm = ref({ new_password: '', confirm_password: '' })
const resetting = ref(false)
const inviteUsesMode = ref('limited')
const inviteUses = ref(1)

const siteSettings = ref({
  site_name: '皮肤站',
  site_url: '',
  require_invite: false,
  allow_register: true,
  max_texture_size: 1024,
  rate_limit_enabled: true,
  rate_limit_auth_attempts: 5,
  rate_limit_auth_window: 15,
  jwt_expire_days: 7
})

const activeRoute = computed(() => route.path)
const active = computed(() => {
  if (route.path.includes('/users')) return 'users'
  if (route.path.includes('/invites')) return 'invites'
  return 'settings'
})

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

function formatDate(timestamp) {
  if (!timestamp) return '-'
  const date = new Date(timestamp)
  return date.toLocaleString('zh-CN')
}

async function loadSettings() {
  try {
    const res = await axios.get('/admin/settings', { headers: authHeaders() })
    if (res.data) {
      Object.assign(siteSettings.value, res.data)
    }
  } catch (e) {
    console.error('Load settings error:', e)
  }
}

async function saveSettings() {
  try {
    await axios.post('/admin/settings', siteSettings.value, { headers: authHeaders() })
    ElMessage.success('保存成功')
  } catch (e) {
    ElMessage.error('保存失败: ' + (e.response?.data?.detail || e.message))
  }
}

async function refreshUsers() {
  try {
    const res = await axios.get('/admin/users', { headers: authHeaders() })
    users.value = res.data
  } catch (e) {
    ElMessage.error('获取用户列表失败')
  }
}

async function toggleAdmin(user) {
  try {
    await ElMessageBox.confirm(
      `确定要${user.is_admin ? '取消' : '设置'} ${user.email} 的管理员权限吗？`,
      '确认操作',
      { type: 'warning' }
    )
    // 阻止管理员取消自己的管理员权限
    const token = localStorage.getItem('jwt')
    if (token) {
      const payload = JSON.parse(atob(token.split('.')[1]))
      if (payload.sub === user.id && user.is_admin) {
        ElMessage.warning('不能取消自身的管理员权限')
        return
      }
    }
    await axios.post(`/admin/users/${user.id}/toggle-admin`, {}, { headers: authHeaders() })
    ElMessage.success('操作成功')
    refreshUsers()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('操作失败: ' + (e.response?.data?.detail || e.message))
    }
  }
}

async function deleteUser(user) {
  try {
    await ElMessageBox.confirm(
      `确定要删除用户 ${user.email} 吗？此操作将删除该用户的所有数据！`,
      '危险操作',
      { type: 'error', confirmButtonText: '确定删除' }
    )
    await axios.delete(`/admin/users/${user.id}`, { headers: authHeaders() })
    ElMessage.success('删除成功')
    refreshUsers()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('删除失败: ' + (e.response?.data?.detail || e.message))
    }
  }
}

function showResetPasswordDialog(user) {
  currentUser.value = user
  resetPasswordForm.value = { new_password: '', confirm_password: '' }
  resetPasswordDialogVisible.value = true
}

async function confirmResetPassword() {
  if (!resetPasswordForm.value.new_password) {
    ElMessage.error('请输入新密码')
    return
  }
  if (resetPasswordForm.value.new_password.length < 6) {
    ElMessage.error('密码长度不能少于6个字符')
    return
  }
  if (resetPasswordForm.value.new_password !== resetPasswordForm.value.confirm_password) {
    ElMessage.error('两次输入的密码不一致')
    return
  }

  try {
    resetting.value = true
    await axios.post('/admin/users/reset-password', {
      user_id: currentUser.value.id,
      new_password: resetPasswordForm.value.new_password
    }, { headers: authHeaders() })
    ElMessage.success('密码重置成功')
    resetPasswordDialogVisible.value = false
  } catch (error) {
    console.error(error)
    ElMessage.error(error.response?.data?.detail || '重置失败')
  } finally {
    resetting.value = false
  }
}

async function loadInvites() {
  try {
    const res = await axios.get('/admin/invites', { headers: authHeaders() })
    invites.value = res.data
  } catch (e) {
    ElMessage.error('获取邀请码列表失败')
  }
}

function generateRandomCode() {
  // 生成一个随机的邀请码（16个字符，URL安全）
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789'
  let code = ''
  for (let i = 0; i < 16; i++) {
    code += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  return code
}

function showInviteDialog() {
  inviteMode.value = 'auto'
  customInviteCode.value = ''
  previewInviteCode.value = generateRandomCode()
  inviteUsesMode.value = 'limited'
  inviteUses.value = 1
  inviteDialogVisible.value = true
}

function refreshPreview() {
  previewInviteCode.value = generateRandomCode()
}

function getRemainingColor(row) {
  if (!row.total_uses) return '#67c23a' // 无限制，绿色
  const remaining = row.total_uses - (row.used_count || 0)
  const percentage = remaining / row.total_uses
  if (percentage <= 0) return '#f56c6c' // 红色
  if (percentage <= 0.3) return '#e6a23c' // 黄色
  return '#67c23a' // 绿色
}

async function confirmCreateInvite() {
  const code = inviteMode.value === 'auto' ? previewInviteCode.value : customInviteCode.value.trim()

  // 验证邀请码
  if (!code) {
    ElMessage.warning('请输入邀请码')
    return
  }

  if (code.length < 6) {
    ElMessage.warning('邀请码至少需要6个字符')
    return
  }

  if (!/^[a-zA-Z0-9_-]+$/.test(code)) {
    ElMessage.warning('邀请码只能包含字母、数字、下划线和横线')
    return
  }

  creating.value = true
  try {
    const payload = { code }

    // 添加使用次数
    if (inviteUsesMode.value === 'unlimited') {
      payload.total_uses = null
    } else {
      payload.total_uses = inviteUses.value
    }

    const res = await axios.post('/admin/invites', payload, { headers: authHeaders() })
    ElMessage.success('创建成功！邀请码：' + res.data.code)
    inviteDialogVisible.value = false
    loadInvites()
  } catch (e) {
    ElMessage.error('创建失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    creating.value = false
  }
}

async function deleteInvite(invite) {
  try {
    await ElMessageBox.confirm('确定要删除此邀请码吗？', '确认', { type: 'warning' })
    await axios.delete(`/admin/invites/${invite.code}`, { headers: authHeaders() })
    ElMessage.success('删除成功')
    loadInvites()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

onMounted(() => {
  loadSettings()
  if (route.path.includes('/users')) {
    refreshUsers()
  } else if (route.path.includes('/invites')) {
    loadInvites()
  }
})

// 监听路由变化，自动刷新对应页面数据
watch(() => route.path, (newPath) => {
  if (newPath.includes('/admin/settings')) {
    loadSettings()
  } else if (newPath.includes('/admin/users')) {
    refreshUsers()
  } else if (newPath.includes('/admin/invites')) {
    loadInvites()
  }
})
</script>

<style scoped>
.admin-container {
  min-height: 100vh;
  background: #f5f7fa;
}

.admin-container :deep(.el-container) {
  min-height: 100vh;
}

.admin-sidebar {
  background: #fff;
  border-right: 1px solid #e4e7ed;
  padding: 20px 0;
  min-height: 100vh;
}

.admin-title {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 20px 0;
  margin: 0 16px 4px 16px;
  font-size: 18px;
  font-weight: 600;
  color: #303133;
}

.title-divider {
  height: 1px;
  background: #ebeef5;
  margin: 0 16px 4px 16px;
}

.sidebar-menu {
  border: none;
}

.sidebar-menu .el-menu-item {
  height: 50px;
  line-height: 50px;
  margin: 4px 16px;
  padding: 0 16px !important;
  border-radius: 8px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
}

.sidebar-menu .el-menu-item::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  height: 100%;
  width: 3px;
  background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
  transform: translateX(-100%);
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.sidebar-menu .el-menu-item:hover::before {
  transform: translateX(0);
}

.sidebar-menu .el-menu-item:hover {
  background-color: #ecf5ff;
  transform: translateX(4px);
}

.sidebar-menu .el-menu-item.is-active {
  background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
  color: #fff;
  transform: translateX(0);
}

.admin-main {
  padding: 30px;
  background: #f5f7fa;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
  animation: fadeInUp 0.5s cubic-bezier(0.4, 0, 0.2, 1);
  width: 100%;
  max-width: 800px;
}

@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
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

.settings-card {
  width: 100%;
  max-width: 800px;
  padding: 30px;
  animation: cardSlideIn 0.5s cubic-bezier(0.4, 0, 0.2, 1) 0.1s backwards;
}

@keyframes cardSlideIn {
  from {
    opacity: 0;
    transform: translateY(30px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.settings-card :deep(.el-form-item) {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.settings-card :deep(.el-form-item:hover) {
  transform: translateX(4px);
}

.settings-card :deep(.el-button) {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.settings-card :deep(.el-button:hover) {
  transform: scale(1.05);
  box-shadow: 0 6px 20px rgba(64, 158, 255, 0.3);
}

.settings-section,
.users-section,
.invites-section {
  width: 100%;
  max-width: 1200px;
  display: flex;
  flex-direction: column;
  align-items: center;
}
</style>
