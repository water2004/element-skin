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
      <div
        v-for="(endpoint, index) in endpoints"
        :key="endpoint.key"
        class="rounded-xl border border-[var(--color-border)] bg-[var(--color-background-soft)] p-4"
      >
        <div class="mb-4 flex items-center justify-between gap-3">
          <div class="font-semibold text-[var(--color-heading)]">Endpoint {{ index + 1 }}</div>
          <ActionBar>
            <el-switch
              :model-value="endpoint.enabled"
              :disabled="disabled"
              active-text="启用"
              inactive-text="停用"
              @update:model-value="updateEndpoint(index, { enabled: $event })"
            />
            <el-button type="danger" link :disabled="disabled" @click="emit('remove', index)">
              <el-icon><Delete /></el-icon>
              移除
            </el-button>
          </ActionBar>
        </div>
        <el-form label-position="top" :disabled="disabled">
          <el-form-item label="接收地址" required>
            <el-input
              :model-value="endpoint.url"
              placeholder="https://hooks.example/events"
              @update:model-value="updateEndpoint(index, { url: $event })"
            />
            <div class="form-tip">仅允许公网 HTTPS 地址，不跟随重定向。</div>
          </el-form-item>
          <el-form-item label="监听事件" required>
            <el-checkbox-group
              v-if="events.length"
              :model-value="endpoint.events"
              class="grid w-full gap-2 md:grid-cols-2"
              @update:model-value="updateEvents(index, $event)"
            >
              <UiOptionCard
                v-for="event in events"
                :key="event.type"
                as="ElCheckbox"
                :value="event.type"
              >
                <span class="ui-option-card__info min-w-0">
                  <span class="ui-option-card__title font-mono text-xs">{{ event.type }}</span>
                  <span class="ui-option-card__subtitle !font-sans">{{ event.description }}</span>
                </span>
              </UiOptionCard>
            </el-checkbox-group>
            <el-alert
              v-else
              class="w-full"
              type="info"
              :closable="false"
              title="请先申请支持 Webhook 事件的读取权限"
            />
          </el-form-item>
        </el-form>
      </div>
    </div>
  </UiCard>
</template>

<script setup lang="ts">
import { Delete, Plus } from '@element-plus/icons-vue'
import ActionBar from '@/components/common/ActionBar.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiOptionCard from '@/components/ui/UiOptionCard.vue'
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

function updateEvents(index: number, values: Array<string | number>) {
  updateEndpoint(index, { events: values.map(String) })
}
</script>
