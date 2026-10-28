<template>
  <div class="flex-1 px-4 py-10">
    <UiCard class="mx-auto max-w-3xl [--el-card-padding:1rem] sm:[--el-card-padding:2rem]">
      <PageHeader
        title="设备授权"
        subtitle="输入设备上显示的用户代码，确认第三方应用可访问的站点能力。"
      />

      <div class="flex gap-3">
        <el-input
          v-model="userCode"
          maxlength="9"
          placeholder="ABCD-1234"
          @keyup.enter="loadDetails"
        />
        <el-button type="primary" :loading="loading" @click="loadDetails">查询</el-button>
      </div>

      <el-alert
        v-if="message"
        class="mt-5"
        :type="messageType"
        :closable="false"
        :title="message"
      />

      <OAuthConsentPanel
        v-if="details"
        class="mt-6"
        :client="details.client"
        :scopes="details.scopes"
        :request-details="deviceDetails"
        :deciding="deciding"
        approve-label="批准"
        @approve="decide(true)"
        @deny="decide(false)"
      />
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import PageHeader from '@/components/common/PageHeader.vue'
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  decideDeviceAuthorization,
  getDeviceAuthorization,
  type DeviceAuthorizationDetails,
} from '@/api/oauth'
import OAuthConsentPanel from '@/components/oauth/OAuthConsentPanel.vue'
import UiCard from '@/components/ui/UiCard.vue'
import { getErrorMessage } from '@/utils/error'

interface OAuthConsentDetail {
  label: string
  value: string
}

const route = useRoute()
const userCode = ref(String(route.query.user_code || ''))
const details = ref<DeviceAuthorizationDetails | null>(null)
const loading = ref(false)
const deciding = ref(false)
const message = ref('')
const messageType = ref<'success' | 'warning' | 'info' | 'error'>('info')

const normalizedUserCode = computed(() => userCode.value.trim().toUpperCase())
const deviceDetails = computed<OAuthConsentDetail[]>(() => {
  if (!details.value) return []
  return [
    { label: '设备代码', value: normalizedUserCode.value },
    { label: '授权状态', value: details.value.status },
    { label: '过期时间', value: formatTime(details.value.expires_at) },
  ]
})

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
