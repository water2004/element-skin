<template>
  <UiDialog
    v-model="visible"
    title="绑定正版角色"
    :close-on-click-modal="false"
    :destroy-on-close="true"
  >
    <div class="grid gap-5 py-2">
      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="系统将按正版 UUID 绑定角色"
        description="如果该 UUID 已属于您，将直接建立绑定；如果不存在，将自动创建角色；如果已属于其他用户，则会拒绝绑定。"
      />

      <el-form v-if="availableMicrosoftIdentities.length" label-position="top">
        <el-form-item label="选择 Microsoft 账户" required>
          <el-select
            v-model="selectedIdentityId"
            class="w-full"
            placeholder="请选择 Microsoft 账户"
          >
            <el-option
              v-for="identity in availableMicrosoftIdentities"
              :key="identity.id"
              :value="identity.id"
              :label="identityLabel(identity)"
            >
              <div class="flex justify-between gap-3">
                <span>{{ identityLabel(identity) }}</span>
                <span class="text-xs text-[var(--color-text-light)]">{{ identity.email }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
      </el-form>

      <div
        v-if="availableMicrosoftIdentities.length && attentionMicrosoftIdentities.length"
        class="flex flex-col gap-2 rounded-xl border border-[var(--el-color-warning-light-5)] bg-[var(--el-color-warning-light-9)] p-3 sm:flex-row sm:items-center sm:justify-between"
      >
        <div class="text-sm text-[var(--el-color-warning-dark-2)]">
          {{ attentionMicrosoftIdentities.length }} 个 Microsoft
          身份当前不可用，需要重新连接或由管理员恢复提供方。
        </div>
        <el-button link type="primary" @click="$emit('manage-identities')"> 管理身份 </el-button>
      </div>

      <el-empty
        v-if="!availableMicrosoftIdentities.length"
        :description="
          !microsoftIdentities.length
            ? '请先在身份管理页绑定 Microsoft 账号'
            : usableMicrosoftIdentities.length
              ? '所有可用的 Microsoft 账户都已绑定正版角色'
              : '当前没有可用于正版绑定的 Microsoft 账户'
        "
        :image-size="72"
      >
        <el-button type="primary" @click="$emit('manage-identities')">前往身份管理</el-button>
      </el-empty>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <el-button :disabled="loading" @click="visible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="loading"
          :disabled="!selectedIdentityId"
          @click="confirm"
        >
          建立绑定
        </el-button>
      </div>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import type { ExternalIdentity, OfficialProfileBinding } from '@/api/types'

const visible = defineModel<boolean>('visible', { required: true })
const props = defineProps<{
  identities: ExternalIdentity[]
  bindings: OfficialProfileBinding[]
  loading: boolean
}>()
const emit = defineEmits<{
  confirm: [payload: { identity_id: string }]
  'manage-identities': []
}>()

const selectedIdentityId = ref('')
const microsoftIdentities = computed(() =>
  props.identities.filter((identity) => identity.provider_adapter === 'microsoft'),
)
const usableMicrosoftIdentities = computed(() =>
  microsoftIdentities.value.filter(
    (identity) => identity.authorization_status === 'active' && identity.provider_enabled,
  ),
)
const boundIdentityIds = computed(
  () => new Set(props.bindings.map((binding) => binding.identity_id)),
)
const availableMicrosoftIdentities = computed(() =>
  usableMicrosoftIdentities.value.filter((identity) => !boundIdentityIds.value.has(identity.id)),
)
const attentionMicrosoftIdentities = computed(() =>
  microsoftIdentities.value.filter(
    (identity) => identity.authorization_status !== 'active' || !identity.provider_enabled,
  ),
)

function identityLabel(identity: ExternalIdentity) {
  return identity.label || identity.display_name || identity.email || identity.provider_name
}

function confirm() {
  if (!selectedIdentityId.value) return
  emit('confirm', { identity_id: selectedIdentityId.value })
}

watch(
  visible,
  (opened) => {
    if (opened) selectedIdentityId.value = availableMicrosoftIdentities.value[0]?.id || ''
  },
  { immediate: true },
)
</script>
