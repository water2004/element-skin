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
          <div class="header-actions">
            <el-button size="small" @click="addFallback">新增服务</el-button>
            <el-button type="primary" size="small" @click="saveSettings" :loading="saving">保存设置</el-button>
          </div>
        </div>
      </template>
      <el-form label-position="top">
        <el-form-item label="Fallback 策略">
          <el-select v-model="settings.fallback_strategy" placeholder="选择策略" style="width: 220px">
            <el-option label="顺序尝试" value="serial" />
            <el-option label="并发尝试" value="parallel" />
          </el-select>
          <span class="setting-desc">按优先级顺序或并发请求回退服务。</span>
        </el-form-item>
        <el-form-item label="Fallback 服务启用与顺序">
          <el-table :data="fallbacks" style="width: 100%" size="small">
            <el-table-column label="服务" min-width="120">
              <template #default="scope">
                <el-input v-model="scope.row.name" size="small" placeholder="service-name" @blur="normalizeFallbackLists" />
              </template>
            </el-table-column>
            <el-table-column label="URLs" min-width="240">
              <template #default="scope">
                <div class="fallback-urls">
                  <el-input v-model="scope.row.session_url" size="small" placeholder="Session URL" />
                  <el-input v-model="scope.row.account_url" size="small" placeholder="Account URL" />
                  <el-input v-model="scope.row.services_url" size="small" placeholder="Services URL" />
                </div>
              </template>
            </el-table-column>
            <el-table-column label="Skin Domains" min-width="200">
              <template #default="scope">
                <el-input v-model="scope.row.skin_domains_text" size="small" placeholder="textures.minecraft.net" />
              </template>
            </el-table-column>
            <el-table-column label="缓存 TTL" width="120">
              <template #default="scope">
                <el-input-number v-model="scope.row.cache_ttl" :min="1" :step="60" size="small" />
              </template>
            </el-table-column>
            <el-table-column label="操作" width="200" align="right">
              <template #default="scope">
                <div class="fallback-actions">
                  <el-switch :model-value="isEnabled(scope.row.name)" @change="toggleEnabled(scope.row.name)" />
                  <div class="fallback-actions-row">
                    <el-button size="small" @click="moveUp(scope.row.name)">上移</el-button>
                    <el-button size="small" @click="moveDown(scope.row.name)">下移</el-button>
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
  fallback_enabled_services: [],
  fallback_priority: [],
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
      fallback_enabled_services: res.data.fallback_enabled_services || [],
      fallback_priority: res.data.fallback_priority || [],
      fallback_strategy: res.data.fallback_strategy || 'serial'
    }
    fallbacks.value = Array.isArray(res.data.fallbacks) ? res.data.fallbacks : []
    normalizeFallbackEntries()
    const fallbackNames = fallbacks.value.map((item) => item.name)
    if (!settings.value.fallback_enabled_services.length) {
      settings.value.fallback_enabled_services = [...fallbackNames]
    }
    if (!settings.value.fallback_priority.length) {
      settings.value.fallback_priority = [...fallbackNames]
    } else {
      const known = settings.value.fallback_priority.filter((name) => fallbackNames.includes(name))
      const missing = fallbackNames.filter((name) => !known.includes(name))
      settings.value.fallback_priority = [...known, ...missing]
    }
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

function isEnabled(name) {
  return settings.value.fallback_enabled_services.includes(name)
}

function toggleEnabled(name) {
  const list = settings.value.fallback_enabled_services
  if (list.includes(name)) {
    settings.value.fallback_enabled_services = list.filter((item) => item !== name)
  } else {
    settings.value.fallback_enabled_services = [...list, name]
  }
}

function moveUp(name) {
  const list = [...settings.value.fallback_priority]
  const index = list.indexOf(name)
  if (index > 0) {
    const temp = list[index - 1]
    list[index - 1] = list[index]
    list[index] = temp
    settings.value.fallback_priority = list
    applyPriorityToFallbacks()
  }
}

function moveDown(name) {
  const list = [...settings.value.fallback_priority]
  const index = list.indexOf(name)
  if (index !== -1 && index < list.length - 1) {
    const temp = list[index + 1]
    list[index + 1] = list[index]
    list[index] = temp
    settings.value.fallback_priority = list
    applyPriorityToFallbacks()
  }
}

function normalizeFallbackEntries() {
  fallbacks.value = fallbacks.value.map((item, index) => ({
    name: item.name || `fallback_${index + 1}`,
    session_url: item.session_url || '',
    account_url: item.account_url || '',
    services_url: item.services_url || '',
    skin_domains: Array.isArray(item.skin_domains) ? item.skin_domains : [],
    skin_domains_text: Array.isArray(item.skin_domains)
      ? item.skin_domains.join(',')
      : '',
    cache_ttl: Number(item.cache_ttl || 60)
  }))
  normalizeFallbackLists()
  applyPriorityToFallbacks()
}

function normalizeFallbackLists() {
  const names = fallbacks.value.map((item) => item.name).filter((name) => name)
  const enabled = settings.value.fallback_enabled_services.filter((name) => names.includes(name))
  const priority = settings.value.fallback_priority.filter((name) => names.includes(name))
  const missingEnabled = names.filter((name) => !enabled.includes(name))
  const missingPriority = names.filter((name) => !priority.includes(name))
  settings.value.fallback_enabled_services = [...enabled, ...missingEnabled]
  settings.value.fallback_priority = [...priority, ...missingPriority]
}

function applyPriorityToFallbacks() {
  const byName = new Map(fallbacks.value.map((item) => [item.name, item]))
  const ordered = settings.value.fallback_priority
    .map((name) => byName.get(name))
    .filter((item) => item)
  const remaining = fallbacks.value.filter((item) => !settings.value.fallback_priority.includes(item.name))
  fallbacks.value = [...ordered, ...remaining]
}

function addFallback() {
  fallbacks.value.push({
    name: `fallback_${fallbacks.value.length + 1}`,
    session_url: '',
    account_url: '',
    services_url: '',
    skin_domains: [],
    skin_domains_text: '',
    cache_ttl: 60
  })
  normalizeFallbackLists()
  applyPriorityToFallbacks()
}

function removeFallback(index) {
  fallbacks.value.splice(index, 1)
  normalizeFallbackLists()
  applyPriorityToFallbacks()
}

function serializeFallbacks() {
  return fallbacks.value.map((item) => ({
    name: (item.name || '').trim(),
    session_url: (item.session_url || '').trim(),
    account_url: (item.account_url || '').trim(),
    services_url: (item.services_url || '').trim(),
    skin_domains: (item.skin_domains_text || '')
      .split(',')
      .map((value) => value.trim())
      .filter((value) => value),
    cache_ttl: Number(item.cache_ttl || 60)
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
}
.page-header {
  margin-bottom: 24px;
}
.page-header h2 {
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