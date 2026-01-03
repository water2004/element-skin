<template>
  <div class="admin-mojang">
    <div class="page-header">
      <h2>Mojang API 兼容与状态</h2>
    </div>

    <!-- API Status Section -->
    <el-card class="box-card mb-4" shadow="hover">
      <template #header>
        <div class="card-header">
          <span>Mojang 服务状态</span>
          <el-button size="small" :loading="checkingStatus" @click="checkMojangStatus" circle>
            <el-icon><Refresh /></el-icon>
          </el-button>
        </div>
      </template>
      <el-row :gutter="20">
        <el-col :span="8" v-for="(url, key) in statusUrls" :key="key">
          <div class="status-item">
            <span class="status-name">{{ key.toUpperCase() }}</span>
            <el-tag :type="getStatusType(statusData[key])" size="small">
              {{ formatStatus(statusData[key]) }}
            </el-tag>
          </div>
        </el-col>
      </el-row>
    </el-card>

    <!-- Compatibility Settings -->
    <el-card class="box-card mb-4" shadow="hover">
      <template #header>
        <div class="card-header">
          <span>兼容性设置</span>
          <el-button type="primary" size="small" @click="saveSettings" :loading="saving">保存设置</el-button>
        </div>
      </template>
      <el-form label-position="top">
        <el-form-item label="转发 Profile 请求 (fallback_mojang_profile)">
          <div class="setting-row">
            <el-switch v-model="settings.fallback_mojang_profile" />
            <span class="setting-desc">当本地查无此人时，是否转发请求到 Mojang API 查询角色信息 (UUID, 皮肤等)。</span>
          </div>
        </el-form-item>
        <el-form-item label="转发 HasJoined 验证 (fallback_mojang_hasjoined)">
          <div class="setting-row">
            <el-switch v-model="settings.fallback_mojang_hasjoined" />
            <span class="setting-desc">当本地验证失败时，是否尝试向 Mojang 验证会话 (支持正版登录)。</span>
          </div>
        </el-form-item>
        <el-form-item label="启用正版账户白名单 (enable_official_whitelist)">
          <div class="setting-row">
            <el-switch v-model="settings.enable_official_whitelist" />
            <span class="setting-desc">开启后，只有在白名单内的用户才会被转发到 Mojang 进行验证 (需开启 HasJoined 转发)。</span>
          </div>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Whitelist Management -->
    <el-card class="box-card" shadow="hover" v-if="settings.enable_official_whitelist">
      <template #header>
        <div class="card-header">
          <span>正版账户白名单管理</span>
          <el-button type="success" size="small" @click="dialogVisible = true">
            <el-icon><Plus /></el-icon> 添加用户
          </el-button>
        </div>
      </template>
      
      <el-table :data="whitelist" style="width: 100%" v-loading="loadingList">
        <el-table-column prop="username" label="正版用户名" />
        <el-table-column prop="created_at" label="添加时间">
          <template #default="scope">
            {{ new Date(scope.row.created_at).toLocaleString() }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120" align="right">
          <template #default="scope">
            <el-button type="danger" size="small" @click="removeUser(scope.row.username)" circle>
              <el-icon><Delete /></el-icon>
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Add User Dialog -->
    <el-dialog v-model="dialogVisible" title="添加正版用户" width="400px">
      <el-form @submit.prevent="addUser">
        <el-form-item label="用户名">
          <el-input v-model="newUsername" placeholder="输入 Mojang 正版用户名" />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="addUser" :loading="adding">添加</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Plus, Delete } from '@element-plus/icons-vue'

const settings = ref({
  fallback_mojang_profile: false,
  fallback_mojang_hasjoined: false,
  enable_official_whitelist: false
})
const saving = ref(false)
const statusUrls = ref({})
const statusData = ref({})
const checkingStatus = ref(false)

const whitelist = ref([])
const loadingList = ref(false)
const dialogVisible = ref(false)
const newUsername = ref('')
const adding = ref(false)

const jwt = localStorage.getItem('jwt')
const headers = { Authorization: 'Bearer ' + jwt }

// --- Settings ---
async function fetchSettings() {
  try {
    const res = await axios.get('/admin/settings', { headers })
    settings.value = {
      fallback_mojang_profile: res.data.fallback_mojang_profile,
      fallback_mojang_hasjoined: res.data.fallback_mojang_hasjoined,
      enable_official_whitelist: res.data.enable_official_whitelist
    }
    statusUrls.value = {
      session: res.data.mojang_session_url,
      account: res.data.mojang_account_url,
      services: res.data.mojang_services_url
    }
    checkMojangStatus()
  } catch (e) {
    ElMessage.error('无法加载设置')
  }
}

async function saveSettings() {
  saving.value = true
  try {
    await axios.post('/admin/settings', settings.value, { headers })
    ElMessage.success('设置已保存')
    if (settings.value.enable_official_whitelist) {
      fetchWhitelist()
    }
  } catch (e) {
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

// --- Mojang Status ---
async function checkMojangStatus() {
  checkingStatus.value = true
  for (const key in statusUrls.value) {
    statusData.value[key] = 'checking'
    try {
      // Simple fetch check (requires CORS or no-cors mode, here assuming backend proxy or loose browser check)
      // Since these are external, we might rely on browser reporting simple success/fail or just use fetch
      // Note: 'no-cors' limits response access, but we just want to know if it connects.
      await fetch(statusUrls.value[key], { mode: 'no-cors', method: 'HEAD' })
      statusData.value[key] = 'online'
    } catch (e) {
      statusData.value[key] = 'offline'
    }
  }
  checkingStatus.value = false
}

function getStatusType(status) {
  if (status === 'online') return 'success'
  if (status === 'checking') return 'info'
  return 'danger'
}

function formatStatus(status) {
  if (status === 'online') return '在线'
  if (status === 'checking') return '...'
  return '异常'
}

// --- Whitelist ---
async function fetchWhitelist() {
  if (!settings.value.enable_official_whitelist) return
  loadingList.value = true
  try {
    const res = await axios.get('/admin/official-whitelist', { headers })
    whitelist.value = res.data
  } catch (e) {
    ElMessage.error('无法加载白名单')
  } finally {
    loadingList.value = false
  }
}

async function addUser() {
  if (!newUsername.value) return
  adding.value = true
  try {
    await axios.post('/admin/official-whitelist', { username: newUsername.value }, { headers })
    ElMessage.success('添加成功')
    newUsername.value = ''
    dialogVisible.value = false
    fetchWhitelist()
  } catch (e) {
    ElMessage.error('添加失败')
  } finally {
    adding.value = false
  }
}

async function removeUser(username) {
  try {
    await ElMessageBox.confirm(`确定要移除 ${username} 吗?`, '移除确认', {
      type: 'warning'
    })
    await axios.delete(`/admin/official-whitelist/${username}`, { headers })
    ElMessage.success('已移除')
    fetchWhitelist()
  } catch (e) {
    // cancelled or failed
  }
}

onMounted(() => {
  fetchSettings().then(() => {
     if (settings.value.enable_official_whitelist) fetchWhitelist()
  })
})
</script>

<style scoped>
.admin-mojang {
  max-width: 800px;
  margin: 0 auto;
}
.page-header {
  margin-bottom: 24px;
}
.page-header h2 {
  font-weight: 600;
  color: #303133;
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.setting-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.setting-desc {
  font-size: 13px;
  color: #909399;
}
.status-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px;
  border-radius: 4px;
  background: #f5f7fa;
}
.status-name {
  font-weight: 500;
  font-size: 14px;
}
.mb-4 {
  margin-bottom: 16px;
}
</style>