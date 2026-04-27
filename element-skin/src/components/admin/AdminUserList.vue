<template>
  <div class="users-section animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <div class="page-header-icon"><UserFilled /></div>
        <div class="page-header-text">
          <h2>用户管理</h2>
          <p class="subtitle">管理站内所有用户及其角色的状态与权限</p>
        </div>
      </div>
      <div class="page-header-actions">
        <el-button type="primary" :icon="Refresh" @click="refreshUsersFromFirst" plain class="hover-lift">
          刷新列表
        </el-button>
      </div>
    </div>

    <div class="search-bar-container">
      <el-input
        v-model="searchQuery"
        placeholder="搜索用户名 / 邮箱 / 角色名"
        clearable
        @clear="handleClearSearch"
        @keyup.enter="handleSearch"
        size="large"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
        <template #append>
          <el-button :icon="Search" @click="handleSearch">搜索</el-button>
        </template>
      </el-input>
    </div>

    <el-card class="surface-card" shadow="never">
      <el-table :data="users" style="width: 100%" class="modern-table" v-loading="loading">
        <el-table-column prop="display_name" label="用户名" min-width="150">
          <template #default="{ row }">
            <div class="user-cell">
              <el-avatar :size="32" :shape="userAvatars[row.avatar_hash] ? 'square' : 'circle'" :src="userAvatars[row.avatar_hash] || ''" class="mr-2">
                {{ row.display_name?.charAt(0).toUpperCase() || row.email.charAt(0).toUpperCase() }}
              </el-avatar>
              <span>{{ row.display_name || '未设置' }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="email" label="邮箱" min-width="220" />
        <el-table-column label="身份状态" width="120">
          <template #default="{ row }">
            <el-tag v-if="row.is_admin" type="danger" effect="light" size="small">管理员</el-tag>
            <el-tag v-else-if="getUserBanStatus(row)" type="warning" effect="light" size="small">已封禁</el-tag>
            <el-tag v-else type="success" effect="light" size="small">正常</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="角色数" width="100" align="center">
          <template #default="{ row }">
            <el-badge :value="row.profile_count || 0" :type="row.profile_count > 0 ? 'primary' : 'info'" class="profile-badge" />
          </template>
        </el-table-column>
        <el-table-column label="管理操作" width="120" fixed="right" align="center">
          <template #default="{ row }">
            <el-button
              size="small"
              type="primary"
              @click="showUserDetailDialog(row)"
              plain
              class="hover-lift"
            >
              管理
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-container">
        <CursorPager
          v-if="users.length > 0"
          :count="users.length"
          :loading="usersPagination.isLoading.value"
          :disabled-prev="!usersPagination.canGoPrev.value"
          :disabled-next="!usersPagination.canGoNext.value"
          @prev="handleUsersPrevPage"
          @next="handleUsersNextPage"
        />
      </div>
    </el-card>

    <!-- User Detail Dialog -->
    <el-dialog
      v-model="userDetailDialogVisible"
      title=""
      class="dialog-viewer"
      destroy-on-close
      align-center
      append-to-body
    >
      <div v-if="currentUser" class="user-detail-container">
        <!-- User Identity Panel -->
        <div class="identity-panel mb-6">
          <el-avatar :size="80" :shape="userAvatars[currentUser.avatar_hash] ? 'square' : 'circle'" :src="userAvatars[currentUser.avatar_hash] || ''" class="panel-avatar">
            {{ currentUser.email.charAt(0).toUpperCase() }}
          </el-avatar>
          <div class="panel-info">
            <div class="panel-name">
              <h3>{{ currentUser.display_name || '未设置显示名' }}</h3>
              <el-tag v-if="currentUser.is_admin" type="danger" size="small" class="ml-2">管理员</el-tag>
            </div>
            <div class="panel-email">{{ currentUser.email }}</div>
            <div class="panel-id">UID: {{ currentUser.id }}</div>
          </div>
          <div class="panel-status">
            <div v-if="getUserBanStatus(currentUser)" class="ban-info">
              <el-tag type="warning" effect="dark">
                <el-icon><Warning /></el-icon> 封禁中
              </el-tag>
              <div class="ban-timer">{{ formatBanRemaining(currentUser.banned_until) }} 后解封</div>
            </div>
            <el-tag v-else type="success" effect="dark">
              <el-icon><CircleCheck /></el-icon> 状态正常
            </el-tag>
          </div>
        </div>

        <el-tabs type="border-card" class="detail-tabs">
          <el-tab-pane label="角色列表">
            <el-table :data="userProfiles || []" size="small" max-height="300">
              <el-table-column prop="name" label="角色名称" />
              <el-table-column prop="model" label="模型" width="100">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.model === 'slim' ? 'success' : 'info'">{{ row.model }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="id" label="角色 UUID" width="300" />
            </el-table>
            <el-empty v-if="!userProfiles?.length" description="该用户暂无角色" :image-size="60" />
            <div class="pagination-container" style="margin-top: 10px;">
              <CursorPager
                v-if="userProfiles.length > 0"
                :count="userProfiles.length"
                :loading="profilesPagination.isLoading.value"
                :disabled-prev="!profilesPagination.canGoPrev.value"
                :disabled-next="!profilesPagination.canGoNext.value"
                @prev="handleProfilesPrevPage"
                @next="handleProfilesNextPage"
              />
            </div>
          </el-tab-pane>
          
          <el-tab-pane label="危险操作">
            <div class="actions-grid">
              <div class="action-card-item">
                <div class="action-text-box">
                  <div class="a-title">管理权限</div>
                  <div class="a-desc">授予或撤销该用户的管理员权限。</div>
                </div>
                <el-button 
                  :type="currentUser.is_admin ? 'warning' : 'primary'" 
                  @click="toggleAdmin(currentUser)"
                  :disabled="isCurrentUserSelf(currentUser)"
                  class="hover-lift"
                >
                  {{ currentUser.is_admin ? '撤销管理' : '设为管理' }}
                </el-button>
              </div>

              <div class="action-card-item">
                <div class="action-text-box">
                  <div class="a-title">账号封禁</div>
                  <div class="a-desc">暂时禁止该用户登录 Minecraft 客户端。</div>
                </div>
                <el-button 
                  v-if="!getUserBanStatus(currentUser)" 
                  type="warning" 
                  @click="showBanDialog"
                  :disabled="currentUser.is_admin || isCurrentUserSelf(currentUser)"
                  class="hover-lift"
                >
                  执行封禁
                </el-button>
                <el-button v-else type="success" @click="unbanUser(currentUser)" class="hover-lift">
                  解除封禁
                </el-button>
              </div>

              <div class="action-card-item">
                <div class="action-text-box">
                  <div class="a-title">强制重置密码</div>
                  <div class="a-desc">系统管理员手动为该用户设置新密码。</div>
                </div>
                <el-button @click="showResetPasswordDialog(currentUser)" class="hover-lift">
                  重置密码
                </el-button>
              </div>

              <div class="action-card-item dangerous">
                <div class="action-text-box">
                  <div class="a-title">注销账号</div>
                  <div class="a-desc">永久删除该用户及其所有关联的角色、皮肤。</div>
                </div>
                <el-button 
                  type="danger" 
                  @click="deleteUser(currentUser)"
                  :disabled="currentUser.is_admin || isCurrentUserSelf(currentUser)"
                  class="hover-lift"
                >
                  删除用户
                </el-button>
              </div>
            </div>
          </el-tab-pane>
        </el-tabs>
      </div>
    </el-dialog>

    <!-- Reset Password Dialog -->
    <el-dialog v-model="resetPasswordDialogVisible" title="重置用户密码" class="dialog-form" align-center append-to-body>
      <el-form label-position="top">
        <el-form-item label="新密码 (最少6位)">
          <el-input v-model="resetPasswordForm.new_password" type="password" show-password />
        </el-form-item>
        <el-form-item label="确认新密码">
          <el-input v-model="resetPasswordForm.confirm_password" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetPasswordDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmResetPassword" :loading="resetting">确认重置</el-button>
      </template>
    </el-dialog>

    <!-- Ban User Dialog -->
    <el-dialog v-model="banDialogVisible" title="设置封禁时长" class="dialog-form" align-center append-to-body>
      <div class="ban-dialog-body">
        <el-radio-group v-model="banDurationType" class="mb-4 capsule-radio">
          <el-radio-button value="preset">快速选择</el-radio-button>
          <el-radio-button value="custom">精确小时</el-radio-button>
        </el-radio-group>

        <div v-if="banDurationType === 'preset'" class="preset-durations mb-4">
          <el-button 
            v-for="p in presetDurations" 
            :key="p.value" 
            :type="banPresetDuration === p.value ? 'primary' : ''"
            size="small"
            @click="banPresetDuration = p.value"
          >{{ p.label }}</el-button>
        </div>
        
        <div v-else class="custom-duration mb-4">
          <el-input-number v-model="banCustomHours" :min="1" :max="8760" style="width: 100%" />
        </div>

        <div class="ban-preview">
          解封时间：<span>{{ formatBanUntilTime() }}</span>
        </div>
      </div>
      <template #footer>
        <el-button @click="banDialogVisible = false">取消</el-button>
        <el-button type="danger" @click="confirmBanUser" :loading="banning">确认封禁</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, watch } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { 
  Refresh, UserFilled, Warning, CircleCheck, Search 
} from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import { getAvatarForHash } from '@/composables/useAvatar'
import { useCursorPagination } from '@/composables/useCursorPagination'

