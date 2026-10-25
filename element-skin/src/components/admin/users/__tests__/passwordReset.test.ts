import { createApp, nextTick } from 'vue'
import ElementPlus, { ElMessage } from 'element-plus'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import fixtures from '../../../../../../testdata/password-policy.json'
import AdminUserList from '../AdminUserList.vue'

const mocks = vi.hoisted(() => ({
  getUsers: vi.fn(),
  getUser: vi.fn(),
  getUserProfiles: vi.fn(),
  getUserPermissions: vi.fn(),
  grantUserRole: vi.fn(),
  revokeUserRole: vi.fn(),
  transferProtectedSubject: vi.fn(),
  setUserPermissionOverride: vi.fn(),
  clearUserPermissionOverride: vi.fn(),
  deleteUser: vi.fn(),
  banUser: vi.fn(),
  unbanUser: vi.fn(),
  resetUserPassword: vi.fn(),
}))
vi.mock('@/api/admin/users', () => mocks)
vi.mock('@/composables/useAvatar', () => ({ getAvatarForHash: vi.fn() }))

beforeEach(() => {
  vi.clearAllMocks()
  mocks.getUsers.mockResolvedValue({
    data: { items: [], has_next: false, next_cursor: '', page_size: 15 },
  })
  mocks.resetUserPassword.mockResolvedValue({ data: null })
})

interface ResetSetup {
  currentUser: { id: string }
  resetPasswordForm: { new_password: string; confirm_password: string }
  resetPasswordDialogVisible: boolean
  resetting: boolean
  confirmResetPassword: () => Promise<void>
}

describe('administrator password reset', () => {
  it.each(fixtures.filter((fixture) => fixture.entrypoint_test))('$name', async (fixture) => {
    if (fixture.password !== '' && fixture.basic_errors.length > 0) {
      mocks.resetUserPassword.mockRejectedValue({
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
      mounted.setup.resetPasswordForm = {
        new_password: fixture.password,
        confirm_password: fixture.password,
      }
      await mounted.setup.confirmResetPassword()
      const accepted = fixture.basic_errors.length === 0
      expect(mocks.resetUserPassword).toHaveBeenCalledTimes(fixture.password !== '' ? 1 : 0)
      expect(mounted.setup.resetPasswordDialogVisible).toBe(!accepted)
      expect(mounted.setup.resetting).toBe(false)
      if (fixture.password !== '') {
        expect(mocks.resetUserPassword).toHaveBeenCalledWith({
          user_id: 'password-user',
          new_password: fixture.password,
        })
      }
      if (!accepted) {
        expect(document.body.textContent).toContain(
          fixture.password === '' ? '请输入新密码' : '密码的UTF-8编码不能超过72字节',
        )
        expect(mounted.setup.resetPasswordForm.new_password).toBe(fixture.password)
      }
    } finally {
      mounted.unmount()
    }
  })

  it('shows the backend composition rejection and preserves the dialog input', async () => {
    const fixture = fixtures.find((fixture) => fixture.name === 'eight digits')!
    mocks.resetUserPassword.mockRejectedValue({
      response: {
        data: {
          error: {
            object: 'password',
            operation: 'validate',
            reason: 'invalid',
            params: { rules: fixture.strong_errors },
          },
        },
      },
    })
    const mounted = await mountPage()
    try {
      mounted.setup.resetPasswordForm = {
        new_password: fixture.password,
        confirm_password: fixture.password,
      }
      await mounted.setup.confirmResetPassword()
      expect(mocks.resetUserPassword).toHaveBeenCalledTimes(1)
      expect(document.body.textContent).toContain(
        '密码需包含小写字母、大写字母、数字、标点或符号中的至少三类',
      )
      expect(mounted.setup.resetPasswordDialogVisible).toBe(true)
      expect(mounted.setup.resetPasswordForm.new_password).toBe(fixture.password)
      expect(mounted.setup.resetting).toBe(false)
    } finally {
      mounted.unmount()
    }
  })
})

async function mountPage() {
  const host = document.createElement('div')
  document.body.appendChild(host)
  const app = createApp(AdminUserList)
  app.use(ElementPlus)
  app.mount(host)
  await Promise.resolve()
  await nextTick()
  const setup = app._instance?.setupState as unknown as ResetSetup
  setup.currentUser = { id: 'password-user' }
  setup.resetPasswordDialogVisible = true
  return {
    setup,
    unmount() {
      ElMessage.closeAll()
      app.unmount()
      host.remove()
    },
  }
}
