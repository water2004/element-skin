<template>
  <PageContainer variant="wide" class="flex flex-col animate-fade-in">
    <PageHeader
      title="仪表盘"
      subtitle="这里汇总了您的资源数量、启动器接入入口与备用服务的健康状态"
    ></PageHeader>

    <div class="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_340px] gap-6 items-start">
      <div class="order-2 min-w-0 lg:order-1">
        <section class="flex flex-col gap-4 mb-8">
          <UiCard class="dashboard-stat-card" shadow="hover">
            <div class="stat-card-content">
              <el-statistic title="材质数量" :value="textureCount">
                <template #prefix>
                  <el-icon><Box /></el-icon>
                </template>
              </el-statistic>
              <el-statistic title="角色数量" :value="profileCount">
                <template #prefix>
                  <el-icon><User /></el-icon>
                </template>
              </el-statistic>
            </div>
          </UiCard>
        </section>

        <section class="flex flex-col gap-4 mb-8">
          <div class="flex justify-between items-baseline gap-3">
            <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">快速接入启动器</h2>
          </div>
          <UiCard class="launcher-card" shadow="hover">
            <div class="launcher-access">
              <p class="launcher-copy">
                点击下方按钮复制 API 地址，或直接将其拖到支持 authlib-injector 的启动器窗口中。
              </p>
              <el-input v-model="apiUrl" readonly maxlength="256" class="api-url-input" />
              <UiButton
                tag="a"
                variant="primary"
                size="large"
                hover-effect
                :icon="CopyDocument"
                class="drag-btn font-medium whitespace-nowrap"
                :href="`authlib-injector:yggdrasil-server:${encodeURIComponent(apiUrl)}`"
                draggable="true"
                title="点击复制，或拖到启动器"
                @click.prevent="copyApiUrl"
              >
                复制或拖到启动器
              </UiButton>
            </div>
          </UiCard>
        </section>

        <section v-if="fallbackEntries.length" class="flex flex-col gap-4 mb-0">
          <div class="flex justify-between items-baseline gap-3">
            <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">服务状态</h2>
            <div class="service-status-actions">
              <span class="refresh-countdown">自动刷新 {{ refreshCountdown }}s</span>
              <UiButton
                loading-mode="icon"
                :icon="Refresh"
                @click="loadFallbackStatus"
                :loading="isChecking"
                size="small"
                variant="tertiary"
              >
                <span>刷新</span>
              </UiButton>
            </div>
          </div>

          <div class="flex flex-col gap-4">
            <FallbackStatusCard v-for="entry in fallbackEntries" :key="entry.id" :entry="entry" />
          </div>
        </section>
      </div>

      <aside class="order-1 lg:order-2 flex flex-col gap-4">
        <div class="flex flex-wrap justify-between items-center gap-3">
          <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">公告</h2>
          <el-button size="small" text @click="goNotifications">
            <span>全部</span>
          </el-button>
        </div>
        <UiCard v-for="notice in dashboardNotices" :key="notice.id" shadow="hover" hoverable>
          <button class="w-full text-left" @click="openNotice(notice.id)">
            <div class="flex items-center justify-between gap-3">
              <div class="flex min-w-0 items-center gap-2">
                <div class="flex shrink-0 items-center gap-1.5">
                  <el-tag size="small" :type="levelTagType(notice.level)">
                    {{ levelLabel(notice.level) }}
                  </el-tag>
                  <el-tag v-if="notice.pinned" size="small" type="warning">置顶</el-tag>
                </div>
                <div class="min-w-0 truncate font-semibold text-[var(--color-heading)]">
                  {{ notice.title }}
                </div>
              </div>
              <el-button
                v-if="notice.dismissible"
                size="small"
                text
                class="shrink-0"
                @click.stop="dismissDashboardNotice(notice.id)"
              >
                忽略
              </el-button>
            </div>
            <div
              class="mt-2 text-sm leading-6 text-[var(--color-text-light)] line-clamp-2 [&_a]:text-[var(--el-color-primary)] [&_p]:m-0"
              v-html="noticePreview(notice)"
            />
            <div class="mt-2 text-xs text-[var(--color-text-light)]">
              {{ formatShortDate(notice.created_at) }}
            </div>
          </button>
        </UiCard>
        <UiCard v-if="!dashboardNotices.length && !noticesLoading" shadow="never">
          <el-empty description="暂无公告" />
        </UiCard>
      </aside>
    </div>
  </PageContainer>
</template>

<script setup lang="ts">
import PageContainer from '@/components/common/PageContainer.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import UiButton from '@/components/ui/UiButton.vue'
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { getPublicSettings, getPublicFallbackStatus } from '@/api/public'
import { getMe } from '@/api/me'
import { dismissNotice, getNotices } from '@/api/notices'
import { useRouter } from 'vue-router'
import { Box, User, CopyDocument, Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import type { FallbackStatusEntry, NoticeLevel, NoticeView } from '@/api/types'
import FallbackStatusCard from './FallbackStatusCard.vue'
import UiCard from '@/components/ui/UiCard.vue'
import { useNotificationIndicator } from '@/composables/useNotificationIndicator'
import { renderMarkdown } from '@/utils/markdown'

const router = useRouter()
const textureCount = ref(0)
const profileCount = ref(0)
const apiUrl = ref('')
const dashboardNotices = ref<NoticeView[]>([])
const noticesLoading = ref(false)
const { refreshUnreadNotifications } = useNotificationIndicator()

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
  navigator.clipboard
    .writeText(apiUrl.value)
    .then(() => {
      ElMessage.success('API 地址已复制')
    })
    .catch(() => {
      ElMessage.error('复制失败，请手动复制')
    })
}

