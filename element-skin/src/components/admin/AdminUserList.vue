<template>
  <div class="users-section">
    <div class="section-header">
      <h2>用户管理</h2>
      <el-button type="primary" @click="refreshUsers">
        <el-icon><Refresh /></el-icon>
        刷新
      </el-button>
    </div>

    <el-card class="list-card">
      <el-table :data="users" style="width: 100%">
        <el-table-column prop="email" label="邮箱" min-width="220" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag v-if="row.is_admin" type="danger">管理员</el-tag>
            <el-tag v-else-if="getUserBanStatus(row)" type="warning">封禁</el-tag>
            <el-tag v-else type="info">用户</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="角色数" width="80" align="center">
          <template #default="{ row }">
            {{ row.profile_count || 0 }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right" align="center">
          <template #default="{ row }">
            <el-button
              size="small"
              type="primary"
              @click="showUserDetailDialog(row)"
              link
            >
              管理
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

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

    <!-- 用户详情对话框 -->
    <el-dialog
      v-model="userDetailDialogVisible"
      :title="currentUser?.email || '用户详情'"
      width="650px"
      :close-on-click-modal="false"
      :destroy-on-close="true"
      class="user-detail-dialog"
    >
      <div v-if="currentUser" class="user-detail-content">
        <!-- 用户状态卡片 -->
        <div class="user-status-card bg-gradient-purple">
          <el-avatar :size="80" class="user-detail-avatar">
            {{ currentUser.email.charAt(0).toUpperCase() }}
          </el-avatar>
          <div class="user-detail-info">
            <h3>{{ currentUser.display_name || '未设置显示名' }}</h3>
            <p class="user-email">{{ currentUser.email }}</p>
            <div class="user-status-tag">
              <el-tag v-if="currentUser.is_admin" type="danger" size="large" effect="dark" class="status-tag">
                <el-icon style="vertical-align: middle;"><User /></el-icon>
                <span style="margin-left: 6px; vertical-align: middle;">管理员</span>
              </el-tag>
              <el-tag v-else-if="getUserBanStatus(currentUser)" type="warning" size="large" effect="dark" class="status-tag">
                <el-icon style="vertical-align: middle;"><Warning /></el-icon>
                <span style="margin-left: 6px; vertical-align: middle;">封禁中</span>
              </el-tag>
              <el-tag v-else type="success" size="large" effect="dark" class="status-tag">
                <el-icon style="vertical-align: middle;"><CircleCheck /></el-icon>
                <span style="margin-left: 6px; vertical-align: middle;">正常用户</span>
              </el-tag>
            </div>
          </div>
        </div>

        <!-- 详细信息 -->
        <div class="user-info-grid">
          <div class="info-item">
            <span class="info-label">用户ID</span>
            <el-text class="info-value" copyable>{{ currentUser.id }}</el-text>
          </div>
          <div class="info-item">
            <span class="info-label">角色数量</span>
            <span class="info-value">{{ currentUser.profile_count || 0 }}</span>
          </div>
          <div v-if="getUserBanStatus(currentUser)" class="info-item info-full">
            <span class="info-label">封禁剩余</span>
            <span class="info-value ban-time">{{ formatBanRemaining(currentUser.banned_until) }}</span>
          </div>
        </div>

        <!-- 操作按钮组 -->
        <el-divider />

        <div class="action-section">
          <div class="action-row">
            <el-button
              class="action-btn"
              :type="currentUser.is_admin ? 'warning' : 'primary'"
              @click="toggleAdmin(currentUser)"
              :disabled="isCurrentUserSelf(currentUser)"
              size="large"
            >
              <el-icon><User /></el-icon>
              <span>{{ currentUser.is_admin ? '取消管理员' : '设为管理员' }}</span>
            </el-button>

            <el-button
              v-if="!getUserBanStatus(currentUser)"
              class="action-btn"
              type="warning"
              @click="showBanDialog"
              :disabled="currentUser.is_admin"
              size="large"
            >
              <el-icon><Warning /></el-icon>
              <span>封禁用户</span>
            </el-button>
            <el-button
              v-else
              class="action-btn"
              type="success"
              @click="unbanUser(currentUser)"
              size="large"
            >
              <el-icon><CircleCheck /></el-icon>
              <span>解除封禁</span>
            </el-button>
          </div>

          <div class="action-row">
            <el-button
              class="action-btn"
              @click="showResetPasswordDialog(currentUser)"
              size="large"
            >
              <el-icon><Key /></el-icon>
              <span>重置密码</span>
            </el-button>

            <el-button
              class="action-btn"
              type="danger"
              @click="deleteUser(currentUser)"
              :disabled="currentUser.is_admin"
              size="large"
            >
              <el-icon><Delete /></el-icon>
              <span>删除用户</span>
            </el-button>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- 封禁用户对话框 -->
    <el-dialog
      v-model="banDialogVisible"
      title="封禁用户"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-alert
        type="warning"
        :closable="false"
        style="margin-bottom: 20px;"
      >
        <template #title>
          <div style="font-weight: 600;">封禁说明</div>
        </template>
        封禁后，用户将无法通过 Minecraft 客户端登录游戏，但仍可以正常访问皮肤站进行皮肤管理等操作。
      </el-alert>

      <el-form label-width="100px">
        <el-form-item label="用户">
          <el-text>{{ currentUser?.email }}</el-text>
        </el-form-item>

        <el-form-item label="封禁时长">
          <div class="ban-duration-wrapper">
            <el-radio-group v-model="banDurationType" class="ban-type-selector">
              <el-radio value="preset">预设时长</el-radio>
              <el-radio value="custom">自定义时长</el-radio>
            </el-radio-group>

            <div v-if="banDurationType === 'preset'" class="duration-content">
              <div class="preset-grid">
                <el-button
                  v-for="preset in presetDurations"
                  :key="preset.value"
                  :type="banPresetDuration === preset.value ? 'primary' : ''"
                  @click="banPresetDuration = preset.value"
                  size="default"
                >
                  {{ preset.label }}
                </el-button>
              </div>
            </div>

            <div v-if="banDurationType === 'custom'" class="duration-content">
              <el-input-number
                v-model="banCustomHours"
                :min="1"
                :max="8760"
                :step="1"
                controls-position="right"
                size="large"
                style="width: 100%;"
              />
              <el-text size="small" type="info" class="duration-hint">
                输入小时数（最多365天 = 8760小时）
              </el-text>
            </div>
          </div>
        </el-form-item>

        <el-form-item label="解封时间">
          <el-text type="primary" size="large" style="font-weight: 600;">{{ formatBanUntilTime() }}</el-text>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="banDialogVisible = false">取消</el-button>
        <el-button type="danger" @click="confirmBanUser" :loading="banning">
          <el-icon><Check /></el-icon>
          确认封禁
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Check, User, Warning, CircleCheck, Key, Delete } from '@element-plus/icons-vue'

