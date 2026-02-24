<template>
  <div class="admin-fallback">
    <div class="page-header">
      <div class="header-content">
        <el-icon class="header-icon"><Connection /></el-icon>
        <div class="header-text">
          <h2>Fallback 服务配置</h2>
          <p class="subtitle">管理外部 Yggdrasil 或 Mojang API 的回退逻辑与白名单</p>
        </div>
      </div>
      <el-button type="primary" :icon="Check" @click="saveSettings" :loading="saving" class="save-btn">
        保存更改
      </el-button>
    </div>

    <!-- Global Strategy Card -->
    <el-card class="modern-card mb-6" shadow="never">
      <template #header>
        <div class="card-header-title">
          <el-icon><Setting /></el-icon>
          <span>全局调度策略</span>
        </div>
      </template>
      <div class="strategy-container">
        <el-radio-group v-model="settings.fallback_strategy" class="modern-radio">
          <el-radio-button label="serial">
            <div class="radio-content">
              <el-icon><Sort /></el-icon>
              <span>顺序重试</span>
            </div>
          </el-radio-button>
          <el-radio-button label="parallel">
            <div class="radio-content">
              <el-icon><Operation /></el-icon>
              <span>并发请求</span>
            </div>
          </el-radio-button>
        </el-radio-group>
        <div class="strategy-info">
          <p v-if="settings.fallback_strategy === 'serial'">
            系统将按照列表优先级顺序逐个尝试 Fallback 端点，直到获得成功响应或遍历完所有服务。
          </p>
          <p v-else>
            系统将同时向所有启用的端点发起并发请求，并采用最快返回的有效响应，适用于追求高性能的场景。
          </p>
        </div>
      </div>
    </el-card>

    <!-- Endpoints List -->
    <el-card class="modern-card" shadow="never">
      <template #header>
        <div class="card-header">
          <div class="card-header-title">
            <el-icon><List /></el-icon>
            <span>Fallback 服务链</span>
          </div>
          <el-button size="small" :icon="Plus" @click="addFallback" plain>添加端点</el-button>
        </div>
      </template>

      <el-table 
        :data="fallbacks" 
        row-key="rowKey"
        class="modern-table"
        @expand-change="handleExpandChange"
      >
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="expanded-wrapper">
              <div class="config-section">
                <div class="section-title">API 接口定义</div>
                <div class="url-grid">
                  <div class="url-item">
                    <label>Session URL</label>
                    <el-input v-model="row.session_url" placeholder="https://sessionserver.mojang.com" />
                  </div>
                  <div class="url-item">
                    <label>Account URL</label>
                    <el-input v-model="row.account_url" placeholder="https://api.mojang.com" />
                  </div>
                  <div class="url-item">
                    <label>Services URL</label>
                    <el-input v-model="row.services_url" placeholder="https://api.minecraftservices.com" />
                  </div>
                  <div class="url-item">
                    <label>材质域名 (逗号分隔)</label>
                    <el-input v-model="row.skin_domains_text" placeholder="textures.minecraft.net" />
                  </div>
                </div>
              </div>

              <div class="config-section mt-6">
                <div class="section-title">功能与权限控制</div>
                <div class="features-panel">
                  <div class="feature-card" :class="{ active: row.enable_profile }">
                    <div class="feature-main">
                      <el-switch v-model="row.enable_profile" />
                      <div class="feature-info">
                        <span class="f-name">Profile 转发</span>
                        <span class="f-desc">允许向此端点查询 UUID 和皮肤材质</span>
                      </div>
                    </div>
                  </div>
                  <div class="feature-card" :class="{ active: row.enable_hasjoined }">
                    <div class="feature-main">
                      <el-switch v-model="row.enable_hasjoined" />
                      <div class="feature-info">
                        <span class="f-name">Auth 认证回退</span>
                        <span class="f-desc">本地验证失败后尝试以此端点验证 session</span>
                      </div>
                    </div>
                  </div>
                  <div class="feature-card" :class="{ active: row.enable_whitelist }">
                    <div class="feature-main">
                      <el-switch v-model="row.enable_whitelist" @change="(val) => onWhitelistToggle(row, val)" />
                      <div class="feature-info">
                        <span class="f-name">开启白名单</span>
                        <span class="f-desc">仅允许特定玩家使用此端点进行验证</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Whitelist Section -->
              <transition name="el-zoom-in-top">
                <div v-if="row.enable_whitelist" class="whitelist-section mt-6">
                  <div class="section-header-small">
                    <div class="section-title">端点白名单列表</div>
                    <div class="add-user-form" v-if="row.id">
                      <el-input 
                        v-model="row._new_user" 
                        placeholder="输入 Minecraft ID" 
                        size="small"
                        @keyup.enter="addUser(row)"
                      >
                        <template #append>
                          <el-button @click="addUser(row)" :loading="row._adding">添加</el-button>
                        </template>
                      </el-input>
                    </div>
                  </div>
                  
                  <div v-if="!row.id" class="save-notice">
                    <el-icon><InfoFilled /></el-icon>
                    <span>请先保存端点设置，随后即可管理白名单用户。</span>
                  </div>
                  
                  <div v-else class="whitelist-table-wrapper">
                    <el-table :data="row._whitelist || []" size="small" class="inner-table" max-height="250">
                      <el-table-column prop="username" label="玩家 ID" />
                      <el-table-column prop="created_at" label="授权时间" width="160">
                        <template #default="scope">
                          {{ new Date(scope.row.created_at).toLocaleDateString() }}
                        </template>
                      </el-table-column>
                      <el-table-column label="操作" width="60" align="center">
                        <template #default="scope">
                          <el-button type="danger" :icon="Delete" size="small" @click="removeUser(row, scope.row.username)" link />
                        </template>
                      </el-table-column>
                    </el-table>
                  </div>
                </div>
              </transition>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="服务备注" min-width="240">
          <template #default="scope">
            <div class="note-container">
              <div class="priority-pill">{{ scope.$index + 1 }}</div>
              <el-input 
                v-model="scope.row.note" 
                placeholder="设置端点备注 (如: Mojang 官方)" 
                class="flat-input"
              />
              <div class="row-indicators">
                <el-tooltip content="Profile 转发" v-if="scope.row.enable_profile">
                  <el-icon class="i-profile"><User /></el-icon>
                </el-tooltip>
                <el-tooltip content="Auth 认证" v-if="scope.row.enable_hasjoined">
                  <el-icon class="i-auth"><Lock /></el-icon>
                </el-tooltip>
                <el-tooltip content="白名单保护" v-if="scope.row.enable_whitelist">
                  <el-icon class="i-whitelist"><ShieldCheck /></el-icon>
                </el-tooltip>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="缓存 (秒)" width="120" align="center">
          <template #default="scope">
            <el-input-number 
              v-model="scope.row.cache_ttl" 
              :min="0" 
              :controls="false" 
              size="small" 
              class="narrow-num"
            />
          </template>
        </el-table-column>

        <el-table-column label="调度控制" width="160" align="right">
          <template #default="scope">
            <div class="action-btns">
              <el-tooltip content="上移">
                <el-button :icon="ArrowUp" size="small" circle @click="moveUp(scope.$index)" :disabled="scope.$index === 0" />
              </el-tooltip>
              <el-tooltip content="下移">
                <el-button :icon="ArrowDown" size="small" circle @click="moveDown(scope.$index)" :disabled="scope.$index === fallbacks.length - 1" />
              </el-tooltip>
              <el-button :icon="Delete" size="small" type="danger" circle plain @click="removeFallback(scope.$index)" />
            </div>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, reactive } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { 
  Plus, Delete, ArrowUp, ArrowDown, Connection, Check, Setting, 
  Sort, Operation, List, InfoFilled, User, Lock, Ticket as ShieldCheck 
} from '@element-plus/icons-vue'

