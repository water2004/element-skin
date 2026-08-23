<template>
  <UiCard>
    <template #header>
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex min-w-0 items-center gap-3">
          <div class="flex h-10 w-10 shrink-0 items-center justify-center">
            <img
              v-if="group.icon_url"
              :src="group.icon_url"
              alt=""
              class="h-8 w-8 object-contain"
            />
            <el-icon v-else :size="20" class="text-[var(--el-color-primary)]">
              <Connection />
            </el-icon>
          </div>
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h3 class="m-0 truncate text-base font-semibold text-[var(--color-heading)]">
                {{ group.name }}
              </h3>
              <el-tag size="small" type="info">{{ group.identities.length }} 个身份</el-tag>
              <el-tag v-if="group.adapter === 'microsoft'" size="small" type="success">
                正版能力
              </el-tag>
            </div>
            <p class="mt-1 mb-0 text-xs text-[var(--color-text-light)]">
              每个身份都是独立的外部账号连接
            </p>
          </div>
        </div>

        <ActionBar>
          <el-button
            v-if="canAdd && group.enabled && group.link_enabled"
            :loading="authorizingProviderId === group.id"
            :disabled="!!authorizingProviderId || !!reconnectingIdentityId"
            @click="$emit('add', group.id)"
          >
            <el-icon><Plus /></el-icon>
            添加另一个
          </el-button>
        </ActionBar>
      </div>
    </template>

    <div class="divide-y divide-[var(--color-border)]">
      <article
        v-for="identity in group.identities"
        :key="identity.id"
        class="py-5 first:pt-0 last:pb-0"
      >
        <div class="flex flex-col gap-4 lg:flex-row lg:items-start">
          <div class="flex min-w-0 flex-1 gap-3">
            <el-avatar :size="48" :src="identity.avatar_url || undefined" class="shrink-0">
              {{ identityInitial(identity) }}
            </el-avatar>

            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-semibold text-[var(--color-heading)]">
                  {{ identityName(identity) }}
                </span>
                <el-tag v-if="!group.enabled" size="small" type="warning"> 提供方已停用 </el-tag>
                <el-tag
                  v-else-if="identity.authorization_status === 'reauthorization_required'"
                  size="small"
                  type="danger"
                >
                  需要重新连接
                </el-tag>
                <el-tag v-else size="small" type="success">连接正常</el-tag>
              </div>

              <div class="mt-1 truncate text-sm text-[var(--color-text-light)]">
                {{ identityDescription(identity) }}
              </div>
              <div
                class="mt-1 truncate text-xs text-[var(--color-text-light)]"
                :title="identity.subject"
              >
                账号标识：{{ identity.subject }}
              </div>

              <div
                v-if="!group.enabled"
                class="mt-3 rounded-lg border border-[var(--el-color-warning-light-5)] bg-[var(--el-color-warning-light-9)] px-3 py-2 text-sm text-[var(--el-color-warning-dark-2)]"
              >
                管理员已停用此身份提供方，当前无法使用它的外部能力或重新连接。
              </div>
              <div
                v-else-if="identity.authorization_status === 'reauthorization_required'"
                class="mt-3 rounded-lg border border-[var(--el-color-danger-light-5)] bg-[var(--el-color-danger-light-9)] px-3 py-2 text-sm text-[var(--el-color-danger-dark-2)]"
              >
                <span v-if="group.link_enabled">
                  长期授权已失效。请重新连接这个账号，已有身份和正版角色关系都会保留。
                </span>
                <span v-else> 长期授权已失效，但管理员当前未开放重新连接，请联系管理员。 </span>
              </div>

              <div
                v-if="group.adapter === 'microsoft'"
                class="mt-3 flex flex-wrap items-center gap-x-3 gap-y-2 text-sm"
              >
                <span
                  class="inline-flex items-center gap-1.5 font-medium text-[var(--color-heading)]"
                >
                  <el-icon class="text-[var(--el-color-success)]"><LinkIcon /></el-icon>
                  正版角色
                </span>
                <div v-if="bindingsFor(identity.id).length" class="flex min-w-0 flex-wrap gap-2">
                  <span
                    v-for="binding in bindingsFor(identity.id)"
                    :key="binding.id"
                    class="inline-flex min-w-0 items-center gap-1.5 rounded-lg border border-[var(--color-border)] bg-[var(--color-card-background)] px-2.5 py-1 text-xs text-[var(--color-heading)]"
                  >
                    <span class="max-w-36 truncate">{{ binding.profile.name }}</span>
                    <template v-if="binding.profile.name !== binding.remote_name">
                      <el-icon class="shrink-0 text-[var(--color-text-light)]"
                        ><ArrowRight
                      /></el-icon>
                      <span class="max-w-36 truncate text-[var(--color-text-light)]">
                        {{ binding.remote_name }}
                      </span>
                    </template>
                  </span>
                </div>
                <span v-else class="text-[var(--color-text-light)]">尚未绑定</span>
                <el-button link type="primary" size="small" @click="$emit('manage-roles')">
                  {{ bindingsFor(identity.id).length ? '管理角色' : '绑定角色' }}
                  <el-icon class="ml-1"><ArrowRight /></el-icon>
                </el-button>
              </div>
            </div>
          </div>

          <ActionBar class="shrink-0">
            <el-button
              v-if="
                canAdd &&
                group.enabled &&
                group.link_enabled &&
                identity.authorization_status === 'reauthorization_required'
              "
              type="primary"
              :loading="reconnectingIdentityId === identity.id"
              :disabled="!!authorizingProviderId || !!reconnectingIdentityId"
              @click="$emit('reconnect', identity)"
            >
              重新连接
            </el-button>
            <el-button v-if="canUpdate" @click="$emit('rename', identity)"> 修改标签 </el-button>
            <el-button v-if="canDelete" type="danger" plain @click="$emit('remove', identity)">
              删除
            </el-button>
          </ActionBar>
        </div>
      </article>
    </div>
  </UiCard>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ArrowRight, Connection, Link as LinkIcon, Plus } from '@element-plus/icons-vue'
