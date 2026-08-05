<template>
  <UiDialog v-model="visible" destroy-on-close align-center variant="wide-form">
    <div class="p-6">
      <div class="mb-5 flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="m-0 text-xl font-semibold text-[var(--color-heading)]">
              {{ dialogTitle }}
            </h2>
            <el-tag v-if="app" size="small" :type="statusType(app.status)">
              {{ statusLabel(app.status) }}
            </el-tag>
          </div>
          <el-text
            v-if="app"
            copyable
            class="mt-2 block font-mono text-xs text-[var(--color-text-light)]"
          >
            {{ app.client_id }}
          </el-text>
          <p v-else class="mt-2 mb-0 text-sm text-[var(--color-text-light)]">
            提交后等待管理员审核，通过后即可开始 OAuth 授权流程。
          </p>
        </div>
        <el-tag v-if="app" size="small">{{ clientTypeLabel(app.client_type) }}</el-tag>
      </div>

      <el-alert
        v-if="newSecret"
        class="mb-5"
        type="success"
        :closable="false"
        title="Client Secret 只显示一次"
      >
        <div class="mt-2 flex flex-col gap-2 sm:flex-row">
          <el-input :model-value="newSecret" readonly />
          <el-button @click="copySecret">
            <el-icon><CopyDocument /></el-icon>
            复制
          </el-button>
        </div>
      </el-alert>

      <el-alert
        v-if="app && app.status !== 'active'"
        class="mb-5"
        type="info"
        :closable="false"
        :title="statusHint(app.status)"
      />

      <el-form label-position="top">
        <div class="grid gap-4 md:grid-cols-2">
          <el-form-item label="名称">
            <el-input v-model="form.name" maxlength="80" show-word-limit />
          </el-form-item>
          <el-form-item label="类型">
            <el-select v-model="form.client_type" class="w-full">
              <el-option label="Confidential" value="confidential" />
              <el-option label="Public" value="public" />
            </el-select>
          </el-form-item>
        </div>

        <el-form-item label="回调地址">
          <el-input v-model="form.redirect_uri" placeholder="https://app.example/callback" />
        </el-form-item>

        <el-form-item label="网站地址">
          <el-input v-model="form.website_url" placeholder="https://app.example" />
        </el-form-item>

        <el-form-item label="说明">
          <el-input v-model="form.description" type="textarea" :rows="3" maxlength="160" />
        </el-form-item>

        <el-form-item label="申请权限">
          <PermissionTagPicker v-model="form.permissions" :permissions="delegablePermissions" />
          <div class="form-tip">
            这里仅配置访问本站 /v2 API 的权限。只使用 OIDC
            登录的应用可以不选择；openid、profile、email 和 offline_access 由授权请求的 scope
            参数声明。
          </div>
        </el-form-item>
      </el-form>
    </div>

    <template #footer>
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex flex-wrap gap-2">
          <el-button
            v-if="app?.client_type === 'confidential'"
            :loading="rotating"
            @click="emitRotate"
          >
            <el-icon><Key /></el-icon>
            轮换密钥
          </el-button>
          <el-button v-if="app" type="danger" plain :loading="deleting" @click="emitDelete">
            <el-icon><Delete /></el-icon>
            删除
          </el-button>
        </div>
        <div class="flex flex-wrap justify-end gap-2">
          <el-button @click="visible = false">取消</el-button>
          <el-button
            v-if="app && app.status !== 'pending'"
            :loading="saving"
            @click="emitSave(false)"
          >
            仅保存
          </el-button>
          <el-button type="primary" :loading="saving" @click="emitSave(app?.status !== 'pending')">
            <el-icon><Upload v-if="app" /><Plus v-else /></el-icon>
            {{ primaryLabel }}
          </el-button>
        </div>
      </div>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { CopyDocument, Delete, Key, Plus, Upload } from '@element-plus/icons-vue'
