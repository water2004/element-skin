<template>
  <UiDialog
    :model-value="modelValue"
    :title="createMode ? '新建公告' : '编辑公告'"
    variant="wide-form"
    :dirty="dirty"
    :busy="saving"
    @update:model-value="emit('update:modelValue', $event)"
    @closed="emit('closed')"
  >
    <div
      class="grid max-h-[72vh] grid-cols-1 gap-5 overflow-auto p-6 lg:grid-cols-[minmax(0,1fr)_420px]"
    >
      <el-form label-position="top" :disabled="saving">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-[1fr_auto]">
          <el-form-item label="标题">
            <el-input
              v-model="title"
              maxlength="80"
              show-word-limit
              placeholder="例如：OAuth 应用注册开放说明"
            />
          </el-form-item>
          <el-form-item label="展示方式">
            <el-radio-group v-model="displayMode">
              <el-radio-button value="inline">短公告</el-radio-button>
              <el-radio-button value="detail">长公告</el-radio-button>
            </el-radio-group>
          </el-form-item>
        </div>
        <el-form-item label="摘要">
          <el-input
            v-model="summary"
            type="textarea"
            :rows="4"
            maxlength="160"
            show-word-limit
            :placeholder="
              form.display_mode === 'detail'
                ? '长公告必填；仪表盘和通知列表会展示摘要'
                : '短公告内容，会直接展示在仪表盘和通知列表'
            "
          />
        </el-form-item>
        <el-form-item v-if="form.display_mode === 'detail'" label="正文 Markdown">
          <el-input
            v-model="contentMarkdown"
            type="textarea"
            :rows="18"
            maxlength="20000"
            show-word-limit
            placeholder="支持标题、段落、列表、引用、代码块和链接；原始 HTML 会被清洗"
          />
        </el-form-item>
      </el-form>

      <aside
        class="rounded-xl border border-[var(--color-border)] bg-[var(--color-background-soft)] p-4"
      >
        <div class="mb-4 flex items-center justify-between gap-3">
          <div class="font-semibold text-[var(--color-heading)]">预览</div>
          <div class="flex items-center gap-2">
            <el-tag size="small" :type="form.display_mode === 'detail' ? 'primary' : 'info'">
              {{ form.display_mode === 'detail' ? '长公告' : '短公告' }}
            </el-tag>
            <el-tag size="small" :type="noticeLevelTagType(form.level)">
              {{ noticeLevelLabel(form.level) }}
            </el-tag>
          </div>
        </div>
        <UiCard shadow="never">
          <article class="p-1">
            <h2 class="m-0 text-2xl font-semibold text-[var(--color-heading)]">
              {{ form.title || '未命名公告' }}
            </h2>
            <p class="mt-4 mb-0 text-sm leading-7 text-[var(--color-text-light)]">
              {{ form.summary || '暂无摘要' }}
            </p>
            <div
              v-if="form.display_mode === 'detail'"
              class="mt-6 border-t border-[var(--color-border)] pt-5 text-sm leading-7 text-[var(--color-text)] [&_a]:text-[var(--el-color-primary)] [&_blockquote]:border-l-4 [&_blockquote]:border-[var(--el-color-primary)] [&_blockquote]:pl-3 [&_blockquote]:text-[var(--color-text-light)] [&_code]:rounded [&_code]:bg-[var(--color-background-soft)] [&_code]:px-1.5 [&_code]:py-0.5 [&_h1]:text-xl [&_h1]:font-semibold [&_h2]:text-lg [&_h2]:font-semibold [&_h3]:font-semibold [&_ol]:pl-5 [&_p]:my-3 [&_pre]:overflow-auto [&_pre]:rounded-xl [&_pre]:bg-[var(--color-background-soft)] [&_pre]:p-3 [&_ul]:pl-5"
              v-html="previewHtml"
            />
            <div v-if="form.link_url && form.link_text" class="mt-6">
              <el-button size="small" type="primary">{{ form.link_text }}</el-button>
            </div>
          </article>
        </UiCard>
      </aside>
    </div>

    <template #footer="{ requestClose }">
      <div class="px-6 pb-6">
        <el-button :disabled="saving" @click="requestClose">取消</el-button>
        <el-button type="primary" :disabled="saving" @click="emit('continue')"
          >下一步：发布设置</el-button
        >
      </div>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import type { NoticeDraft } from '@/api/admin/notices'
import { noticeLevelLabel, noticeLevelTagType } from './noticeForm'

const props = defineProps<{
  modelValue: boolean
  createMode: boolean
  form: NoticeDraft
  previewHtml: string
  dirty: boolean
  saving: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'update:form': [value: NoticeDraft]
  continue: []
  closed: []
}>()

const title = draftField('title')
const displayMode = draftField('display_mode')
const summary = draftField('summary')
const contentMarkdown = draftField('content_markdown')

function draftField<K extends keyof NoticeDraft>(field: K) {
  return computed<NoticeDraft[K]>({
    get: () => props.form[field],
    set: (value) => emit('update:form', { ...props.form, [field]: value }),
  })
}
</script>
