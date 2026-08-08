import { createApp, h, nextTick, ref } from 'vue'
import ElementPlus, { ElMessageBox } from 'element-plus'
import { createMemoryHistory, createRouter, RouterView, type RouteRecordRaw } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { OAuthClient, OAuthGrant } from '@/api/oauth'
import type { PermissionDefinition, User } from '@/api/types'
import DashboardOAuthAppForm from '../DashboardOAuthAppForm.vue'
import DashboardOAuthApps from '../DashboardOAuthApps.vue'

const apiMocks = vi.hoisted(() => ({
  createOAuthApp: vi.fn(),
  deleteOAuthApp: vi.fn(),
  getOAuthApp: vi.fn(),
  getOAuthWebhookEventCatalog: vi.fn(),
  getPermissionCatalog: vi.fn(),
  listOAuthApps: vi.fn(),
  listOAuthGrants: vi.fn(),
  revokeOAuthGrant: vi.fn(),
  rotateOAuthSecret: vi.fn(),
  submitOAuthAppReview: vi.fn(),
  updateOAuthApp: vi.fn(),
}))

vi.mock('@/api/oauth', () => apiMocks)

const client: OAuthClient = {
  client_id: 'client-1',
  owner_user_id: 'user-1',
  name: 'Coverage app',
  description: 'Coverage description',
  redirect_uri: '',
  website_url: '',
  client_type: 'confidential',
  status: 'pending',
  created_at: 1000,
  updated_at: 1000,
  permissions: [],
  webhook_endpoints: [],
}

const activeGrant: OAuthGrant = {
  id: 'grant-1',
  user_id: 'user-1',
  subject_id: 'user:user-1',
  client_id: client.client_id,
  status: 'active',
  created_at: 1000,
  revoked_at: null,
  permissions: ['profile.read.owned'],
}

const profileReadPermission: PermissionDefinition = {
  id: 1,
  code: 'profile.read.owned',
  description: '读取自己的角色',
  bit_index: 1,
  resource: 'profile',
  resource_description: '角色',
  action: 'read',
  action_description: '读取',
  scope: 'user',
  scope_description: '用户',
}

beforeEach(() => {
  vi.clearAllMocks()
  apiMocks.getPermissionCatalog.mockResolvedValue({ data: { permissions: [] } })
  apiMocks.getOAuthWebhookEventCatalog.mockResolvedValue({ data: { events: [] } })
  apiMocks.listOAuthApps.mockResolvedValue({ data: { items: [client] } })
  apiMocks.listOAuthGrants.mockResolvedValue({ data: { items: [activeGrant] } })
  apiMocks.revokeOAuthGrant.mockResolvedValue({ data: undefined })
})

