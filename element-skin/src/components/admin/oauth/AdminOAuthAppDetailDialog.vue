<template>
  <UiDialog v-model="visible" destroy-on-close align-center variant="wide-form">
    <div v-loading="loading" class="min-h-[320px] p-6">
      <div v-if="app">
        <div class="mb-5 flex flex-wrap items-start justify-between gap-4">
          <div class="min-w-0">
            <div class="mb-2 flex flex-wrap items-center gap-2">
              <h2 class="m-0 break-words text-xl font-semibold text-[var(--color-heading)]">
                {{ app.name }}
              </h2>
              <el-tag size="small" :type="statusType(app.status)">
                {{ statusLabel(app.status) }}
              </el-tag>
              <el-tag size="small">{{ clientTypeLabel(app.client_type) }}</el-tag>
            </div>
            <el-text copyable class="font-mono text-xs text-[var(--color-text-light)]">
              {{ app.client_id }}
            </el-text>
          </div>

          <div class="flex flex-wrap justify-end gap-2">
            <el-button
              v-if="app.status !== 'active'"
              type="success"
              plain
              :loading="reviewing"
              @click="$emit('review', app.client_id, 'active')"
            >
              通过
            </el-button>
            <el-button
              v-if="app.status !== 'rejected'"
              type="danger"
              plain
              :loading="reviewing"
              @click="$emit('review', app.client_id, 'rejected')"
            >
              驳回
            </el-button>
            <el-button
              v-if="app.status !== 'disabled'"
              type="warning"
              plain
              :loading="reviewing"
              @click="$emit('review', app.client_id, 'disabled')"
            >
              停用
            </el-button>
          </div>
        </div>

        <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
          <div class="space-y-4">
            <UiCard shadow="never">
              <template #header>
                <div class="font-semibold text-[var(--color-heading)]">申请信息</div>
              </template>
              <el-descriptions :column="1" border>
                <el-descriptions-item label="所有者">
                  <span class="font-mono text-xs">{{ app.owner_user_id }}</span>
                </el-descriptions-item>
                <el-descriptions-item label="回调地址">
                  <el-text copyable class="break-all">{{ app.redirect_uri }}</el-text>
                </el-descriptions-item>
                <el-descriptions-item label="站点地址">
                  <span class="break-all">{{ app.website_url || '-' }}</span>
                </el-descriptions-item>
                <el-descriptions-item label="创建时间">
                  {{ formatDate(app.created_at) }}
                </el-descriptions-item>
                <el-descriptions-item label="更新时间">
                  {{ formatDate(app.updated_at) }}
                </el-descriptions-item>
              </el-descriptions>
            </UiCard>

            <UiCard shadow="never">
              <template #header>
                <div class="font-semibold text-[var(--color-heading)]">应用说明</div>
              </template>
              <p class="m-0 whitespace-pre-wrap text-sm leading-6 text-[var(--color-text)]">
                {{ app.description || '开发者未填写说明。' }}
              </p>
            </UiCard>
          </div>

          <UiCard shadow="never">
            <template #header>
              <div class="font-semibold text-[var(--color-heading)]">申请权限</div>
            </template>
            <div v-if="app.permissions.length > 0" class="space-y-4">
              <section v-for="group in groupedPermissions" :key="group.name">
                <div class="mb-2 flex items-center justify-between gap-3">
                  <span class="text-sm font-semibold text-[var(--color-heading)]">
                    {{ group.name }}
                  </span>
                  <el-tag size="small" effect="plain">{{ group.items.length }} 项</el-tag>
                </div>
                <div class="flex flex-wrap gap-2">
                  <PermissionToneTag
                    v-for="code in group.items"
                    :key="code"
                    :label="permissionLabel(code)"
                    :title="code"
                    tone="violet"
                  />
                </div>
              </section>
            </div>
            <el-empty v-else description="未申请权限" :image-size="80" />
          </UiCard>
        </div>
      </div>
    </div>
  </UiDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { PermissionDefinition } from '@/api/types'
import type { OAuthClient, OAuthClientStatus } from '@/api/oauth'
import UiCard from '@/components/ui/UiCard.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import PermissionToneTag from '@/components/admin/users/PermissionToneTag.vue'

const visible = defineModel<boolean>('visible', { required: true })

const props = defineProps<{
  app: OAuthClient | null
  catalog: PermissionDefinition[]
  loading: boolean
  reviewing: boolean
}>()

defineEmits<{
  review: [clientId: string, status: Exclude<OAuthClientStatus, 'pending'>]
}>()

const groupedPermissions = computed(() => {
  const groups = new Map<string, string[]>()
  for (const code of props.app?.permissions ?? []) {
    const key = permissionGroupLabel(code)
    groups.set(key, [...(groups.get(key) ?? []), code])
  }
  return [...groups.entries()].map(([name, items]) => ({ name, items }))
})

function permissionLabel(code: string) {
  return props.catalog.find((item) => item.code === code)?.description || code
}

function permissionGroupLabel(code: string) {
  const namespace = code.split('.')[0] || code
  const labels: Record<string, string> = {
    account: '账号',
    cache: '缓存',
    config: '公开站点配置',
    homepage_media: '首页媒体',
    invite: '邀请码',
    material: '材质',
    minecraft_session: 'Minecraft 会话',
    external_identity: '外部身份',
    identity_provider: 'OIDC 身份提供方',
    official_profile: '正版角色绑定',
    notice: '通知',
    oauth_app: '第三方应用',
    oauth_grant: '应用授权',
    oauth_token: '应用令牌',
    permission: '权限',
    permission_audit: '权限审计',
    permission_protected: '受保护权限主体',
    permission_role: '权限角色',
    profile: '角色',
    settings: '站点设置',
    user: '用户',
    wardrobe: '衣柜',
    wardrobe_item: '衣柜条目',
    whitelist: '官方白名单',
    yggdrasil_session: 'Yggdrasil 会话',
    yggdrasil_token: 'Yggdrasil token',
  }
  return labels[namespace] || namespace
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

function formatDate(value?: number) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}
</script>
