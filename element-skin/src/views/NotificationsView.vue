<template>
  <div class="flex flex-col animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <div>
          <h1>通知中心</h1>
          <p>查看与你相关的站内消息和公告</p>
        </div>
      </div>
      <div class="page-header-actions">
        <UiSegmented v-model="readScope" @change="refreshFirstPage">
          <el-radio-button value="all">全部</el-radio-button>
          <el-radio-button value="unread">未读</el-radio-button>
        </UiSegmented>
        <el-tag size="large" type="info">全部通知</el-tag>
        <el-button
          :icon="Refresh"
          :loading="loading"
          plain
          class="hover-lift"
          @click="refreshFirstPage"
        >
          刷新
        </el-button>
      </div>
    </div>

    <div
      class="grid grid-cols-1 gap-5 lg:grid-cols-[360px_minmax(0,1fr)] xl:grid-cols-[420px_minmax(0,1fr)]"
    >
      <UiCard shadow="never" class="overflow-hidden">
        <div
          ref="listScrollRef"
          class="flex max-h-[calc(100vh-190px)] min-h-[560px] flex-col overflow-auto lg:max-h-[calc(100vh-210px)]"
          v-loading="loading"
          element-loading-background="transparent"
        >
          <button
            v-for="notice in notices"
            :key="notice.id"
            class="w-full border-b border-[var(--color-border)] px-4 py-3 text-left transition last:border-b-0"
            :class="
              selectedId === notice.id
                ? 'bg-[rgba(64,158,255,0.08)]'
                : 'bg-transparent hover:bg-[var(--color-background-soft)]'
            "
            @click="selectNotice(notice.id)"
          >
            <div class="flex items-start gap-3">
              <span
                class="mt-2 h-2 w-2 shrink-0 rounded-full"
                :class="notice.read ? 'bg-[var(--color-border)]' : 'bg-[var(--el-color-primary)]'"
              />
              <div class="min-w-0 flex-1">
                <div class="flex items-center justify-between gap-3">
                  <div class="truncate font-semibold text-[var(--color-heading)]">
                    {{ notice.title }}
                  </div>
                  <span class="shrink-0 text-xs text-[var(--color-text-light)]">
                    {{ formatShortDate(notice.created_at) }}
                  </span>
                </div>
                <div
                  class="mt-1 text-sm leading-6 text-[var(--color-text-light)] line-clamp-2 [&_a]:text-[var(--el-color-primary)] [&_p]:m-0"
                  v-html="noticePreview(notice)"
                />
                <div class="mt-2 flex items-center justify-between gap-3">
                  <div class="flex min-w-0 flex-wrap items-center gap-2">
                    <el-tag v-if="notice.pinned" size="small" type="warning">置顶</el-tag>
                    <el-tag size="small" type="info">{{ noticeTypeLabel(notice.type) }}</el-tag>
                    <el-tag size="small" :type="levelTagType(notice.level)">
                      {{ levelLabel(notice.level) }}
                    </el-tag>
                  </div>
                  <el-button
                    v-if="notice.dismissible && canDismissNotice"
                    size="small"
                    text
                    @click.stop="dismiss(notice.id)"
                  >
                    忽略
                  </el-button>
                </div>
              </div>
            </div>
          </button>

          <el-empty v-if="!notices.length && !loading" class="my-auto" description="暂无通知" />

          <div
            v-if="notices.length > 0"
            ref="loadMoreRef"
            class="flex min-h-12 items-center justify-center text-xs text-[var(--color-text-light)]"
          >
            <span v-if="loadingMore">加载中...</span>
            <span v-else-if="hasNext">继续向下滚动加载更多</span>
            <span v-else>没有更多通知了</span>
          </div>
        </div>
      </UiCard>

      <UiCard
        shadow="never"
        class="min-h-[560px]"
        v-loading="detailLoading"
        element-loading-background="transparent"
      >
        <article v-if="selectedNotice" class="px-2 py-1 lg:px-4">
          <div class="mb-4 flex flex-wrap items-center gap-2">
            <el-tag v-if="selectedNotice.pinned" size="small" type="warning">置顶</el-tag>
            <el-tag size="small" type="info">{{ noticeTypeLabel(selectedNotice.type) }}</el-tag>
            <el-tag size="small" :type="levelTagType(selectedNotice.level)">
              {{ levelLabel(selectedNotice.level) }}
            </el-tag>
            <span class="text-xs text-[var(--color-text-light)]">
              {{ selectedNotice.read ? '已读' : '未读' }}
            </span>
            <span class="text-xs text-[var(--color-text-light)]">
              {{ formatLongDate(selectedNotice.created_at) }}
            </span>
          </div>

          <h2 class="m-0 text-3xl font-semibold text-[var(--color-heading)]">
            {{ selectedNotice.title }}
          </h2>
          <p
            v-if="selectedNotice.summary"
            class="mt-4 mb-0 text-[var(--color-text-light)] leading-7"
          >
            {{ selectedNotice.summary }}
          </p>

          <div
            v-if="selectedNotice.display_mode === 'detail'"
            class="mt-6 border-t border-[var(--color-border)] pt-6"
          >
            <div
              class="text-[var(--color-text)] leading-8 [&_p]:my-4 [&_h1]:text-2xl [&_h1]:font-semibold [&_h2]:text-xl [&_h2]:font-semibold [&_h3]:text-lg [&_h3]:font-semibold [&_ul]:pl-6 [&_ol]:pl-6 [&_li]:my-1 [&_blockquote]:border-l-4 [&_blockquote]:border-[var(--el-color-primary)] [&_blockquote]:pl-4 [&_blockquote]:text-[var(--color-text-light)] [&_code]:rounded [&_code]:bg-[var(--color-background-soft)] [&_code]:px-1.5 [&_code]:py-0.5 [&_pre]:overflow-auto [&_pre]:rounded-xl [&_pre]:bg-[var(--color-background-soft)] [&_pre]:p-4 [&_a]:text-[var(--el-color-primary)]"
              v-html="renderedSelectedContent"
            />
          </div>

          <div v-if="selectedNotice.link_url && selectedNotice.link_text" class="mt-8">
            <el-button
              type="primary"
              tag="a"
              :href="selectedNotice.link_url"
              target="_blank"
              rel="noreferrer"
            >
              {{ selectedNotice.link_text }}
            </el-button>
          </div>
        </article>

        <div v-else class="flex min-h-[520px] items-center justify-center">
          <el-empty description="选择一条通知查看详情" />
        </div>
      </UiCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiSegmented from '@/components/ui/UiSegmented.vue'
