<template>
  <div class="dashboard-home animate-fade-in">
    <!-- Stats Section -->
    <div class="stats-section">
      <el-row :gutter="20">
        <el-col :xs="24" :sm="12">
          <el-card shadow="hover" class="surface-card">
            <div class="stats-card-content">
              <div class="stats-card-icon bg-gradient-blue">
                <el-icon><Box /></el-icon>
              </div>
              <div class="stats-card-info">
                <div class="stats-card-label">材质数量</div>
                <div class="stats-card-value">{{ textureCount }}</div>
              </div>
            </div>
          </el-card>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-card shadow="hover" class="surface-card">
            <div class="stats-card-content">
              <div class="stats-card-icon bg-gradient-purple">
                <el-icon><User /></el-icon>
              </div>
              <div class="stats-card-info">
                <div class="stats-card-label">角色数量</div>
                <div class="stats-card-value">{{ profileCount }}</div>
              </div>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </div>

    <!-- Quick Config Section -->
    <div class="config-section">
      <el-card shadow="hover" class="surface-card config-card">
        <template #header>
          <div class="card-header">
            <span>快速配置启动器</span>
          </div>
        </template>
        <div class="config-content">
          <p class="config-desc">
            将下方的 API 地址复制到您的启动器，或直接拖动“添加到启动器”按钮到支持 authlib-injector 的启动器窗口中。
          </p>
          <div class="api-url-box">
            <el-input v-model="apiUrl" readonly>
              <template #append>
                <el-button @click="copyApiUrl">
                  <el-icon><CopyDocument /></el-icon> 复制
                </el-button>
              </template>
            </el-input>
          </div>
          <div class="drag-action">
            <a 
              class="el-button el-button--primary is-round drag-btn" 
              :href="`authlib-injector:yggdrasil-server:${encodeURIComponent(apiUrl)}`"
              title="拖动我到启动器"
            >
              <el-icon><Pointer /></el-icon>
              <span>拖拽添加到启动器</span>
            </a>
          </div>
        </div>
      </el-card>
    </div>

    <!-- Fallback Service Status Section -->
    <div v-if="fallbackEntries.length" class="mojang-status-section">
      <div class="section-header">
        <h2>Fallback 服务状态</h2>
        <el-button @click="loadFallbackStatus" :loading="isChecking" size="small">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </div>

      <div class="status-container">
        <el-card
          v-for="entry in fallbackEntries"
          :key="entry.id"
          shadow="hover"
          class="surface-card fallback-status-card"
        >
          <div class="fallback-card-header">
            <div class="fallback-title-block">
              <span class="fallback-priority">#{{ entry.priority }}</span>
              <span class="fallback-note">{{ entry.note || '未命名端点' }}</span>
            </div>
            <div class="fallback-overall" :class="overallClass(entry)">
              <el-icon v-if="overallStatus(entry) === 'online'"><Check /></el-icon>
              <el-icon v-else-if="overallStatus(entry) === 'partial'"><Warning /></el-icon>
              <el-icon v-else-if="overallStatus(entry) === 'unknown'"><Loading /></el-icon>
              <el-icon v-else><CircleClose /></el-icon>
              <span>{{ overallText(entry) }}</span>
            </div>
          </div>

          <div class="fallback-current">
            <div
              v-for="api in API_ROWS"
              :key="api.key"
              class="fallback-current-cell"
              :class="currentCellClass(entry, api.key)"
            >
              <span class="fallback-current-label">{{ api.label }}</span>
              <span class="fallback-current-status">{{ currentStatusText(entry, api.key) }}</span>
            </div>
          </div>

          <div class="fallback-history">
            <div class="fallback-history-header">
              <span>近 24 小时探测</span>
              <span class="fallback-history-meta">{{ historyMeta(entry) }}</span>
            </div>
            <div class="fallback-history-grid">
              <div
                v-for="api in API_ROWS"
                :key="api.key"
                class="fallback-history-row"
              >
                <span class="fallback-history-row-label">{{ api.label }}</span>
                <div class="fallback-history-track">
                  <span
                    v-for="(tick, idx) in entry.history"
                    :key="idx"
                    class="fallback-history-dot"
                    :class="dotClass(tick[api.key])"
                    :title="tickTitle(tick, api.key)"
                  />
                  <span v-if="!entry.history.length" class="fallback-history-empty">暂无历史数据</span>
                </div>
              </div>
            </div>
          </div>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getPublicSettings, getPublicFallbackStatus } from '@/api/public'
