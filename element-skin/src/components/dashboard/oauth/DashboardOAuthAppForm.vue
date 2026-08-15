<template>
  <div class="space-y-6 animate-fade-in">
    <div class="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
      <div class="flex min-w-0 items-start gap-3">
        <el-button
          circle
          plain
          aria-label="返回第三方应用"
          @click="router.push({ name: 'dashboard-oauth' })"
        >
          <el-icon><ArrowLeft /></el-icon>
        </el-button>
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h1 class="m-0 text-2xl font-semibold text-[var(--color-heading)]">
              {{ app ? '修改第三方应用' : '申请第三方应用' }}
            </h1>
            <el-tag v-if="app" size="small" :type="statusType(app.status)">
              {{ statusLabel(app.status) }}
            </el-tag>
          </div>
          <p class="mt-2 mb-0 text-sm text-[var(--color-text-light)]">
            {{
              app
                ? '在完整页面中维护授权方式、API 权限和可选的 Webhook endpoint。'
                : '填写应用能力并提交审核；不需要事件通知时无需配置 Webhook。'
            }}
          </p>
          <el-text
            v-if="app"
            copyable
            class="mt-2 block font-mono text-xs text-[var(--color-text-light)]"
          >
            {{ app.client_id }}
          </el-text>
        </div>
      </div>
    </div>

    <el-skeleton v-if="loading" :rows="10" animated />

    <template v-else>
      <el-alert
        v-if="clientSecret"
        type="success"
        :closable="false"
        title="Client Secret 只显示一次"
      >
        <el-text
          copyable
          class="mt-2 !flex rounded-lg bg-[var(--color-background-soft)] px-3 py-2 font-mono text-xs"
        >
          <span class="min-w-0 break-all">{{ clientSecret }}</span>
        </el-text>
      </el-alert>

      <el-alert
        v-for="item in visibleEndpointSecrets"
        :key="item.id"
        type="success"
        :closable="false"
        title="Webhook 签名密钥只显示一次"
      >
        <div class="mt-1 text-xs text-[var(--color-text-light)]">{{ item.url }}</div>
        <el-text
          copyable
          class="mt-2 !flex rounded-lg bg-[var(--color-background-soft)] px-3 py-2 font-mono text-xs"
        >
          <span class="min-w-0 break-all">{{ item.secret }}</span>
        </el-text>
      </el-alert>

      <el-alert
        v-if="app && app.status !== 'active'"
        type="info"
        :closable="false"
        :title="statusHint(app.status)"
      />

      <UiCard class="p-6">
        <div class="mb-5">
          <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">基本信息</h2>
          <p class="mt-1 mb-0 text-sm text-[var(--color-text-light)]">
            OAuth 回调地址与 Webhook endpoint 相互独立，均可按实际授权方式选择配置。
          </p>
        </div>
        <el-form label-position="top" :disabled="!canEditFields">
          <div class="grid gap-4 md:grid-cols-2">
            <el-form-item label="应用名称" required>
              <el-input v-model="form.name" maxlength="80" show-word-limit />
            </el-form-item>
            <el-form-item label="应用类型">
              <el-select v-model="form.client_type" class="w-full">
                <el-option label="机密应用（Confidential）" value="confidential" />
                <el-option label="公开应用（Public）" value="public" />
              </el-select>
            </el-form-item>
          </div>
          <el-form-item label="OAuth 回调地址（可选）">
            <el-input
              v-model="form.redirect_uri"
              placeholder="https://app.example/oauth/callback"
            />
            <div class="form-tip">
              Authorization Code 流程需要配置；Client Credentials 或 Device Code 可以留空。
            </div>
          </el-form-item>
          <el-form-item label="应用网站（可选）">
            <el-input v-model="form.website_url" placeholder="https://app.example" />
          </el-form-item>
          <el-form-item label="应用说明">
            <el-input
              v-model="form.description"
              type="textarea"
              :rows="4"
              maxlength="160"
              show-word-limit
            />
          </el-form-item>
        </el-form>
      </UiCard>

      <UiCard class="p-6">
        <div class="mb-5">
          <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">申请 API 权限</h2>
          <p class="mt-1 mb-0 text-sm text-[var(--color-text-light)]">
            Webhook 可监听事件会根据这里申请的权限实时收窄；移除权限时，对应事件也会被移除。
          </p>
        </div>
        <PermissionTagPicker
          v-model="form.permissions"
          :permissions="delegablePermissions"
          :disabled="!canEditFields"
        />
        <div class="form-tip mt-3">
          只使用 OIDC 登录的应用可以不选择 API 权限；openid、profile、email 和 offline_access
          仍由授权请求的 scope 参数声明。
        </div>
      </UiCard>

      <OAuthWebhookEndpointEditor
        :endpoints="form.webhook_endpoints"
        :events="availableWebhookEvents"
        :disabled="!canEditFields"
        @add="addEndpoint"
        @remove="removeEndpoint"
        @update-endpoint="updateEndpoint"
      />

      <UiCard class="p-5">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <ActionBar align="start">
            <el-button
              v-if="app?.client_type === 'confidential' && canUpdateApps"
              :loading="rotating"
              @click="rotateSecret"
            >
              <el-icon><Key /></el-icon>
              轮换 Client Secret
            </el-button>
            <el-button
              v-if="app && canDeleteApps"
              type="danger"
              plain
              :loading="deleting"
              @click="deleteApp"
            >
              <el-icon><Delete /></el-icon>
              删除应用
            </el-button>
          </ActionBar>
          <ActionBar>
            <el-button @click="router.push({ name: 'dashboard-oauth' })">取消</el-button>
            <el-button
              v-if="app && canUpdateApps && app.status !== 'pending'"
              :loading="saving"
              @click="save(false)"
            >
              仅保存
            </el-button>
            <el-button
              v-if="canEditFields"
              type="primary"
              :loading="saving"
              @click="save(app?.status !== 'pending')"
            >
              <el-icon><Upload v-if="app" /><Plus v-else /></el-icon>
              {{ primaryLabel }}
            </el-button>
          </ActionBar>
        </div>
      </UiCard>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onMounted, reactive, ref, watch, type Ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, Delete, Key, Plus, Upload } from '@element-plus/icons-vue'
