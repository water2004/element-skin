import { createApp, h, nextTick, ref } from 'vue'
import ElementPlus from 'element-plus'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ExternalIdentity, OfficialProfileBinding } from '@/api/types'
import OfficialBindingDialog from '../OfficialBindingDialog.vue'
import ProfileCard from '@/components/common/ProfileCard.vue'
import RolePreviewDialog from '../RolePreviewDialog.vue'

import { profile, binding } from './fixtures/officialBinding'

vi.mock('@/components/SkinViewer.vue', () => ({ default: { render: () => h('div') } }))
vi.mock('@/components/CapeViewer.vue', () => ({ default: { render: () => h('div') } }))

const cleanup: Array<() => void> = []
afterEach(() =>
  cleanup
    .splice(0)
    .reverse()
    .forEach((dispose) => dispose()),
)

describe('official profile binding UI', () => {
  it('keeps unbound role cards free of official badges and binding commands', async () => {
    const mounted = mountComponent(ProfileCard, {
      profile,
      delayIndex: 0,
      texturesUrl: (hash: string | null | undefined) => `/textures/${hash || ''}`,
    })
    await flushUI()

    expect(mounted.host.textContent).not.toContain('正版')
    expect(mounted.host.textContent).toContain('RemoteSteve')
    expect(buttonText(mounted.host)).toEqual([])
    expect(mounted.host.textContent).not.toContain('同步正版数据')
    expect(mounted.host.textContent).not.toContain('解除绑定')
    mounted.unmount()
  })

  it('keeps sync and unbind operations inside the role detail dialog', async () => {
    const synced: OfficialProfileBinding[] = []
    const unbound: OfficialProfileBinding[] = []
    const mounted = mountComponent(RolePreviewDialog, {
      visible: true,
      profile,
      texturesUrl: (hash: string | null | undefined) => `/textures/${hash || ''}`,
      officialBinding: binding,
      officialState: 'ready',
      busy: false,
      revision: 1,
      saveName: async () => true,
      canSyncOfficialBinding: true,
      canDeleteOfficialBinding: true,
      canRename: false,
      canSetAvatar: false,
      canClearTexture: false,
      canDelete: false,
      'onUpdate:visible': () => undefined,
      onSyncOfficial: (item: OfficialProfileBinding) => synced.push(item),
      onUnbindOfficial: (item: OfficialProfileBinding) => unbound.push(item),
    })
    await flushUI()

    const dialog = document.body.querySelector<HTMLElement>('.ui-dialog--viewer')
    expect(dialog?.textContent).toContain('正版绑定')
    expect(dialog?.textContent).toContain('Xbox account')
    findButton(dialog!, '同步正版数据').click()
    findButton(dialog!, '解除绑定').click()
    await nextTick()
    expect(synced).toEqual([binding])
    expect(unbound).toEqual([binding])
    mounted.unmount()
  })

  it('shows the shared official badge on a bound role card', async () => {
    const mounted = mountComponent(ProfileCard, {
      profile,
      delayIndex: 0,
      official: true,
      texturesUrl: String,
    })
    await flushUI()
    const badge = mounted.host.querySelector('.el-tag')!
    expect(badge.textContent?.trim()).toBe('正版')
    for (const token of ['el-tag--success', 'el-tag--dark', 'is-round', 'font-bold'])
      expect(badge.classList.contains(token)).toBe(true)
    expect(badge.classList.contains('el-tag--small')).toBe(false)
    expect(buttonText(mounted.host)).toEqual([])
  })

  it('groups official information and its commands before common operations', async () => {
    mountComponent(RolePreviewDialog, {
      visible: true,
      profile: { ...profile, skin_hash: 'skin', cape_hash: 'cape' },
      texturesUrl: String,
      officialBinding: binding,
      officialState: 'ready',
      busy: false,
      revision: 1,
      saveName: async () => true,
      canRename: false,
      canSetAvatar: true,
      canClearTexture: true,
      canDelete: true,
      canSyncOfficialBinding: true,
      canDeleteOfficialBinding: true,
    })
    await flushUI()
    const dialog = document.body.querySelector<HTMLElement>('.ui-dialog--viewer')!
    const official = dialog.querySelector<HTMLElement>('section[aria-label="正版绑定"]')!
    const common = dialog.querySelector<HTMLElement>('section[aria-label="通用操作"]')!
    expect(official.querySelector('h3')?.textContent).toBe('正版绑定')
    const infoRow = official.querySelector('.flex-wrap')!
    expect(infoRow.textContent?.replace(/\s+/g, '')).toBe('RemoteSteveXboxaccount正版')
    expect(infoRow.querySelectorAll('.el-tag')).toHaveLength(1)
    expect(
      official.querySelector('[aria-label="查看完整正版 UUID，点击可展开"]')?.textContent?.trim(),
    ).toBe('01234567...')
    expect(buttonText(official).slice(-2)).toEqual(['同步正版数据', '解除绑定'])
    expect(buttonText(common)).toEqual(['用作头像', '清除皮肤', '清除披风', '删除角色'])
    expect(official.compareDocumentPosition(common) & Node.DOCUMENT_POSITION_FOLLOWING).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    )
    expect(common.textContent).not.toContain('同步正版数据')
  })

  it.each([
    { state: 'loading', text: '正版绑定加载中' },
    { state: 'error', text: '正版绑定加载失败' },
    { state: 'forbidden', text: '无权读取正版绑定' },
  ])('does not present a stale binding as usable while $state', async ({ state, text }) => {
    mountComponent(RolePreviewDialog, {
      visible: true,
      profile,
      texturesUrl: String,
      officialBinding: binding,
      officialState: state,
      busy: false,
      revision: 1,
      saveName: async () => true,
      canRename: false,
      canSetAvatar: false,
      canClearTexture: false,
      canDelete: false,
      canSyncOfficialBinding: true,
      canDeleteOfficialBinding: true,
    })
    await flushUI()
    const section = document.body.querySelector('section[aria-label="正版绑定"]')!
    expect(section.querySelector('[role="status"]')?.textContent?.trim()).toBe(text)
    expect(section.querySelector('.el-tag')).toBeNull()
    expect(section.querySelector('button')).toBeNull()
  })

  it.each([false, true])(
    'preserves a readable binding without unauthorized commands (busy: %s)',
    async (busy) => {
      mountComponent(RolePreviewDialog, {
        visible: true,
        profile,
        texturesUrl: String,
        officialBinding: { ...binding, identity: { ...binding.identity, label: '' } },
        officialState: 'ready',
        busy,
        revision: 1,
        saveName: async () => true,
        canRename: false,
        canSetAvatar: false,
        canClearTexture: false,
        canDelete: false,
        canSyncOfficialBinding: true,
        canDeleteOfficialBinding: false,
      })
      await flushUI()
      const section = document.body.querySelector<HTMLElement>('section[aria-label="正版绑定"]')!
      expect(section.textContent).toContain('Microsoft')
      expect(section.textContent).not.toContain('Xbox account')
      expect(section.textContent).not.toContain('解除绑定')
      expect(findButton(section, '同步正版数据').disabled).toBe(busy)
      expect(section.querySelector('.el-tag--success')?.textContent).toBe('正版')
    },
  )

  it('offers only unbound usable Microsoft accounts and never asks for a local role', async () => {
    const identities: ExternalIdentity[] = [
      externalIdentity('identity-1', 'Bound account', 'microsoft'),
      externalIdentity('identity-2', 'Available account', 'microsoft'),
      externalIdentity('identity-3', 'Generic account', 'generic_oidc'),
    ]
    const mounted = mountComponent(OfficialBindingDialog, {
      visible: true,
      identities,
      bindings: [binding],
      loading: false,
      'onUpdate:visible': () => undefined,
    })
    await flushUI()

    const dialog = document.body.querySelector<HTMLElement>('.ui-dialog--form')
    expect(dialog?.textContent).toContain('选择 Microsoft 账户')
    expect(dialog?.textContent).toContain('Available account')
    expect(dialog?.textContent).not.toContain('Bound account')
    expect(dialog?.textContent).not.toContain('Generic account')
    expect(dialog?.textContent).not.toContain('选择本站角色')
    expect(findButton(dialog!, '建立绑定').disabled).toBe(false)
    mounted.unmount()
  })
})