describe('OAuth application pages', () => {
  it('creates an application with the exact form payload and shows its one-time secret', async () => {
    apiMocks.createOAuthApp.mockResolvedValue({
      data: { ...client, client_secret: 'client-secret-once' },
    })
    const mounted = await mountPage('/dashboard/oauth/apps/new', [
      { path: '/dashboard/oauth', component: blankComponent() },
      {
        path: '/dashboard/oauth/apps/new',
        name: 'dashboard-oauth-app-create',
        component: DashboardOAuthAppForm,
      },
      {
        path: '/dashboard/oauth/apps/:client_id/edit',
        name: 'dashboard-oauth-app-edit',
        component: DashboardOAuthAppForm,
      },
    ])
    await flushUI()

    const nameInput = mounted.root.querySelector('input') as HTMLInputElement | null
    expect(nameInput).not.toBeNull()
    setInputValue(nameInput!, 'Created in component test')
    await nextTick()
    findButton(mounted.root, '提交审核').click()
    await flushUI()

    expect(apiMocks.createOAuthApp).toHaveBeenCalledTimes(1)
    expect(apiMocks.createOAuthApp).toHaveBeenCalledWith({
      name: 'Created in component test',
      description: '',
      redirect_uri: '',
      website_url: '',
      client_type: 'confidential',
      permissions: [],
      webhook_endpoints: [],
    })
    expect(mounted.router.currentRoute.value.name).toBe('dashboard-oauth-app-edit')
    expect(mounted.root.textContent).toContain('Client Secret 只显示一次')
    expect(
      [...mounted.root.querySelectorAll('input')].some(
        (input) => input.value === 'client-secret-once',
      ),
    ).toBe(true)
    mounted.unmount()
  })

  it('loads the application list, revokes a grant, and navigates to the editor', async () => {
    const mounted = await mountPage(
      '/dashboard/oauth',
      [
        { path: '/dashboard/oauth', name: 'dashboard-oauth', component: DashboardOAuthApps },
        {
          path: '/dashboard/oauth/apps/new',
          name: 'dashboard-oauth-app-create',
          component: blankComponent(),
        },
        {
          path: '/dashboard/oauth/apps/:client_id/edit',
          name: 'dashboard-oauth-app-edit',
          component: blankComponent(),
        },
      ],
      [
        'oauth_app.read.owned',
        'oauth_app.create.owned',
        'oauth_app.update.owned',
        'oauth_grant.read.owned',
        'oauth_grant.revoke.owned',
      ],
    )
    await flushUI()

    expect(mounted.root.textContent).toContain('Coverage app')
    expect(mounted.root.textContent).toContain('profile.read.owned')
    findButton(mounted.root, '撤销授权').click()
    await flushUI()
    expect(apiMocks.revokeOAuthGrant).toHaveBeenCalledTimes(1)
    expect(apiMocks.revokeOAuthGrant).toHaveBeenCalledWith('grant-1')
    expect(apiMocks.listOAuthGrants).toHaveBeenCalledTimes(2)

    findButton(mounted.root, '管理').click()
    await flushUI()
    expect(mounted.router.currentRoute.value.name).toBe('dashboard-oauth-app-edit')
    expect(mounted.router.currentRoute.value.params.client_id).toBe(client.client_id)
    mounted.unmount()
  })

  it('loads and resubmits an edited application, rotates its secret, and deletes it', async () => {
    const editable: OAuthClient = {
      ...client,
      name: 'Rejected app',
      status: 'rejected',
      permissions: ['profile.read.owned'],
      webhook_endpoints: [
        {
          id: 'wh_1',
          url: 'https://hooks.example/events',
          status: 'active',
          enabled: true,
          events: ['profile.updated'],
          created_at: 1000,
          updated_at: 1000,
        },
      ],
    }
    apiMocks.getPermissionCatalog.mockResolvedValue({
      data: { permissions: [profileReadPermission] },
    })
    apiMocks.getOAuthWebhookEventCatalog.mockResolvedValue({
      data: {
        events: [
          {
            type: 'profile.updated',
            description: '角色更新',
            required_permissions: ['profile.read.owned'],
          },
        ],
      },
    })
    apiMocks.getOAuthApp.mockResolvedValue({ data: editable })
    apiMocks.rotateOAuthSecret.mockResolvedValue({
      data: { ...editable, client_secret: 'rotated-secret' },
    })
    apiMocks.updateOAuthApp.mockImplementation(
      (_clientID: string, payload: Record<string, unknown>) =>
        Promise.resolve({ data: { ...editable, ...payload } }),
    )
    apiMocks.submitOAuthAppReview.mockResolvedValue({
      data: { ...editable, name: 'Edited app', status: 'pending' },
    })
    apiMocks.deleteOAuthApp.mockResolvedValue({ data: undefined })
    vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm')

    const mounted = await mountPage(
      '/dashboard/oauth/apps/client-1/edit',
      [
        { path: '/dashboard/oauth', name: 'dashboard-oauth', component: blankComponent() },
        {
          path: '/dashboard/oauth/apps/:client_id/edit',
          name: 'dashboard-oauth-app-edit',
          component: DashboardOAuthAppForm,
        },
      ],
      ['oauth_app.read.owned', 'oauth_app.update.owned', 'oauth_app.delete.owned'],
    )
    await flushUI()

    expect(mounted.root.textContent).toContain('应用审核未通过')
    expect(
      [...mounted.root.querySelectorAll('input')].some(
        (input) => input.value === 'https://hooks.example/events',
      ),
    ).toBe(true)
    findButton(mounted.root, '添加 endpoint').click()
    await nextTick()
    const removeButtons = [...mounted.root.querySelectorAll('button')].filter((button) =>
      button.textContent?.includes('移除'),
    )
    expect(removeButtons).toHaveLength(2)
    removeButtons[1].click()
    await nextTick()
    findButton(mounted.root, '轮换 Client Secret').click()
    await flushUI()
    expect(apiMocks.rotateOAuthSecret).toHaveBeenCalledWith(client.client_id)
    expect(
      [...mounted.root.querySelectorAll('input')].some((input) => input.value === 'rotated-secret'),
    ).toBe(true)

    const nameInput = [...mounted.root.querySelectorAll('input')].find(
      (input) => input.value === editable.name,
    ) as HTMLInputElement
    setInputValue(nameInput, 'Edited app')
    await nextTick()
    findButton(mounted.root, '保存并重新提交').click()
    await flushUI()
    expect(apiMocks.updateOAuthApp).toHaveBeenCalledWith(client.client_id, {
      name: 'Edited app',
      description: editable.description,
      redirect_uri: '',
      website_url: '',
      client_type: 'confidential',
      permissions: ['profile.read.owned'],
      webhook_endpoints: [
        {
          id: 'wh_1',
          url: 'https://hooks.example/events',
          enabled: true,
          events: ['profile.updated'],
        },
      ],
    })
    expect(apiMocks.submitOAuthAppReview).toHaveBeenCalledWith(client.client_id)

    findButton(mounted.root, '删除应用').click()
    await flushUI()
    expect(ElMessageBox.confirm).toHaveBeenCalledTimes(1)
    expect(apiMocks.deleteOAuthApp).toHaveBeenCalledWith(client.client_id)
    expect(mounted.router.currentRoute.value.name).toBe('dashboard-oauth')
    mounted.unmount()
  })

  it('loads only permitted sections and exposes only permitted actions', async () => {
    const routes: RouteRecordRaw[] = [
      { path: '/dashboard/oauth', name: 'dashboard-oauth', component: DashboardOAuthApps },
      {
        path: '/dashboard/oauth/apps/new',
        name: 'dashboard-oauth-app-create',
        component: blankComponent(),
      },
      {
        path: '/dashboard/oauth/apps/:client_id/edit',
        name: 'dashboard-oauth-app-edit',
        component: blankComponent(),
      },
    ]

    const appReader = await mountPage('/dashboard/oauth', routes, ['oauth_app.read.owned'])
    await flushUI()
    expect(apiMocks.listOAuthApps).toHaveBeenCalledTimes(1)
    expect(apiMocks.listOAuthGrants).not.toHaveBeenCalled()
    expect(appReader.root.textContent).toContain('Coverage app')
    expect(buttonText(appReader.root)).not.toContain('申请新应用')
    expect(buttonText(appReader.root)).not.toContain('管理')
    appReader.unmount()

    vi.clearAllMocks()
    apiMocks.getPermissionCatalog.mockResolvedValue({ data: { permissions: [] } })
    apiMocks.listOAuthApps.mockResolvedValue({ data: { items: [client] } })
    apiMocks.listOAuthGrants.mockResolvedValue({ data: { items: [activeGrant] } })
    const grantReader = await mountPage('/dashboard/oauth', routes, ['oauth_grant.read.owned'])
    await flushUI()
    expect(apiMocks.listOAuthApps).not.toHaveBeenCalled()
    expect(apiMocks.listOAuthGrants).toHaveBeenCalledTimes(1)
    expect(grantReader.root.textContent).toContain('profile.read.owned')
    expect(buttonText(grantReader.root)).not.toContain('撤销授权')
    grantReader.unmount()

    vi.clearAllMocks()
    apiMocks.getPermissionCatalog.mockResolvedValue({ data: { permissions: [] } })
    const creator = await mountPage('/dashboard/oauth', routes, ['oauth_app.create.owned'])
    await flushUI()
    expect(apiMocks.listOAuthApps).not.toHaveBeenCalled()
    expect(apiMocks.listOAuthGrants).not.toHaveBeenCalled()
    expect(buttonText(creator.root)).toContain('申请新应用')
    creator.unmount()
  })

  it('keeps an application read-only when the user can only read and delete it', async () => {
    apiMocks.getOAuthApp.mockResolvedValue({ data: { ...client, status: 'active' } })
    apiMocks.deleteOAuthApp.mockResolvedValue({ data: undefined })
    vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm')
    const mounted = await mountPage(
      '/dashboard/oauth/apps/client-1/edit',
      [
        { path: '/dashboard/oauth', name: 'dashboard-oauth', component: blankComponent() },
        {
          path: '/dashboard/oauth/apps/:client_id/edit',
          name: 'dashboard-oauth-app-edit',
          component: DashboardOAuthAppForm,
        },
      ],
      ['oauth_app.read.owned', 'oauth_app.delete.owned'],
    )
    await flushUI()

    const nameInput = [...mounted.root.querySelectorAll('input')].find(
      (input) => input.value === client.name,
    )
    expect(nameInput?.disabled).toBe(true)
    expect(buttonText(mounted.root)).toContain('删除应用')
    expect(buttonText(mounted.root)).not.toContain('轮换 Client Secret')
    expect(buttonText(mounted.root)).not.toContain('仅保存')
    expect(buttonText(mounted.root)).not.toContain('保存修改')
    findButton(mounted.root, '删除应用').click()
    await flushUI()
    expect(apiMocks.deleteOAuthApp).toHaveBeenCalledWith(client.client_id)
    mounted.unmount()
  })
})

async function mountPage(
  path: string,
  routes: RouteRecordRaw[],
  permissions: string[] = ['oauth_app.create.owned'],
) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: blankComponent() }, ...routes],
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

function blankComponent() {
  return { render: () => h('div') }
}

function findButton(root: HTMLElement, text: string) {
  const button = [...root.querySelectorAll('button')].find((item) =>
    item.textContent?.includes(text),
  )
  if (!button) throw new Error(`button not found: ${text}`)
  return button
}

function buttonText(root: HTMLElement) {
  return [...root.querySelectorAll('button')].map((button) => button.textContent?.trim() ?? '')
}

function setInputValue(input: HTMLInputElement, value: string) {
  input.value = value
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

async function flushUI() {
  await Promise.resolve()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
}