import { getMe } from '@/api/me'
import {
  Box, User, CopyDocument, Pointer,
  Check, Loading, Warning, Refresh, CircleClose
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import type { FallbackStatusEntry, FallbackStatusTick } from '@/api/types'

// --- Stats & Config ---
const textureCount = ref(0)
const profileCount = ref(0)
const apiUrl = ref('')

function getApiUrl() {
    const base = import.meta.env.VITE_API_BASE || ''
    if (base.startsWith('http')) {
        return base
    }
    const origin = window.location.origin
    const path = base.startsWith('/') ? base : '/' + base
    let full = origin + path
    if (full.endsWith('/') && full.length > 1) {
        full = full.slice(0, -1)
    }
    return full
}

function copyApiUrl() {
  if (!apiUrl.value) return
  navigator.clipboard.writeText(apiUrl.value).then(() => {
    ElMessage.success('API 地址已复制')
  }).catch(() => {
    ElMessage.error('复制失败，请手动复制')
  })
}

// --- Fallback Status ---
type ApiKey = 'session' | 'account' | 'services'
const API_ROWS: { key: ApiKey; label: string }[] = [
  { key: 'session', label: 'Session' },
  { key: 'account', label: 'Account' },
  { key: 'services', label: 'Services' }
]

const fallbackEntries = ref<FallbackStatusEntry[]>([])
const isChecking = ref(false)

async function loadFallbackStatus() {
  isChecking.value = true
  try {
    const res = await getPublicFallbackStatus()
    const list = (res.data.endpoints || []).slice()
    list.sort((a, b) => (a.priority || 0) - (b.priority || 0))
    fallbackEntries.value = list
  } catch (e) {
    ElMessage.error('加载 Fallback 状态失败')
  } finally {
    isChecking.value = false
  }
}

function currentStatus(entry: FallbackStatusEntry, key: ApiKey): 'up' | 'down' | 'unknown' {
  if (!entry.latest) return 'unknown'
  const value = entry.latest[key]
  return value === 'up' ? 'up' : value === 'down' ? 'down' : 'unknown'
}

function currentCellClass(entry: FallbackStatusEntry, key: ApiKey) {
  return `status-${currentStatus(entry, key)}`
}

function currentStatusText(entry: FallbackStatusEntry, key: ApiKey) {
  const status = currentStatus(entry, key)
  if (status === 'up') return '在线'
  if (status === 'down') return '离线'
  return '未探测'
}

function overallStatus(entry: FallbackStatusEntry) {
  if (!entry.latest) return 'unknown'
  const values: ApiKey[] = ['session', 'account', 'services']
  const ups = values.filter(k => entry.latest![k] === 'up').length
  if (ups === values.length) return 'online'
  if (ups === 0) return 'offline'
  return 'partial'
}

function overallClass(entry: FallbackStatusEntry) {
  return `overall-${overallStatus(entry)}`
}

function overallText(entry: FallbackStatusEntry) {
  switch (overallStatus(entry)) {
    case 'online': return '全部在线'
    case 'partial': return '部分在线'
    case 'offline': return '全部离线'
    default: return '尚未探测'
  }
}

function dotClass(state: string) {
  return state === 'up' ? 'dot-up' : state === 'down' ? 'dot-down' : 'dot-unknown'
}

function tickTitle(tick: FallbackStatusTick, key: ApiKey) {
  const status = tick[key] === 'up' ? '在线' : '离线'
  return `${new Date(tick.checked_at).toLocaleString()} · ${status}`
}

function historyMeta(entry: FallbackStatusEntry) {
  const total = entry.history.length
  if (!total) return ''
  const ups: Record<ApiKey, number> = { session: 0, account: 0, services: 0 }
  for (const tick of entry.history) {
    if (tick.session === 'up') ups.session++
    if (tick.account === 'up') ups.account++
    if (tick.services === 'up') ups.services++
  }
  const sumUp = ups.session + ups.account + ups.services
  const total3 = total * 3
  const pct = total3 ? Math.round((sumUp / total3) * 100) : 0
  return `${total} 次探测 · 可用率 ${pct}%`
}

// --- Lifecycle ---
onMounted(async () => {
  try {
    const res = await getPublicSettings()
    if (res.data.api_url) {
      apiUrl.value = res.data.api_url.endsWith('/') ? res.data.api_url.slice(0, -1) : res.data.api_url
    } else {
      apiUrl.value = getApiUrl()
    }
  } catch (e) {
    apiUrl.value = getApiUrl()
  }

  await loadFallbackStatus()

  try {
      const res = await getMe()
      if (res.data) {
          profileCount.value = res.data.profile_count || 0
          textureCount.value = res.data.texture_count || 0
      }
  } catch (e) {
      console.error('Failed to load user stats', e)
  }
})
</script>

<style scoped>
.dashboard-home {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* Config Section Specifics */
.card-header {
  font-weight: 600;
  font-size: 18px;
  color: var(--color-heading);
}
.config-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  align-items: center;
  padding: 10px 0;
}
.config-desc {
  font-size: 14px;
  color: var(--color-text);
  text-align: center;
  margin: 0;
}
.api-url-box {
  width: 100%;
  max-width: 500px;
}
.drag-action {
  margin-top: 8px;
}
.drag-btn {
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 40px;
  padding: 0 20px;
  font-weight: 500;
  transition: transform 0.2s;
}
.drag-btn:hover {
  transform: translateY(-2px);
  color: white;
}
.drag-btn:active {
  transform: translateY(0);
}

/* Mojang Status Section Specifics */
.mojang-status-section {
    margin-top: 12px;
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.section-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--color-heading);
}

