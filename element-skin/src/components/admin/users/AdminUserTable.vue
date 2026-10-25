<template>
  <el-table :data="users" class="modern-table w-full" v-loading="loading">
    <el-table-column prop="display_name" label="用户名" min-width="150">
      <template #default="{ row }">
        <div class="flex items-center">
          <el-avatar
            :size="32"
            :shape="row.avatar_hash ? 'square' : 'circle'"
            :class="[row.avatar_hash ? 'has-custom' : 'avatar-fallback', 'mr-2']"
            :src="userAvatars[row.avatar_hash || ''] || ''"
          >
            {{ !row.avatar_hash ? userAvatarInitial(row) : '' }}
          </el-avatar>
          <span>{{ row.display_name || '未设置' }}</span>
        </div>
      </template>
    </el-table-column>
    <el-table-column prop="email" label="邮箱" min-width="220" />
    <el-table-column label="身份状态" width="120">
      <template #default="{ row }">
        <el-tag v-if="row.protected" type="danger" effect="dark" size="small"> 超级管理员 </el-tag>
        <el-tag v-else-if="hasUserRole(row, 'admin')" type="danger" effect="light" size="small">
          管理员
        </el-tag>
        <el-tag
          v-else-if="hasUserRole(row, 'moderator')"
          type="warning"
          effect="light"
          size="small"
        >
          审核员
        </el-tag>
        <el-tag v-else-if="isUserBanned(row)" type="warning" effect="light" size="small">
          已封禁
        </el-tag>
        <el-tag v-else type="success" effect="light" size="small">正常</el-tag>
      </template>
    </el-table-column>
    <el-table-column label="管理操作" width="120" align="center">
      <template #default="{ row }">
        <el-button size="small" type="primary" @click="emit('manage', row)">管理</el-button>
      </template>
    </el-table-column>
  </el-table>
</template>

<script setup lang="ts">
import type { User } from '@/api/types'
import { hasUserRole, isUserBanned, userAvatarInitial } from './userListDisplay'

defineProps<{
  users: User[]
  loading: boolean
  userAvatars: Record<string, string>
}>()

const emit = defineEmits<{
  manage: [user: User]
}>()
</script>

<style scoped>
.modern-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.modern-table :deep(.el-table__row) {
  transition: background-color var(--motion-duration-base) ease;
}
</style>
