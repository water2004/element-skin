<template>
  <div class="space-y-6">
    <section class="rounded-lg border border-[var(--color-border)] p-5">
      <div class="flex flex-col gap-5 sm:flex-row sm:items-start sm:justify-between">
        <div class="min-w-0">
          <div class="flex items-center gap-3">
            <div
              class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-sky-50 text-sky-600 ring-1 ring-sky-100 dark:bg-sky-500/10 dark:text-sky-300 dark:ring-sky-500/20"
            >
              <el-icon :size="24"><Connection /></el-icon>
            </div>
            <div class="min-w-0">
              <h2 class="m-0 truncate text-xl font-semibold text-[var(--color-heading)]">
                {{ client.name }}
              </h2>
              <p class="mt-1 mb-0 break-all text-xs text-[var(--color-text-light)]">
                {{ client.client_id }}
              </p>
            </div>
          </div>
          <p v-if="client.description" class="mt-4 mb-0 text-sm leading-6 text-[var(--color-text)]">
            {{ client.description }}
          </p>
        </div>
        <a
          v-if="client.website_url"
          class="inline-flex items-center gap-1.5 text-sm text-[var(--color-primary)] hover:underline"
          :href="client.website_url"
          target="_blank"
          rel="noopener noreferrer"
        >
          <el-icon><Link /></el-icon>
          应用网站
        </a>
      </div>

      <div v-if="requestDetails.length" class="mt-5 grid gap-3 text-sm sm:grid-cols-2">
        <div
          v-for="item in requestDetails"
          :key="item.label"
          class="rounded-md bg-[var(--color-background-soft)] px-3 py-2"
        >
          <div class="text-xs text-[var(--color-text-light)]">
            {{ item.label }}
          </div>
          <div class="mt-1 break-all text-[var(--color-heading)]">
            {{ item.value }}
          </div>
        </div>
      </div>
    </section>

    <section>
      <div class="mb-3 flex items-center justify-between gap-3">
        <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">请求权限</h2>
        <el-tag>{{ scopes.length + oidcScopes.length }} 项</el-tag>
      </div>
      <div class="space-y-3">
        <div
          v-for="scope in oidcScopes"
          :key="scope"
          class="rounded-lg border border-[var(--color-border)] p-4"
        >
          <div class="flex flex-wrap items-center gap-2">
            <PermissionToneTag label="OpenID Connect" tone="sky" variant="category" />
            <PermissionToneTag :label="scope" tone="slate" :title="oidcScopeDescription(scope)" />
          </div>
          <p class="mt-3 mb-0 text-sm text-[var(--color-text)]">
            {{ oidcScopeDescription(scope) }}
          </p>
        </div>
        <div
          v-for="scope in scopes"
          :key="scope.code"
          class="rounded-lg border border-[var(--color-border)] p-4"
        >
          <div class="flex flex-wrap items-center gap-2">
            <PermissionToneTag :label="scope.resource_description" tone="sky" variant="category" />
            <PermissionToneTag :label="scope.code" tone="slate" :title="scope.description" />
          </div>
          <p class="mt-3 mb-0 text-sm text-[var(--color-text)]">
            {{ scope.description }}
          </p>
          <p class="mt-1 mb-0 text-xs text-[var(--color-text-light)]">
            {{ scope.action_description }} / {{ scope.scope_description }}
          </p>
        </div>
      </div>
    </section>

    <div class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
      <el-button :disabled="deciding" @click="emit('deny')">{{ denyLabel }}</el-button>
      <el-button type="primary" :loading="deciding" @click="emit('approve')">
        {{ approveLabel }}
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Connection, Link } from '@element-plus/icons-vue'
import type { OAuthClient, OAuthPermissionScope } from '@/api/oauth'
import PermissionToneTag from '@/components/permissions/PermissionToneTag.vue'

interface OAuthConsentDetail {
  label: string
  value: string
}

withDefaults(
  defineProps<{
    client: OAuthClient
    scopes: OAuthPermissionScope[]
    oidcScopes?: string[]
    requestDetails?: OAuthConsentDetail[]
    deciding?: boolean
    approveLabel?: string
    denyLabel?: string
  }>(),
  {
    requestDetails: () => [],
    oidcScopes: () => [],
    deciding: false,
    approveLabel: '允许',
    denyLabel: '拒绝',
  },
)

const emit = defineEmits<{
  approve: []
  deny: []
}>()

function oidcScopeDescription(scope: string) {
  const descriptions: Record<string, string> = {
    openid: '确认你在本站的身份，并向应用提供稳定的用户标识。',
    profile: '读取你的显示名称和首选语言。',
    email: '读取你的邮箱地址及其验证状态。',
    offline_access: '在你不使用应用时继续访问；本站会向应用签发可撤销的刷新令牌。',
  }
  return descriptions[scope] || scope
}
</script>