import { useRoute, useRouter } from 'vue-router'
import {
  createOAuthApp,
  deleteOAuthApp,
  getOAuthApp,
  getOAuthWebhookEventCatalog,
  getPermissionCatalog,
  rotateOAuthSecret,
  submitOAuthAppReview,
  updateOAuthApp,
  type OAuthClient,
  type OAuthClientInput,
  type OAuthClientStatus,
  type OAuthWebhookEndpoint,
  type OAuthWebhookEventDefinition,
} from '@/api/oauth'
import type { PermissionDefinition, User } from '@/api/types'
import ActionBar from '@/components/common/ActionBar.vue'
import PermissionTagPicker from '@/components/permissions/PermissionTagPicker.vue'
import UiCard from '@/components/ui/UiCard.vue'
import { getErrorMessage } from '@/utils/error'
import {
  availableOAuthWebhookEvents,
  delegableOAuthPermissions,
  endpointsWithAllowedEvents,
  oauthAppValidationError,
  oauthClientPayload,
  permissionsForOAuthClientType,
  type OAuthAppFormState,
  type WebhookEndpointForm,
} from './oauthAppFormState'
import OAuthWebhookEndpointEditor from './OAuthWebhookEndpointEditor.vue'

const route = useRoute()
const router = useRouter()
const user = inject<Ref<User | null>>('user', ref(null))
const app = ref<OAuthClient | null>(null)
const catalog = ref<PermissionDefinition[]>([])
const webhookCatalog = ref<OAuthWebhookEventDefinition[]>([])
const loading = ref(true)
const saving = ref(false)
const rotating = ref(false)
const deleting = ref(false)
const clientSecret = ref('')
const endpointSecrets = ref<Record<string, string>>({})
let endpointSequence = 0

const form = reactive<OAuthAppFormState>({
  name: '',
  description: '',
  redirect_uri: '',
  website_url: '',
  client_type: 'confidential',
  permissions: [],
  webhook_endpoints: [],
})

const clientId = computed(() => String(route.params.client_id ?? ''))
const userPermissions = computed(() => new Set(user.value?.permissions ?? []))
const canCreateApps = computed(() => userPermissions.value.has('oauth_app.create.owned'))
const canUpdateApps = computed(() => userPermissions.value.has('oauth_app.update.owned'))
const canDeleteApps = computed(() => userPermissions.value.has('oauth_app.delete.owned'))
const canEditFields = computed(() => (app.value ? canUpdateApps.value : canCreateApps.value))
const primaryLabel = computed(() => {
  if (!app.value) return '提交审核'
  return app.value.status === 'pending' ? '保存修改' : '保存并重新提交'
})
const delegablePermissions = computed(() =>
  delegableOAuthPermissions(catalog.value, user.value?.permissions ?? [], form.client_type),
)
const availableWebhookEvents = computed(() =>
  availableOAuthWebhookEvents(webhookCatalog.value, form.permissions, form.client_type),
)
const availableWebhookEventSet = computed(
  () => new Set(availableWebhookEvents.value.map((event) => event.type)),
)
const visibleEndpointSecrets = computed(() =>
  Object.entries(endpointSecrets.value).map(([id, secret]) => ({
    id,
    secret,
    url: form.webhook_endpoints.find((endpoint) => endpoint.id === id)?.url ?? id,
  })),
)

onMounted(load)

