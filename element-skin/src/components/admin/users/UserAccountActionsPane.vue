<template>
  <div class="grid gap-4 md:grid-cols-2">
    <div
      class="flex items-center justify-between gap-4 rounded-lg border border-[var(--color-border)] p-4"
    >
      <div>
        <div class="font-semibold text-[var(--color-heading)]">账号封禁</div>
        <div class="mt-1 text-sm text-[var(--color-text-light)]">
          禁止该用户加入 Minecraft 服务器。
        </div>
      </div>
      <el-button
        v-if="!isBanned"
        type="warning"
        :disabled="isProtectedUser || isSelf"
        @click="$emit('show-ban')"
      >
        执行封禁
      </el-button>
      <el-button v-else type="success" @click="$emit('unban', user)">解除封禁</el-button>
    </div>
    <div
      class="flex items-center justify-between gap-4 rounded-lg border border-[var(--color-border)] p-4"
    >
      <div>
        <div class="font-semibold text-[var(--color-heading)]">强制重置密码</div>
        <div class="mt-1 text-sm text-[var(--color-text-light)]">手动为该用户设置新密码。</div>
      </div>
      <el-button @click="$emit('show-reset-password')">重置密码</el-button>
    </div>
    <div
      class="flex items-center justify-between gap-4 rounded-lg border border-[var(--el-color-danger-light-7)] p-4 md:col-span-2"
    >
      <div>
        <div class="font-semibold text-[var(--el-color-danger)]">注销账号</div>
        <div class="mt-1 text-sm text-[var(--color-text-light)]">永久删除该用户及其关联数据。</div>
      </div>
      <el-button
        type="danger"
        :disabled="isProtectedUser || isSelf"
        @click="$emit('delete-user', user)"
      >
        删除用户
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { User, UserPermissionsResponse } from '@/api/types'

const props = defineProps<{
  user: User
  isBanned: boolean
  isSelf: boolean
  permissionState: UserPermissionsResponse | null
}>()

defineEmits<{
  'show-ban': []
  unban: [user: User]
  'show-reset-password': []
  'delete-user': [user: User]
}>()

const roleIds = computed(() => new Set(props.permissionState?.roles || props.user.roles || []))
const isProtectedUser = computed(() => {
  const protectedRoles = props.permissionState?.catalog.roles.filter((role) => role.protected) || []
  return protectedRoles.some((role) => roleIds.value.has(role.id))
})
</script>