const settings = ref({
  fallback_strategy: 'serial'
})
const fallbacks = ref([])
const saving = ref(false)

const jwt = localStorage.getItem('jwt')
const headers = { Authorization: 'Bearer ' + jwt }

async function fetchSettings() {
  try {
    const res = await axios.get('/admin/settings', { headers })
    settings.value.fallback_strategy = res.data.fallback_strategy || 'serial'
    
    const raw = Array.isArray(res.data.fallbacks) ? res.data.fallbacks : []
    fallbacks.value = raw.map((item, index) => {
      return reactive({
        ...item,
        rowKey: item.id || `new_${Date.now()}_${index}`,
        note: item.note || '',
        skin_domains_text: Array.isArray(item.skin_domains) ? item.skin_domains.join(',') : String(item.skin_domains || ''),
        _whitelist: [],
        _new_user: '',
        _adding: false,
        _loaded: false
      })
    })
    fallbacks.value.sort((a, b) => a.priority - b.priority)
  } catch (e) {
    ElMessage.error('加载 Fallback 配置失败')
  }
}

async function saveSettings() {
  saving.value = true
  try {
    const payload = {
      fallback_strategy: settings.value.fallback_strategy,
      fallbacks: fallbacks.value.map(item => ({
        id: item.id,
        priority: item.priority,
        session_url: item.session_url,
        account_url: item.account_url,
        services_url: item.services_url,
        cache_ttl: item.cache_ttl,
        enable_profile: !!item.enable_profile,
        enable_hasjoined: !!item.enable_hasjoined,
        enable_whitelist: !!item.enable_whitelist,
        note: item.note,
        skin_domains: item.skin_domains_text.split(',').map(s => s.trim()).filter(s => s)
      }))
    }
    await axios.post('/admin/settings', payload, { headers })
    ElMessage.success('配置已更新')
    await fetchSettings()
  } catch (e) {
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

function addFallback() {
  fallbacks.value.push(reactive({
    id: null,
    rowKey: `new_${Date.now()}_${fallbacks.value.length}`,
    priority: fallbacks.value.length + 1,
    session_url: '',
    account_url: '',
    services_url: '',
    cache_ttl: 60,
    enable_profile: true,
    enable_hasjoined: true,
    enable_whitelist: false,
    note: '',
    skin_domains_text: '',
    _whitelist: [],
    _new_user: '',
    _adding: false,
    _loaded: false
  }))
}

function removeFallback(index) {
  fallbacks.value.splice(index, 1)
  syncPriority()
}

function moveUp(index) {
  if (index === 0) return
  const list = [...fallbacks.value]
  const temp = list[index]
  list[index] = list[index - 1]
  list[index - 1] = temp
  fallbacks.value = list
  syncPriority()
}

function moveDown(index) {
  if (index === fallbacks.value.length - 1) return
  const list = [...fallbacks.value]
  const temp = list[index]
  list[index] = list[index + 1]
  list[index + 1] = temp
  fallbacks.value = list
  syncPriority()
}

function syncPriority() {
  fallbacks.value.forEach((item, index) => {
    item.priority = index + 1
  })
}

function handleExpandChange(row, expandedRows) {
  const isExpanded = expandedRows.find(r => r.rowKey === row.rowKey)
  if (isExpanded && row.enable_whitelist && row.id && !row._loaded) {
    fetchWhitelist(row)
  }
}

function onWhitelistToggle(row, val) {
  if (val && row.id && !row._loaded) {
    fetchWhitelist(row)
  }
}

async function fetchWhitelist(row) {
  if (!row.id) return
  try {
    const res = await axios.get('/admin/official-whitelist', {
      headers,
      params: { endpoint_id: row.id }
    })
    row._whitelist = res.data
    row._loaded = true
  } catch (e) {
    ElMessage.error(`白名单加载失败: ${row.note || '未命名端点'}`)
  }
}

async function addUser(row) {
  if (!row._new_user || !row.id) return
  row._adding = true
  try {
    await axios.post('/admin/official-whitelist', {
      username: row._new_user,
      endpoint_id: row.id
    }, { headers })
    ElMessage.success('用户添加成功')
    row._new_user = ''
    await fetchWhitelist(row)
  } catch (e) {
    ElMessage.error('添加用户失败')
  } finally {
    row._adding = false
  }
}

async function removeUser(row, username) {
  try {
    await ElMessageBox.confirm(`确定要移除授权用户 ${username} 吗？`, '移除确认', { 
      confirmButtonText: '确定移除',
      cancelButtonText: '取消',
      type: 'warning' 
    })
    await axios.delete(`/admin/official-whitelist/${username}`, {
      headers,
      params: { endpoint_id: row.id }
    })
    ElMessage.success('已移除授权')
    await fetchWhitelist(row)
  } catch (e) {}
}

onMounted(fetchSettings)
</script>

<style scoped>
.admin-fallback {
  max-width: 1100px;
  margin: 0 auto;
  padding: 20px 0;
}

/* Page Header */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 30px;
}
.header-content {
  display: flex;
  align-items: center;
  gap: 16px;
}
.header-icon {
  font-size: 32px;
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  padding: 12px;
  border-radius: 12px;
}
.header-text h2 {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
  color: var(--color-heading);
}
.header-text .subtitle {
  margin: 4px 0 0 0;
  color: var(--color-text-light);
  font-size: 14px;
}

/* Modern Card */
.modern-card {
  border: 1px solid var(--color-border);
  border-radius: 12px;
  overflow: hidden;
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.card-header-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  color: var(--color-heading);
}

/* Strategy Panel */
.strategy-container {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 10px 0;
}
.modern-radio :deep(.el-radio-button__inner) {
  height: 48px;
  display: flex;
  align-items: center;
  padding: 0 30px;
  border-radius: 8px !important;
  margin-right: 12px;
  border: 1px solid var(--color-border) !important;
  box-shadow: none !important;
}
.modern-radio :deep(.el-radio-button.is-active .el-radio-button__inner) {
  background-color: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  border-color: var(--el-color-primary) !important;
}
.radio-content {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}
.strategy-info {
  font-size: 13px;
  color: var(--color-text-light);
  background: var(--color-background-soft);
  padding: 12px 16px;
  border-radius: 8px;
  border-left: 4px solid var(--el-color-primary);
}

/* Note Column */
.note-container {
  display: flex;
  align-items: center;
  gap: 10px;
}
.priority-pill {
  background: var(--el-color-primary);
  color: #fff;
  font-size: 11px;
  font-weight: bold;
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  flex-shrink: 0;
}
.flat-input :deep(.el-input__wrapper) {
  box-shadow: none !important;
  padding: 0;
  background: transparent;
}
.flat-input :deep(.el-input__inner) {
  font-weight: 500;
  color: var(--color-text-primary);
}
.row-indicators {
  display: flex;
  gap: 6px;
  margin-left: auto;
}
.row-indicators .el-icon {
  font-size: 14px;
}
.i-profile { color: var(--el-color-success); }
.i-auth { color: var(--el-color-primary); }
.i-whitelist { color: var(--el-color-warning); }

/* Expanded Area */
.expanded-wrapper {
  padding: 24px 30px;
  background: var(--color-background-soft);
  border-top: 1px solid var(--color-border);
}
.section-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-secondary);
  margin-bottom: 12px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.url-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}
