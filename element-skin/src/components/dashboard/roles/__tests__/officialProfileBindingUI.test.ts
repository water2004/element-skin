import { createApp, h, nextTick, ref } from 'vue'
import ElementPlus from 'element-plus'
import { describe, expect, it } from 'vitest'
import type { ExternalIdentity, OfficialProfileBinding, Profile } from '@/api/types'
import OfficialBindingDialog from '../OfficialBindingDialog.vue'
import RoleCard from '../RoleCard.vue'
import RolePreviewDialog from '../RolePreviewDialog.vue'

const profile: Profile = {
  id: '0123456789abcdef0123456789abcdef',
  name: 'RemoteSteve',
  model: 'slim',
  skin_hash: null,
  cape_hash: null,
}

const binding: OfficialProfileBinding = {
  id: 'binding-1',
  identity_id: 'identity-1',
  profile_id: profile.id,
  remote_uuid: profile.id,
  remote_name: 'RemoteSteve',
  remote_skin_url: '',
  remote_cape_url: '',
  remote_skin_model: 'slim',
  created_at: 1000,
  updated_at: 1000,
  last_synced_at: null,
  profile,
  identity: {
    id: 'identity-1',
    label: 'Xbox account',
    provider_id: 'microsoft-provider',
    provider_name: 'Microsoft',
    provider_adapter: 'microsoft',
  },
}

describe('official profile binding UI', () => {
  it('shows only a green official badge on the role card without binding operations', async () => {
    const mounted = mountComponent(RoleCard, {
      profile,
      delayIndex: 0,
      isDark: false,
      texturesUrl: (hash: string | null | undefined) => `/textures/${hash || ''}`,
      officialBinding: binding,
    })
    await flushUI()

    expect(mounted.host.textContent).toContain('正版')
    expect(mounted.host.textContent).toContain('RemoteSteve')
    expect(buttonText(mounted.host)).toEqual(['删除'])
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
      canSyncOfficialBinding: true,
      canDeleteOfficialBinding: true,
      'onUpdate:visible': () => undefined,
      onSyncOfficial: (item: OfficialProfileBinding) => synced.push(item),
      onUnbindOfficial: (item: OfficialProfileBinding) => unbound.push(item),
    })
    await flushUI()

    const dialog = document.body.querySelector<HTMLElement>('.ui-dialog--viewer')
    expect(dialog?.textContent).toContain('正版角色')
    expect(dialog?.textContent).toContain('Xbox account')
    findButton(dialog!, '同步正版数据').click()
    findButton(dialog!, '解除绑定').click()
    await nextTick()
    expect(synced).toEqual([binding])
    expect(unbound).toEqual([binding])
    mounted.unmount()
  })

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
  return {
    host,
    unmount: () => {
      app.unmount()
      host.remove()
    },
  }
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
