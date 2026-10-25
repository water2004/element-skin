<template>
  <div class="rounded-xl border border-[var(--color-border)] bg-[var(--color-background-soft)] p-4">
    <div class="mb-4 flex items-center justify-between gap-3">
      <div class="font-semibold text-[var(--color-heading)]">Endpoint {{ number }}</div>
      <ActionBar>
        <el-switch
          :model-value="endpoint.enabled"
          :disabled="disabled"
          active-text="启用"
          inactive-text="停用"
          @update:model-value="emit('update', { enabled: $event })"
        />
        <el-button type="danger" link :disabled="disabled" @click="emit('remove')">
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
          @update:model-value="emit('update', { url: $event })"
        />
        <div class="form-tip">仅允许公网 HTTPS 地址，不跟随重定向。</div>
      </el-form-item>
      <el-form-item label="监听事件" required>
        <el-checkbox-group
          v-if="events.length"
          :model-value="endpoint.events"
          class="grid w-full gap-2 md:grid-cols-2"
          @update:model-value="updateEvents($event)"
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
</template>

<script setup lang="ts">
import { Delete } from '@element-plus/icons-vue'
import ActionBar from '@/components/common/ActionBar.vue'
import UiOptionCard from '@/components/ui/UiOptionCard.vue'
import type { OAuthWebhookEventDefinition } from '@/api/oauth'
import type { WebhookEndpointForm } from './oauthAppFormState'

defineProps<{
  endpoint: WebhookEndpointForm
  events: OAuthWebhookEventDefinition[]
  disabled: boolean
  number: number
}>()

const emit = defineEmits<{
  update: [patch: Partial<WebhookEndpointForm>]
  remove: []
}>()

function updateEvents(values: Array<string | number>) {
  emit('update', { events: values.map(String) })
}
</script>
