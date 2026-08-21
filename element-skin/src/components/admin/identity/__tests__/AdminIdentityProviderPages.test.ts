import { createApp, h, nextTick, ref } from 'vue'
import ElementPlus from 'element-plus'
import { createMemoryHistory, createRouter, RouterView, type RouteRecordRaw } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AdminIdentityProvider, User } from '@/api/types'
import AdminIdentityProviderForm from '../AdminIdentityProviderForm.vue'
import AdminIdentityProviders from '../AdminIdentityProviders.vue'

const adminApiMocks = vi.hoisted(() => ({
  createIdentityProvider: vi.fn(),
  deleteIdentityProvider: vi.fn(),
  getAdminIdentityProvider: vi.fn(),
  getAdminIdentityProviders: vi.fn(),
  updateIdentityProvider: vi.fn(),
}))
const identityApiMocks = vi.hoisted(() => ({
  getIdentityProviders: vi.fn(),
}))
const oauthApiMocks = vi.hoisted(() => ({
  getOpenIDConfiguration: vi.fn(),
}))

vi.mock('@/api/admin/identity-providers', () => adminApiMocks)
vi.mock('@/api/identity', () => identityApiMocks)
vi.mock('@/api/oauth', () => oauthApiMocks)

const redirectUri = 'https://skin.example/api/v2/auth/oidc/callback'
const provider: AdminIdentityProvider = {
  id: 'provider-1',
  name: 'Microsoft',
  adapter: 'microsoft',
  icon_url: '',
  login_enabled: true,
  link_enabled: true,
  issuer_url: 'https://login.microsoftonline.com/9188040d-6c67-4c5b-b112-36a304b66dad/v2.0',
  authorization_endpoint: 'https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize',
  token_endpoint: 'https://login.microsoftonline.com/consumers/oauth2/v2.0/token',
  userinfo_endpoint: 'https://graph.microsoft.com/oidc/userinfo',
  jwks_uri: 'https://login.microsoftonline.com/consumers/discovery/v2.0/keys',
  client_id: 'client-id',
  has_client_secret: true,
  scopes: ['openid', 'profile', 'email', 'XboxLive.signin', 'offline_access'],
  enabled: true,
  display_order: 1,
  created_at: 1000,
  updated_at: 1000,
}

beforeEach(() => {
  vi.clearAllMocks()
  adminApiMocks.getAdminIdentityProviders.mockResolvedValue({
    data: { items: [provider], redirect_uri: redirectUri },
  })
  adminApiMocks.getAdminIdentityProvider.mockResolvedValue({ data: provider })
  identityApiMocks.getIdentityProviders.mockResolvedValue({
    data: { items: [], redirect_uri: redirectUri },
  })
  oauthApiMocks.getOpenIDConfiguration.mockResolvedValue({
    data: {
      issuer: 'https://skin.example/api',
      authorization_endpoint: 'https://skin.example/oauth/authorize',
      token_endpoint: 'https://skin.example/api/oauth/token',
      userinfo_endpoint: 'https://skin.example/api/oauth/userinfo',
      jwks_uri: 'https://skin.example/api/oauth/jwks',
      revocation_endpoint: 'https://skin.example/api/oauth/revoke',
      response_types_supported: ['code'],
      grant_types_supported: ['authorization_code', 'refresh_token'],
      subject_types_supported: ['pairwise'],
      scopes_supported: ['openid', 'profile', 'email', 'offline_access'],
    },
  })
})

