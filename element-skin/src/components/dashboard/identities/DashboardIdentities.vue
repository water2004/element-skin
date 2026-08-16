<template>
  <div class="identity-section animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <div>
          <h1>身份管理</h1>
          <p>连接并管理用于第三方登录和外部能力的账号</p>
        </div>
      </div>
      <UiButton
        v-if="canCreateIdentity"
        size="large"
        variant="gradient-primary"
        :disabled="!linkProviders.length"
        @click="showAddIdentityDialog = true"
      >
        <el-icon><Plus /></el-icon>
        <span class="ml-2">添加身份</span>
      </UiButton>
    </div>

    <div v-loading="loading" class="min-h-[280px]">
      <div v-if="identityGroups.length" class="grid gap-5">
        <div
          class="flex flex-wrap items-center gap-x-5 gap-y-2 text-sm text-[var(--color-text-light)]"
        >
          <span>已连接 {{ identities.length }} 个身份</span>
          <span>来自 {{ identityGroups.length }} 个提供方</span>
          <span v-if="reauthorizationCount" class="text-[var(--el-color-danger)]">
            {{ reauthorizationCount }} 个需要重新连接
          </span>
        </div>

        <IdentityProviderSection
          v-for="group in identityGroups"
          :key="group.id"
          :group="group"
          :bindings="bindings"
          :can-add="canCreateIdentity"
          :can-update="canUpdateIdentity"
          :can-delete="canDeleteIdentity"
          :authorizing-provider-id="authorizingProviderId"
          :reconnecting-identity-id="reconnectingIdentityId"
          @add="connectProvider"
          @reconnect="reconnectIdentity"
          @rename="renameIdentity"
          @remove="removeIdentity"
          @manage-roles="goToRoles"
        />
      </div>

      <el-empty v-else-if="!loading" description="尚未连接外部身份">
        <el-button
          v-if="canCreateIdentity && linkProviders.length"
          type="primary"
          @click="showAddIdentityDialog = true"
        >
          添加第一个身份
        </el-button>
      </el-empty>
    </div>

    <AddIdentityDialog
      v-model="showAddIdentityDialog"
      :providers="linkProviders"
      :identities="identities"
      :authorizing-provider-id="authorizingProviderId"
      @connect="connectProvider"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onMounted, ref, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import UiButton from '@/components/ui/UiButton.vue'
import AddIdentityDialog from './AddIdentityDialog.vue'
import IdentityProviderSection from './IdentityProviderSection.vue'
import {
  deleteExternalIdentity,
  getExternalIdentities,
  getIdentityProviders,
  patchExternalIdentity,
  startIdentityAuthorization,
} from '@/api/identity'
import { getOfficialProfileBindings } from '@/api/official-profiles'
import type { ExternalIdentity, IdentityProvider, OfficialProfileBinding, User } from '@/api/types'
import type { IdentityProviderGroup } from './viewTypes'
import { getErrorMessage } from '@/utils/error'

const user = inject<Ref<User | null>>('user', ref(null))
const route = useRoute()
const router = useRouter()
const loading = ref(false)
const showAddIdentityDialog = ref(false)
const authorizingProviderId = ref('')
const reconnectingIdentityId = ref('')
const providers = ref<IdentityProvider[]>([])
const identities = ref<ExternalIdentity[]>([])
const bindings = ref<OfficialProfileBinding[]>([])

const permissions = computed(() => new Set(user.value?.permissions || []))
const canReadIdentity = computed(() => permissions.value.has('external_identity.read.owned'))
const canReadOfficialProfile = computed(() => permissions.value.has('official_profile.read.owned'))
const canCreateIdentity = computed(() => permissions.value.has('external_identity.create.owned'))
const canUpdateIdentity = computed(() => permissions.value.has('external_identity.update.owned'))
const canDeleteIdentity = computed(() => permissions.value.has('external_identity.delete.owned'))
const linkProviders = computed(() => providers.value.filter((provider) => provider.link_enabled))
const reauthorizationCount = computed(
  () =>
    identities.value.filter(
      (identity) => identity.authorization_status === 'reauthorization_required',
    ).length,
)

