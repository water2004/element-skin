<template>
  <div class="max-w-[1000px] mx-auto py-5 animate-fade-in">
    <PageHeader title="OIDC 身份提供方" subtitle="配置允许用户登录和绑定的外部 OIDC 端点">
      <template #icon><Connection /></template>
      <template #actions>
        <ActionBar>
          <el-button v-if="canCreate" type="primary" :icon="Plus" @click="openCreate">
            添加提供方
          </el-button>
        </ActionBar>
      </template>
    </PageHeader>

    <UiCard shadow="never" class="mb-6">
      <div class="flex flex-col gap-5">
        <div>
          <div class="flex items-center gap-2 text-lg font-semibold text-[var(--color-heading)]">
            <el-icon><Promotion /></el-icon>
            本站 OIDC 信息
          </div>
          <p class="mt-2 mb-0 text-sm leading-6 text-[var(--color-text-light)]">
            外部站点可以使用本站进行 OIDC
            登录。应用先在“第三方应用”中登记精确回调地址，审核通过后使用 Authorization Code + PKCE
            发起授权。
          </p>
        </div>

        <div v-loading="discoveryLoading" class="grid gap-3 md:grid-cols-2">
          <div
            v-for="endpoint in serverEndpoints"
            :key="endpoint.label"
            class="rounded-lg border border-[var(--color-border)] bg-[var(--color-background-soft)] p-3"
          >
            <div class="text-xs text-[var(--color-text-light)]">
              {{ endpoint.label }}
            </div>
            <el-text copyable class="mt-1 block break-all font-mono text-xs">
              {{ endpoint.value }}
            </el-text>
          </div>
        </div>

        <el-alert
          v-if="discoveryError"
          type="error"
          :closable="false"
          title="无法读取本站 OIDC 发现文档"
        />
        <el-alert
          v-else
          type="info"
          :closable="false"
          title="标准身份 scopes 由用户授权"
          description="openid、profile、email 和 offline_access 不属于站点 API 权限，无需管理员逐项审批；应用仍需登记并通过应用审核。"
        />

        <div class="flex flex-wrap gap-3">
          <el-button
            v-if="canManageOwnApps"
            type="primary"
            @click="router.push('/dashboard/oauth')"
          >
            登记第三方应用
          </el-button>
          <el-button v-if="canReviewApps" @click="router.push('/admin/oauth-apps')">
            审核应用与授权
          </el-button>
        </div>
      </div>
    </UiCard>

    <div v-loading="loading" class="min-h-[220px]">
      <div v-if="providers.length" class="grid gap-4">
        <UiCard v-for="provider in providers" :key="provider.id" hoverable>
          <div class="flex flex-col md:flex-row md:items-start gap-4">
            <div class="flex h-12 w-12 shrink-0 items-center justify-center">
              <img
                v-if="provider.icon_url"
                :src="provider.icon_url"
                alt=""
                class="h-12 w-12 object-contain"
              />
              <el-icon v-else :size="28" class="text-[var(--el-color-primary)]">
                <Connection />
              </el-icon>
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-semibold text-lg text-[var(--color-heading)]">
                  {{ provider.name }}
                </span>
                <el-tag :type="provider.enabled ? 'success' : 'info'">
                  {{ provider.enabled ? '已启用' : '已停用' }}
                </el-tag>
                <el-tag v-if="provider.adapter === 'microsoft'" type="success">Microsoft</el-tag>
                <el-tag v-else>通用 OIDC</el-tag>
              </div>
              <div class="mt-2 text-sm text-[var(--color-text-light)] break-all">
                {{ provider.issuer_url }}
              </div>
              <div class="mt-3 flex flex-wrap gap-2">
                <el-tag v-if="provider.login_enabled" size="small">登录</el-tag>
                <el-tag v-if="provider.link_enabled" size="small">绑定</el-tag>
                <el-tag v-for="scope in provider.scopes" :key="scope" size="small" type="info">
                  {{ scope }}
                </el-tag>
              </div>
            </div>
            <ActionBar>
              <el-button v-if="canUpdate" @click="openEdit(provider)">编辑</el-button>
              <el-button v-if="canDelete" type="danger" plain @click="removeProvider(provider)">
                删除
              </el-button>
            </ActionBar>
          </div>
        </UiCard>
      </div>
      <el-empty v-else-if="!loading && canRead" description="尚未配置外部 OIDC 提供方" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onMounted, ref, type Ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Connection, Plus, Promotion } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import ActionBar from '@/components/common/ActionBar.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import UiCard from '@/components/ui/UiCard.vue'