describe('OIDC identity provider pages', () => {
  it('explains both OIDC roles with exact discovery endpoints and no Microsoft banner', async () => {
    const mounted = await mountPage('/admin/identity-providers', providerRoutes(), [
      'identity_provider.read.any',
    ])
    await flushUI()

    expect(mounted.root.textContent).toContain('本站作为 OIDC Provider')
    expect(mounted.root.textContent).toContain(
      'https://skin.example/api/.well-known/openid-configuration',
    )
    expect(mounted.root.textContent).toContain('https://skin.example/oauth/authorize')
    expect(mounted.root.textContent).toContain('本站作为 OIDC Client')
    expect(mounted.root.textContent).not.toContain('Microsoft 也是普通 OIDC 身份提供方')
    expect(oauthApiMocks.getOpenIDConfiguration).toHaveBeenCalledTimes(1)
    mounted.unmount()
  })

  it('navigates from the provider list to standalone create and edit routes', async () => {
    const createPage = await mountPage('/admin/identity-providers', providerRoutes(), [
      'identity_provider.read.any',
      'identity_provider.create.any',
      'identity_provider.update.any',
    ])
    await flushUI()

    findButton(createPage.root, '添加提供方').click()
    await flushUI()
    expect(createPage.router.currentRoute.value.name).toBe('admin-identity-provider-create')
    createPage.unmount()

    const editPage = await mountPage('/admin/identity-providers', providerRoutes(), [
      'identity_provider.read.any',
      'identity_provider.update.any',
    ])
    await flushUI()
    findButton(editPage.root, '编辑').click()
    await flushUI()
    expect(editPage.router.currentRoute.value.name).toBe('admin-identity-provider-edit')
    expect(editPage.router.currentRoute.value.params.provider_id).toBe(provider.id)
    expect(adminApiMocks.getAdminIdentityProvider).toHaveBeenCalledTimes(1)
    expect(adminApiMocks.getAdminIdentityProvider).toHaveBeenCalledWith(provider.id)
    editPage.unmount()
  })

  it('creates a provider with the exact standalone form payload', async () => {
    adminApiMocks.createIdentityProvider.mockResolvedValue({ data: provider })
    const mounted = await mountPage('/admin/identity-providers/new', providerRoutes(), [
      'identity_provider.create.any',
    ])
    await flushUI()

    setInputValue(inputForLabel(mounted.root, '显示名称'), ' Generic Provider ')
    setInputValue(inputForLabel(mounted.root, 'Issuer URL'), ' https://issuer.example ')
    setInputValue(inputForLabel(mounted.root, 'Client ID'), ' client-id ')
    setInputValue(inputForLabel(mounted.root, 'Client Secret'), 'client-secret')
    await nextTick()
    findButton(mounted.root, '添加提供方').click()
    await flushUI()

    expect(adminApiMocks.createIdentityProvider).toHaveBeenCalledTimes(1)
    expect(adminApiMocks.createIdentityProvider).toHaveBeenCalledWith({
      name: 'Generic Provider',
      issuer_url: 'https://issuer.example',
      client_id: 'client-id',
      client_secret: 'client-secret',
      scopes: ['openid', 'profile', 'email'],
      adapter: 'generic_oidc',
      icon_url: '',
      enabled: true,
      login_enabled: true,
      link_enabled: true,
      display_order: 0,
    })
    expect(mounted.router.currentRoute.value.name).toBe('admin-identity-provider-edit')
    mounted.unmount()
  })
})

function providerRoutes(): RouteRecordRaw[] {
  return [
    {
      path: '/admin/identity-providers',
      name: 'admin-identity-providers',
      component: AdminIdentityProviders,
    },
    {
      path: '/admin/identity-providers/new',
      name: 'admin-identity-provider-create',
      component: AdminIdentityProviderForm,
    },
    {
      path: '/admin/identity-providers/:provider_id/edit',
      name: 'admin-identity-provider-edit',
      component: AdminIdentityProviderForm,
    },
  ]
}

async function mountPage(path: string, routes: RouteRecordRaw[], permissions: string[]) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { render: () => h('div') } }, ...routes],
  })
  const host = document.createElement('div')
  document.body.appendChild(host)
  const app = createApp({ render: () => h(RouterView) })
  app.use(router)
  app.use(ElementPlus)
  app.provide('user', ref({ id: 'user-1', permissions } as unknown as User))
  await router.push(path)
  await router.isReady()
  app.mount(host)
  return {
    root: host,
    router,
    unmount: () => {
      app.unmount()
      host.remove()
    },
  }
}

function findButton(root: HTMLElement, label: string) {
  const button = [...root.querySelectorAll('button')].find((item) =>
    item.textContent?.includes(label),
  )
  if (!button) throw new Error(`button not found: ${label}`)
  return button
}

function setInputValue(input: HTMLInputElement, value: string) {
  input.value = value
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

function inputForLabel(root: HTMLElement, label: string) {
  const formItem = [...root.querySelectorAll<HTMLElement>('.el-form-item')].find((item) =>
    item.querySelector('label')?.textContent?.includes(label),
  )
  const input = formItem?.querySelector<HTMLInputElement>('input.el-input__inner')
  if (!input) throw new Error(`input not found: ${label}`)
  return input
}

async function flushUI() {
  await Promise.resolve()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
}
