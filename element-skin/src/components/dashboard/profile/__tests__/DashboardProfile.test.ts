import { createApp, nextTick, ref } from 'vue'
import ElementPlus, { ElMessage } from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { User } from '@/api/types'
import fixtures from '../../../../../../testdata/password-policy.json'
import DashboardProfile from '../DashboardProfile.vue'
import { accountDeletionFailures } from './fixtures/accountDeletion'

const mocks = vi.hoisted(() => ({
  changePassword: vi.fn(),
  patchMe: vi.fn(),
  deleteMe: vi.fn(),
  sendEmailChangeCode: vi.fn(),
  changeEmail: vi.fn(),
  siteLogout: vi.fn(),
}))
vi.mock('@/api/me', () => mocks)
vi.mock('@/api/auth', () => ({ siteLogout: mocks.siteLogout }))
vi.mock('@/composables/useAvatar', () => ({
  useAvatar: () => ({ currentAvatarImg: ref('') }),
}))
vi.mock('@/easter-eggs', () => ({
  isEasterEggDisabled: () => false,
  setEasterEggDisabled: vi.fn(),
}))

beforeEach(() => {
  vi.clearAllMocks()
  mocks.changePassword.mockResolvedValue({ data: null })
  mocks.patchMe.mockResolvedValue({ data: null })
  mocks.deleteMe.mockResolvedValue({ data: null })
})

interface ProfileSetup {
  form: {
    display_name: string
    old_password: string
    new_password: string
    confirm_password: string
  }
  showDeleteDialog: boolean
  deleteConfirmText: string
  updateProfile: () => Promise<void>
  confirmDeleteAccount: () => Promise<void>
}

describe('profile password and account deletion', () => {
  it.each(fixtures.filter((fixture) => fixture.entrypoint_test && fixture.password !== ''))(
    'password: $name',
    async (fixture) => {
      if (fixture.basic_errors.length > 0) {
        mocks.changePassword.mockRejectedValue({
          response: {
            data: {
              error: {
                object: 'password',
                operation: 'validate',
                reason: 'invalid',
                params: { rules: fixture.basic_errors },
              },
            },
          },
        })
      }
      const mounted = await mountPage()
      try {
        Object.assign(mounted.setup.form, {
          old_password: 'Password123',
          new_password: fixture.password,
          confirm_password: fixture.password,
        })
        await mounted.setup.updateProfile()
        const accepted = fixture.basic_errors.length === 0
        expect(mocks.changePassword).toHaveBeenCalledTimes(1)
        expect(mocks.changePassword).toHaveBeenCalledWith({
          old_password: 'Password123',
          new_password: fixture.password,
        })
        expect(mocks.patchMe).toHaveBeenCalledTimes(accepted ? 1 : 0)
        if (accepted) {
          expect(mocks.changePassword).toHaveBeenCalledWith({
            old_password: 'Password123',
            new_password: fixture.password,
          })
          expect(mocks.patchMe).toHaveBeenCalledWith({ display_name: 'ProfileUser' })
          expect(mounted.setup.form.new_password).toBe('')
        } else {
          expect(document.body.textContent).toContain('密码的UTF-8编码不能超过72字节')
          expect(mounted.setup.form.new_password).toBe(fixture.password)
        }
      } finally {
        mounted.unmount()
      }
    },
  )

  it('clears the local session after deletion without requesting logout or reloading', async () => {
    const mounted = await mountPage()
    const events: string[] = []
    mocks.deleteMe.mockImplementation(async () => {
      events.push('delete')
      return { data: null }
    })
    mounted.clearSessionState.mockImplementation(() => {
      events.push('clear')
      expect(mounted.setup.showDeleteDialog).toBe(false)
      expect(mounted.setup.deleteConfirmText).toBe('')
      mounted.user.value = null
    })
    try {
      mounted.setup.showDeleteDialog = true
      mounted.setup.deleteConfirmText = '注销账号'
      await mounted.setup.confirmDeleteAccount()
      expect(mocks.deleteMe).toHaveBeenCalledTimes(1)
      expect(mounted.clearSessionState).toHaveBeenCalledTimes(1)
      expect(mocks.siteLogout).not.toHaveBeenCalled()
      expect(mounted.router.currentRoute.value.path).toBe('/')
      expect(mounted.replace).toHaveBeenCalledExactlyOnceWith('/')
      expect(mounted.user.value).toBeNull()
      expect(events).toEqual(['delete', 'clear'])
    } finally {
      mounted.unmount()
    }
  })

  it.each(accountDeletionFailures)(
    'keeps account state when deletion fails: $error.reason',
    async ({ error, message }) => {
      const mounted = await mountPage()
      mocks.deleteMe.mockRejectedValue({
        response: {
          data: { error },
        },
      })
      try {
        mounted.setup.showDeleteDialog = true
        mounted.setup.deleteConfirmText = '注销账号'
        await mounted.setup.confirmDeleteAccount()
        expect(mocks.deleteMe).toHaveBeenCalledTimes(1)
        expect(mounted.clearSessionState).not.toHaveBeenCalled()
        expect(mocks.siteLogout).not.toHaveBeenCalled()
        expect(mounted.replace).not.toHaveBeenCalled()
        expect(mounted.router.currentRoute.value.path).toBe('/dashboard/profile')
        expect(mounted.user.value?.id).toBe('profile-user')
        expect(mounted.setup.showDeleteDialog).toBe(true)
        expect(mounted.setup.deleteConfirmText).toBe('注销账号')
        expect(document.body.textContent).toContain('注销失败: ' + message)
      } finally {
        mounted.unmount()
      }
    },
  )
})

async function mountPage() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/dashboard/profile', component: DashboardProfile },
      { path: '/', component: { template: '<div>home</div>' } },
    ],
  })
  await router.push('/dashboard/profile')
  const host = document.createElement('div')
  document.body.appendChild(host)
  const user = ref<User | null>({
    id: 'profile-user',
    email: 'profile@example.com',
    display_name: 'ProfileUser',
    permissions: ['account.update.self', 'account.delete.self'],
  } as User)
  const clearSessionState = vi.fn(() => {
    user.value = null
  })
  const replace = vi.spyOn(router, 'replace')
  const app = createApp(DashboardProfile)
  app.use(router)
  app.use(ElementPlus)
  app.provide('user', user)
  app.provide('clearSessionState', clearSessionState)
  app.mount(host)
  await nextTick()
  return {
    user,
    clearSessionState,
    router,
    replace,
    setup: app._instance?.setupState as unknown as ProfileSetup,
    unmount() {
      ElMessage.closeAll()
      app.unmount()
      host.remove()
    },
  }
}