import ActionBar from '@/components/common/ActionBar.vue'
import UiCard from '@/components/ui/UiCard.vue'
import type { ExternalIdentity, OfficialProfileBinding } from '@/api/types'
import type { IdentityProviderGroup } from './viewTypes'

const props = defineProps<{
  group: IdentityProviderGroup
  bindings: OfficialProfileBinding[]
  canAdd: boolean
  canUpdate: boolean
  canDelete: boolean
  authorizingProviderId: string
  reconnectingIdentityId: string
}>()

defineEmits<{
  add: [providerId: string]
  reconnect: [identity: ExternalIdentity]
  rename: [identity: ExternalIdentity]
  remove: [identity: ExternalIdentity]
  'manage-roles': []
}>()

const bindingsByIdentity = computed(() => {
  const grouped = new Map<string, OfficialProfileBinding[]>()
  for (const binding of props.bindings) {
    const items = grouped.get(binding.identity_id)
    if (items) items.push(binding)
    else grouped.set(binding.identity_id, [binding])
  }
  return grouped
})

function bindingsFor(identityId: string) {
  return bindingsByIdentity.value.get(identityId) ?? []
}

function identityName(identity: ExternalIdentity) {
  return identity.label || identity.display_name || identity.email || '未命名身份'
}

function identityDescription(identity: ExternalIdentity) {
  const values = [identity.display_name, identity.email].filter(
    (value, index, items) => value && items.indexOf(value) === index,
  )
  return values.join(' · ') || '此提供方未返回显示名称或邮箱'
}

function identityInitial(identity: ExternalIdentity) {
  return identityName(identity).charAt(0).toUpperCase()
}
</script>
