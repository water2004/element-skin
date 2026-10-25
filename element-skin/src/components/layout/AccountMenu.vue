<template>
  <el-popover
    placement="bottom-end"
    :width="240"
    trigger="hover"
    popper-class="account-popover"
    :show-arrow="false"
    :offset="4"
  >
    <template #reference>
      <div class="account-trigger">
        <el-avatar
          :shape="hasCustomAvatar ? 'square' : 'circle'"
          size="small"
          :class="[
            'account-avatar',
            {
              'bg-gradient-to-br from-[#b37feb] to-[#8553cf]': !hasCustomAvatar,
              'has-custom': hasCustomAvatar,
            },
          ]"
          :src="avatarSrc"
        >
          {{ hasCustomAvatar ? '' : avatarInitial }}
        </el-avatar>
        <span class="account-name">{{ accountName }}</span>
      </div>
    </template>
    <div
      class="account-panel rounded-[16px] border border-[var(--color-border)] bg-[var(--color-card-background)] shadow-[0_4px_12px_rgba(0,0,0,0.05)]"
    >
      <div class="account-header">
        <el-avatar
          :shape="hasCustomAvatar ? 'square' : 'circle'"
          :size="48"
          :class="[
            'account-avatar',
            {
              'bg-gradient-to-br from-[#b37feb] to-[#8553cf]': !hasCustomAvatar,
              'has-custom': hasCustomAvatar,
            },
          ]"
          :src="avatarSrc"
        >
          {{ hasCustomAvatar ? '' : avatarInitial }}
        </el-avatar>
        <div class="account-meta">
          <h4>{{ accountName }}</h4>
          <p>{{ roleLabel }}</p>
        </div>
      </div>
      <div class="account-actions">
        <template v-if="links.length">
          <div class="account-section-label">账户与应用</div>
          <UiButton
            v-for="item in links"
            :key="item.path"
            variant="outline"
            @click="emit('navigate', item.path)"
          >
            <span>{{ item.title }}</span>
          </UiButton>
        </template>
        <template v-if="canAccessAdmin">
          <div class="account-section-label">面板</div>
          <UiButton variant="outline" @click="emit('navigate', '/admin')">
            <span>管理面板</span>
          </UiButton>
        </template>
        <div class="account-action-separator" />
        <UiButton variant="outline-danger" @click="emit('logout')">
          <span>退出登录</span>
        </UiButton>
      </div>
    </div>
  </el-popover>
</template>

<script setup lang="ts">
import { computed } from 'vue'

import UiButton from '@/components/ui/UiButton.vue'
import type { AccountLink } from './appNavigation'

const props = defineProps<{
  avatarSrc: string
  accountName: string
  roleLabel: string
  canAccessAdmin: boolean
  links: readonly AccountLink[]
}>()

const emit = defineEmits<{
  navigate: [path: string]
  logout: []
}>()

const hasCustomAvatar = computed(() => props.avatarSrc.length > 0)
const avatarInitial = computed(() => (props.accountName || 'U').slice(0, 1).toUpperCase())
</script>

<style>
.account-popover {
  padding: 0 !important;
  background: var(--color-popover-background) !important;
  border: 1px solid var(--color-border) !important;
}
</style>

<style scoped>
.account-trigger {
  display: flex;
  align-items: center;
  cursor: pointer;
  gap: 8px;
  padding: 6px 12px;
  border-radius: 20px;
  transition: background-color 0.2s;
}

.account-trigger:hover {
  background: var(--color-background-soft);
}

.account-name {
  font-size: 14px;
  color: var(--color-text);
  font-weight: 500;
}

.account-panel {
  padding: 20px;
  width: 100%;
  border: none !important;
}

.account-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--color-border);
}

.account-meta h4 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-heading);
}

.account-meta p {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--color-text-light);
}

.account-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.account-section-label {
  margin: 4px 0 -2px;
  color: var(--color-text-light);
  font-size: 12px;
}

.account-action-separator {
  height: 1px;
  margin: 4px 0;
  background: var(--color-border);
}

.account-actions :deep(.el-button) {
  width: 100%;
}

.account-actions :deep(.el-button + .el-button) {
  margin-left: 0 !important;
}

.account-avatar.has-custom {
  background: transparent !important;
}

.account-avatar.has-custom :deep(img) {
  object-fit: contain;
}
</style>