const identityGroups = computed<IdentityProviderGroup[]>(() => {
  const grouped = new Map<string, IdentityProviderGroup>()
  for (const identity of identities.value) {
    const current = grouped.get(identity.provider_id)
    if (current) {
      current.identities.push(identity)
      continue
    }
    grouped.set(identity.provider_id, {
      id: identity.provider_id,
      name: identity.provider_name,
      adapter: identity.provider_adapter,
      icon_url: identity.provider_icon_url,
      enabled: identity.provider_enabled,
      link_enabled: identity.provider_link_enabled,
      identities: [identity],
    })
  }
  const order = new Map(providers.value.map((provider, index) => [provider.id, index]))
  return [...grouped.values()].sort((left, right) => {
    const leftOrder = order.get(left.id) ?? Number.MAX_SAFE_INTEGER
    const rightOrder = order.get(right.id) ?? Number.MAX_SAFE_INTEGER
    return leftOrder - rightOrder || left.name.localeCompare(right.name)
  })
})

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
  } catch (error: unknown) {
    ElMessage.error('加载身份失败: ' + getErrorMessage(error, '加载失败'))
  } finally {
    loading.value = false
  }
}

async function connectProvider(providerId: string) {
  try {
    authorizingProviderId.value = providerId
    const response = await startIdentityAuthorization({ provider_id: providerId, intent: 'link' })
    window.location.assign(response.data.authorization_url)
  } catch (error: unknown) {
    authorizingProviderId.value = ''
    ElMessage.error('无法连接身份: ' + getErrorMessage(error, '无法开始授权'))
  }
}

async function reconnectIdentity(identity: ExternalIdentity) {
  try {
    reconnectingIdentityId.value = identity.id
    const response = await startIdentityAuthorization({
      provider_id: identity.provider_id,
      intent: 'link',
      identity_id: identity.id,
    })
    window.location.assign(response.data.authorization_url)
  } catch (error: unknown) {
    reconnectingIdentityId.value = ''
    ElMessage.error('无法重新连接: ' + getErrorMessage(error, '无法开始授权'))
  }
}

async function renameIdentity(identity: ExternalIdentity) {
  try {
    const result = await ElMessageBox.prompt('设置一个只在本站显示的标签', '修改身份标签', {
      inputValue: identity.label,
      inputPlaceholder: identity.display_name || identity.email || identity.provider_name,
      inputValidator: (value) =>
        [...String(value || '')].length <= 80 || '标签长度不能超过 80 个字符',
    })
    await patchExternalIdentity(identity.id, { label: result.value.trim() })
    ElMessage.success('身份标签已更新')
    await loadIdentityState()
  } catch (error: unknown) {
    if (error !== 'cancel' && error !== 'close') {
      ElMessage.error('更新失败: ' + getErrorMessage(error, '更新失败'))
    }
  }
}

async function removeIdentity(identity: ExternalIdentity) {
  try {
    await ElMessageBox.confirm(
      '删除身份会同时解除它关联的正版角色关系，但不会删除任何本站角色。此操作不可撤销。',
      '删除外部身份',
      { type: 'warning', confirmButtonText: '删除身份', cancelButtonText: '取消' },
    )
    await deleteExternalIdentity(identity.id)
    ElMessage.success('身份已删除')
    await loadIdentityState()
  } catch (error: unknown) {
    if (error !== 'cancel' && error !== 'close') {
      ElMessage.error('删除失败: ' + getErrorMessage(error, '删除失败'))
    }
  }
}

function goToRoles() {
  void router.push('/dashboard/roles')
}

onMounted(async () => {
  const identityError = route.query.identity_error
  if (identityError === 'account_mismatch') {
    ElMessage.error('重新连接失败：请选择这个身份原来对应的外部账号')
  } else if (identityError === 'already_linked') {
    ElMessage.error('该外部身份已经绑定，不能重复绑定')
  } else if (identityError === 'authorization_incomplete') {
    ElMessage.info('身份连接未完成，原有身份没有改变')
  } else if (typeof route.query.linked_identity_id === 'string') {
    ElMessage.success('身份连接已更新')
  }
  if (typeof identityError === 'string' || typeof route.query.linked_identity_id === 'string') {
    const query = { ...route.query }
    delete query.identity_error
    delete query.linked_identity_id
    await router.replace({ query })
  }
  await loadIdentityState()
})
</script>