const users = ref([])
const limit = 15
const usersPagination = useCursorPagination(limit)
const loading = ref(false)
const searchQuery = ref('')
const activeSearchQuery = ref('')  // 当前生效的搜索词（点击搜索按钮后才同步）
const userAvatars = reactive({})   // hash -> base64 avatar image cache
const currentUser = ref(null)
const userProfiles = ref([])
const profileLimit = 10
const profilesPagination = useCursorPagination(profileLimit)
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
  { label: '1小时', value: 1 }, { label: '1天', value: 24 }, 
  { label: '3天', value: 72 }, { label: '7天', value: 168 }, { label: '30天', value: 720 }
]

const authHeaders = () => ({ Authorization: 'Bearer ' + localStorage.getItem('jwt') })

function buildSearchParams(extraParams = {}) {
  const params = { limit, ...extraParams }
  if (activeSearchQuery.value) params.q = activeSearchQuery.value
  return params
}

async function refreshUsers() {
  loading.value = true
  usersPagination.isLoading.value = true
  try {
    const res = await axios.get('/admin/users', { 
      headers: authHeaders(),
      params: buildSearchParams({ cursor: usersPagination.currentCursor.value })
    })
    users.value = res.data.items
    usersPagination.setPageData(res.data)
  } catch (e) {
    ElMessage.error('加载用户列表失败')
  } finally {
    loading.value = false
    usersPagination.isLoading.value = false
  }
}

