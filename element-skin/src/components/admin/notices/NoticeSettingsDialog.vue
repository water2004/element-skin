<template>
  <UiDialog
    :model-value="modelValue"
    title="发布设置"
    variant="wide-form"
    :dirty="dirty"
    :busy="saving"
    @update:model-value="emit('update:modelValue', $event)"
    @closed="emit('closed')"
  >
    <div class="max-h-[72vh] overflow-auto p-6">
      <el-form label-position="top" :disabled="saving">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <el-form-item label="级别">
            <el-select v-model="level">
              <el-option label="普通" value="info" />
              <el-option label="成功" value="success" />
              <el-option label="重要" value="warning" />
              <el-option label="紧急" value="danger" />
            </el-select>
          </el-form-item>
          <el-form-item label="可见人群">
            <el-select v-model="audience">
              <el-option label="所有用户" value="users" />
              <el-option label="管理员" value="admins" />
            </el-select>
          </el-form-item>
          <el-form-item label="控制项">
            <div class="flex h-8 flex-wrap items-center gap-4">
              <el-checkbox v-model="enabled">启用</el-checkbox>
              <el-checkbox v-model="pinned">置顶</el-checkbox>
              <el-checkbox v-model="dismissible">可忽略</el-checkbox>
            </div>
          </el-form-item>
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <el-form-item label="生效时间">
            <el-date-picker
              v-model="startsAt"
              type="datetime"
              value-format="x"
              clearable
              placeholder="立即生效"
              class="w-full"
            />
          </el-form-item>
          <el-form-item label="过期时间">
            <el-date-picker
              v-model="endsAt"
              type="datetime"
              value-format="x"
              clearable
              placeholder="无期限"
              class="w-full"
            />
          </el-form-item>
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <el-form-item label="链接文字">
            <el-input v-model="linkText" maxlength="40" />
          </el-form-item>
          <el-form-item label="链接地址">
            <el-input v-model="linkUrl" maxlength="512" placeholder="/oauth/apps 或 https://..." />
          </el-form-item>
        </div>
      </el-form>
    </div>

    <template #footer="{ requestClose }">
      <div class="px-6 pb-6">
        <el-button :disabled="saving" @click="emit('back')">返回编辑</el-button>
        <el-button :disabled="saving" @click="requestClose">取消</el-button>
        <el-button type="primary" :loading="saving" @click="emit('save')">
          {{ createMode ? '创建公告' : '保存' }}
        </el-button>
      </div>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import type { NoticeDraft } from '@/api/admin/notices'

const props = defineProps<{
  modelValue: boolean
  createMode: boolean
  saving: boolean
  form: NoticeDraft
  dirty: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'update:form': [value: NoticeDraft]
  back: []
  save: []
  closed: []
}>()

const level = draftField('level')
const audience = draftField('audience')
const enabled = draftField('enabled')
const pinned = draftField('pinned')
const dismissible = draftField('dismissible')
const startsAt = draftField('starts_at')
const endsAt = draftField('ends_at')
const linkText = draftField('link_text')
const linkUrl = draftField('link_url')

function draftField<K extends keyof NoticeDraft>(field: K) {
  return computed<NoticeDraft[K]>({
    get: () => props.form[field],
    set: (value) => emit('update:form', { ...props.form, [field]: value }),
  })
}
</script>
