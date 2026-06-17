<template>
  <el-dialog
    v-model="visible"
    title="绑定正版角色"
    class="dialog-form dialog-microsoft-login"
    :close-on-click-modal="false"
    :destroy-on-close="true"
    :before-close="beforeClose"
    append-to-body
  >
    <div class="py-3">
      <div v-if="profile" class="flex flex-col items-center text-center">
        <div class="selection-item is-checked cursor-default pointer-events-none">
          <div class="selection-info">
            <span class="title">{{ profile?.name }}</span>
            <span class="subtitle">{{ formatUUID(profile?.id || '') }}</span>
          </div>
          <div class="ml-auto">
            <el-tag v-if="profile?.has_game" type="success" effect="dark">拥有游戏</el-tag>
            <el-tag v-else type="danger" effect="dark">无游戏权限</el-tag>
          </div>
        </div>
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
  </el-dialog>
</template>

<script setup lang="ts">
import { formatUUID } from '@/utils/format'

const visible = defineModel<boolean>('visible', { required: true })

const props = defineProps<{
  profile: any | null
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