async function refreshUsersFromFirst() {
  usersPagination.reset()
  await refreshUsers()
}

/** Load avatars for all users on the current page (sequentially, one WebGL at a time) */
async function loadAvatarsForUsers(userList) {
  for (const u of userList) {
    if (u.avatar_hash && !userAvatars[u.avatar_hash]) {
      const img = await getAvatarForHash(u.avatar_hash)
      if (img) userAvatars[u.avatar_hash] = img
    }
  }
}

async function handleUsersNextPage() {
  await usersPagination.goToNextPage(async (cursor, pageLimit) => {
    const res = await axios.get('/admin/users', {
      headers: authHeaders(),
      params: buildSearchParams({ cursor, limit: pageLimit })
    })
    users.value = res.data.items
    return res.data
  })
}

async function handleUsersPrevPage() {
  await usersPagination.goToPrevPage(async (cursor, pageLimit) => {
    const res = await axios.get('/admin/users', {
      headers: authHeaders(),
      params: buildSearchParams({ cursor, limit: pageLimit })
    })
    users.value = res.data.items
    return res.data
  })
}

function handleSearch() {
  activeSearchQuery.value = searchQuery.value.trim()
  usersPagination.reset()
  refreshUsers()
}

function handleClearSearch() {
  searchQuery.value = ''
  activeSearchQuery.value = ''
  usersPagination.reset()
  refreshUsers()
}