const users = ref([])
const currentUser = ref(null)
const userDetailDialogVisible = ref(false)
const resetPasswordDialogVisible = ref(false)
const resetPasswordForm = ref({ new_password: '', confirm_password: '' })
const resetting = ref(false)
const banDialogVisible = ref(false)
const banDurationType = ref('preset')
const banPresetDuration = ref(24)
const banCustomHours = ref(24)
const banning = ref(false)

const presetDurations = [
  { label: '1小时', value: 1 },
  { label: '6小时', value: 6 },
  { label: '1天', value: 24 },
  { label: '3天', value: 72 },
  { label: '7天', value: 168 },
  { label: '30天', value: 720 }
]

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

async function refreshUsers() {
  try {
    const res = await axios.get('/admin/users', { headers: authHeaders() })
    users.value = res.data
  } catch (e) {
    ElMessage.error('获取用户列表失败')
  }
}

function getUserBanStatus(user) {
  if (!user.banned_until) return false
  return Date.now() < user.banned_until
}

function formatBanRemaining(bannedUntil) {
  if (!bannedUntil) return ''
  const remaining = bannedUntil - Date.now()
  if (remaining <= 0) return '已到期'

  const days = Math.floor(remaining / (1000 * 60 * 60 * 24))
  const hours = Math.floor((remaining % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
  const minutes = Math.floor((remaining % (1000 * 60 * 60)) / (1000 * 60))

  if (days > 0) {
    return `${days}天 ${hours}小时`
  } else if (hours > 0) {
    return `${hours}小时 ${minutes}分钟`
  } else {
    return `${minutes}分钟`
  }
}

function formatBanUntilTime() {
  let hours = 0
  if (banDurationType.value === 'preset') {
    hours = banPresetDuration.value
  } else {
    hours = banCustomHours.value
  }

  const until = new Date(Date.now() + hours * 60 * 60 * 1000)
  return until.toLocaleString('zh-CN')
}

function isCurrentUserSelf(user) {
  const token = localStorage.getItem('jwt')
  if (!token) return false
  try {
    const payload = JSON.parse(atob(token.split('.')[1]))
    return payload.sub === user.id
  } catch (e) {
    return false
  }
}

function showUserDetailDialog(user) {
  currentUser.value = user
  userDetailDialogVisible.value = true
}

async function toggleAdmin(user) {
  try {
    await ElMessageBox.confirm(
      `确定要${user.is_admin ? '取消' : '设置'} ${user.email} 的管理员权限吗？`,
      '确认操作',
      { type: 'warning' }
    )
    
    if (isCurrentUserSelf(user) && user.is_admin) {
      ElMessage.warning('不能取消自身的管理员权限')
      return
    }
    
    await axios.post(`/admin/users/${user.id}/toggle-admin`, {}, { headers: authHeaders() })
    ElMessage.success('操作成功')
    refreshUsers()
    // Update local user object if it's the one in dialog
    if (currentUser.value && currentUser.value.id === user.id) {
      currentUser.value.is_admin = !currentUser.value.is_admin
    }
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
    userDetailDialogVisible.value = false
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

function showBanDialog() {
  banDurationType.value = 'preset'
  banPresetDuration.value = 24
  banCustomHours.value = 24
  banDialogVisible.value = true
}

async function confirmBanUser() {
  if (!currentUser.value) return

  let hours = 0
  if (banDurationType.value === 'preset') {
    hours = banPresetDuration.value
  } else {
    hours = banCustomHours.value
  }

  const bannedUntil = Date.now() + hours * 60 * 60 * 1000

  try {
    banning.value = true
    await axios.post(`/admin/users/${currentUser.value.id}/ban`, {
      banned_until: bannedUntil
    }, { headers: authHeaders() })
    ElMessage.success('封禁成功')
    banDialogVisible.value = false
    refreshUsers()
    
    // Update local user object
    const updatedUser = users.value.find(u => u.id === currentUser.value.id)
    if (updatedUser) {
      currentUser.value = updatedUser
    } else {
      currentUser.value.banned_until = bannedUntil
    }
  } catch (error) {
    console.error(error)
    ElMessage.error(error.response?.data?.detail || '封禁失败')
  } finally {
    banning.value = false
  }
}

async function unbanUser(user) {
  try {
    await ElMessageBox.confirm('确定要解除该用户的封禁吗？', '确认操作', { type: 'info' })
    await axios.post(`/admin/users/${user.id}/unban`, {}, { headers: authHeaders() })
    ElMessage.success('解封成功')
    refreshUsers()
    
    // Update local user object
    const updatedUser = users.value.find(u => u.id === user.id)
    if (updatedUser) {
      currentUser.value = updatedUser
    } else {
      currentUser.value.banned_until = 0
    }
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('解封失败: ' + (e.response?.data?.detail || e.message))
    }
  }
}

onMounted(() => {
  refreshUsers()
})
</script>

<style scoped>
.users-section {
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.user-detail-content {
  padding: 0;
}

.user-status-card {
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 24px;
  border-radius: 12px;
  margin-bottom: 24px;
  color: white;
}

.user-detail-avatar {
  background: rgba(255, 255, 255, 0.2);
  color: white;
  font-size: 32px;
  font-weight: 600;
  border: 3px solid rgba(255, 255, 255, 0.3);
  flex-shrink: 0;
}

.user-detail-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  min-width: 0;
}

.user-detail-info h3 {
  margin: 0 0 8px 0;
  font-size: 24px;
  font-weight: 600;
  width: 100%;
}

.user-email {
  margin: 0 0 12px 0;
  opacity: 0.9;
  font-size: 14px;
  width: 100%;
}

.user-status-tag {
  margin-top: 8px;
  width: auto;
}

.user-status-tag .status-tag {
  display: inline-flex;
  align-items: center;
  white-space: nowrap;
}

.user-info-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
  margin-bottom: 16px;
}

.info-item {
  padding: 16px;
  background: var(--color-background-soft);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.info-item.info-full {
  grid-column: span 2;
}

.info-label {
  font-size: 13px;
  color: var(--color-text-light);
  font-weight: 500;
}

.info-value {
  font-size: 15px;
  color: var(--color-heading);
  font-weight: 600;
}

.info-value.ban-time {
  color: #e6a23c;
  font-size: 16px;
}

.action-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.action-row {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.action-btn {
  flex: 1;
}

.ban-duration-wrapper {
  width: 100%;
}

.ban-type-selector {
  width: 100%;
  margin-bottom: 16px;
}

.ban-type-selector .el-radio {
  margin-right: 24px;
}

.duration-content {
  width: 100%;
  padding: 16px;
  background: var(--color-background-soft);
  border-radius: 8px;
}

.preset-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  width: 100%;
}

.preset-grid .el-button {
  width: 100%;
  margin: 0;
  padding: 8px 15px;
  justify-content: center;
}

.duration-hint {
  display: block;
  margin-top: 12px;
  padding-left: 4px;
}

.list-card {
  width: 100%;
  max-width: 100%;
  border: 1px solid var(--color-border);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  background: var(--color-card-background);
}

@media (max-width: 768px) {
  .users-section {
    padding: 0;
  }
  .list-card :deep(.el-card__body) {
    padding: 10px;
  }
}
</style>