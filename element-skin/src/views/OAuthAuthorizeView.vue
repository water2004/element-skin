<template>
  <div class="min-h-[calc(100vh-160px)] px-4 py-10">
    <UiCard class="mx-auto max-w-3xl p-8">
      <div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="m-0 text-2xl font-semibold text-[var(--color-heading)]">授权第三方应用</h1>
          <p class="mt-2 mb-0 text-sm text-[var(--color-text-light)]">
            请确认该应用请求的站点能力，授权后应用可代表你调用对应接口。
          </p>
        </div>
        <el-button :loading="loading" @click="loadDetails">刷新</el-button>
      </div>

      <el-alert
        v-if="message"
        class="mb-5"
        :type="messageType"
        :closable="false"
        :title="message"
      />

      <div
        v-if="loading && !details"
        class="py-12 text-center text-sm text-[var(--color-text-light)]"
      >
        正在读取授权请求...
      </div>

      <OAuthConsentPanel
        v-else-if="details"
        :client="details.client"
        :scopes="details.scopes"
        :request-details="authorizationDetails"
        :deciding="deciding"
        @approve="approve"
        @deny="deny"
      />

      <div v-else class="rounded-lg border border-[var(--color-border)] p-6 text-center">
        <el-icon class="text-[var(--color-text-light)]" :size="32"><Warning /></el-icon>
        <p class="mt-3 mb-0 text-sm text-[var(--color-text-light)]">
          授权请求不可用，请从第三方应用重新发起授权。
        </p>
      </div>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Warning } from '@element-plus/icons-vue'
import {
  approveOAuthAuthorization,
  getOAuthAuthorizationDetails,
  type OAuthAuthorizationDetails,
  type OAuthAuthorizationRequest,
} from '@/api/oauth'
import OAuthConsentPanel from '@/components/oauth/OAuthConsentPanel.vue'
import UiCard from '@/components/ui/UiCard.vue'
import { getErrorMessage, isApiError } from '@/utils/error'

interface OAuthConsentDetail {
  label: string
  value: string
}

const route = useRoute()
const details = ref<OAuthAuthorizationDetails | null>(null)
const loading = ref(false)
const deciding = ref(false)
const message = ref('')
const messageType = ref<'success' | 'warning' | 'info' | 'error'>('info')

const authorizationRequest = computed<OAuthAuthorizationRequest>(() => ({
  response_type: queryString('response_type'),
  client_id: queryString('client_id'),
  redirect_uri: queryString('redirect_uri'),
  scope: queryString('scope'),
  state: queryString('state') || undefined,
  code_challenge: queryString('code_challenge'),
  code_challenge_method: queryString('code_challenge_method'),
}))

const authorizationDetails = computed<OAuthConsentDetail[]>(() => {
  if (!details.value) return []
  return [
    { label: '回调地址', value: details.value.redirect_uri },
    { label: '请求状态', value: details.value.state || '未提供' },
  ]
})

onMounted(loadDetails)

async function loadDetails() {
  details.value = null
  message.value = ''
  loading.value = true
  try {
    const res = await getOAuthAuthorizationDetails(authorizationRequest.value)
    details.value = res.data
  } catch (error) {
    const fallback = '授权请求无效或已过期'
    messageType.value = 'error'
    message.value = isApiError(error, 'permission', 'check', 'denied')
      ? '你无权授权此应用'
      : getErrorMessage(error, fallback)
  } finally {
    loading.value = false
  }
}

async function approve() {
  if (!details.value) return
  deciding.value = true
  message.value = ''
  try {
    const res = await approveOAuthAuthorization(authorizationRequest.value)
    window.location.assign(res.data.redirect_url)
  } catch (error) {
    messageType.value = 'error'
    message.value = isApiError(error, 'permission', 'check', 'denied')
      ? '你无权授权此应用'
      : getErrorMessage(error, '提交授权失败')
  } finally {
    deciding.value = false
  }
}

function deny() {
  if (!details.value) return
  window.location.assign(deniedRedirectURL(details.value.redirect_uri, details.value.state))
}

function deniedRedirectURL(redirectURI: string, state?: string) {
  const url = new URL(redirectURI)
  url.searchParams.set('error', 'access_denied')
  if (state) url.searchParams.set('state', state)
  return url.toString()
}

function queryString(key: string) {
  const value = route.query[key]
  if (Array.isArray(value)) return value[0] ?? ''
  return typeof value === 'string' ? value : ''
}
</script>
