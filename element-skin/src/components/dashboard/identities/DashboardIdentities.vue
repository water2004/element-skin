<template>
  <div class="max-w-[1000px] mx-auto py-5 animate-fade-in">
    <PageHeader title="身份管理" subtitle="管理用于登录和外部能力授权的多个身份">
      <template #icon><Connection /></template>
    </PageHeader>

    <UiCard v-if="canCreateIdentity" class="mb-6">
      <template #header>
        <div>
          <div class="font-semibold text-[var(--color-heading)]">添加外部身份</div>
          <div class="text-xs text-[var(--color-text-light)] mt-1">
            同一个身份提供方可以绑定多个账号；授权时会让您选择具体账号。
          </div>
        </div>
      </template>
      <div v-if="linkProviders.length" class="flex flex-wrap gap-3">
        <el-button
          v-for="provider in linkProviders"
          :key="provider.id"
          :loading="authorizingProviderId === provider.id"
          :disabled="!!authorizingProviderId"
          @click="linkProvider(provider.id)"
        >
          <img
            v-if="provider.icon_url"
            :src="provider.icon_url"
            alt=""
            class="w-5 h-5 rounded-sm object-contain"
          />
          <el-icon v-else><Link /></el-icon>
          添加 {{ provider.name }} 身份
        </el-button>
      </div>
      <el-empty v-else description="管理员尚未开放可绑定的身份提供方" :image-size="64" />
    </UiCard>

    <div v-loading="loading" class="min-h-[240px]">
      <div v-if="identities.length" class="grid gap-4">
        <UiCard v-for="identity in identities" :key="identity.id" hoverable>
          <div class="flex flex-col md:flex-row md:items-start gap-4">
            <el-avatar :size="52" :src="identity.avatar_url || undefined">
              {{ identityInitial(identity) }}
            </el-avatar>
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-semibold text-lg text-[var(--color-heading)]">
                  {{ identity.label || identity.display_name || identity.provider_name }}
                </span>
                <el-tag size="small">{{ identity.provider_name }}</el-tag>
                <el-tag
                  v-if="identity.provider_adapter === 'microsoft'"
                  size="small"
                  type="success"
                >
                  Microsoft
                </el-tag>
              </div>
              <div class="mt-2 text-sm text-[var(--color-text-light)] break-all">
                <span v-if="identity.display_name">{{ identity.display_name }}</span>
                <span v-if="identity.email">
                  {{ identity.display_name ? ' · ' : '' }}{{ identity.email }}
                </span>
              </div>
              <div class="mt-1 text-xs text-[var(--color-text-light)] break-all">
                标识：{{ identity.subject }}
              </div>

              <div
                v-if="bindingsFor(identity.id).length"
                class="mt-4 rounded-xl border border-[var(--color-border)] p-3"
              >
                <div class="text-xs font-semibold text-[var(--color-text-light)] mb-2">
                  已绑定正版角色
                </div>
                <div class="grid gap-2">
                  <div
                    v-for="binding in bindingsFor(identity.id)"
                    :key="binding.id"
                    class="flex flex-wrap items-center justify-between gap-2"
                  >
                    <div>
                      <span class="font-medium text-[var(--color-heading)]">
                        {{ binding.profile.name }}
                      </span>
                      <span class="text-xs text-[var(--color-text-light)] ml-2">
                        远端 {{ binding.remote_name }}
                      </span>
                    </div>
                    <div class="flex gap-2">
                      <el-button
                        v-if="canRefreshOfficialProfile"
                        link
                        type="primary"
                        :loading="syncingBindingId === binding.id"
                        @click="syncBinding(binding)"
                      >
                        同步
                      </el-button>
                      <el-button
                        v-if="canDeleteOfficialProfile"
                        link
                        type="danger"
                        @click="removeBinding(binding)"
                      >
                        解除绑定
                      </el-button>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="flex flex-wrap md:justify-end gap-2">
              <el-button
                v-if="canCreateIdentity && providerCanLink(identity.provider_id)"
                @click="linkProvider(identity.provider_id)"
              >
                重新授权
              </el-button>
              <el-button v-if="canUpdateIdentity" @click="renameIdentity(identity)">
                修改标签
              </el-button>
              <el-button
                v-if="canDeleteIdentity"
                type="danger"
                plain
                @click="removeIdentity(identity)"
              >
                删除身份
              </el-button>
            </div>
          </div>
        </UiCard>
      </div>
      <el-empty v-else-if="!loading" description="尚未绑定外部身份" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onMounted, ref, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Connection, Link } from '@element-plus/icons-vue'
import PageHeader from '@/components/common/PageHeader.vue'
import UiCard from '@/components/ui/UiCard.vue'
import {
  deleteExternalIdentity,
  getExternalIdentities,
  getIdentityProviders,
  patchExternalIdentity,
  startIdentityAuthorization,
} from '@/api/identity'
import {
  deleteOfficialProfileBinding,
  getOfficialProfileBindings,
  syncOfficialProfileBinding,
} from '@/api/official-profiles'
import type { ExternalIdentity, IdentityProvider, OfficialProfileBinding, User } from '@/api/types'
import { getErrorMessage } from '@/utils/error'

