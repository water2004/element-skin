<template>
  <UiDialog
    v-model="visible"
    title="绑定正版角色"
    class="dialog-microsoft-login"
    :close-on-click-modal="false"
    :destroy-on-close="true"
    :before-close="beforeClose"
  >
    <div class="py-3">
      <div v-if="profile" class="flex flex-col items-center text-center">
        <UiOptionCard class="is-checked cursor-default pointer-events-none">
          <div class="ui-option-card__info">
            <span class="ui-option-card__title">{{ profile?.name }}</span>
            <span class="ui-option-card__subtitle">{{ formatUUID(profile?.id || '') }}</span>
          </div>
          <div class="ml-auto">
            <el-tag v-if="profile?.has_game" type="success" effect="dark">拥有游戏</el-tag>
            <el-tag v-else type="danger" effect="dark">无游戏权限</el-tag>
          </div>
        </UiOptionCard>
      </div>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <el-button :disabled="importing" @click="$emit('cancel')">取消</el-button>
        <el-button
          type="primary"
          :loading="importing"
          :disabled="!profile?.has_game"
          @click="$emit('confirm')"
        >
          确认导入
        </el-button>
      </div>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
import type { MicrosoftGameProfile } from '@/api/types'
import { formatUUID } from '@/utils/format'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiOptionCard from '@/components/ui/UiOptionCard.vue'

const visible = defineModel<boolean>('visible', { required: true })

const props = defineProps<{
  profile: MicrosoftGameProfile | null
  importing: boolean
}>()

const emit = defineEmits<{
  cancel: []
  confirm: []
}>()

function beforeClose(done?: () => void) {
  if (props.importing) return
  emit('cancel')
  if (done) done()
}
</script>