async function showUserDetailDialog(user) {
  try {
    const res = await axios.get(`/admin/users/${user.id}`, { headers: authHeaders() })
    currentUser.value = res.data
    profilesPagination.reset()
    await fetchUserProfilesAdmin()
    userDetailDialogVisible.value = true
  } catch (e) {
    ElMessage.error('无法加载用户详情')
  }
}

async function fetchUserProfilesAdmin() {
  if (!currentUser.value) return
  try {
    const res = await axios.get(`/admin/users/${currentUser.value.id}/profiles`, { 
      headers: authHeaders(),
      params: { cursor: profilesPagination.currentCursor.value, limit: profileLimit }
    })
    userProfiles.value = res.data.items
    profilesPagination.setPageData(res.data)
  } catch (e) {
    ElMessage.error('无法加载用户角色列表')
  }
}

async function handleProfilesNextPage() {
  if (!currentUser.value) return
  await profilesPagination.goToNextPage(async (cursor, pageLimit) => {
    const res = await axios.get(`/admin/users/${currentUser.value.id}/profiles`, {
      headers: authHeaders(),
      params: { cursor, limit: pageLimit }
    })
    userProfiles.value = res.data.items
    return res.data
  })
}

async function handleProfilesPrevPage() {
  if (!currentUser.value) return
  await profilesPagination.goToPrevPage(async (cursor, pageLimit) => {
    const res = await axios.get(`/admin/users/${currentUser.value.id}/profiles`, {
      headers: authHeaders(),
      params: { cursor, limit: pageLimit }
    })
    userProfiles.value = res.data.items
    return res.data
  })
}

async function toggleAdmin(user) {
  try {
    await ElMessageBox.confirm(`确定要切换 ${user.email} 的管理员状态吗？`, '确认', { type: 'warning' })
    await axios.post(`/admin/users/${user.id}/toggle-admin`, {}, { headers: authHeaders() })
    ElMessage.success('操作成功')
    await refreshUsers()
    if (currentUser.value) currentUser.value.is_admin = !currentUser.value.is_admin
  } catch (e) {}
}

async function deleteUser(user) {
  try {
    await ElMessageBox.confirm('永久删除该用户？此操作不可逆！', '极端警告', { type: 'error' })
    await axios.delete(`/admin/users/${user.id}`, { headers: authHeaders() })
    ElMessage.success('用户已删除')
    userDetailDialogVisible.value = false
    await refreshUsersFromFirst()
  } catch (e) {}
}

function showResetPasswordDialog(user) {
  resetPasswordForm.value = { new_password: '', confirm_password: '' }
  resetPasswordDialogVisible.value = true
}

async function confirmResetPassword() {
  const f = resetPasswordForm.value
  if (!f.new_password || f.new_password.length < 6) return ElMessage.error('密码长度不足')
  if (f.new_password !== f.confirm_password) return ElMessage.error('两次密码不一致')
  
  resetting.value = true
  try {
    await axios.post('/admin/users/reset-password', {
      user_id: currentUser.value.id,
      new_password: f.new_password
    }, { headers: authHeaders() })
    ElMessage.success('密码已重置')
    resetPasswordDialogVisible.value = false
  } catch (e) {
    ElMessage.error('重置失败')
  } finally {
    resetting.value = false
  }
}

function showBanDialog() {
  banDialogVisible.value = true
}

async function confirmBanUser() {
  const hours = banDurationType.value === 'preset' ? banPresetDuration.value : banCustomHours.value
  const bannedUntil = Date.now() + hours * 60 * 60 * 1000
  
  banning.value = true
  try {
    await axios.post(`/admin/users/${currentUser.value.id}/ban`, { banned_until: bannedUntil }, { headers: authHeaders() })
    ElMessage.success('封禁已执行')
    banDialogVisible.value = false
    await refreshUsers()
    if (currentUser.value) currentUser.value.banned_until = bannedUntil
  } catch (e) {
    ElMessage.error('封禁失败')
  } finally {
    banning.value = false
  }
}

async function unbanUser(user) {
  try {
    await axios.post(`/admin/users/${user.id}/unban`, {}, { headers: authHeaders() })
    ElMessage.success('封禁已解除')
    await refreshUsers()
    if (currentUser.value) currentUser.value.banned_until = 0
  } catch (e) {}
}

