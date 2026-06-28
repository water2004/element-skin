<template>
  <div class="mx-auto w-full max-w-[1480px] py-5 animate-fade-in">
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
        <el-tag size="large" type="info">公告</el-tag>
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

    <div class="grid grid-cols-1 gap-5 lg:grid-cols-[420px_minmax(0,1fr)]">
      <UiCard shadow="never" class="overflow-hidden">
        <div
          ref="listScrollRef"
          class="flex max-h-[calc(100vh-190px)] min-h-[560px] flex-col overflow-auto"
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
                    <el-tag size="small" type="info">公告</el-tag>
                    <el-tag size="small" :type="levelTagType(notice.level)">
                      {{ levelLabel(notice.level) }}
                    </el-tag>
                  </div>
                  <el-button
                    v-if="notice.dismissible"
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
        <article v-if="selectedNotice" class="px-2 py-1">
          <div class="mb-4 flex flex-wrap items-center gap-2">
            <el-tag v-if="selectedNotice.pinned" size="small" type="warning">置顶</el-tag>
            <el-tag size="small" type="info">公告</el-tag>
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

          <div class="mt-6 border-t border-[var(--color-border)] pt-6">
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
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiSegmented from '@/components/ui/UiSegmented.vue'
import { dismissNotice, getNotice, getNotices } from '@/api/notices'
import type { NoticeLevel, NoticeView } from '@/api/types'
import { renderMarkdown } from '@/utils/markdown'

const route = useRoute()
const router = useRouter()
const notices = ref<NoticeView[]>([])
const selectedNotice = ref<NoticeView | null>(null)
const selectedId = computed(() => selectedNotice.value?.id || String(route.params.id || ''))
const loading = ref(false)
const detailLoading = ref(false)
const loadingMore = ref(false)
const hasNext = ref(false)
const nextCursor = ref<string | null>(null)
const listScrollRef = ref<HTMLElement | null>(null)
const loadMoreRef = ref<HTMLElement | null>(null)
const readScope = ref<'all' | 'unread'>('all')
const limit = 20
let loadObserver: IntersectionObserver | null = null

const renderedSelectedContent = computed(() =>
  renderMarkdown(selectedNotice.value?.content_markdown || ''),
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

function noticePreview(notice: NoticeView) {
  return renderMarkdown(notice.display_mode === 'detail' ? notice.summary : notice.content_markdown)
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

async function loadDetail(id: string) {
  if (!id) {
    selectedNotice.value = null
    return
  }
  detailLoading.value = true
  try {
    const res = await getNotice(id)
    selectedNotice.value = res.data
    notices.value = notices.value.map((item) =>
      item.id === id ? { ...item, read: true, read_at: res.data.read_at } : item,
    )
  } catch {
    selectedNotice.value = null
    ElMessage.error('加载通知详情失败')
  } finally {
    detailLoading.value = false
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
      type: 'announcement',
      include_read: readScope.value === 'all',
    })
    notices.value = res.data.items
    hasNext.value = res.data.has_next
    nextCursor.value = res.data.next_cursor

    const routeID = String(route.params.id || '')
    if (routeID) {
      await loadDetail(routeID)
    } else if (res.data.items.length > 0) {
      const first = res.data.items[0]
      if (first) {
        selectedNotice.value = first
        router.replace(`/notifications/${first.id}`)
      }
    } else {
      selectedNotice.value = null
      router.replace('/notifications')
    }
  } catch {
    ElMessage.error('加载通知失败')
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
      type: 'announcement',
      include_read: readScope.value === 'all',
    })
    const existing = new Set(notices.value.map((notice) => notice.id))
    notices.value = notices.value.concat(
      res.data.items.filter((notice) => !existing.has(notice.id)),
    )
    hasNext.value = res.data.has_next
    nextCursor.value = res.data.next_cursor
  } catch {
    ElMessage.error('加载更多通知失败')
  } finally {
    loadingMore.value = false
  }
}

function ensureLoadObserver() {
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
  await loadNotices()
}

async function dismiss(id: string) {
  try {
    await dismissNotice(id)
    notices.value = notices.value.filter((item) => item.id !== id)
    if (selectedNotice.value?.id === id) {
      selectedNotice.value = notices.value[0] || null
      if (selectedNotice.value) router.replace(`/notifications/${selectedNotice.value.id}`)
      else router.replace('/notifications')
    }
    ElMessage.success('已忽略')
  } catch {
    ElMessage.error('忽略通知失败')
  }
}

watch(
  () => route.params.id,
  (id) => {
    if (typeof id === 'string' && id && id !== selectedNotice.value?.id) {
      void loadDetail(id)
    }
  },
)

onMounted(refreshFirstPage)

onBeforeUnmount(() => {
  if (loadObserver) {
    loadObserver.disconnect()
    loadObserver = null
  }
})
</script>
