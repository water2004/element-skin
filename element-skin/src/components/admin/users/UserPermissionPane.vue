<template>
  <div v-loading="permissionsLoading" class="space-y-5">
    <section>
      <div class="mb-3 flex items-center justify-between gap-3">
        <h4 class="m-0 text-base font-semibold text-[var(--color-heading)]">角色授权</h4>
        <el-text size="small" type="info">角色提供批量权限，单项覆盖用于精细调整</el-text>
      </div>
      <div class="rounded-lg bg-[var(--color-background-soft)] p-4">
        <div class="mb-4 flex flex-wrap gap-2">
          <el-tag
            v-for="role in editor.assignedRoleLabels.value"
            :key="role.id"
            :type="role.protected ? 'danger' : 'info'"
            :closable="editor.roleTagClosable(role)"
            disable-transitions
            @close="emit('revoke-role', role.id)"
          >
            {{ role.name }}
          </el-tag>
          <el-text v-if="!editor.assignedRoleLabels.value.length" type="info" size="small">
            暂无额外角色
          </el-text>
        </div>
        <div class="flex flex-col gap-2 md:flex-row">
          <el-select
            v-model="editor.selectedRoleId.value"
            class="md:w-72"
            placeholder="选择要授予的角色"
            filterable
            clearable
          >
            <el-option
              v-for="role in editor.grantableRoles.value"
              :key="role.id"
              :label="role.name"
              :value="role.id"
              :disabled="role.protected && !editor.canManageProtected.value"
            >
              <div class="flex items-center justify-between gap-3">
                <span>{{ role.name }}</span>
                <el-tag v-if="role.protected" type="danger" size="small">受保护</el-tag>
              </div>
            </el-option>
          </el-select>
          <el-button
            type="primary"
            :icon="Plus"
            :disabled="!editor.selectedRoleId.value || !editor.canGrantPermission.value"
            @click="grantSelectedRole"
          >
            添加角色
          </el-button>
        </div>
      </div>
    </section>

    <section>
      <div class="mb-3 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <h4 class="m-0 text-base font-semibold text-[var(--color-heading)]">单项权限</h4>
      </div>
      <div class="space-y-4">
        <div class="grid gap-4">
          <div class="flex flex-col gap-2">
            <div class="text-sm font-semibold text-[var(--color-heading)]">继承的权限</div>
            <div v-if="editor.inheritedPermissionGroups.value.length" class="space-y-3">
              <div class="flex flex-wrap gap-2">
                <PermissionToneTag
                  v-for="group in editor.inheritedPermissionGroups.value"
                  :key="group.resource"
                  :label="group.resourceDescription"
                  :tone="group.tone"
                  :count="group.items.length"
                  :active="editor.isSelectedPermissionGroup(group, 'inherited')"
                  variant="category"
                  clickable
                  @click="editor.selectPermissionGroup(group.resource, 'inherited')"
                />
              </div>
              <div class="min-h-10 rounded-lg bg-[var(--color-background-soft)] px-3 py-2">
                <div class="mb-2 flex items-center gap-1.5">
                  <span class="text-sm font-medium text-[var(--color-heading)]">
                    {{ editor.selectedInheritedPermissionGroup.value?.resourceDescription }}
                  </span>
                  <span class="text-xs text-[var(--color-text-light)]">
                    {{ editor.selectedInheritedPermissionGroup.value?.items.length || 0 }} 项
                  </span>
                </div>
                <div class="flex flex-wrap gap-2">
                  <span
                    v-for="item in editor.selectedInheritedPermissionGroup.value?.items || []"
                    :key="item.code"
                  >
                    <PermissionToneTag
                      :label="item.label"
                      :tone="editor.selectedInheritedPermissionGroup.value?.tone || 'slate'"
                      :title="item.code"
                    />
                  </span>
                </div>
              </div>
            </div>
            <el-text v-else type="info" size="small">暂无继承权限</el-text>
          </div>
          <div class="flex flex-col gap-2">
            <div class="text-sm font-semibold text-[var(--color-heading)]">覆盖</div>
            <div v-if="editor.overridePermissionGroups.value.length" class="space-y-3">
              <div class="flex flex-wrap gap-2">
                <PermissionToneTag
                  v-for="group in editor.overridePermissionGroups.value"
                  :key="group.resource"
                  :label="group.resourceDescription"
                  :tone="group.tone"
                  :count="group.items.length"
                  :active="editor.isSelectedPermissionGroup(group, 'override')"
                  variant="category"
                  clickable
                  @click="editor.selectPermissionGroup(group.resource, 'override')"
                />
              </div>
              <div class="min-h-10 rounded-lg bg-[var(--color-background-soft)] px-3 py-2">
                <div class="mb-2 flex items-center gap-1.5">
                  <span class="text-sm font-medium text-[var(--color-heading)]">
                    {{ editor.selectedOverridePermissionGroup.value?.resourceDescription }}
                  </span>
                  <span class="text-xs text-[var(--color-text-light)]">
                    {{ editor.selectedOverridePermissionGroup.value?.items.length || 0 }} 项
                  </span>
                </div>
                <div class="flex flex-wrap gap-2">
                  <span
                    v-for="item in editor.selectedOverridePermissionGroup.value?.items || []"
                    :key="item.code"
                  >
                    <PermissionToneTag
                      :label="item.label"
                      :tone="editor.selectedOverridePermissionGroup.value?.tone || 'slate'"
                      :title="item.code"
                      :badge-label="item.effect === 'allow' ? '允许' : '拒绝'"
                      :badge-tone="item.effect"
                      removable
                      @remove="emit('clear-permission', item.code)"
                    />
                  </span>
                </div>
              </div>
            </div>
            <el-text v-else type="info" size="small">暂无单项权限覆盖</el-text>
          </div>
        </div>
        <div class="grid gap-2 pt-1 md:grid-cols-[minmax(0,1fr)_120px_auto]">
          <el-select
            v-model="editor.selectedPermissionCode.value"
            placeholder="选择要覆盖的权限"
            filterable
            clearable
          >
            <el-option
              v-for="item in editor.grantablePermissionOptions.value"
              :key="item.code"
              :label="`${item.code} · ${item.description}`"
              :value="item.code"
              :disabled="editor.permissionControlDisabled(item)"
            >
              <div class="flex flex-col">
                <span class="font-mono text-xs">{{ item.code }}</span>
                <span class="text-xs text-[var(--color-text-light)]">
                  {{ item.description }}
                </span>
              </div>
            </el-option>
          </el-select>
          <el-select v-model="editor.selectedPermissionEffect.value">
            <el-option label="允许" value="allow" :disabled="!editor.canGrantPermission.value" />
            <el-option label="拒绝" value="deny" :disabled="!editor.canRevokePermission.value" />
          </el-select>
          <el-button
            type="primary"
            :icon="Plus"
            :disabled="!editor.canAddSelectedPermission.value"
            @click="setSelectedPermission"
          >
            添加覆盖
          </el-button>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { Plus } from '@element-plus/icons-vue'
import PermissionToneTag from '@/components/admin/users/PermissionToneTag.vue'
import { useUserPermissionEditor } from '@/components/admin/users/useUserPermissionEditor'
import type { PermissionOverrideEffect, User, UserPermissionsResponse } from '@/api/types'

const props = defineProps<{
  user: User
  visible: boolean
  isSelf: boolean
  permissionState: UserPermissionsResponse | null
  permissionsLoading: boolean
  currentPermissions: string[]
}>()

const emit = defineEmits<{
  'grant-role': [roleId: string]
  'revoke-role': [roleId: string]
  'set-permission': [permissionCode: string, effect: PermissionOverrideEffect]
  'clear-permission': [permissionCode: string]
}>()

const editor = useUserPermissionEditor({
  visible: () => props.visible,
  user: () => props.user,
  permissionState: () => props.permissionState,
  currentPermissions: () => props.currentPermissions,
  isSelf: () => props.isSelf,
})

function grantSelectedRole() {
  const roleId = editor.consumeSelectedRole()
  if (roleId) emit('grant-role', roleId)
}

function setSelectedPermission() {
  const selected = editor.consumeSelectedPermission()
  if (selected) emit('set-permission', selected.code, selected.effect)
}
</script>
