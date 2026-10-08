<template>
  <UiDialog v-model="visible" title="重置用户密码" align-center>
    <el-form label-position="top">
      <el-form-item label="新密码">
        <el-input v-model="newPassword" type="password" placeholder="请输入新密码" show-password />
      </el-form-item>
      <el-form-item label="确认新密码">
        <el-input v-model="confirmPassword" type="password" show-password />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="resetting" @click="$emit('confirm')">确认重置</el-button>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
import UiDialog from '@/components/ui/UiDialog.vue'

const visible = defineModel<boolean>('visible', { required: true })
const newPassword = defineModel<string>('newPassword', { required: true })
const confirmPassword = defineModel<string>('confirmPassword', { required: true })

defineProps<{
  resetting: boolean
}>()

defineEmits<{
  confirm: []
}>()
</script>
