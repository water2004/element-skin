<template>
  <div class="min-h-[calc(100vh-160px)] px-4 py-10">
    <UiCard class="mx-auto max-w-2xl p-8">
      <div class="mb-6">
        <h1 class="m-0 text-2xl font-semibold text-[var(--color-heading)]">设备授权</h1>
        <p class="mt-2 mb-0 text-sm text-[var(--color-text-light)]">
          输入设备上显示的用户代码，确认第三方应用可访问的站点能力。
        </p>
      </div>

      <div class="flex gap-3">
        <el-input v-model="userCode" maxlength="9" placeholder="ABCD-1234" @keyup.enter="loadDetails" />
        <el-button type="primary" :loading="loading" @click="loadDetails">查询</el-button>
      </div>

      <el-alert
        v-if="message"
        class="mt-5"
        :type="messageType"
        :closable="false"
        :title="message"
      />

      <div v-if="details" class="mt-6 space-y-5">
        <div class="rounded-lg border border-[var(--color-border)] p-5">
          <div class="flex items-start justify-between gap-4">
            <div>
              <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">
                {{ details.client.name }}
              </h2>
              <p class="mt-1 mb-0 break-all text-xs text-[var(--color-text-light)]">
                {{ details.client.client_id }}
              </p>
            </div>
            <el-tag>{{ details.status }}</el-tag>
          </div>
          <div class="mt-4 flex flex-wrap gap-2">
            <PermissionToneTag
              v-for="scope in details.scopes"
              :key="scope.code"
              :label="scope.code"
              tone="sky"
              :title="scope.description"
            />
          </div>
          <p class="mt-4 mb-0 text-xs text-[var(--color-text-light)]">
            过期时间：{{ formatTime(details.expires_at) }}
          </p>
        </div>

        <div class="flex justify-end gap-3">
          <el-button :loading="deciding" @click="decide(false)">拒绝</el-button>
          <el-button type="primary" :loading="deciding" @click="decide(true)">批准</el-button>
        </div>
      </div>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  decideDeviceAuthorization,
  getDeviceAuthorization,
  type DeviceAuthorizationDetails,
} from '@/api/oauth'
import UiCard from '@/components/ui/UiCard.vue'
import PermissionToneTag from '@/components/admin/users/PermissionToneTag.vue'
import { getErrorMessage } from '@/utils/error'

const route = useRoute()
const userCode = ref(String(route.query.user_code || ''))
const details = ref<DeviceAuthorizationDetails | null>(null)
const loading = ref(false)
const deciding = ref(false)
const message = ref('')
const messageType = ref<'success' | 'warning' | 'info' | 'error'>('info')

const normalizedUserCode = computed(() => userCode.value.trim().toUpperCase())

onMounted(() => {
  if (normalizedUserCode.value) loadDetails()
})

async function loadDetails() {
  if (!normalizedUserCode.value) {
    ElMessage.warning('请输入用户代码')
    return
  }
  loading.value = true
  message.value = ''
  try {
    const res = await getDeviceAuthorization(normalizedUserCode.value)
    details.value = res.data
  } catch (error) {
    details.value = null
    messageType.value = 'error'
    message.value = getErrorMessage(error, '设备授权不存在或已过期')
  } finally {
    loading.value = false
  }
}

async function decide(approve: boolean) {
  if (!details.value) return
  deciding.value = true
  try {
    await decideDeviceAuthorization(normalizedUserCode.value, approve)
    messageType.value = approve ? 'success' : 'warning'
    message.value = approve ? '已批准授权，可返回设备继续登录' : '已拒绝授权'
    details.value = null
  } catch (error) {
    messageType.value = 'error'
    message.value = getErrorMessage(error, '提交授权结果失败')
  } finally {
    deciding.value = false
  }
}

function formatTime(value: number) {
  return new Date(value).toLocaleString('zh-CN')
}
</script>
