<template>
  <UiDialog v-model="visible" title="添加外部身份" :close-on-click-modal="false">
    <div class="grid gap-4">
      <p class="m-0 text-sm leading-6 text-[var(--color-text-light)]">
        选择身份提供方后会前往其登录页面。同一个提供方可以连接多个账号，已有身份不会被覆盖。
      </p>

      <div v-if="providers.length" class="grid gap-3">
        <div
          v-for="provider in providers"
          :key="provider.id"
          class="flex items-center gap-3 rounded-xl border border-[var(--color-border)] p-4"
        >
          <div
            class="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-[var(--color-background-soft)]"
          >
            <img
              v-if="provider.icon_url"
              :src="provider.icon_url"
              alt=""
              class="h-7 w-7 object-contain"
            />
            <el-icon v-else :size="22" class="text-[var(--el-color-primary)]">
              <Connection />
            </el-icon>
          </div>

          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-semibold text-[var(--color-heading)]">{{ provider.name }}</span>
              <el-tag v-if="identityCount(provider.id)" size="small" type="info">
                已连接 {{ identityCount(provider.id) }} 个
              </el-tag>
            </div>
            <div class="mt-1 text-xs leading-5 text-[var(--color-text-light)]">
              {{
                provider.adapter === 'microsoft'
                  ? '可用于第三方登录和正版角色能力'
                  : '可用于第三方登录及该提供方开放的能力'
              }}
            </div>
          </div>

          <el-button
            type="primary"
            :loading="authorizingProviderId === provider.id"
            :disabled="!!authorizingProviderId"
            @click="$emit('connect', provider.id)"
          >
            {{ identityCount(provider.id) ? '添加另一个' : '连接' }}
          </el-button>
        </div>
      </div>

      <el-empty v-else description="管理员尚未开放可连接的身份提供方" :image-size="72" />
    </div>

    <template #footer>
      <el-button :disabled="!!authorizingProviderId" @click="visible = false">关闭</el-button>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
import { Connection } from '@element-plus/icons-vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import type { ExternalIdentity, IdentityProvider } from '@/api/types'

const visible = defineModel<boolean>({ required: true })
const props = defineProps<{
  providers: IdentityProvider[]
  identities: ExternalIdentity[]
  authorizingProviderId: string
}>()

defineEmits<{
  connect: [providerId: string]
}>()

function identityCount(providerId: string) {
  return props.identities.filter((identity) => identity.provider_id === providerId).length
}
</script>
