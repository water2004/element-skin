<template>
  <div class="space-y-6 animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <h1>第三方应用</h1>
        <p>管理你申请的应用，以及你授权给外部应用的访问能力</p>
      </div>
    </div>

    <UiCard class="p-6">
      <div class="mb-5 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">作为应用开发者</h2>
          <p class="mt-1 mb-0 text-sm text-[var(--color-text-light)]">
            提交应用审核，通过后才能开始 OAuth 授权流程。
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <el-button :loading="loading" @click="loadApps">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
          <el-button type="primary" @click="openCreateDialog">
            <el-icon><Plus /></el-icon>
            申请新应用
          </el-button>
        </div>
      </div>

      <el-empty v-if="!loading && apps.length === 0" description="还没有申请应用" />
      <div v-else v-loading="loading" class="divide-y divide-[var(--color-border)]">
        <div
          v-for="app in apps"
          :key="app.client_id"
          class="flex flex-col gap-3 py-4 first:pt-0 last:pb-0 lg:flex-row lg:items-center lg:justify-between"
        >
          <div class="min-w-0 flex-1">
            <div class="mb-2 flex flex-wrap items-center gap-2">
              <span class="max-w-full truncate font-semibold text-[var(--color-heading)]">
                {{ app.name }}
              </span>
              <el-tag size="small" :type="statusType(app.status)">
                {{ statusLabel(app.status) }}
              </el-tag>
              <el-tag size="small">{{ clientTypeLabel(app.client_type) }}</el-tag>
            </div>
            <div class="truncate font-mono text-xs text-[var(--color-text-light)]">
              {{ app.client_id }}
            </div>
            <p
              v-if="app.description"
              class="mt-2 mb-0 line-clamp-2 text-sm text-[var(--color-text-light)]"
            >
              {{ app.description }}
            </p>
          </div>

          <div class="flex min-w-0 flex-col gap-3 lg:w-[420px]">
            <div class="flex flex-wrap gap-2">
              <PermissionToneTag
                v-for="code in app.permissions.slice(0, 5)"
                :key="code"
                :label="permissionLabel(code)"
                :title="code"
                tone="violet"
              />
              <el-text v-if="app.permissions.length > 5" size="small" type="info">
                +{{ app.permissions.length - 5 }}
              </el-text>
            </div>
            <div class="flex justify-end">
              <el-button type="primary" plain @click="openEditDialog(app)">
                <el-icon><Edit /></el-icon>
                编辑
              </el-button>
            </div>
          </div>
        </div>
      </div>
    </UiCard>

    <UiCard class="p-6">
      <div class="mb-5 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">作为用户已授权</h2>
          <p class="mt-1 mb-0 text-sm text-[var(--color-text-light)]">
            管理外部应用已经获得的用户委托权限。
          </p>
        </div>
        <el-button :loading="grantsLoading" @click="loadGrants">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </div>

      <el-empty v-if="!grantsLoading && grants.length === 0" description="暂无已授权应用" />
      <div v-else v-loading="grantsLoading" class="grid gap-3">
        <div
          v-for="grant in grants"
          :key="grant.id"
          class="rounded-lg bg-[var(--color-background-soft)] p-4"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <div class="truncate font-semibold text-[var(--color-heading)]">
                {{ clientName(grant.client_id) }}
              </div>
              <div class="mt-1 truncate text-xs text-[var(--color-text-light)]">
                {{ grant.client_id }}
              </div>
            </div>
            <el-tag size="small" :type="grant.status === 'active' ? 'success' : 'info'">
              {{ grant.status === 'active' ? '已授权' : '已撤销' }}
            </el-tag>
          </div>
          <div class="mt-3 flex flex-wrap gap-2">
            <PermissionToneTag
              v-for="code in grant.permissions"
              :key="code"
              :label="permissionLabel(code)"
              :title="code"
              tone="sky"
            />
          </div>
          <p
            v-if="grant.status === 'revoked'"
            class="mt-3 mb-0 text-xs text-[var(--color-text-light)]"
          >
            {{ revokedGrantCleanupText(grant) }}
          </p>
          <div class="mt-3 flex justify-end">
            <el-button
              v-if="grant.status === 'active'"
              type="danger"
              link
              :loading="revokingGrantId === grant.id"
              @click="revokeGrant(grant.id)"
            >
              撤销授权
            </el-button>
          </div>
        </div>
      </div>
    </UiCard>

    <DashboardOAuthAppDialog
      v-model:visible="appDialogVisible"
      :app="editingApp"
      :catalog="catalog"
      :user-permissions="user?.permissions ?? []"
      :new-secret="newSecret"
      :saving="saving"
      :rotating="rotating"
      :deleting="deleting"
      @save="saveApp"
      @rotate="rotateSecret"
      @delete="deleteApp"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onMounted, ref, type Ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Edit, Plus, Refresh } from '@element-plus/icons-vue'
import {
  createOAuthApp,
  deleteOAuthApp,
  getPermissionCatalog,
  listOAuthApps,
  listOAuthGrants,
  revokeOAuthGrant,
  rotateOAuthSecret,
  submitOAuthAppReview,
  updateOAuthApp,
  type OAuthClient,
  type OAuthClientInput,
  type OAuthClientStatus,
  type OAuthGrant,
} from '@/api/oauth'
import type { PermissionDefinition, User } from '@/api/types'
import UiCard from '@/components/ui/UiCard.vue'
import PermissionToneTag from '@/components/admin/users/PermissionToneTag.vue'
import DashboardOAuthAppDialog from '@/components/dashboard/oauth/DashboardOAuthAppDialog.vue'
import { getErrorMessage } from '@/utils/error'