function noticePreview(notice: NoticeView) {
  return renderMarkdown(notice.summary || '')
}

function levelLabel(level: NoticeLevel) {
  return (
    {
      info: '普通',
      success: '成功',
      warning: '重要',
      danger: '紧急',
    } satisfies Record<NoticeLevel, string>
  )[level]
}

function levelTagType(level: NoticeLevel) {
  return level === 'danger'
    ? 'danger'
    : level === 'warning'
      ? 'warning'
      : level === 'success'
        ? 'success'
        : 'info'
}

function formatShortDate(ts: number) {
  return new Date(ts).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function goNotifications() {
  router.push('/notifications')
}

function openNotice(id: string) {
  router.push(`/notifications/${id}`)
}

async function loadDashboardNotices() {
  noticesLoading.value = true
  try {
    const res = await getNotices({ type: 'announcement', limit: 4, dashboard: true })
    dashboardNotices.value = res.data.items
  } catch {
    dashboardNotices.value = []
  } finally {
    noticesLoading.value = false
  }
}

async function dismissDashboardNotice(id: string) {
  try {
    await dismissNotice(id)
    dashboardNotices.value = dashboardNotices.value.filter((notice) => notice.id !== id)
    void refreshUnreadNotifications()
    ElMessage.success('已忽略')
  } catch {
    ElMessage.error('忽略公告失败')
  }
}

const fallbackEntries = ref<FallbackStatusEntry[]>([])
const isChecking = ref(false)
const STATUS_REFRESH_INTERVAL_SECONDS = 60
const refreshCountdown = ref(STATUS_REFRESH_INTERVAL_SECONDS)
let refreshTimer: ReturnType<typeof setInterval> | null = null

async function loadFallbackStatus() {
  if (isChecking.value) return
  isChecking.value = true
  try {
    const res = await getPublicFallbackStatus()
    const list = (res.data.endpoints || []).slice()
    list.sort((a, b) => (a.priority || 0) - (b.priority || 0))
    fallbackEntries.value = list
  } catch {
    ElMessage.error('加载 Fallback 状态失败')
  } finally {
    isChecking.value = false
    refreshCountdown.value = STATUS_REFRESH_INTERVAL_SECONDS
  }
}

function startStatusRefreshTimer() {
  if (refreshTimer) clearInterval(refreshTimer)
  refreshTimer = setInterval(() => {
    if (isChecking.value) return
    if (refreshCountdown.value <= 1) {
      void loadFallbackStatus()
      return
    }
    refreshCountdown.value -= 1
  }, 1000)
}

onMounted(async () => {
  try {
    const res = await getPublicSettings()
    if (res.data.api_url) {
      apiUrl.value = res.data.api_url.endsWith('/')
        ? res.data.api_url.slice(0, -1)
        : res.data.api_url
    } else {
      apiUrl.value = getApiUrl()
    }
  } catch {
    apiUrl.value = getApiUrl()
  }

  await loadFallbackStatus()
  await loadDashboardNotices()
  startStatusRefreshTimer()

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

onBeforeUnmount(() => {
  if (refreshTimer) clearInterval(refreshTimer)
  refreshTimer = null
})
</script>

<style scoped>
.dashboard-stat-card :deep(.el-card__body) {
  padding: 26px 32px;
}

.stat-card-content {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  align-items: center;
  justify-content: center;
  gap: var(--space-panel-padding);
  min-height: 96px;
}

.dashboard-stat-card :deep(.el-statistic) {
  min-width: 0;
  text-align: center;
}

.dashboard-stat-card :deep(.el-statistic__head) {
  margin-bottom: 8px;
  color: var(--color-text-light);
  font-size: 14px;
  font-weight: 600;
}

.dashboard-stat-card :deep(.el-statistic__content) {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--color-heading);
  font-size: 30px;
  font-weight: 700;
}

.dashboard-stat-card :deep(.el-statistic__prefix) {
  display: inline-flex;
  align-items: center;
  color: var(--el-color-primary);
  font-size: 26px;
}

.launcher-card :deep(.el-card__body) {
  padding: 28px;
}

.launcher-access {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  text-align: center;
}

.launcher-copy {
  max-width: 720px;
  margin: 0;
  color: var(--color-text-light);
  font-size: 14px;
  line-height: 1.6;
}

.api-url-input {
  width: min(760px, 100%);
}

.drag-btn {
  text-decoration: none;
  min-width: 220px;
  min-height: 40px;
  padding: 0 20px;
}

.service-status-actions {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.refresh-countdown {
  color: var(--color-text-light);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

@media (max-width: 640px) {
  .dashboard-stat-card :deep(.el-card__body),
  .launcher-card :deep(.el-card__body) {
    padding: 22px 18px;
  }

  .stat-card-content {
    align-items: stretch;
    justify-content: center;
    grid-template-columns: 1fr;
    gap: 18px;
    min-height: 88px;
  }

  .dashboard-stat-card :deep(.el-statistic) {
    min-width: 0;
  }

  .drag-btn {
    width: 100%;
    min-width: 0;
  }

  .service-status-actions {
    align-items: flex-end;
    flex-direction: column;
    gap: 4px;
  }
}
</style>
