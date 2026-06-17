<template>
  <el-dialog
    v-model="visible"
    title="从外部皮肤站导入角色"
    class="dialog-form dialog-ygg-import"
    append-to-body
    :before-close="beforeClose"
  >
    <div v-if="step === 'input'">
      <el-form label-position="top">
        <el-form-item label="Yggdrasil API 地址">
          <el-input v-model="apiUrl" placeholder="https://skin.example.com/api/yggdrasil" />
          <div class="form-tip">通常以 /api/yggdrasil 结尾</div>
        </el-form-item>
        <el-form-item label="用户名/邮箱">
          <el-input v-model="username" placeholder="外部皮肤站的登录用户名" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input
            v-model="password"
            type="password"
            show-password
            placeholder="外部皮肤站的登录密码"
          />
        </el-form-item>
      </el-form>
    </div>

    <div v-else-if="step === 'select'">
      <p class="mb-4">请选择要导入的角色：</p>
      <el-checkbox-group v-model="selectedProfiles" class="selection-list">
        <el-checkbox
          v-for="profile in profiles"
          :key="profile.id"
          :value="profile.id"
          border
          class="selection-item"
        >
          <div class="selection-info">
            <span class="title">{{ profile.name }}</span>
            <span class="subtitle">{{ formatUUID(profile.id) }}</span>
          </div>
        </el-checkbox>
      </el-checkbox-group>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <el-button :disabled="loading" @click="$emit('cancel')">取消</el-button>
        <el-button v-if="step === 'input'" type="primary" :loading="loading" @click="$emit('next')">
          下一步
        </el-button>
        <el-button
          v-else
          type="primary"
          :loading="loading"
          :disabled="selectedProfiles.length === 0"
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
const apiUrl = defineModel<string>('apiUrl', { required: true })
const username = defineModel<string>('username', { required: true })
const password = defineModel<string>('password', { required: true })
const selectedProfiles = defineModel<string[]>('selectedProfiles', { required: true })

const props = defineProps<{
  step: 'input' | 'select'
  profiles: Array<{ id: string; name: string }>
  loading: boolean
}>()

const emit = defineEmits<{
  cancel: []
  next: []
  confirm: []
}>()

function beforeClose(done?: () => void) {
  if (props.loading) return
  emit('cancel')
  if (done) done()
}
</script>