import { deleteIdentityProvider, getAdminIdentityProviders } from '@/api/admin/identity-providers'
import type { AdminIdentityProvider, User } from '@/api/types'
import { getErrorMessage } from '@/utils/error'
import { getOpenIDConfiguration, type OpenIDConfiguration } from '@/api/oauth'

const router = useRouter()
const user = inject<Ref<User | null>>('user', ref(null))
const providers = ref<AdminIdentityProvider[]>([])
const loading = ref(false)
const discovery = ref<OpenIDConfiguration | null>(null)
const discoveryLoading = ref(false)
const discoveryError = ref(false)
const permissionSet = computed(() => new Set(user.value?.permissions || []))
const canRead = computed(() => permissionSet.value.has('identity_provider.read.any'))
const canCreate = computed(() => permissionSet.value.has('identity_provider.create.any'))
const canUpdate = computed(() => permissionSet.value.has('identity_provider.update.any'))
const canDelete = computed(() => permissionSet.value.has('identity_provider.delete.any'))
const canManageOwnApps = computed(() => permissionSet.value.has('oauth_app.read.owned'))
const canReviewApps = computed(
  () =>
    permissionSet.value.has('oauth_app.read.any') ||
    permissionSet.value.has('oauth_grant.read.any'),
)
const serverEndpoints = computed(() => {
  if (!discovery.value) return []
  return [
    {
      label: '发现文档',
      value: `${discovery.value.issuer}/.well-known/openid-configuration`,
    },
    { label: 'Issuer', value: discovery.value.issuer },
    {
      label: 'Authorization Endpoint',
      value: discovery.value.authorization_endpoint,
    },
    { label: 'Token Endpoint', value: discovery.value.token_endpoint },
    { label: 'UserInfo Endpoint', value: discovery.value.userinfo_endpoint },
    { label: 'JWKS URI', value: discovery.value.jwks_uri },
  ]
})

function openCreate() {
  void router.push({ name: 'admin-identity-provider-create' })
}

function openEdit(provider: AdminIdentityProvider) {
  void router.push({
    name: 'admin-identity-provider-edit',
    params: { provider_id: provider.id },
  })
}

async function loadProviders() {
  if (!canRead.value) return
  loading.value = true
  try {
    const response = await getAdminIdentityProviders()
    providers.value = response.data.items
  } catch (e: unknown) {
    ElMessage.error('加载提供方失败: ' + getErrorMessage(e, '加载失败'))
  } finally {
    loading.value = false
  }
}

async function removeProvider(provider: AdminIdentityProvider) {
  try {
    await ElMessageBox.confirm(
      '删除后将同时移除该提供方下的所有外部身份、授权凭据和正版绑定关系；本站角色不会被删除。此操作无法恢复。',
      `删除 ${provider.name}`,
      { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' },
    )
    await deleteIdentityProvider(provider.id)
    ElMessage.success('提供方已删除')
    await loadProviders()
  } catch (e: unknown) {
    if (e !== 'cancel' && e !== 'close') {
      ElMessage.error('删除失败: ' + getErrorMessage(e, '删除失败'))
    }
  }
}

onMounted(loadProviders)

onMounted(async () => {
  discoveryLoading.value = true
  discoveryError.value = false
  try {
    const response = await getOpenIDConfiguration()
    discovery.value = response.data
  } catch {
    discoveryError.value = true
  } finally {
    discoveryLoading.value = false
  }
})
</script>
