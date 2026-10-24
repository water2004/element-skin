<template>
  <UiDialog
    v-model="visible"
    title="重置用户密码"
    align-center
    :busy="resetting"
    :dirty="!!newPassword || !!confirmPassword"
    @closed="clearPasswords"
  >
    <el-form label-position="top" :disabled="resetting">
      <el-form-item label="新密码 (最少6位)">
        <el-input v-model="newPassword" type="password" show-password />
      </el-form-item>
      <el-form-item label="确认新密码">
        <el-input v-model="confirmPassword" type="password" show-password />
      </el-form-item>
    </el-form>
    <template #footer="{ requestClose }">
      <el-button :disabled="resetting" @click="requestClose">取消</el-button>
      <el-button type="primary" :loading="resetting" @click="$emit('confirm')">确认重置</el-button>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
import UiDialog from '@/components/ui/UiDialog.vue'
import { onBeforeUnmount } from 'vue'

const visible = defineModel<boolean>('visible', { required: true })
const newPassword = defineModel<string>('newPassword', { required: true })
const confirmPassword = defineModel<string>('confirmPassword', { required: true })

defineProps<{
  resetting: boolean
}>()

defineEmits<{
  confirm: []
}>()

function clearPasswords() {
  newPassword.value = ''
  confirmPassword.value = ''
}
onBeforeUnmount(clearPasswords)
</script>