.url-item label {
  display: block;
  font-size: 12px;
  color: var(--color-text-light);
  margin-bottom: 6px;
}

/* Features Panel */
.features-panel {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}
.feature-card {
  background: var(--color-card-background);
  border: 1px solid var(--color-border);
  padding: 16px;
  border-radius: 10px;
  transition: all 0.3s;
}
.feature-card.active {
  border-color: var(--el-color-primary-light-5);
  background: var(--el-color-primary-light-9);
}
.feature-main {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}
.feature-info {
  display: flex;
  flex-direction: column;
}
.f-name { font-size: 14px; font-weight: 600; color: var(--color-heading); }
.f-desc { font-size: 11px; color: var(--color-text-light); margin-top: 2px; }

/* Whitelist Section */
.whitelist-section {
  background: var(--color-card-background);
  border: 1px solid var(--color-border);
  border-radius: 10px;
  padding: 20px;
}
.section-header-small {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
}
.add-user-form { width: 300px; }
.save-notice {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--el-color-warning);
  font-size: 13px;
  padding: 12px;
  background: var(--el-color-warning-light-9);
  border-radius: 6px;
}

/* Helpers */
.mb-6 { margin-bottom: 24px; }
.mt-6 { margin-top: 24px; }
.narrow-num :deep(.el-input__inner) { text-align: center; }

.action-btns {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

@media (max-width: 768px) {
  .url-grid, .features-panel { grid-template-columns: 1fr; }
  .expanded-wrapper { padding: 16px; }
  .section-header-small { flex-direction: column; align-items: flex-start; gap: 10px; }
  .add-user-form { width: 100%; }
}
</style>