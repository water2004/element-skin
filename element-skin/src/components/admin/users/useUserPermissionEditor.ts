import { computed, ref, watch } from 'vue'
import type {
  PermissionDefinition,
  PermissionOverrideEffect,
  PermissionRole,
  User,
  UserPermissionsResponse,
} from '@/api/types'

type PermissionGroupKind = 'inherited' | 'override'
type PermissionTone = 'emerald' | 'sky' | 'violet' | 'amber' | 'rose' | 'slate' | 'cyan'

interface PermissionDisplayItem {
  code: string
  label: string
  resource: string
  resourceDescription: string
  effect?: PermissionOverrideEffect
}

interface PermissionDisplayGroup {
  resource: string
  resourceDescription: string
  tone: PermissionTone
  items: PermissionDisplayItem[]
}

interface UserPermissionEditorInput {
  visible: () => boolean
  user: () => User
  permissionState: () => UserPermissionsResponse | null
  currentPermissions: () => string[]
  isSelf: () => boolean
}

export function useUserPermissionEditor(input: UserPermissionEditorInput) {
  const selectedRoleId = ref('')
  const selectedPermissionCode = ref('')
  const selectedPermissionEffect = ref<PermissionOverrideEffect>('allow')
  const selectedInheritedResource = ref('')
  const selectedOverrideResource = ref('')

  const roleIds = computed(
    () => new Set(input.permissionState()?.roles || input.user().roles || []),
  )
  const effectivePermissions = computed(
    () => new Set(input.permissionState()?.effective_permissions || input.user().permissions || []),
  )
  const overrideMap = computed(() => {
    const out = new Map<string, PermissionOverrideEffect>()
    for (const item of input.permissionState()?.overrides || [])
      out.set(item.permission_code, item.effect)
    return out
  })
  const currentPermissionSet = computed(() => new Set(input.currentPermissions()))
  const canManageProtected = computed(() =>
    currentPermissionSet.value.has('permission_protected.manage.any'),
  )
  const canGrantPermission = computed(() => currentPermissionSet.value.has('permission.grant.any'))
  const canRevokePermission = computed(() =>
    currentPermissionSet.value.has('permission.revoke.any'),
  )
  const assignedRoleLabels = computed(() => {
    const roles = input.permissionState()?.catalog.roles || []
    const selected = roles.filter((role) => roleIds.value.has(role.id))
    if (selected.length) return selected
    return (input.user().roles || []).map((role) => ({
      id: role,
      name: role,
      description: '',
      system_role: true,
      protected: role === 'super_admin',
      permissions: [],
    }))
  })
  const grantableRoles = computed(() =>
    (input.permissionState()?.catalog.roles || []).filter((role) => !roleIds.value.has(role.id)),
  )
  const permissionByCode = computed(() => {
    const out = new Map<string, PermissionDefinition>()
    for (const item of input.permissionState()?.catalog.permissions || []) out.set(item.code, item)
    return out
  })
  const inheritedPermissionGroups = computed(() => {
    const inherited = new Map<string, PermissionDisplayItem>()
    for (const role of input.permissionState()?.catalog.roles || []) {
      if (!roleIds.value.has(role.id)) continue
      for (const code of role.permissions) {
        if (overrideMap.value.has(code)) continue
        if (!effectivePermissions.value.has(code)) continue
        const definition = permissionByCode.value.get(code)
        inherited.set(code, createPermissionDisplayItem(code, definition))
      }
    }
    return groupPermissionItems([...inherited.values()])
  })
  const overridePermissionGroups = computed(() => {
    const items = (input.permissionState()?.overrides || []).map((item) => ({
      ...createPermissionDisplayItem(
        item.permission_code,
        permissionByCode.value.get(item.permission_code),
      ),
      effect: item.effect,
    }))
    return groupPermissionItems(items)
  })
  const selectedInheritedPermissionGroup = computed(() =>
    selectedPermissionGroup(inheritedPermissionGroups.value, selectedInheritedResource.value),
  )
  const selectedOverridePermissionGroup = computed(() =>
    selectedPermissionGroup(overridePermissionGroups.value, selectedOverrideResource.value),
  )
  const grantablePermissionOptions = computed(() =>
    (input.permissionState()?.catalog.permissions || []).filter(
      (item) => !overrideMap.value.has(item.code),
    ),
  )
  const selectedPermission = computed(() =>
    selectedPermissionCode.value ? permissionByCode.value.get(selectedPermissionCode.value) : null,
  )
  const canAddSelectedPermission = computed(() => {
    if (!selectedPermission.value) return false
    if (selectedPermissionEffect.value === 'allow' && !canGrantPermission.value) return false
    if (selectedPermissionEffect.value === 'deny' && !canRevokePermission.value) return false
    return !permissionControlDisabled(selectedPermission.value)
  })

  watch(
    [() => input.visible(), inheritedPermissionGroups, overridePermissionGroups],
    ([open, inheritedGroups, overrideGroups]) => {
      if (!open) {
        selectedRoleId.value = ''
        selectedPermissionCode.value = ''
        selectedPermissionEffect.value = 'allow'
        selectedInheritedResource.value = ''
        selectedOverrideResource.value = ''
        return
      }

      selectedInheritedResource.value = normalizeSelectedResource(
        selectedInheritedResource.value,
        inheritedGroups,
      )
      selectedOverrideResource.value = normalizeSelectedResource(
        selectedOverrideResource.value,
        overrideGroups,
      )
    },
  )

  function selectPermissionGroup(resource: string, kind: PermissionGroupKind) {
    if (kind === 'inherited') selectedInheritedResource.value = resource
    else selectedOverrideResource.value = resource
  }

  function isSelectedPermissionGroup(group: PermissionDisplayGroup, kind: PermissionGroupKind) {
    return kind === 'inherited'
      ? selectedInheritedResource.value === group.resource
      : selectedOverrideResource.value === group.resource
  }

  function roleTagClosable(role: PermissionRole) {
    if (role.id === 'user') return false
    if (input.isSelf() && role.protected) return false
    if (role.protected && !canManageProtected.value) return false
    return canRevokePermission.value
  }

  function permissionControlDisabled(row: PermissionDefinition) {
    if (input.isSelf() && isProtectedPermission(row)) return true
    if (isProtectedPermission(row) && !canManageProtected.value) return true
    const current = overrideMap.value.get(row.code) || 'inherit'
    if (current === 'allow') return !canRevokePermission.value
    if (current === 'deny') return !canGrantPermission.value
    return !canGrantPermission.value && !canRevokePermission.value
  }

  function consumeSelectedRole() {
    if (!selectedRoleId.value) return ''
    const roleId = selectedRoleId.value
    selectedRoleId.value = ''
    return roleId
  }

  function consumeSelectedPermission() {
    if (!selectedPermissionCode.value || !canAddSelectedPermission.value) return null
    const payload = {
      code: selectedPermissionCode.value,
      effect: selectedPermissionEffect.value,
    }
    selectedPermissionCode.value = ''
    return payload
  }

  return {
    selectedRoleId,
    selectedPermissionCode,
    selectedPermissionEffect,
    canManageProtected,
    canGrantPermission,
    canRevokePermission,
    assignedRoleLabels,
    grantableRoles,
    inheritedPermissionGroups,
    overridePermissionGroups,
    selectedInheritedPermissionGroup,
    selectedOverridePermissionGroup,
    grantablePermissionOptions,
    canAddSelectedPermission,
    selectPermissionGroup,
    isSelectedPermissionGroup,
    roleTagClosable,
    permissionControlDisabled,
    consumeSelectedRole,
    consumeSelectedPermission,
  }
}

