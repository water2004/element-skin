<template>
  <div
    class="mb-6 flex flex-col gap-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-background-soft)] p-5 md:flex-row md:items-center"
  >
    <el-avatar
      :size="72"
      :shape="user.avatar_hash ? 'square' : 'circle'"
      :class="[
        user.avatar_hash ? 'has-custom' : 'bg-[var(--color-background-mute)]',
        'text-xl font-semibold text-[var(--color-text-light)]',
      ]"
      :src="userAvatars[user.avatar_hash || ''] || ''"
    >
      {{ !user.avatar_hash ? user.email.charAt(0).toUpperCase() : '' }}
    </el-avatar>
    <div class="min-w-0 flex-1">
      <div class="flex flex-wrap items-center gap-2">
        <h3 class="m-0 text-xl font-semibold text-[var(--color-heading)]">
          {{ user.display_name || '未设置显示名' }}
        </h3>
        <el-tag
          v-for="role in assignedRoleLabels"
          :key="role.id"
          :type="role.protected ? 'danger' : 'info'"
          size="small"
        >
          {{ role.name }}
        </el-tag>
      </div>
      <p class="mt-1 mb-0 text-sm text-[var(--color-text-light)]">{{ user.email }}</p>
      <p class="mt-1 mb-0 font-mono text-xs text-[var(--color-text-light)]">UID: {{ user.id }}</p>
    </div>
    <div class="md:text-right">
      <el-tag v-if="isBanned" type="warning" effect="dark">
        <el-icon><Warning /></el-icon>
        封禁中
      </el-tag>
      <el-tag v-else type="success" effect="dark">
        <el-icon><CircleCheck /></el-icon>
        状态正常
      </el-tag>
      <div v-if="isBanned" class="mt-1 text-xs text-[var(--el-color-warning)]">
        {{ banRemaining }} 后解封
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { CircleCheck, Warning } from '@element-plus/icons-vue'
import type { User, UserPermissionsResponse } from '@/api/types'

const props = defineProps<{
  user: User
  userAvatars: Record<string, string>
  permissionState: UserPermissionsResponse | null
  isBanned: boolean
  banRemaining: string
}>()

const roleIds = computed(() => new Set(props.permissionState?.roles || props.user.roles || []))
const assignedRoleLabels = computed(() => {
  const roles = props.permissionState?.catalog.roles || []
  const selected = roles.filter((role) => roleIds.value.has(role.id))
  if (selected.length) return selected
  return (props.user.roles || []).map((role) => ({
    id: role,
    name: role,
    description: '',
    system_role: true,
    protected: role === 'super_admin',
    permissions: [],
  }))
})
</script>

<style scoped>
.has-custom {
  background: transparent !important;
  border: none !important;
}

.has-custom :deep(img) {
  object-fit: contain;
}
</style>
