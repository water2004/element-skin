import type { PermissionDefinition, PermissionOverrideEffect } from '@/api/types'

export type PermissionTone = 'emerald' | 'sky' | 'violet' | 'amber' | 'rose' | 'slate' | 'cyan'

export interface PermissionDisplayItem {
  code: string
  label: string
  resource: string
  resourceDescription: string
  effect?: PermissionOverrideEffect
}

export interface PermissionDisplayGroup {
  resource: string
  resourceDescription: string
  tone: PermissionTone
  items: PermissionDisplayItem[]
}

export function createPermissionDisplayItem(
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

export function groupPermissionItems(items: PermissionDisplayItem[]): PermissionDisplayGroup[] {
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

export function selectedPermissionGroup(
  groups: PermissionDisplayGroup[],
  selectedResource: string,
) {
  return groups.find((group) => group.resource === selectedResource) || groups[0] || null
}

export function normalizeSelectedResource(
  selectedResource: string,
  groups: PermissionDisplayGroup[],
) {
  if (groups.some((group) => group.resource === selectedResource)) return selectedResource
  return groups[0]?.resource || ''
}

export function permissionTone(resource: string): PermissionTone {
  const fixedTones: Record<string, PermissionTone> = {
    audit: 'sky',
    auth: 'violet',
    cache: 'cyan',
    invite: 'sky',
    media: 'sky',
    external_identity: 'violet',
    identity_provider: 'cyan',
    official_profile: 'emerald',
    minecraft_profile: 'cyan',
    minecraft_session: 'cyan',
    minecraft_texture_property: 'cyan',
    notification: 'amber',
    oauth_app: 'violet',
    oauth_grant: 'violet',
    oauth_token: 'violet',
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
