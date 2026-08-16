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

    <el-alert
      type="info"
      :closable="false"
      show-icon
      class="mb-6"
      title="Microsoft 也是普通 OIDC 身份提供方"
      description="选择 Microsoft 适配器只会开放本站的正版角色能力；身份登录、绑定、重新授权和令牌存储仍走同一套 OIDC 路径。"
    />

    <OidcRedirectUriNotice :uri="redirectUri" class="mb-6 mt-4" />

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
import { Connection, Plus } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import ActionBar from '@/components/common/ActionBar.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import UiCard from '@/components/ui/UiCard.vue'
import { deleteIdentityProvider, getAdminIdentityProviders } from '@/api/admin/identity-providers'
import type { AdminIdentityProvider, User } from '@/api/types'
import { getErrorMessage } from '@/utils/error'
import OidcRedirectUriNotice from './OidcRedirectUriNotice.vue'

const router = useRouter()
const user = inject<Ref<User | null>>('user', ref(null))
const providers = ref<AdminIdentityProvider[]>([])
const loading = ref(false)
const redirectUri = ref('')
const permissionSet = computed(() => new Set(user.value?.permissions || []))
const canRead = computed(() => permissionSet.value.has('identity_provider.read.any'))
const canCreate = computed(() => permissionSet.value.has('identity_provider.create.any'))
const canUpdate = computed(() => permissionSet.value.has('identity_provider.update.any'))
const canDelete = computed(() => permissionSet.value.has('identity_provider.delete.any'))

function openCreate() {
  void router.push({ name: 'admin-identity-provider-create' })
}

function openEdit(provider: AdminIdentityProvider) {
  void router.push({ name: 'admin-identity-provider-edit', params: { provider_id: provider.id } })
}

async function loadProviders() {
  if (!canRead.value) return
  loading.value = true
  try {
    const response = await getAdminIdentityProviders()
    providers.value = response.data.items
    redirectUri.value = response.data.redirect_uri
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
</script>
