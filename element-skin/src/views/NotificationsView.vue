<template>
  <div class="max-w-[1180px] mx-auto py-5 animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <div>
          <h1>通知中心</h1>
          <p>查看与你相关的站内消息和公告</p>
        </div>
      </div>
      <div class="page-header-actions">
        <el-radio-group v-model="readScope" @change="refreshFirstPage">
          <el-radio-button value="all">全部</el-radio-button>
          <el-radio-button value="unread">未读</el-radio-button>
        </el-radio-group>
        <el-select v-model="noticeType" class="w-[128px]" @change="refreshFirstPage">
          <el-option label="公告" value="announcement" />
        </el-select>
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

    <div class="grid grid-cols-1 lg:grid-cols-[360px_minmax(0,1fr)] gap-5 items-start">
      <UiCard shadow="never">
        <div
          class="flex flex-col gap-2"
          v-loading="loading"
          element-loading-background="transparent"
        >
          <button
            v-for="notice in notices"
            :key="notice.id"
            class="w-full rounded-xl border px-4 py-3 text-left transition"
            :class="
              selectedId === notice.id
                ? 'border-[var(--el-color-primary)] bg-[rgba(64,158,255,0.08)]'
                : 'border-transparent bg-[var(--color-background-soft)] hover:border-[var(--color-border)] hover:bg-[var(--color-card-background)]'
            "
            @click="selectNotice(notice.id)"
          >
            <div class="mb-2 flex items-center gap-2">
              <span
                class="h-2 w-2 rounded-full"
                :class="notice.read ? 'bg-[var(--color-border)]' : 'bg-[var(--el-color-primary)]'"
              />
              <el-tag v-if="notice.pinned" size="small" type="warning">置顶</el-tag>
              <el-tag size="small" type="info">公告</el-tag>
              <el-tag size="small" :type="levelTagType(notice.level)">
                {{ levelLabel(notice.level) }}
              </el-tag>
            </div>
            <div class="font-semibold text-[var(--color-heading)] line-clamp-1">
              {{ notice.title }}
            </div>
            <div
              class="mt-2 text-sm text-[var(--color-text-light)] leading-6 line-clamp-2 [&_p]:m-0 [&_a]:text-[var(--el-color-primary)]"
              v-html="noticePreview(notice)"
            />
            <div
              class="mt-3 flex items-center justify-between gap-3 text-xs text-[var(--color-text-light)]"
            >
              <span>{{ formatShortDate(notice.created_at) }}</span>
              <el-button
                v-if="notice.dismissible"
                size="small"
                text
                @click.stop="dismiss(notice.id)"
              >
                忽略
              </el-button>
            </div>
          </button>

          <el-empty v-if="!notices.length && !loading" description="暂无通知" />

          <CursorPager
            v-if="notices.length > 0"
            class="mt-3"
            :count="notices.length"
            :loading="pagination.isLoading.value"
            :disabled-prev="!pagination.canGoPrev.value"
            :disabled-next="!pagination.canGoNext.value"
            @prev="handlePrevPage"
            @next="handleNextPage"
          />
        </div>
      </UiCard>

      <UiCard
        shadow="never"
        class="min-h-[520px]"
        v-loading="detailLoading"
        element-loading-background="transparent"
      >
        <article v-if="selectedNotice" class="p-1">
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

        <div v-else class="flex min-h-[480px] items-center justify-center">
          <el-empty description="选择一条通知查看详情" />
        </div>
      </UiCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import UiCard from '@/components/ui/UiCard.vue'
import { dismissNotice, getNotice, getNotices } from '@/api/notices'
import type { NoticeLevel, NoticeType, NoticeView } from '@/api/types'
import { useCursorPagination } from '@/composables/useCursorPagination'
import { renderMarkdown } from '@/utils/markdown'

const route = useRoute()
const router = useRouter()
const notices = ref<NoticeView[]>([])
const selectedNotice = ref<NoticeView | null>(null)
const selectedId = computed(() => selectedNotice.value?.id || String(route.params.id || ''))
const loading = ref(false)
const detailLoading = ref(false)
const readScope = ref<'all' | 'unread'>('all')
const noticeType = ref<NoticeType>('announcement')
const limit = 10
const pagination = useCursorPagination<NoticeView>(limit)

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
      cursor: pagination.currentCursor.value,
      limit,
      type: noticeType.value,
      include_read: readScope.value === 'all',
    })
    notices.value = res.data.items
    pagination.setPageData(res.data)

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
    }
  } catch {
    ElMessage.error('加载通知失败')
  } finally {
    loading.value = false
  }
}

async function handleNextPage() {
  await pagination.goToNextPage(async (cursor, pageLimit) => {
    const res = await getNotices({
      cursor,
      limit: pageLimit,
      type: noticeType.value,
      include_read: readScope.value === 'all',
    })
    notices.value = res.data.items
    selectedNotice.value = res.data.items[0] || null
    if (selectedNotice.value) router.replace(`/notifications/${selectedNotice.value.id}`)
    return res.data
  })
}

async function handlePrevPage() {
  await pagination.goToPrevPage(async (cursor, pageLimit) => {
    const res = await getNotices({
      cursor,
      limit: pageLimit,
      type: noticeType.value,
      include_read: readScope.value === 'all',
    })
    notices.value = res.data.items
    selectedNotice.value = res.data.items[0] || null
    if (selectedNotice.value) router.replace(`/notifications/${selectedNotice.value.id}`)
    return res.data
  })
}

async function refreshFirstPage() {
  pagination.reset()
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
</script>
