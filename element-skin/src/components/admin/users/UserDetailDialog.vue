<template>
  <UiDialog v-model="visible" destroy-on-close align-center variant="wide-form">
    <div v-if="user" class="p-6">
      <UserDetailHeader
        :user="user"
        :user-avatars="userAvatars"
        :permission-state="permissionState"
        :is-banned="isBanned"
        :ban-remaining="banRemaining"
      />

      <el-tabs type="border-card">
        <el-tab-pane label="角色列表">
          <UserProfilesPane
            :profiles="profiles"
            :loading="profilesLoading"
            :disabled-prev="profilesPrevDisabled"
            :disabled-next="profilesNextDisabled"
            @prev="$emit('profiles-prev')"
            @next="$emit('profiles-next')"
          />
        </el-tab-pane>

        <el-tab-pane label="权限">
          <UserPermissionPane
            :user="user"
            :visible="visible"
            :is-self="isSelf"
            :permission-state="permissionState"
            :permissions-loading="permissionsLoading"
            :current-permissions="currentPermissions"
            @grant-role="(roleId) => $emit('grant-role', roleId)"
            @revoke-role="(roleId) => $emit('revoke-role', roleId)"
            @set-permission="
              (permissionCode, effect) => $emit('set-permission', permissionCode, effect)
            "
            @clear-permission="(permissionCode) => $emit('clear-permission', permissionCode)"
          />
        </el-tab-pane>

        <el-tab-pane label="账号操作">
          <UserAccountActionsPane
            :user="user"
            :is-banned="isBanned"
            :is-self="isSelf"
            :permission-state="permissionState"
            @show-ban="$emit('show-ban')"
            @unban="(targetUser) => $emit('unban', targetUser)"
            @show-reset-password="$emit('show-reset-password')"
            @delete-user="(targetUser) => $emit('delete-user', targetUser)"
          />
        </el-tab-pane>
      </el-tabs>
    </div>
  </UiDialog>
</template>

<script setup lang="ts">
import type { PermissionOverrideEffect, Profile, User, UserPermissionsResponse } from '@/api/types'
import UiDialog from '@/components/ui/UiDialog.vue'
import UserAccountActionsPane from '@/components/admin/users/UserAccountActionsPane.vue'
import UserDetailHeader from '@/components/admin/users/UserDetailHeader.vue'
import UserPermissionPane from '@/components/admin/users/UserPermissionPane.vue'
import UserProfilesPane from '@/components/admin/users/UserProfilesPane.vue'

const visible = defineModel<boolean>('visible', { required: true })

defineProps<{
  user: User | null
  profiles: Profile[]
  userAvatars: Record<string, string>
  profilesLoading: boolean
  profilesPrevDisabled: boolean
  profilesNextDisabled: boolean
  isBanned: boolean
  banRemaining: string
  isSelf: boolean
  permissionState: UserPermissionsResponse | null
  permissionsLoading: boolean
  currentPermissions: string[]
}>()

defineEmits<{
  'profiles-prev': []
  'profiles-next': []
  'grant-role': [roleId: string]
  'revoke-role': [roleId: string]
  'set-permission': [permissionCode: string, effect: PermissionOverrideEffect]
  'clear-permission': [permissionCode: string]
  'show-ban': []
  unban: [user: User]
  'show-reset-password': []
  'delete-user': [user: User]
}>()
</script>
