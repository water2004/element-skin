<template>
  <UiCard class="p-6">
    <div class="mb-5 flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
      <div>
        <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">Webhook endpoints</h2>
        <p class="mt-1 mb-0 text-sm text-[var(--color-text-light)]">
          完全可选。站点只异步发送基础标识，接收方再使用 API 获取所需信息。
        </p>
      </div>
      <el-button :disabled="disabled || endpoints.length >= maxEndpoints" @click="emit('add')">
        <el-icon><Plus /></el-icon>
        添加 endpoint
      </el-button>
    </div>

    <el-empty
      v-if="endpoints.length === 0"
      description="未配置 Webhook，应用不会产生任何投递任务"
    />

    <div v-else class="space-y-4">
      <OAuthWebhookEndpointForm
        v-for="(endpoint, index) in endpoints"
        :key="endpoint.key"
        :endpoint="endpoint"
        :events="events"
        :disabled="disabled"
        :number="index + 1"
        @update="updateEndpoint(index, $event)"
        @remove="emit('remove', index)"
      />
    </div>
  </UiCard>
</template>

<script setup lang="ts">
import { Plus } from '@element-plus/icons-vue'
import UiCard from '@/components/ui/UiCard.vue'
import OAuthWebhookEndpointForm from './OAuthWebhookEndpointForm.vue'
import type { OAuthWebhookEventDefinition } from '@/api/oauth'
import type { WebhookEndpointForm } from './oauthAppFormState'

withDefaults(
  defineProps<{
    endpoints: WebhookEndpointForm[]
    events: OAuthWebhookEventDefinition[]
    disabled: boolean
    maxEndpoints?: number
  }>(),
  {
    maxEndpoints: 5,
  },
)

const emit = defineEmits<{
  add: []
  remove: [index: number]
  'update-endpoint': [index: number, patch: Partial<WebhookEndpointForm>]
}>()

function updateEndpoint(index: number, patch: Partial<WebhookEndpointForm>) {
  emit('update-endpoint', index, patch)
}
</script>