watch(
  () => form.client_type,
  (clientType) => {
    form.permissions = permissionsForOAuthClientType(form.permissions, catalog.value, clientType)
  },
)

watch(availableWebhookEventSet, (available) => {
  form.webhook_endpoints = endpointsWithAllowedEvents(form.webhook_endpoints, available)
})

async function load() {
  loading.value = true
  try {
    const [permissionsRes, eventsRes] = await Promise.all([
      getPermissionCatalog(),
      getOAuthWebhookEventCatalog(),
    ])
    catalog.value = permissionsRes.data.permissions
    webhookCatalog.value = eventsRes.data.events
    if (clientId.value) {
      const res = await getOAuthApp(clientId.value)
      app.value = res.data
      hydrate(res.data)
    }
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '加载应用配置失败'))
    await router.push({ name: 'dashboard-oauth' })
  } finally {
    loading.value = false
  }
}

function hydrate(next: OAuthClient) {
  form.name = next.name
  form.description = next.description
  form.redirect_uri = next.redirect_uri
  form.website_url = next.website_url
  form.client_type = next.client_type
  form.permissions = [...next.permissions]
  form.webhook_endpoints = next.webhook_endpoints.map(endpointForm)
}

function endpointForm(endpoint: OAuthWebhookEndpoint): WebhookEndpointForm {
  return {
    key: endpoint.id,
    id: endpoint.id,
    url: endpoint.url,
    enabled: endpoint.enabled,
    events: [...endpoint.events],
  }
}

function addEndpoint() {
  if (form.webhook_endpoints.length >= 5) return
  endpointSequence += 1
  form.webhook_endpoints.push({
    key: `new-${endpointSequence}`,
    url: '',
    enabled: true,
    events: [],
  })
}

function removeEndpoint(index: number) {
  form.webhook_endpoints.splice(index, 1)
}

function updateEndpoint(index: number, patch: Partial<WebhookEndpointForm>) {
  const endpoint = form.webhook_endpoints[index]
  if (endpoint) Object.assign(endpoint, patch)
}

function payload(): OAuthClientInput {
  return oauthClientPayload(form)
}

function validateForm() {
  const detail = oauthAppValidationError(form)
  if (detail) ElMessage.warning(detail)
  if (detail) return false
  return true
}

async function save(resubmit: boolean) {
  if (!canEditFields.value) return
  if (!validateForm()) return
  saving.value = true
  clientSecret.value = ''
  endpointSecrets.value = {}
  try {
    if (!app.value) {
      const res = await createOAuthApp(payload())
      app.value = res.data
      captureSecrets(res.data)
      hydrate(res.data)
      await router.replace({
        name: 'dashboard-oauth-app-edit',
        params: { client_id: res.data.client_id },
      })
      ElMessage.success('应用已提交审核')
      return
    }
    const updated = await updateOAuthApp(app.value.client_id, payload())
    let next = updated.data
    captureSecrets(next)
    if (resubmit && next.status !== 'pending') {
      const submitted = await submitOAuthAppReview(next.client_id)
      next = submitted.data
    }
    app.value = next
    hydrate(next)
    ElMessage.success(resubmit && next.status === 'pending' ? '应用已重新提交审核' : '应用已保存')
  } catch (error) {
    ElMessage.error(getErrorMessage(error, app.value ? '保存应用失败' : '提交应用失败'))
  } finally {
    saving.value = false
  }
}

function captureSecrets(next: OAuthClient) {
  clientSecret.value = next.client_secret ?? ''
  const secrets: Record<string, string> = {}
  for (const endpoint of next.webhook_endpoints) {
    if (endpoint.signing_secret) secrets[endpoint.id] = endpoint.signing_secret
  }
  endpointSecrets.value = secrets
}

async function rotateSecret() {
  if (!app.value || !canUpdateApps.value) return
  rotating.value = true
  try {
    const res = await rotateOAuthSecret(app.value.client_id)
    app.value = res.data
    clientSecret.value = res.data.client_secret ?? ''
    ElMessage.success('Client Secret 已轮换')
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '轮换失败'))
  } finally {
    rotating.value = false
  }
}

async function deleteApp() {
  if (!app.value || !canDeleteApps.value) return
  await ElMessageBox.confirm(
    '删除后应用、Webhook endpoints 和授权都会被清除，确认删除？',
    '删除应用',
  )
  deleting.value = true
  try {
    await deleteOAuthApp(app.value.client_id)
    ElMessage.success('应用已删除')
    await router.push({ name: 'dashboard-oauth' })
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '删除失败'))
  } finally {
    deleting.value = false
  }
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
  if (status === 'pending') return '应用正在等待管理员审核，审核通过前不会产生 Webhook 投递。'
  if (status === 'rejected') return '应用审核未通过，可以调整信息后重新提交。'
  return '应用已被管理员停用，OAuth 授权和 Webhook 投递均不可用。'
}
</script>