const user = inject<Ref<User | null>>('user', ref(null))
const route = useRoute()
const router = useRouter()
const loading = ref(false)
const authorizingProviderId = ref('')
const syncingBindingId = ref('')
const providers = ref<IdentityProvider[]>([])
const identities = ref<ExternalIdentity[]>([])
const bindings = ref<OfficialProfileBinding[]>([])

const permissions = computed(() => new Set(user.value?.permissions || []))
const canReadIdentity = computed(() => permissions.value.has('external_identity.read.owned'))
const canReadOfficialProfile = computed(() => permissions.value.has('official_profile.read.owned'))
const canCreateIdentity = computed(() => permissions.value.has('external_identity.create.owned'))
const canUpdateIdentity = computed(() => permissions.value.has('external_identity.update.owned'))
const canDeleteIdentity = computed(() => permissions.value.has('external_identity.delete.owned'))
const canRefreshOfficialProfile = computed(() =>
  permissions.value.has('official_profile.refresh.owned'),
)
const canDeleteOfficialProfile = computed(() =>
  permissions.value.has('official_profile.delete.owned'),
)
const linkProviders = computed(() => providers.value.filter((provider) => provider.link_enabled))

function bindingsFor(identityId: string) {
  return bindings.value.filter((binding) => binding.identity_id === identityId)
}

function providerCanLink(providerId: string) {
  return linkProviders.value.some((provider) => provider.id === providerId)
}

function identityInitial(identity: ExternalIdentity) {
  return (identity.label || identity.display_name || identity.provider_name || '?')
    .charAt(0)
    .toUpperCase()
}

async function loadIdentityState() {
  loading.value = true
  try {
    const [providerResponse, identityResponse, bindingResponse] = await Promise.all([
      getIdentityProviders(),
      canReadIdentity.value ? getExternalIdentities() : Promise.resolve(null),
      canReadOfficialProfile.value ? getOfficialProfileBindings() : Promise.resolve(null),
    ])
    providers.value = providerResponse.data.items
    identities.value = identityResponse?.data.items || []
    bindings.value = bindingResponse?.data.items || []
  } catch (e: unknown) {
    ElMessage.error('加载身份失败: ' + getErrorMessage(e, '加载失败'))
  } finally {
    loading.value = false
  }
}

async function linkProvider(providerId: string) {
  try {
    authorizingProviderId.value = providerId
    const response = await startIdentityAuthorization({ provider_id: providerId, intent: 'link' })
    window.location.assign(response.data.authorization_url)
  } catch (e: unknown) {
    authorizingProviderId.value = ''
    ElMessage.error('授权失败: ' + getErrorMessage(e, '无法开始授权'))
  }
}

async function renameIdentity(identity: ExternalIdentity) {
  try {
    const result = await ElMessageBox.prompt('设置便于区分多个账号的标签', '修改身份标签', {
      inputValue: identity.label,
      inputPlaceholder: identity.display_name || identity.email || identity.provider_name,
      inputValidator: (value) =>
        [...String(value || '')].length <= 80 || '标签长度不能超过 80 个字符',
    })
    await patchExternalIdentity(identity.id, { label: result.value.trim() })
    ElMessage.success('身份标签已更新')
    await loadIdentityState()
  } catch (e: unknown) {
    if (e !== 'cancel' && e !== 'close') {
      ElMessage.error('更新失败: ' + getErrorMessage(e, '更新失败'))
    }
  }
}

async function removeIdentity(identity: ExternalIdentity) {
  try {
    await ElMessageBox.confirm(
      '删除身份不会删除本站账号或角色；若它仍绑定正版角色，需要先解除绑定。',
      '删除外部身份',
      { type: 'warning', confirmButtonText: '删除身份', cancelButtonText: '取消' },
    )
    await deleteExternalIdentity(identity.id)
    ElMessage.success('身份已删除')
    await loadIdentityState()
  } catch (e: unknown) {
    if (e !== 'cancel' && e !== 'close') {
      ElMessage.error('删除失败: ' + getErrorMessage(e, '删除失败'))
    }
  }
}

async function syncBinding(binding: OfficialProfileBinding) {
  try {
    syncingBindingId.value = binding.id
    await syncOfficialProfileBinding(binding.id)
    ElMessage.success('正版角色已同步到本站角色')
    await loadIdentityState()
  } catch (e: unknown) {
    ElMessage.error('同步失败: ' + getErrorMessage(e, '同步失败'))
  } finally {
    syncingBindingId.value = ''
  }
}

async function removeBinding(binding: OfficialProfileBinding) {
  try {
    await ElMessageBox.confirm(
      `解除 ${binding.profile.name} 与 ${binding.remote_name} 的绑定？本站角色和材质不会被删除。`,
      '解除正版绑定',
      { type: 'warning', confirmButtonText: '解除绑定', cancelButtonText: '取消' },
    )
    await deleteOfficialProfileBinding(binding.id)
    ElMessage.success('已解除正版绑定')
    await loadIdentityState()
  } catch (e: unknown) {
    if (e !== 'cancel' && e !== 'close') {
      ElMessage.error('解除失败: ' + getErrorMessage(e, '解除失败'))
    }
  }
}

onMounted(async () => {
  if (typeof route.query.linked_identity_id === 'string') {
    ElMessage.success('外部身份已绑定或重新授权')
    await router.replace({ query: {} })
  }
  await loadIdentityState()
})
</script>
