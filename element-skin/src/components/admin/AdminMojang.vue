<template>
  <div class="admin-mojang">
    <div class="section-header">
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
          <div class="header-actions">
            <el-button size="small" @click="addFallback">新增服务</el-button>
            <el-button type="primary" size="small" @click="saveSettings" :loading="saving">保存设置</el-button>
          </div>
        </div>
      </template>
      <el-form label-position="top">
        <el-form-item label="Fallback 策略">
          <el-select v-model="settings.fallback_strategy" placeholder="选择策略" style="width: 120px">
            <el-option label="顺序尝试" value="serial" />
            <el-option label="并发尝试" value="parallel" />
          </el-select>
          <span class="setting-desc">按优先级顺序或并发请求回退服务。</span>
        </el-form-item>
        <el-form-item label="Fallback 服务顺序">
          <el-table :data="fallbacks" style="width: 100%" size="small">
            <el-table-column label="Endpoint ID" min-width="40">
              <template #default="scope">
                <span class="mono">{{ scope.row.id ?? 'new' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="URLs" min-width="200">
              <template #default="scope">
                <div class="fallback-urls">
                  <el-input v-model="scope.row.session_url" size="small" placeholder="Session URL" />
                  <el-input v-model="scope.row.account_url" size="small" placeholder="Account URL" />
                  <el-input v-model="scope.row.services_url" size="small" placeholder="Services URL" />
                </div>
              </template>
            </el-table-column>
            <el-table-column label="Skin Domains" min-width="160">
              <template #default="scope">
                <el-input v-model="scope.row.skin_domains_text" size="small" placeholder="textures.minecraft.net" />
              </template>
            </el-table-column>
            <el-table-column label="缓存 TTL" width="140">
              <template #default="scope">
                <el-input-number v-model="scope.row.cache_ttl" :min="1" :step="60" size="small" />
              </template>
            </el-table-column>
            <el-table-column label="操作" width="200" align="right">
              <template #default="scope">
                <div class="fallback-actions">
                  <div class="fallback-actions-row">
                    <el-button size="small" @click="moveUp(scope.$index)">上移</el-button>
                    <el-button size="small" @click="moveDown(scope.$index)">下移</el-button>
                    <el-button size="small" type="danger" @click="removeFallback(scope.$index)">删除</el-button>
                  </div>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </el-form-item>
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
  enable_official_whitelist: false,
  fallback_strategy: 'serial'
})
const saving = ref(false)
const statusUrls = ref({})
const statusData = ref({})
const checkingStatus = ref(false)
const fallbacks = ref([])

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
      enable_official_whitelist: res.data.enable_official_whitelist,
      fallback_strategy: res.data.fallback_strategy || 'serial'
    }
    fallbacks.value = Array.isArray(res.data.fallbacks) ? res.data.fallbacks : []
    normalizeFallbackEntries()
    statusUrls.value = res.data.fallback_status_urls || {
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
    const payload = {
      ...settings.value,
      fallbacks: serializeFallbacks()
    }
    await axios.post('/admin/settings', payload, { headers })
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

function moveUp(index) {
  if (index <= 0) return
  const list = [...fallbacks.value]
  const temp = list[index - 1]
  list[index - 1] = list[index]
  list[index] = temp
  fallbacks.value = list
  syncPriorityFromOrder()
}

function moveDown(index) {
  const list = [...fallbacks.value]
  if (index < 0 || index >= list.length - 1) return
  const temp = list[index + 1]
  list[index + 1] = list[index]
  list[index] = temp
  fallbacks.value = list
  syncPriorityFromOrder()
}

function normalizeFallbackEntries() {
  fallbacks.value = fallbacks.value.map((item, index) => ({
    id: item.id ?? null,
    priority: Number(item.priority || index + 1),
    session_url: item.session_url || '',
    account_url: item.account_url || '',
    services_url: item.services_url || '',
    cache_ttl: Number(item.cache_ttl || 60),
    skin_domains_text: Array.isArray(item.skin_domains)
      ? item.skin_domains.join(',')
      : String(item.skin_domains || '')
  }))
  fallbacks.value.sort((a, b) => (a.priority || 0) - (b.priority || 0))
  syncPriorityFromOrder()
}

function syncPriorityFromOrder() {
  fallbacks.value = fallbacks.value.map((item, index) => ({
    ...item,
    priority: index + 1
  }))
}

function addFallback() {
  fallbacks.value.push({
    id: null,
    priority: fallbacks.value.length + 1,
    session_url: '',
    account_url: '',
    services_url: '',
    cache_ttl: 60,
    skin_domains_text: ''
  })
  syncPriorityFromOrder()
}

function removeFallback(index) {
  fallbacks.value.splice(index, 1)
  syncPriorityFromOrder()
}

function serializeFallbacks() {
  return fallbacks.value.map((item) => ({
    id: item.id ?? null,
    priority: Number(item.priority || 0),
    session_url: (item.session_url || '').trim(),
    account_url: (item.account_url || '').trim(),
    services_url: (item.services_url || '').trim(),
    cache_ttl: Number(item.cache_ttl || 60),
    skin_domains: (item.skin_domains_text || '')
      .split(',')
      .map((value) => value.trim())
      .filter((value) => value)
  }))
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
  max-width: 1200px;
  margin: 0 auto;
  width: 100%;
  animation: fadeIn 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}
.section-header {
  margin-bottom: 24px;
}
.section-header h2 {
  font-weight: 600;
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
    "Liberation Mono", "Courier New", monospace;
  font-size: 12px;
  color: var(--color-text-secondary);
}
  color: var(--color-heading);
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  color: var(--color-heading);
}
.header-actions {
  display: flex;
  gap: 8px;
}

.box-card {
  border: 1px solid var(--color-border);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  background: var(--color-card-background);
  animation: cardSlideIn 0.5s cubic-bezier(0.4, 0, 0.2, 1);
}

/* Limit inner content width */
.admin-mojang .el-card__body {
  padding: 20px;
}
.admin-mojang .el-form {
  max-width: 100%;
}
.admin-mojang .el-table .cell {
  white-space: normal;
}
.fallback-urls {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.fallback-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 6px;
}
.fallback-actions-row {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 6px;
}

.setting-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.setting-desc {
  font-size: 13px;
  color: var(--color-text-light);
  margin-left: 12px;
}
.status-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px;
  border-radius: 4px;
  background: var(--color-background-soft);
  margin-bottom: 8px; /* For mobile stacking if grid changes */
}
.status-name {
  font-weight: 500;
  font-size: 14px;
}
.mb-4 {
  margin-bottom: 16px;
}

@media (max-width: 768px) {
  .admin-mojang {
    padding: 0;
  }
  .setting-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  .el-col {
    width: 100% !important;
    max-width: 100% !important;
    flex: 0 0 100% !important;
    margin-bottom: 8px;
  }
}
</style>