function createPermissionDisplayItem(
  code: string,
  definition?: PermissionDefinition,
): PermissionDisplayItem {
  return {
    code,
    label: definition?.description || code,
    resource: definition?.resource || code.split('.')[0] || 'other',
    resourceDescription: definition?.resource_description || definition?.resource || '其他权限',
  }
}

function groupPermissionItems(items: PermissionDisplayItem[]): PermissionDisplayGroup[] {
  const groups = new Map<string, PermissionDisplayGroup>()
  for (const item of [...items].sort((a, b) => a.code.localeCompare(b.code))) {
    const group = groups.get(item.resource)
    if (group) {
      group.items.push(item)
      continue
    }
    groups.set(item.resource, {
      resource: item.resource,
      resourceDescription: item.resourceDescription,
      tone: permissionTone(item.resource),
      items: [item],
    })
  }
  return [...groups.values()].sort((a, b) =>
    a.resourceDescription.localeCompare(b.resourceDescription),
  )
}

function selectedPermissionGroup(groups: PermissionDisplayGroup[], selectedResource: string) {
  return groups.find((group) => group.resource === selectedResource) || groups[0] || null
}

function normalizeSelectedResource(selectedResource: string, groups: PermissionDisplayGroup[]) {
  if (groups.some((group) => group.resource === selectedResource)) return selectedResource
  return groups[0]?.resource || ''
}

function permissionTone(resource: string): PermissionTone {
  const fixedTones: Record<string, PermissionTone> = {
    audit: 'sky',
    auth: 'violet',
    cache: 'cyan',
    invite: 'sky',
    media: 'sky',
    microsoft: 'violet',
    notification: 'amber',
    permission: 'rose',
    permission_audit: 'amber',
    permission_protected: 'rose',
    permission_role: 'emerald',
    profile: 'emerald',
    site: 'rose',
    texture: 'emerald',
    user: 'sky',
    wardrobe: 'emerald',
    wardrobe_item: 'emerald',
    yggdrasil: 'amber',
    yggdrasil_session: 'rose',
  }
  if (fixedTones[resource]) return fixedTones[resource]
  const fallbackTones: PermissionTone[] = ['emerald', 'sky', 'violet', 'amber', 'rose', 'cyan']
  const hash = [...resource].reduce((sum, char) => sum + char.charCodeAt(0), 0)
  return fallbackTones[hash % fallbackTones.length] || 'slate'
}

function isProtectedPermission(row: PermissionDefinition) {
  return row.scope === 'system' || row.resource === 'permission_protected'
}
