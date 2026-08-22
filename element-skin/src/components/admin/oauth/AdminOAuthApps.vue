<template>
  <div class="mx-auto max-w-[1200px] py-5 animate-fade-in">
    <PageHeader title="第三方应用" subtitle="查看、审核和停用全站已注册的第三方应用">
      <template #icon><Link /></template>
      <template #actions>
        <el-button :icon="Refresh" plain class="hover-lift" :loading="loading" @click="loadData">
          刷新
        </el-button>
      </template>
    </PageHeader>

    <UiCard v-if="canReadApps" shadow="never">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <UiSegmented v-model="status" @change="loadApps">
          <el-radio-button
            v-for="option in statusOptions"
            :key="option.value"
            :value="option.value"
          >
            {{ option.label }}
          </el-radio-button>
        </UiSegmented>
        <span class="text-sm text-[var(--color-text-light)]"> 当前 {{ apps.length }} 个应用 </span>
      </div>

      <el-table
        :data="apps"
        class="modern-table w-full"
        v-loading="loading"
        @row-click="openDetails"
      >
        <el-table-column label="应用" min-width="280">
          <template #default="{ row }">
            <div class="flex min-w-0 flex-col">
              <span class="font-semibold text-[var(--color-heading)]">{{ row.name }}</span>
              <span class="mt-1 truncate text-sm text-[var(--color-text-light)]">
                {{ row.description || '开发者未填写说明' }}
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="所有者" min-width="180">
          <template #default="{ row }">
            <span class="font-mono text-xs text-[var(--color-text-light)]">
              {{ row.owner_user_id }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="120" align="center">
          <template #default="{ row }">
            <el-tag size="small">{{ clientTypeLabel(row.client_type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="statusType(row.status)">
              {{ statusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="更新时间" width="160">
          <template #default="{ row }">
            <span class="text-xs text-[var(--color-text-light)]">
              {{ formatDate(row.updated_at) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right" align="center">
          <template #default="{ row }">
            <el-button link type="primary" @click.stop="openDetails(row)">详情</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && apps.length === 0" description="暂无第三方应用" />
    </UiCard>

    <UiCard v-if="canReadGrants" shadow="never" class="mt-6">
      <template #header>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div class="font-semibold text-[var(--color-heading)]">用户授权</div>
            <div class="mt-1 text-xs text-[var(--color-text-light)]">
              撤销后，该授权关联的访问令牌、刷新令牌和未使用授权码立即失效。
            </div>
          </div>
          <el-tag effect="plain">{{ grants.length }} 条</el-tag>
        </div>
      </template>

      <el-table :data="grants" class="modern-table w-full" v-loading="grantsLoading">
        <el-table-column label="应用" min-width="220">
          <template #default="{ row }">
            <div class="flex min-w-0 flex-col">
              <span class="font-medium text-[var(--color-heading)]">
                {{ appName(row.client_id) }}
              </span>
              <span class="truncate font-mono text-xs text-[var(--color-text-light)]">
                {{ row.client_id }}
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="用户" min-width="190">
          <template #default="{ row }">
            <span class="font-mono text-xs text-[var(--color-text-light)]">{{ row.user_id }}</span>
          </template>
        </el-table-column>
        <el-table-column label="授权范围" min-width="280">
          <template #default="{ row }">
            <div class="flex flex-wrap gap-1.5">
              <el-tag v-for="scope in grantScopes(row)" :key="scope" size="small" effect="plain">
                {{ scope }}
              </el-tag>
              <span
                v-if="grantScopes(row).length === 0"
                class="text-xs text-[var(--color-text-light)]"
                >-</span
              >
            </div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'active' ? 'success' : 'info'">
              {{ row.status === 'active' ? '有效' : '已撤销' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="授权时间" width="160">
          <template #default="{ row }">
            <span class="text-xs text-[var(--color-text-light)]">{{
              formatDate(row.created_at)
            }}</span>
          </template>
        </el-table-column>
        <el-table-column
          v-if="canRevokeGrants"
          label="操作"
          width="100"
          fixed="right"
          align="center"
        >
          <template #default="{ row }">
            <el-button
              v-if="row.status === 'active'"
              link
              type="danger"
              :loading="revokingGrantId === row.id"
              @click="revokeGrant(row)"
            >
              撤销
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!grantsLoading && grants.length === 0" description="暂无用户授权" />
    </UiCard>

    <AdminOAuthAppDetailDialog
      v-model:visible="detailVisible"
      :app="selectedApp"
      :catalog="catalog"
      :loading="detailLoading"
      :reviewing="reviewingId === selectedApp?.client_id"
      :can-review="canReviewApps"
      @review="review"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onMounted, ref, type Ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Link, Refresh } from '@element-plus/icons-vue'
import {
  getAdminOAuthApp,
  getPermissionCatalog,
  listAdminOAuthApps,
  listAdminOAuthGrants,
  revokeAdminOAuthGrant,
  reviewAdminOAuthApp,
  type OAuthClient,
  type OAuthClientSummary,
  type OAuthClientStatus,
  type OAuthGrant,
} from '@/api/oauth'
import type { PermissionDefinition, User } from '@/api/types'
import UiCard from '@/components/ui/UiCard.vue'
import UiSegmented from '@/components/ui/UiSegmented.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import AdminOAuthAppDetailDialog from '@/components/admin/oauth/AdminOAuthAppDetailDialog.vue'
import { getErrorMessage } from '@/utils/error'

const status = ref<OAuthClientStatus | 'all'>('all')
const user = inject<Ref<User | null>>('user', ref(null))
const permissionSet = computed(() => new Set(user.value?.permissions || []))
const canReadApps = computed(() => permissionSet.value.has('oauth_app.read.any'))
const canReviewApps = computed(() => permissionSet.value.has('oauth_app.review.any'))
const canReadGrants = computed(() => permissionSet.value.has('oauth_grant.read.any'))
const canRevokeGrants = computed(() => permissionSet.value.has('oauth_grant.revoke.any'))
const canReadPermissionCatalog = computed(() => permissionSet.value.has('permission.read.any'))
const apps = ref<OAuthClientSummary[]>([])
const grants = ref<OAuthGrant[]>([])
const catalog = ref<PermissionDefinition[]>([])
const loading = ref(false)
const detailLoading = ref(false)
const reviewingId = ref('')
const detailVisible = ref(false)
const grantsLoading = ref(false)
const revokingGrantId = ref('')
const selectedApp = ref<OAuthClient | null>(null)
const statusOptions: Array<{
  label: string
  value: OAuthClientStatus | 'all'
}> = [
  { label: '全部应用', value: 'all' },
  { label: '待审核', value: 'pending' },
  { label: '已通过', value: 'active' },
  { label: '已驳回', value: 'rejected' },
  { label: '已停用', value: 'disabled' },
]

onMounted(async () => {
  await loadData()
})

async function loadData() {
  await Promise.all([
    canReadPermissionCatalog.value ? loadCatalog() : Promise.resolve(),
    canReadApps.value ? loadApps() : Promise.resolve(),
    canReadGrants.value ? loadGrants() : Promise.resolve(),
  ])
}

async function loadCatalog() {
  const res = await getPermissionCatalog()
  catalog.value = res.data.permissions
}

async function loadApps() {
  if (!canReadApps.value) return
  loading.value = true
  try {
    const res = await listAdminOAuthApps(status.value)
    apps.value = res.data.items
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '加载第三方应用失败'))
  } finally {
    loading.value = false
  }
}

async function review(clientId: string, nextStatus: Exclude<OAuthClientStatus, 'pending'>) {
  if (!canReviewApps.value) return
  const reason = await reviewReason(nextStatus)
  if (reason === null) return
  reviewingId.value = clientId
  try {
    const res = await reviewAdminOAuthApp(clientId, nextStatus, reason)
    apps.value = apps.value.map((app) =>
      app.client_id === clientId ? summaryFromClient(res.data) : app,
    )
    if (selectedApp.value?.client_id === clientId) {
      selectedApp.value = res.data
    }
    ElMessage.success('应用状态已更新')
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '更新应用状态失败'))
  } finally {
    reviewingId.value = ''
  }
}

async function loadGrants() {
  if (!canReadGrants.value) return
  grantsLoading.value = true
  try {
    const response = await listAdminOAuthGrants()
    grants.value = response.data.items
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '加载用户授权失败'))
  } finally {
    grantsLoading.value = false
  }
}

async function revokeGrant(grant: OAuthGrant) {
  if (!canRevokeGrants.value) return
  try {
    await ElMessageBox.confirm(
      '撤销后，该授权下的访问令牌、刷新令牌和未使用授权码都会立即失效。',
      '撤销用户授权',
      { type: 'warning', confirmButtonText: '撤销', cancelButtonText: '取消' },
    )
    revokingGrantId.value = grant.id
    await revokeAdminOAuthGrant(grant.id)
    grants.value = grants.value.map((item) =>
      item.id === grant.id ? { ...item, status: 'revoked' as const, revoked_at: Date.now() } : item,
    )
    ElMessage.success('用户授权已撤销')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      ElMessage.error(getErrorMessage(error, '撤销用户授权失败'))
    }
  } finally {
    revokingGrantId.value = ''
  }
}

async function reviewReason(nextStatus: Exclude<OAuthClientStatus, 'pending'>) {
  if (nextStatus === 'active') return ''
  try {
    const promptResult = (await ElMessageBox.prompt(
      nextStatus === 'rejected' ? '请填写驳回原因' : '请填写停用原因',
      nextStatus === 'rejected' ? '驳回第三方应用' : '停用第三方应用',
      {
        confirmButtonText: nextStatus === 'rejected' ? '确认驳回' : '确认停用',
        cancelButtonText: '取消',
        inputType: 'textarea',
        inputPlaceholder: '原因会发送给应用开发者',
        inputValidator: (value) => {
          const trimmed = value.trim()
          if (!trimmed) return '原因不能为空'
          if ([...trimmed].length > 500) return '原因不能超过 500 个字符'
          return true
        },
      },
    )) as { value: string }
    return promptResult.value.trim()
  } catch {
    return null
  }
}

async function openDetails(app: OAuthClientSummary) {
  selectedApp.value = null
  detailVisible.value = true
  detailLoading.value = true
  try {
    const res = await getAdminOAuthApp(app.client_id)
    selectedApp.value = res.data
  } catch (error) {
    detailVisible.value = false
    ElMessage.error(getErrorMessage(error, '加载应用详情失败'))
  } finally {
    detailLoading.value = false
  }
}

function statusLabel(appStatus: OAuthClientStatus) {
  const labels: Record<OAuthClientStatus, string> = {
    pending: '待审核',
    active: '已通过',
    rejected: '已驳回',
    disabled: '已停用',
  }
  return labels[appStatus]
}

function statusType(appStatus: OAuthClientStatus) {
  if (appStatus === 'active') return 'success'
  if (appStatus === 'rejected') return 'danger'
  if (appStatus === 'pending') return 'warning'
  return 'info'
}

function clientTypeLabel(clientType: OAuthClient['client_type']) {
  return clientType === 'confidential' ? '机密应用' : '公开应用'
}

function appName(clientId: string) {
  return apps.value.find((app) => app.client_id === clientId)?.name || '未知应用'
}

function grantScopes(grant: OAuthGrant) {
  return [...grant.oidc_scopes, ...grant.permissions]
}

function summaryFromClient(app: OAuthClient): OAuthClientSummary {
  return {
    client_id: app.client_id,
    owner_user_id: app.owner_user_id,
    name: app.name,
    description: app.description,
    client_type: app.client_type,
    status: app.status,
    created_at: app.created_at,
    updated_at: app.updated_at,
  }
}

function formatDate(value?: number) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}
</script>
