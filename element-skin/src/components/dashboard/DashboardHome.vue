<template>
  <div class="dashboard-home animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <h1>我的主页</h1>
        <p>这里汇总了您的资源数量、启动器接入入口与备用服务的健康状态</p>
      </div>
    </div>

    <section class="dashboard-section stats-section">
      <div class="stats-grid">
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
      </div>
    </section>

    <section class="dashboard-section">
      <div class="section-header">
        <h2>快速接入启动器</h2>
      </div>
      <el-card shadow="hover" class="surface-card">
        <div class="config-content">
          <p class="config-desc">
            将下方的 API 地址复制到您的启动器，或直接拖动下方按钮到支持 authlib-injector 的启动器窗口中。
          </p>
          <div class="config-actions">
            <el-input v-model="apiUrl" readonly class="api-url-input">
              <template #append>
                <el-button @click="copyApiUrl">
                  <el-icon><CopyDocument /></el-icon>
                  <span>复制</span>
                </el-button>
              </template>
            </el-input>
            <a
              class="el-button el-button--primary drag-btn"
              :href="`authlib-injector:yggdrasil-server:${encodeURIComponent(apiUrl)}`"
              title="拖动我到启动器"
            >
              <el-icon><Pointer /></el-icon>
              <span>拖到启动器</span>
            </a>
          </div>
        </div>
      </el-card>
    </section>

    <section v-if="fallbackEntries.length" class="dashboard-section">
      <div class="section-header">
        <h2>备用服务状态</h2>
        <el-button @click="loadFallbackStatus" :loading="isChecking" size="small" text>
          <el-icon><Refresh /></el-icon>
          <span>刷新</span>
        </el-button>
      </div>

      <div class="fallback-list">
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
              <span>近 24 小时</span>
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
                    v-for="(bucket, idx) in hourlyBuckets(entry, api.key)"
                    :key="idx"
                    class="fallback-history-cell"
                    :class="bucketClass(bucket)"
                    :title="bucketTitle(bucket)"
                  />
                </div>
              </div>
            </div>
            <div class="fallback-history-axis">
              <span>24h 前</span>
              <span>现在</span>
            </div>
          </div>
        </el-card>
      </div>
    </section>
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
import type { FallbackStatusEntry } from '@/api/types'

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

interface HourBucket {
  hourLabel: string
  total: number
  up: number
  down: number
}

function hourlyBuckets(entry: FallbackStatusEntry, key: ApiKey): HourBucket[] {
  const now = Date.now()
  const buckets: HourBucket[] = []
  for (let i = 23; i >= 0; i--) {
    const start = new Date(now - i * 3600_000)
    start.setMinutes(0, 0, 0)
    buckets.push({
      hourLabel: `${start.getHours().toString().padStart(2, '0')}:00`,
      total: 0,
      up: 0,
      down: 0,
    })
  }
  const baseHour = new Date(now)
  baseHour.setMinutes(0, 0, 0)
  const baseMs = baseHour.getTime() - 23 * 3600_000
  for (const tick of entry.history) {
    const t = new Date(tick.checked_at).getTime()
    const idx = Math.floor((t - baseMs) / 3600_000)
    if (idx < 0 || idx >= 24) continue
    const bucket = buckets[idx]
    if (!bucket) continue
    const value = tick[key]
    bucket.total++
    if (value === 'up') bucket.up++
    else if (value === 'down') bucket.down++
  }
  return buckets
}

function bucketClass(bucket: HourBucket) {
  if (bucket.total === 0) return 'cell-empty'
  if (bucket.down === 0) return 'cell-up'
  if (bucket.up === 0) return 'cell-down'
  return 'cell-mixed'
}

function bucketTitle(bucket: HourBucket) {
  if (bucket.total === 0) return `${bucket.hourLabel} · 暂无探测`
  return `${bucket.hourLabel} · ${bucket.total} 次探测 · 在线 ${bucket.up} / 离线 ${bucket.down}`
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
}