import { dismissNotice, getNotice, getNotices } from '@/api/notices'
import type { NoticeLevel, NoticeView, User } from '@/api/types'
import { useNotificationIndicator } from '@/composables/useNotificationIndicator'
import { renderMarkdown } from '@/utils/markdown'

const route = useRoute()
const router = useRouter()
const user = inject<Ref<User | null>>('user', ref(null))
const canDismissNotice = computed(() =>
  (user.value?.permissions ?? []).includes('notice.dismiss.owned'),
)
const notices = ref<NoticeView[]>([])
const selectedNotice = ref<NoticeView | null>(null)
const detailRouteId = computed(() => (typeof route.params.id === 'string' ? route.params.id : ''))
const selectedId = computed(() => detailRouteId.value)
const loading = ref(false)
const detailLoading = ref(false)
const loadingMore = ref(false)
const hasNext = ref(false)
const nextCursor = ref<string | null>(null)
const listScrollRef = ref<HTMLElement | null>(null)
const loadMoreRef = ref<HTMLElement | null>(null)
const readScope = ref<'all' | 'unread'>('all')
const limit = 20
const { refreshUnreadNotifications } = useNotificationIndicator()
let loadObserver: IntersectionObserver | null = null
let detailRequest = 0
let disposed = false
const noticeTypeLabels: Record<string, string> = {
  announcement: '公告',
  system: '系统',
}

const renderedSelectedContent = computed(() =>
  renderMarkdown(
    selectedNotice.value?.display_mode === 'detail' ? selectedNotice.value.content_markdown : '',
  ),
)

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