const user = inject<Ref<User | null>>('user', ref(null))

const apps = ref<OAuthClient[]>([])
const grants = ref<OAuthGrant[]>([])
const catalog = ref<PermissionDefinition[]>([])
const loading = ref(false)
const grantsLoading = ref(false)
const saving = ref(false)
const rotating = ref(false)
const deleting = ref(false)
const revokingGrantId = ref('')
const appDialogVisible = ref(false)
const editingApp = ref<OAuthClient | null>(null)
const newSecret = ref('')

const revokedGrantRetentionMs = 30 * 24 * 60 * 60 * 1000

const permissionByCode = computed(() => {
  const out = new Map<string, PermissionDefinition>()
  for (const item of catalog.value) out.set(item.code, item)
  return out
})

onMounted(async () => {
  await Promise.all([loadCatalog(), loadApps(), loadGrants()])
})

async function loadCatalog() {
  const res = await getPermissionCatalog()
  catalog.value = res.data.permissions
}

async function loadApps() {
  loading.value = true
  try {
    const res = await listOAuthApps()
    apps.value = res.data.items
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '加载应用失败'))
  } finally {
    loading.value = false
  }
}

async function loadGrants() {
  grantsLoading.value = true
  try {
    const res = await listOAuthGrants()
    grants.value = res.data.items
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '加载授权失败'))
  } finally {
    grantsLoading.value = false
  }
}

function openCreateDialog() {
  editingApp.value = null
  newSecret.value = ''
  appDialogVisible.value = true
}

function openEditDialog(app: OAuthClient) {
  editingApp.value = app
  newSecret.value = ''
  appDialogVisible.value = true
}

async function saveApp(payload: OAuthClientInput, options: { resubmit: boolean }) {
  if (!payload.name || !payload.redirect_uri) {
    ElMessage.warning('请填写名称和回调地址')
    return
  }

  saving.value = true
  try {
    if (!editingApp.value) {
      const res = await createOAuthApp(payload)
      apps.value.unshift(res.data)
      editingApp.value = res.data
      newSecret.value = res.data.client_secret ?? ''
      ElMessage.success('应用已提交审核')
      return
    }

    const clientId = editingApp.value.client_id
    const updated = await updateOAuthApp(clientId, payload)
    let next = updated.data
    if (options.resubmit && next.status !== 'pending') {
      const submitted = await submitOAuthAppReview(clientId)
      next = submitted.data
    }
    replaceApp(next)
    editingApp.value = next
    ElMessage.success(
      options.resubmit && next.status === 'pending' ? '应用已重新提交审核' : '应用已保存',
    )
  } catch (error) {
    ElMessage.error(getErrorMessage(error, editingApp.value ? '保存应用失败' : '提交应用失败'))
  } finally {
    saving.value = false
  }
}

async function rotateSecret(clientId: string) {
  rotating.value = true
  try {
    const res = await rotateOAuthSecret(clientId)
    replaceApp(res.data)
    editingApp.value = res.data
    newSecret.value = res.data.client_secret ?? ''
    ElMessage.success('密钥已轮换')
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '轮换失败'))
  } finally {
    rotating.value = false
  }
}

async function deleteApp(clientId: string) {
  await ElMessageBox.confirm('删除后应用将无法继续完成 OAuth 授权，确认删除？', '删除应用')
  deleting.value = true
  try {
    await deleteOAuthApp(clientId)
    apps.value = apps.value.filter((app) => app.client_id !== clientId)
    if (editingApp.value?.client_id === clientId) editingApp.value = null
    appDialogVisible.value = false
    newSecret.value = ''
    ElMessage.success('应用已删除')
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '删除失败'))
  } finally {
    deleting.value = false
  }
}

async function revokeGrant(grantId: string) {
  revokingGrantId.value = grantId
  try {
    await revokeOAuthGrant(grantId)
    await loadGrants()
    ElMessage.success('授权已撤销')
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '撤销授权失败'))
  } finally {
    revokingGrantId.value = ''
  }
}

function replaceApp(next: OAuthClient) {
  apps.value = apps.value.map((app) => (app.client_id === next.client_id ? next : app))
}

function clientName(clientId: string) {
  return apps.value.find((app) => app.client_id === clientId)?.name || '第三方应用'
}

function permissionLabel(code: string) {
  return permissionByCode.value.get(code)?.description || code
}

function revokedGrantCleanupText(grant: OAuthGrant) {
  if (!grant.revoked_at) return '已撤销的授权将在 30 天后自动清除'
  const cleanupAt = grant.revoked_at + revokedGrantRetentionMs
  const remaining = cleanupAt - Date.now()
  if (remaining <= 0) return '已撤销的授权即将自动清除'
  const days = Math.ceil(remaining / (24 * 60 * 60 * 1000))
  return `已撤销的授权将在 ${days} 天后自动清除`
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

function clientTypeLabel(clientType: OAuthClient['client_type']) {
  return clientType === 'confidential' ? '机密应用' : '公开应用'
}
</script>