// Helpers
const getUserBanStatus = (user) => user.banned_until && Date.now() < user.banned_until
const isCurrentUserSelf = (user) => {
  try {
    const token = localStorage.getItem('jwt')
    if (!token) return false
    return JSON.parse(atob(token.split('.')[1])).sub === user.id
  } catch (e) { return false }
}
const formatBanRemaining = (ts) => {
  const m = Math.ceil((ts - Date.now()) / 60000)
  if (m > 1440) return Math.floor(m / 1440) + ' 天'
  if (m > 60) return Math.floor(m / 60) + ' 小时'
  return m + ' 分钟'
}
const formatBanUntilTime = () => {
  const h = banDurationType.value === 'preset' ? banPresetDuration.value : banCustomHours.value
  return new Date(Date.now() + h * 3600000).toLocaleString()
}

onMounted(refreshUsersFromFirst)

// Watch users list changes to load avatars
watch(users, (newUsers) => {
  if (newUsers?.length) loadAvatarsForUsers(newUsers)
})

// When dialog opens and user has avatar_hash, ensure it's loaded
watch(currentUser, async (u) => {
  if (u?.avatar_hash && !userAvatars[u.avatar_hash]) {
    const img = await getAvatarForHash(u.avatar_hash)
    if (img) userAvatars[u.avatar_hash] = img
  }
})
</script>

<style>
@import "@/assets/styles/dialogs.css";
</style>

<style scoped>
@import "@/assets/styles/animations.css";
@import "@/assets/styles/layout.css";
@import "@/assets/styles/cards.css";
@import "@/assets/styles/tags.css";
@import "@/assets/styles/buttons.css";
@import "@/assets/styles/headers.css";

.users-section { max-width: 1000px; margin: 0 auto; padding: 20px 0; }

.search-bar-container {
  margin-bottom: 16px;
}

.search-bar-container :deep(.el-input-group__append) {
  background: var(--el-color-primary);
  color: #fff;
  border-color: var(--el-color-primary);
  cursor: pointer;
}

.search-bar-container :deep(.el-input-group__append:hover) {
  background: var(--el-color-primary-light-3);
  border-color: var(--el-color-primary-light-3);
}

.user-cell { display: flex; align-items: center; }

/* Dialog Styles */
.user-detail-container { padding: 24px; }
.identity-panel { display: flex; align-items: center; gap: 24px; padding: 20px; background: var(--color-background-soft); border-radius: 12px; }
.panel-avatar { background: var(--el-color-primary-light-3); color: white; font-weight: bold; border: 2px solid #fff; box-shadow: 0 4px 12px rgba(0,0,0,0.1); }
.panel-info { flex: 1; }
.panel-name { display: flex; align-items: center; gap: 8px; }
.panel-name h3 { margin: 0; font-size: 20px; color: var(--color-heading); }
.panel-email { color: var(--color-text-light); margin-top: 4px; }
.panel-id { font-size: 11px; font-family: monospace; color: var(--color-text-light); margin-top: 4px; }
.panel-status { text-align: right; }
.ban-timer { font-size: 12px; color: var(--el-color-warning); margin-top: 4px; }

.actions-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; padding: 10px 0; }
.action-card-item { display: flex; justify-content: space-between; align-items: center; padding: 16px; background: var(--color-background-soft); border-radius: 10px; border: 1px solid var(--color-border); }
.action-card-item.dangerous { border-color: rgba(245, 108, 108, 0.3); }
.action-text-box { flex: 1; margin-right: 12px; }
.a-title { font-weight: 600; font-size: 14px; color: var(--color-heading); }
.a-desc { font-size: 12px; color: var(--color-text-light); margin-top: 2px; }

.ban-preview { font-size: 13px; color: var(--color-text-light); padding: 10px; background: var(--color-background-mute); border-radius: 6px; }
.ban-preview span { font-weight: bold; color: var(--el-color-primary); }

.mr-2 { margin-right: 8px; }
.mb-4 { margin-bottom: 16px; }
.mb-6 { margin-bottom: 24px; }

@media (max-width: 768px) {
  .actions-grid { grid-template-columns: 1fr; }
}
</style>