function noticeTypeLabel(type: string) {
  return noticeTypeLabels[type] || type
}

function noticePreview(notice: NoticeView) {
  return renderMarkdown(notice.summary || '')
}

function formatShortDate(ts: number) {
  return new Date(ts).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatLongDate(ts: number) {
  return new Date(ts).toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function clearDetail() {
  detailRequest++
  selectedNotice.value = null
  detailLoading.value = false
}

async function loadDetail(id: string) {
  if (!id) {
    clearDetail()
    return
  }
  const request = ++detailRequest
  const isCurrent = () => !disposed && request === detailRequest && detailRouteId.value === id
  detailLoading.value = true
  try {
    const res = await getNotice(id)
    if (!isCurrent()) return
    selectedNotice.value = res.data
    notices.value = notices.value.map((item) =>
      item.id === id ? { ...item, read: true, read_at: res.data.read_at } : item,
    )
    void refreshUnreadNotifications()
  } catch {
    if (!isCurrent()) return
    selectedNotice.value = null
    ElMessage.error('加载通知详情失败')
  } finally {
    if (isCurrent()) detailLoading.value = false
  }
}

function selectNotice(id: string) {
  router.push(`/notifications/${id}`)
}

async function loadNotices() {
  loading.value = true
  try {
    const res = await getNotices({
      cursor: null,
      limit,
      include_read: readScope.value === 'all',
    })
    if (disposed) return
    notices.value = res.data.items.map((item) =>
      item.id === selectedNotice.value?.id
        ? { ...item, read: selectedNotice.value.read, read_at: selectedNotice.value.read_at }
        : item,
    )
    hasNext.value = res.data.has_next
    nextCursor.value = res.data.next_cursor
  } catch {
    if (!disposed) ElMessage.error('加载通知失败')
  } finally {
    loading.value = false
    await nextTick()
    ensureLoadObserver()
  }
}

async function loadMoreNotices() {
  if (!hasNext.value || !nextCursor.value || loadingMore.value || loading.value) return
  loadingMore.value = true
  try {
    const res = await getNotices({
      cursor: nextCursor.value,
      limit,
      include_read: readScope.value === 'all',
    })
    if (disposed) return
    const existing = new Set(notices.value.map((notice) => notice.id))
    notices.value = notices.value.concat(
      res.data.items.filter((notice) => !existing.has(notice.id)),
    )
    hasNext.value = res.data.has_next
    nextCursor.value = res.data.next_cursor
  } catch {
    if (!disposed) ElMessage.error('加载更多通知失败')
  } finally {
    loadingMore.value = false
  }
}

function ensureLoadObserver() {
  if (disposed) return
  if (loadObserver) loadObserver.disconnect()
  if (!loadMoreRef.value || !listScrollRef.value) return
  loadObserver = new IntersectionObserver(
    (entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        void loadMoreNotices()
      }
    },
    {
      root: listScrollRef.value,
      rootMargin: '120px',
    },
  )
  loadObserver.observe(loadMoreRef.value)
}

async function refreshFirstPage() {
  nextCursor.value = null
  hasNext.value = false
  await Promise.all([loadNotices(), loadDetail(detailRouteId.value)])
}

async function dismiss(id: string) {
  if (!canDismissNotice.value) return
  try {
    await dismissNotice(id)
    if (disposed) return
    notices.value = notices.value.filter((item) => item.id !== id)
    if (detailRouteId.value === id) {
      clearDetail()
      const next = notices.value[0]
      void router.replace(next ? `/notifications/${next.id}` : '/notifications')
    }
    void refreshUnreadNotifications()
    ElMessage.success('已忽略')
  } catch {
    if (!disposed) ElMessage.error('忽略通知失败')
  }
}

watch(
  detailRouteId,
  (id) => {
    clearDetail()
    if (id) void loadDetail(id)
  },
  { flush: 'sync' },
)

onMounted(refreshFirstPage)

onBeforeUnmount(() => {
  disposed = true
  clearDetail()
  if (loadObserver) {
    loadObserver.disconnect()
    loadObserver = null
  }
})
</script>
