<template>
  <div class="max-w-[1100px] mx-auto py-5 animate-fade-in">
    <PageHeader title="通知公告" subtitle="发布公告并控制可见人群、展示方式和生命周期">
      <template #icon><Bell /></template>
      <template #actions>
        <el-select v-model="status" class="w-[140px]" @change="refreshFirstPage">
          <el-option label="全部状态" value="all" />
          <el-option label="已启用" value="enabled" />
          <el-option label="已停用" value="disabled" />
          <el-option label="定时发布" value="scheduled" />
          <el-option label="已过期" value="expired" />
        </el-select>
        <el-button :icon="Refresh" plain class="hover-lift" @click="refreshFirstPage"
          >刷新</el-button
        >
        <el-button type="primary" :icon="Plus" class="hover-lift" @click="openCreateDialog">
          新建公告
        </el-button>
      </template>
    </PageHeader>

    <UiCard shadow="never">
      <el-table :data="notices" class="modern-table w-full">
        <el-table-column label="标题" min-width="260">
          <template #default="{ row }">
            <div class="font-semibold text-[var(--color-heading)]">{{ row.title }}</div>
            <div class="mt-1 text-xs text-[var(--color-text-light)] line-clamp-1">
              {{ row.summary || row.content_markdown }}
            </div>
          </template>
        </el-table-column>
        <el-table-column label="展示" width="110" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="row.display_mode === 'detail' ? 'primary' : 'info'">
              {{ row.display_mode === 'detail' ? '长公告' : '短公告' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="级别" width="90" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="levelTagType(row.level)">{{
              levelLabel(row.level)
            }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="人群" width="100" align="center">
          <template #default="{ row }">
            {{ row.audience === 'admins' ? '管理员' : '用户' }}
          </template>
        </el-table-column>
        <el-table-column label="状态" width="140" align="center">
          <template #default="{ row }">
            <div class="flex flex-col items-center gap-1">
              <el-switch v-model="row.enabled" @change="toggleEnabled(row)" />
              <span class="text-xs text-[var(--color-text-light)]">{{ lifecycleLabel(row) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="置顶" width="80" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.pinned" size="small" type="warning">置顶</el-tag>
            <span v-else class="text-xs text-[var(--color-text-light)]">否</span>
          </template>
        </el-table-column>
        <el-table-column label="有效期" min-width="180">
          <template #default="{ row }">
            <span class="text-xs text-[var(--color-text-light)]">
              {{ formatDate(row.starts_at) }} -
              {{ row.ends_at ? formatDate(row.ends_at) : '无期限' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="发布时间" width="160">
          <template #default="{ row }">
            <span class="text-xs text-[var(--color-text-light)]">{{
              formatDate(row.created_at)
            }}</span>
          </template>
        </el-table-column>
        <el-table-column label="更新时间" width="160">
          <template #default="{ row }">
            <span class="text-xs text-[var(--color-text-light)]">{{
              formatDate(row.updated_at)
            }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="170" fixed="right" align="center">
          <template #default="{ row }">
            <el-button size="small" :icon="Edit" link @click="openEditDialog(row)">编辑</el-button>
            <el-button size="small" :icon="Setting" link @click="openSettingsDialog(row)">
              设置
            </el-button>
            <el-button size="small" type="danger" :icon="Delete" link @click="deleteNotice(row)" />
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-container">
        <CursorPager
          v-if="notices.length > 0"
          :count="notices.length"
          :loading="pagination.isLoading.value"
          :disabled-prev="!pagination.canGoPrev.value"
          :disabled-next="!pagination.canGoNext.value"
          :page-size="pagination.limit.value"
          @prev="handlePrevPage"
          @next="handleNextPage"
          @page-size-change="handlePageSizeChange"
        />
      </div>
    </UiCard>

    <NoticeContentDialog
      v-model="contentDialogVisible"
      :create-mode="createMode"
      :form="form"
      :preview-html="previewHtml"
      @update:form="replaceForm"
      @continue="continueToSettings"
    />

    <NoticeSettingsDialog
      v-model="settingsDialogVisible"
      :create-mode="createMode"
      :saving="saving"
      :form="form"
      @update:form="replaceForm"
      @back="backToContent"
      @save="saveNotice"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Bell, Delete, Edit, Plus, Refresh, Setting } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import UiCard from '@/components/ui/UiCard.vue'
import NoticeContentDialog from '@/components/admin/notices/NoticeContentDialog.vue'
import NoticeSettingsDialog from '@/components/admin/notices/NoticeSettingsDialog.vue'
import {
  createAdminNotice,
  deleteAdminNotice,
  getAdminNotices,
  patchAdminNotice,
  type NoticeDraft,
} from '@/api/admin/notices'
import type { Notice, NoticeStatus } from '@/api/types'
import { useCursorPagination } from '@/composables/useCursorPagination'
import { getErrorMessage } from '@/utils/error'
import { renderMarkdown } from '@/utils/markdown'
import {
  defaultNoticeDraft,
  draftFromNotice,
  normalizedNoticeDraft,
  noticeLevelLabel,
  noticeLevelTagType,
  noticeLifecycleLabel,
  validateNoticeContent,
  validateNoticeSettings,
} from './noticeForm'

const notices = ref<Notice[]>([])
const status = ref<NoticeStatus>('all')
const pagination = useCursorPagination<Notice>(20)
const limit = pagination.limit
const contentDialogVisible = ref(false)
const settingsDialogVisible = ref(false)
const saving = ref(false)
const editingNotice = ref<Notice | null>(null)
const createMode = ref(false)
const form = reactive<NoticeDraft>(defaultNoticeDraft())
const previewHtml = computed(() => renderMarkdown(form.content_markdown || ''))

const levelLabel = noticeLevelLabel
const levelTagType = noticeLevelTagType

function resetForm() {
  Object.assign(form, defaultNoticeDraft())
}

function replaceForm(next: NoticeDraft) {
  Object.assign(form, next)
}

function formatDate(ts: number | null | undefined) {
  return ts
    ? new Date(ts).toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      })
    : '-'
}

function lifecycleLabel(notice: Notice) {
  return noticeLifecycleLabel(notice)
}

function openCreateDialog() {
  editingNotice.value = null
  createMode.value = true
  resetForm()
  contentDialogVisible.value = true
}

function fillForm(notice: Notice) {
  editingNotice.value = notice
  createMode.value = false
  Object.assign(form, draftFromNotice(notice))
}

function openEditDialog(notice: Notice) {
  fillForm(notice)
  contentDialogVisible.value = true
}

function openSettingsDialog(notice: Notice) {
  fillForm(notice)
  settingsDialogVisible.value = true
}

function validateContent() {
  return validateNoticeContent(form)
}

function validateSettings() {
  return validateNoticeSettings(form)
}

function normalizedForm(): NoticeDraft {
  return normalizedNoticeDraft(form)
}

function continueToSettings() {
  const error = validateContent()
  if (error) {
    ElMessage.warning(error)
    return
  }
  contentDialogVisible.value = false
  settingsDialogVisible.value = true
}

function backToContent() {
  settingsDialogVisible.value = false
  contentDialogVisible.value = true
}

async function saveNotice() {
  const contentError = validateContent()
  if (contentError) {
    settingsDialogVisible.value = false
    contentDialogVisible.value = true
    ElMessage.warning(contentError)
    return
  }
  const settingsError = validateSettings()
  if (settingsError) {
    ElMessage.warning(settingsError)
    return
  }
  saving.value = true
  try {
    if (createMode.value) {
      await createAdminNotice(normalizedForm())
      ElMessage.success('已创建')
    } else {
      await patchAdminNotice(editingNotice.value!.id, normalizedForm())
      ElMessage.success('已保存')
    }
    settingsDialogVisible.value = false
    await refreshFirstPage()
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, createMode.value ? '创建失败' : '保存失败'))
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(notice: Notice) {
  try {
    const res = await patchAdminNotice(notice.id, { enabled: notice.enabled })
    Object.assign(notice, res.data)
    ElMessage.success(notice.enabled ? '已启用' : '已停用')
  } catch (e: unknown) {
    notice.enabled = !notice.enabled
    ElMessage.error(getErrorMessage(e, '状态更新失败'))
  }
}

async function deleteNotice(notice: Notice) {
  try {
    await ElMessageBox.confirm(`确定删除公告「${notice.title}」吗？`, '确认删除', {
      type: 'warning',
    })
    await deleteAdminNotice(notice.id)
    ElMessage.success('已删除')
    await refreshFirstPage()
  } catch {}
}

async function loadNotices() {
  try {
    const res = await getAdminNotices({
      cursor: pagination.currentCursor.value,
      limit: limit.value,
      type: 'announcement',
      status: status.value,
    })
    notices.value = res.data.items
    pagination.setPageData(res.data)
  } catch {
    ElMessage.error('加载公告失败')
  }
}

async function handleNextPage() {
  await pagination.goToNextPage(async (cursor, pageLimit) => {
    const res = await getAdminNotices({
      cursor,
      limit: pageLimit,
      type: 'announcement',
      status: status.value,
    })
    notices.value = res.data.items
    return res.data
  })
}

async function handlePrevPage() {
  await pagination.goToPrevPage(async (cursor, pageLimit) => {
    const res = await getAdminNotices({
      cursor,
      limit: pageLimit,
      type: 'announcement',
      status: status.value,
    })
    notices.value = res.data.items
    return res.data
  })
}

async function refreshFirstPage() {
  pagination.reset()
  await loadNotices()
}

async function handlePageSizeChange(value: number) {
  if (!pagination.setLimit(value)) return
  await refreshFirstPage()
}

onMounted(refreshFirstPage)
</script>
