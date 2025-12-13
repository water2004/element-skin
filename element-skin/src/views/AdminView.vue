<template>
  <div class="admin-container">
    <el-container style="height:100%">
      <el-aside width="220px" class="admin-sidebar">
        <div class="admin-title">
          <el-icon size="24"><Tools /></el-icon>
          <span>管理面板</span>
        </div>
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
          </div>

          <el-card class="settings-card">
            <el-form label-width="140px" :model="siteSettings">
              <el-form-item label="站点名称">
                <el-input v-model="siteSettings.site_name" placeholder="皮肤站" />
              </el-form-item>
              <el-form-item label="站点地址">
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
              <el-table-column label="操作" width="250" fixed="right">
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
            <el-button type="primary" @click="generateInvite">
              <el-icon><Plus /></el-icon>
              生成邀请码
            </el-button>
          </div>

          <el-card>
            <el-table :data="invites" style="width: 100%">
              <el-table-column prop="code" label="邀请码" min-width="300">
                <template #default="{ row }">
                  <el-text copyable>{{ row.code }}</el-text>
                </template>
              </el-table-column>
              <el-table-column label="状态" width="100">
                <template #default="{ row }">
                  <el-tag v-if="row.used_by" type="info">已使用</el-tag>
                  <el-tag v-else type="success">未使用</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="used_by" label="使用者" min-width="200" />
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
                    :disabled="!!row.used_by"
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
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Tools, Setting, User, Ticket, Back, Check, Refresh, Plus
} from '@element-plus/icons-vue'

const route = useRoute()
const users = ref([])
const invites = ref([])
const siteSettings = ref({
  site_name: '皮肤站',
  site_url: '',
  require_invite: false,
  allow_register: true,
  max_texture_size: 1024
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

async function loadInvites() {
  try {
    const res = await axios.get('/admin/invites', { headers: authHeaders() })
    invites.value = res.data
  } catch (e) {
    ElMessage.error('获取邀请码列表失败')
  }
}

async function generateInvite() {
  try {
    const res = await axios.post('/admin/invites', {}, { headers: authHeaders() })
    ElMessage.success('生成成功: ' + res.data.code)
    loadInvites()
  } catch (e) {
    ElMessage.error('生成失败: ' + (e.response?.data?.detail || e.message))
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
</script>

<style scoped>
.admin-container {
  height: 100%;
  background: #f5f7fa;
}

.admin-sidebar {
  background: #fff;
  border-right: 1px solid #e4e7ed;
  padding: 20px 0;
}

.admin-title {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 20px;
  margin-bottom: 20px;
  font-size: 18px;
  font-weight: 600;
  color: #303133;
  border-bottom: 1px solid #ebeef5;
}

.sidebar-menu {
  border: none;
}

.sidebar-menu .el-menu-item {
  height: 50px;
  line-height: 50px;
  margin: 4px 12px;
  border-radius: 8px;
}

.sidebar-menu .el-menu-item.is-active {
  background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
  color: #fff;
}

.admin-main {
  padding: 30px;
  background: #f5f7fa;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.section-header h2 {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
  margin: 0;
}

.settings-card {
  max-width: 800px;
  padding: 30px;
}
</style>