import type { PermissionDefinition } from '@/api/types'
import type { OAuthClient, OAuthClientInput, OAuthClientStatus } from '@/api/oauth'
import UiDialog from '@/components/ui/UiDialog.vue'
import PermissionTagPicker from '@/components/permissions/PermissionTagPicker.vue'

const visible = defineModel<boolean>('visible', { required: true })

const props = defineProps<{
  app: OAuthClient | null
  catalog: PermissionDefinition[]
  userPermissions: string[]
  newSecret: string
  saving: boolean
  rotating: boolean
  deleting: boolean
}>()

const emit = defineEmits<{
  save: [payload: OAuthClientInput, options: { resubmit: boolean }]
  rotate: [clientId: string]
  delete: [clientId: string]
}>()

const emptyForm = (): OAuthClientInput => ({
  name: '',
  description: '',
  redirect_uri: '',
  website_url: '',
  client_type: 'confidential',
  permissions: [],
})

const form = reactive<OAuthClientInput>(emptyForm())

const dialogTitle = computed(() => (props.app ? '编辑第三方应用' : '申请新应用'))
const primaryLabel = computed(() => {
  if (!props.app) return '提交审核'
  return props.app.status === 'pending' ? '保存修改' : '保存并重新提交'
})
const userPermissionSet = computed(() => new Set(props.userPermissions))
const permissionByCode = computed(() => {
  const out = new Map<string, PermissionDefinition>()
  for (const item of props.catalog) out.set(item.code, item)
  return out
})
const delegablePermissions = computed(() =>
  props.catalog.filter(
    (item) =>
      item.scope !== 'system' &&
      ((form.client_type === 'confidential' && item.scope === 'server') ||
        userPermissionSet.value.has(item.code)),
  ),
)

watch(
  () => visible.value,
  (isVisible) => {
    if (isVisible) hydrateForm()
  },
)

watch(
  () => props.app,
  () => {
    if (visible.value) hydrateForm()
  },
)

watch(
  () => form.client_type,
  (clientType) => {
    if (clientType === 'confidential') return
    form.permissions = form.permissions.filter(
      (code) => permissionByCode.value.get(code)?.scope !== 'server',
    )
  },
)

function hydrateForm() {
  const next = props.app
    ? {
        name: props.app.name,
        description: props.app.description,
        redirect_uri: props.app.redirect_uri,
        website_url: props.app.website_url,
        client_type: props.app.client_type,
        permissions: [...props.app.permissions],
      }
    : emptyForm()
  Object.assign(form, next)
}

function emitSave(resubmit: boolean) {
  emit(
    'save',
    {
      name: form.name.trim(),
      description: (form.description ?? '').trim(),
      redirect_uri: form.redirect_uri.trim(),
      website_url: (form.website_url ?? '').trim(),
      client_type: form.client_type,
      permissions: [...form.permissions],
    },
    { resubmit },
  )
}

function emitRotate() {
  if (!props.app) return
  emit('rotate', props.app.client_id)
}

function emitDelete() {
  if (!props.app) return
  emit('delete', props.app.client_id)
}

async function copySecret() {
  if (!props.newSecret) return
  await navigator.clipboard.writeText(props.newSecret)
  ElMessage.success('已复制')
}

function statusLabel(status: OAuthClientStatus) {
  const labels: Record<OAuthClientStatus, string> = {
    pending: '待审核',
    active: '已通过',
    rejected: '已驳回',
    disabled: '已停用',
  }
  return labels[status]
}

function statusType(status: OAuthClientStatus) {
  if (status === 'active') return 'success'
  if (status === 'rejected') return 'danger'
  if (status === 'pending') return 'warning'
  return 'info'
}

function statusHint(status: OAuthClientStatus) {
  if (status === 'pending') return '应用正在等待管理员审核，审核通过前不能发起 OAuth 授权。'
  if (status === 'rejected') return '应用审核未通过，可以调整信息后重新提交。'
  return '应用已被管理员停用，当前不能发起 OAuth 授权。'
}

function clientTypeLabel(clientType: OAuthClient['client_type']) {
  return clientType === 'confidential' ? '机密应用' : '公开应用'
}
</script>
