<template>
  <UiDialog
    v-model="visible"
    title="绑定正版角色"
    :close-on-click-modal="false"
    :destroy-on-close="true"
  >
    <div v-if="profile" class="grid gap-5 py-2">
      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="绑定与同步是两个独立操作"
        :description="`这里仅把本站角色 ${profile.name} 关联到 Microsoft 身份。绑定后请在角色卡片上点击“同步”，才会更新名称、皮肤和披风。`"
      />

      <el-form label-position="top">
        <el-form-item label="选择已绑定的 Microsoft 身份" required>
          <el-select
            v-model="selectedIdentityId"
            class="w-full"
            placeholder="请选择 Microsoft 身份"
          >
            <el-option
              v-for="identity in activeMicrosoftIdentities"
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

      <el-empty
        v-if="!activeMicrosoftIdentities.length"
        :description="
          microsoftIdentities.length
            ? 'Microsoft 身份需要重新登录后才能绑定正版角色'
            : '请先在身份管理页绑定 Microsoft 账号'
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
          :disabled="!profile || !selectedIdentityId"
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
import type { ExternalIdentity, Profile } from '@/api/types'

const visible = defineModel<boolean>('visible', { required: true })
const props = defineProps<{
  profile: Profile | null
  identities: ExternalIdentity[]
  loading: boolean
}>()
const emit = defineEmits<{
  confirm: [payload: { identity_id: string; profile_id: string }]
  'manage-identities': []
}>()

const selectedIdentityId = ref('')
const microsoftIdentities = computed(() =>
  props.identities.filter((identity) => identity.provider_adapter === 'microsoft'),
)
const activeMicrosoftIdentities = computed(() =>
  microsoftIdentities.value.filter((identity) => identity.authorization_status === 'active'),
)

function identityLabel(identity: ExternalIdentity) {
  return identity.label || identity.display_name || identity.email || identity.provider_name
}

function confirm() {
  if (!props.profile || !selectedIdentityId.value) return
  emit('confirm', { identity_id: selectedIdentityId.value, profile_id: props.profile.id })
}

watch(visible, (opened) => {
  if (opened) selectedIdentityId.value = activeMicrosoftIdentities.value[0]?.id || ''
})
</script>
