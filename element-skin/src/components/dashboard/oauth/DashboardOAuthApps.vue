<template>
  <div class="space-y-6 animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <h1>第三方应用</h1>
        <p>管理你申请的应用，以及你授权给外部应用的访问能力</p>
      </div>
    </div>

    <UiCard v-if="showAppSection" class="p-6">
      <div class="mb-5 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">作为应用开发者</h2>
          <p class="mt-1 mb-0 text-sm text-[var(--color-text-light)]">
            提交应用审核，通过后才能开始 OAuth 授权流程。
          </p>
        </div>
        <ActionBar>
          <el-button v-if="canReadApps" :loading="loading" @click="loadApps">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
          <el-button
            v-if="canCreateApps"
            type="primary"
            @click="router.push({ name: 'dashboard-oauth-app-create' })"
          >
            <el-icon><Plus /></el-icon>
            申请新应用
          </el-button>
        </ActionBar>
      </div>

      <el-alert
        v-if="!canReadApps"
        type="info"
        :closable="false"
        title="当前权限不能读取已申请应用"
      />
      <el-empty v-else-if="!loading && apps.length === 0" description="还没有申请应用" />
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
              <el-button
                v-if="canManageApps"
                type="primary"
                plain
                @click="
                  router.push({
                    name: 'dashboard-oauth-app-edit',
                    params: { client_id: app.client_id },
                  })
                "
              >
                <el-icon><Edit /></el-icon>
                管理
              </el-button>
            </div>
          </div>
        </div>
      </div>
    </UiCard>

    <UiCard v-if="showGrantSection" class="p-6">
      <div class="mb-5 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">作为用户已授权</h2>
          <p class="mt-1 mb-0 text-sm text-[var(--color-text-light)]">
            管理外部应用已经获得的用户委托权限。
          </p>
        </div>
        <el-button v-if="canReadGrants" :loading="grantsLoading" @click="loadGrants">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </div>

      <el-alert
        v-if="!canReadGrants"
        type="info"
        :closable="false"
        title="当前权限不能读取已授权应用"
      />
      <el-empty v-else-if="!grantsLoading && grants.length === 0" description="暂无已授权应用" />
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
              v-if="grant.status === 'active' && canRevokeGrants"
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
  </div>
</template>

<script setup lang="ts">
import { computed, inject, ref, watch, type Ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Edit, Plus, Refresh } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import {
  getPermissionCatalog,
  listOAuthApps,
  listOAuthGrants,
  revokeOAuthGrant,
  type OAuthClient,
  type OAuthClientStatus,
  type OAuthGrant,
} from '@/api/oauth'
import type { PermissionDefinition, User } from '@/api/types'
import ActionBar from '@/components/common/ActionBar.vue'
import UiCard from '@/components/ui/UiCard.vue'
import PermissionToneTag from '@/components/permissions/PermissionToneTag.vue'
import { getErrorMessage } from '@/utils/error'

const router = useRouter()
const user = inject<Ref<User | null>>('user', ref(null))

const apps = ref<OAuthClient[]>([])
const grants = ref<OAuthGrant[]>([])
const catalog = ref<PermissionDefinition[]>([])
const loading = ref(false)
const grantsLoading = ref(false)
const revokingGrantId = ref('')

const revokedGrantRetentionMs = 30 * 24 * 60 * 60 * 1000
const userPermissions = computed(() => new Set(user.value?.permissions ?? []))
const canReadApps = computed(() => userPermissions.value.has('oauth_app.read.owned'))
const canCreateApps = computed(() => userPermissions.value.has('oauth_app.create.owned'))
const canUpdateApps = computed(() => userPermissions.value.has('oauth_app.update.owned'))
const canDeleteApps = computed(() => userPermissions.value.has('oauth_app.delete.owned'))
const canManageApps = computed(
  () => canReadApps.value && (canUpdateApps.value || canDeleteApps.value),
)
const showAppSection = computed(
  () => canReadApps.value || canCreateApps.value || canUpdateApps.value || canDeleteApps.value,
)
const canReadGrants = computed(() => userPermissions.value.has('oauth_grant.read.owned'))
const canRevokeGrants = computed(() => userPermissions.value.has('oauth_grant.revoke.owned'))
const showGrantSection = computed(() => canReadGrants.value || canRevokeGrants.value)

const permissionByCode = computed(() => {
  const out = new Map<string, PermissionDefinition>()
  for (const item of catalog.value) out.set(item.code, item)
  return out
})

watch(
  () => user.value?.permissions,
  (permissions, previousPermissions) => {
    if (!permissions) return
    const previous = new Set(previousPermissions ?? [])
    const requests: Promise<void>[] = []
    if (!previousPermissions) requests.push(loadCatalog())
    if (canReadApps.value && !previous.has('oauth_app.read.owned')) requests.push(loadApps())
    if (canReadGrants.value && !previous.has('oauth_grant.read.owned')) requests.push(loadGrants())
    void Promise.all(requests)
  },
  { immediate: true },
)

async function loadCatalog() {
  try {
    const res = await getPermissionCatalog()
    catalog.value = res.data.permissions
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '加载权限目录失败'))
  }
}

async function loadApps() {
  if (!canReadApps.value) return
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
  if (!canReadGrants.value) return
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

async function revokeGrant(grantId: string) {
  if (!canRevokeGrants.value) return
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