function externalIdentity(
  id: string,
  label: string,
  adapter: ExternalIdentity['provider_adapter'],
): ExternalIdentity {
  return {
    id,
    provider_id: `${adapter}-provider`,
    provider_name: adapter === 'microsoft' ? 'Microsoft' : 'Generic',
    provider_adapter: adapter,
    provider_icon_url: '',
    provider_enabled: true,
    provider_link_enabled: true,
    subject: `${id}-subject`,
    label,
    email: `${id}@example.com`,
    email_verified: true,
    display_name: label,
    avatar_url: '',
    created_at: 1000,
    updated_at: 1000,
    last_login_at: 1000,
    authorization_status: 'active',
    last_refresh_at: 1000,
    last_refresh_error_at: null,
  }
}

function mountComponent(component: object, props: Record<string, unknown>) {
  const host = document.createElement('div')
  document.body.appendChild(host)
  const app = createApp({ render: () => h(component, props) })
  app.use(ElementPlus)
  app.provide('isDark', ref(false))
  app.mount(host)
  let active = true
  const unmount = () => {
    if (!active) return
    active = false
    app.unmount()
    host.remove()
  }
  cleanup.push(unmount)
  return { host, unmount }
}

function buttonText(root: HTMLElement) {
  return [...root.querySelectorAll('button')].map((button) => button.textContent?.trim() || '')
}

function findButton(root: HTMLElement, label: string) {
  const button = [...root.querySelectorAll('button')].find((item) =>
    item.textContent?.includes(label),
  )
  if (!button) throw new Error(`button not found: ${label}`)
  return button
}

async function flushUI() {
  await Promise.resolve()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
}