.dashboard-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-bottom: 32px;
}
.dashboard-section:last-child {
  margin-bottom: 0;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 12px;
}
.section-header h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--color-heading);
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}
@media (max-width: 640px) {
  .stats-grid { grid-template-columns: 1fr; }
}

/* Quick Config */
.config-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 4px 0;
}
.config-desc {
  font-size: 14px;
  color: var(--color-text-light);
  margin: 0;
  line-height: 1.6;
}
.config-actions {
  display: flex;
  gap: 12px;
  align-items: stretch;
  flex-wrap: wrap;
}
.api-url-input {
  flex: 1 1 320px;
  min-width: 0;
}
.drag-btn {
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: auto;
  padding: 0 16px;
  font-weight: 500;
  white-space: nowrap;
  transition: transform 0.2s;
}
.drag-btn:hover {
  transform: translateY(-1px);
  color: white;
}

/* Fallback list */
.fallback-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.fallback-status-card :deep(.el-card__body) {
  padding: 20px;
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
  min-width: 0;
}
.fallback-priority {
  background: var(--el-color-primary);
  color: #fff;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  flex-shrink: 0;
}
.fallback-note {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-heading);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.fallback-overall {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  flex-shrink: 0;
}
.overall-online { background: rgba(103, 194, 58, 0.15); color: var(--el-color-success); }
.overall-partial { background: rgba(230, 162, 60, 0.15); color: var(--el-color-warning); }
.overall-offline { background: rgba(245, 108, 108, 0.15); color: var(--el-color-danger); }
.overall-unknown { background: rgba(144, 147, 153, 0.15); color: var(--el-color-info); }

.fallback-current {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  margin-bottom: 18px;
}
.fallback-current-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid var(--color-border);
  background: var(--color-background-soft);
}
.fallback-current-label { font-size: 12px; color: var(--color-text-light); font-weight: 600; }
.fallback-current-status { font-size: 14px; font-weight: 600; }
.status-up { border-color: var(--el-color-success-light-5); }
.status-up .fallback-current-status { color: var(--el-color-success); }
.status-down { border-color: var(--el-color-danger-light-5); }
.status-down .fallback-current-status { color: var(--el-color-danger); }
.status-unknown .fallback-current-status { color: var(--el-color-info); }

.fallback-history {
  border-top: 1px solid var(--color-border);
  padding-top: 14px;
}
.fallback-history-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  font-size: 13px;
  color: var(--color-text-light);
  margin-bottom: 10px;
  font-weight: 600;
}
.fallback-history-meta {
  font-size: 12px;
  font-weight: 500;
}
.fallback-history-grid {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.fallback-history-row {
  display: grid;
  grid-template-columns: 70px 1fr;
  align-items: center;
  gap: 10px;
}
.fallback-history-row-label {
  font-size: 12px;
  color: var(--color-text-light);
  font-weight: 600;
}
.fallback-history-track {
  display: grid;
  grid-template-columns: repeat(24, 1fr);
  gap: 2px;
}
.fallback-history-cell {
  height: 14px;
  border-radius: 3px;
  background: var(--color-background-soft);
  border: 1px solid var(--color-border);
}
.cell-up { background: var(--el-color-success); border-color: var(--el-color-success); }
.cell-down { background: var(--el-color-danger); border-color: var(--el-color-danger); }
.cell-mixed { background: var(--el-color-warning); border-color: var(--el-color-warning); }
.cell-empty { background: transparent; }

.fallback-history-axis {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: var(--color-text-light);
  margin-top: 6px;
  padding-left: 80px;
}

@media (max-width: 768px) {
  .fallback-current { grid-template-columns: 1fr; }
  .fallback-history-row { grid-template-columns: 60px 1fr; }
  .fallback-history-axis { padding-left: 70px; }
}
</style>
