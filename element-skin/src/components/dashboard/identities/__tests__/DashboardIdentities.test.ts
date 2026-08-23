import { createApp, h, nextTick, ref } from 'vue'
import ElementPlus, { ElMessageBox } from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ExternalIdentity, IdentityProvider, User } from '@/api/types'
import DashboardIdentities from '../DashboardIdentities.vue'

const identityApiMocks = vi.hoisted(() => ({
  deleteExternalIdentity: vi.fn(),
  getExternalIdentities: vi.fn(),
  getIdentityProviders: vi.fn(),
  patchExternalIdentity: vi.fn(),
  startIdentityAuthorization: vi.fn(),
}))
const officialApiMocks = vi.hoisted(() => ({
  getOfficialProfileBindings: vi.fn(),
}))

vi.mock('@/api/identity', () => identityApiMocks)
vi.mock('@/api/official-profiles', () => officialApiMocks)

const provider: IdentityProvider = {
  id: 'microsoft-provider',
  name: 'Microsoft',
  adapter: 'microsoft',
  icon_url: '',
  login_enabled: true,
  link_enabled: true,
}
const identity: ExternalIdentity = {
  id: 'identity-1',
  provider_id: provider.id,
  provider_name: provider.name,
  provider_adapter: provider.adapter,
  provider_icon_url: '',
  provider_enabled: true,
  provider_link_enabled: true,
  subject: 'microsoft-subject',
  label: '主要账户',
  email: 'user@example.com',
  email_verified: true,
  display_name: 'Microsoft User',
  avatar_url: '',
  created_at: 1000,
  updated_at: 1000,
  last_login_at: 1000,
  authorization_status: 'active',
  last_refresh_at: 1000,
  last_refresh_error_at: null,
}

beforeEach(() => {
  vi.clearAllMocks()
  identityApiMocks.getIdentityProviders.mockResolvedValue({
    data: { items: [provider], redirect_uri: 'https://skin.example/api/v2/auth/oidc/callback' },
  })
  identityApiMocks.getExternalIdentities.mockResolvedValue({ data: { items: [identity] } })
  officialApiMocks.getOfficialProfileBindings.mockResolvedValue({ data: { items: [] } })
})

describe('DashboardIdentities', () => {
  it('loads the permitted identity state exactly once without starting a mutation', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/dashboard/identities', component: DashboardIdentities },
        { path: '/dashboard/roles', component: { render: () => h('div') } },
      ],
    })
    const host = document.createElement('div')
    document.body.appendChild(host)
    const app = createApp({ render: () => h(DashboardIdentities) })
    app.use(router)
    app.use(ElementPlus)
    app.provide(
      'user',
      ref({
        id: 'user-1',
        permissions: [
          'external_identity.read.owned',
          'external_identity.create.owned',
          'external_identity.update.owned',
          'external_identity.delete.owned',
          'official_profile.read.owned',
        ],
      } as User),
    )
    await router.push('/dashboard/identities')
    await router.isReady()
    app.mount(host)
    await flushUI()

    expect(identityApiMocks.getIdentityProviders).toHaveBeenCalledTimes(1)
    expect(identityApiMocks.getIdentityProviders).toHaveBeenCalledWith()
    expect(identityApiMocks.getExternalIdentities).toHaveBeenCalledTimes(1)
    expect(identityApiMocks.getExternalIdentities).toHaveBeenCalledWith()
    expect(officialApiMocks.getOfficialProfileBindings).toHaveBeenCalledTimes(1)
    expect(officialApiMocks.getOfficialProfileBindings).toHaveBeenCalledWith()
    expect(identityApiMocks.startIdentityAuthorization).not.toHaveBeenCalled()
    expect(identityApiMocks.patchExternalIdentity).not.toHaveBeenCalled()
    expect(identityApiMocks.deleteExternalIdentity).not.toHaveBeenCalled()

    app.unmount()
    host.remove()
  })

  it('deletes an identity after confirming that bindings detach and roles remain', async () => {
    const confirm = vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm')
    identityApiMocks.deleteExternalIdentity.mockResolvedValue({ data: null })
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/dashboard/identities', component: DashboardIdentities },
        { path: '/dashboard/roles', component: { render: () => h('div') } },
      ],
    })
    const host = document.createElement('div')
    document.body.appendChild(host)
    const app = createApp({ render: () => h(DashboardIdentities) })
    app.use(router)
    app.use(ElementPlus)
    app.provide(
      'user',
      ref({
        id: 'user-1',
        permissions: [
          'external_identity.read.owned',
          'external_identity.delete.owned',
          'official_profile.read.owned',
        ],
      } as User),
    )
    await router.push('/dashboard/identities')
    await router.isReady()
    app.mount(host)
    await flushUI()

    const deleteButton = [...host.querySelectorAll('button')].find(
      (button) => button.textContent?.trim() === '删除',
    )
    expect(deleteButton).toBeDefined()
    deleteButton?.click()
    await flushUI()
    await flushUI()

    expect(confirm).toHaveBeenCalledTimes(1)
    expect(confirm).toHaveBeenCalledWith(
      '删除身份会同时解除它关联的正版角色关系，但不会删除任何本站角色。此操作不可撤销。',
      '删除外部身份',
      { type: 'warning', confirmButtonText: '删除身份', cancelButtonText: '取消' },
    )
    expect(identityApiMocks.deleteExternalIdentity).toHaveBeenCalledTimes(1)
    expect(identityApiMocks.deleteExternalIdentity).toHaveBeenCalledWith(identity.id)
    expect(identityApiMocks.getExternalIdentities).toHaveBeenCalledTimes(2)

    app.unmount()
    host.remove()
    confirm.mockRestore()
  })
})

async function flushUI() {
  await Promise.resolve()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
}