/* Fallback Status Cards */
.fallback-status-card {
  margin-bottom: 16px;
}
.fallback-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}
.fallback-title-block {
  display: flex;
  align-items: center;
  gap: 10px;
}
.fallback-priority {
  background: var(--el-color-primary);
  color: #fff;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
}
.fallback-note {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-heading);
}
.fallback-overall {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}
.overall-online { background: rgba(103, 194, 58, 0.15); color: var(--el-color-success); }
.overall-partial { background: rgba(230, 162, 60, 0.15); color: var(--el-color-warning); }
.overall-offline { background: rgba(245, 108, 108, 0.15); color: var(--el-color-danger); }
.overall-unknown { background: rgba(144, 147, 153, 0.15); color: var(--el-color-info); }

.fallback-current {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  margin-bottom: 18px;
}
.fallback-current-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px 14px;
  border-radius: 8px;
  border: 1px solid var(--color-border);
  background: var(--color-background-soft);
}
.fallback-current-label { font-size: 12px; color: var(--color-text-light); font-weight: 600; }
.fallback-current-status { font-size: 15px; font-weight: 600; }
.status-up { border-color: var(--el-color-success-light-5); }
.status-up .fallback-current-status { color: var(--el-color-success); }
.status-down { border-color: var(--el-color-danger-light-5); }
.status-down .fallback-current-status { color: var(--el-color-danger); }
.status-unknown .fallback-current-status { color: var(--el-color-info); }

.fallback-history-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  font-size: 13px;
  color: var(--color-text-light);
  margin-bottom: 10px;
}
.fallback-history-meta { font-size: 12px; }
.fallback-history-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.fallback-history-row {
  display: grid;
  grid-template-columns: 80px 1fr;
  align-items: center;
  gap: 12px;
}
.fallback-history-row-label {
  font-size: 12px;
  color: var(--color-text-light);
  font-weight: 600;
}
.fallback-history-track {
  display: flex;
  flex-wrap: wrap;
  gap: 3px;
  align-items: center;
  min-height: 14px;
}
.fallback-history-dot {
  width: 10px;
  height: 10px;
  border-radius: 3px;
  display: inline-block;
}
.dot-up { background: var(--el-color-success); }
.dot-down { background: var(--el-color-danger); }
.dot-unknown { background: var(--el-color-info-light-5); }
.fallback-history-empty { font-size: 12px; color: var(--color-text-light); }

@media (max-width: 768px) {
  .fallback-current { grid-template-columns: 1fr; }
  .fallback-history-row { grid-template-columns: 70px 1fr; }
}
</style>
