import { createApp, defineComponent, h, nextTick, reactive, type VNode } from 'vue'
import ElementPlus, { ElMessage, ElMessageBox } from 'element-plus'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import CreateRoleDialog from '@/components/dashboard/roles/CreateRoleDialog.vue'
import DashboardRoles from '@/components/dashboard/roles/DashboardRoles.vue'
import EmailChangeDialog from '@/components/dashboard/profile/EmailChangeDialog.vue'
import ResetPasswordDialog from '@/components/admin/users/ResetPasswordDialog.vue'
import HomepageMediaDialog from '@/components/admin/homepage/HomepageMediaDialog.vue'
import AdminHomepageMedia from '@/components/admin/homepage/AdminHomepageMedia.vue'
import AdminNotices from '@/components/admin/notices/AdminNotices.vue'
import AdminRolesList from '@/components/admin/roles/AdminRolesList.vue'
import {
  boundProfile,
  otherProfile,
  emailPolicyResponse,
  resetPasswords,
  mediaItem,
  noticeInput,
  expectedNoticeDraft,
  expectedMediaPatch,
} from './fixtures/dialogFlows'

const api = vi.hoisted(() => ({
  getPublicSettings: vi.fn(),
  sendEmailChangeCode: vi.fn(),
  changeEmail: vi.fn(),
  getMe: vi.fn(),
  getProfiles: vi.fn(),
  createProfile: vi.fn(),
  getAdminNotices: vi.fn(),
  createAdminNotice: vi.fn(),
  patchAdminNotice: vi.fn(),
  deleteAdminNotice: vi.fn(),
  getAdminProfiles: vi.fn(),
  patchAdminProfile: vi.fn(),
  deleteAdminProfile: vi.fn(),
  patchProfileSkin: vi.fn(),
  patchProfileCape: vi.fn(),
  listHomepageMedia: vi.fn(),
  patchHomepageMedia: vi.fn(),
  reorderHomepageMedia: vi.fn(),
}))
vi.mock('@/api/public', () => ({ getPublicSettings: api.getPublicSettings }))
vi.mock('@/api/me', () => ({
  getMe: api.getMe,
  sendEmailChangeCode: api.sendEmailChangeCode,
  changeEmail: api.changeEmail,
}))
vi.mock('@/api/profiles', () => ({
  getProfiles: api.getProfiles,
  createProfile: api.createProfile,
  patchProfile: vi.fn(),
  deleteProfile: vi.fn(),
  clearProfileSkin: vi.fn(),
  clearProfileCape: vi.fn(),
}))
vi.mock('@/composables/useAvatar', () => ({ useAvatar: () => ({ setAvatar: vi.fn() }) }))
vi.mock('@/api/admin/notices', () => ({
  getAdminNotices: api.getAdminNotices,
  createAdminNotice: api.createAdminNotice,
  patchAdminNotice: api.patchAdminNotice,
  deleteAdminNotice: api.deleteAdminNotice,
}))
vi.mock('@/api/admin/profiles', () => ({
  getAdminProfiles: api.getAdminProfiles,
  patchAdminProfile: api.patchAdminProfile,
  deleteAdminProfile: api.deleteAdminProfile,
  patchProfileSkin: api.patchProfileSkin,
  patchProfileCape: api.patchProfileCape,
}))
vi.mock('@/api/admin/homepage-media', () => ({
  listHomepageMedia: api.listHomepageMedia,
  patchHomepageMedia: api.patchHomepageMedia,
  reorderHomepageMedia: api.reorderHomepageMedia,
  uploadHomepageImage: vi.fn(),
  uploadHomepagePanorama: vi.fn(),
  deleteHomepageMedia: vi.fn(),
}))
vi.mock('@/components/SkinViewer.vue', () => ({
  default: defineComponent({
    props: ['skinUrl', 'capeUrl'],
    setup: (props) => () =>
      h('div', {
        class: 'test-skin-viewer',
        'data-skin': props.skinUrl,
        'data-cape': props.capeUrl,
      }),
  }),
}))

const cleanup: Array<() => void> = []
beforeEach(() => {
  vi.clearAllMocks()
  api.getPublicSettings.mockResolvedValue(structuredClone(emailPolicyResponse))
  api.getMe.mockResolvedValue({
    data: { id: 'user-a', email: 'user@example.com', permissions: ['profile.create.owned'] },
  })
  api.getProfiles.mockResolvedValue({
    data: { items: [], has_next: false, next_cursor: null, page_size: 20 },
  })
  api.sendEmailChangeCode.mockResolvedValue({ data: { ttl: 60 } })
  api.changeEmail.mockResolvedValue({ data: undefined })
  api.getAdminNotices.mockResolvedValue({
    data: { items: [], has_next: false, next_cursor: null, page_size: 20 },
  })
  api.getAdminProfiles.mockImplementation(async () => ({
    data: {
      items: [structuredClone(boundProfile), structuredClone(otherProfile)],
      has_next: false,
      next_cursor: null,
      page_size: 20,
    },
  }))
  api.patchProfileSkin.mockResolvedValue({ data: undefined })
  api.patchProfileCape.mockResolvedValue({ data: undefined })
  api.listHomepageMedia.mockImplementation(async () => ({
    data: { items: [structuredClone(mediaItem)] },
  }))
  api.patchHomepageMedia.mockResolvedValue({ data: { ...mediaItem, title: 'Edited Village' } })
  vi.spyOn(ElMessage, 'success').mockImplementation(() => ({ close: vi.fn() }))
  vi.spyOn(ElMessage, 'error').mockImplementation(() => ({ close: vi.fn() }))
  vi.spyOn(ElMessage, 'warning').mockImplementation(() => ({ close: vi.fn() }))
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  )
})
afterEach(() => {
  cleanup
    .splice(0)
    .reverse()
    .forEach((dispose) => dispose())
  vi.useRealTimers()
  vi.unstubAllGlobals()
  ElMessageBox.close()
})

async function settle() {
  await nextTick()
  await new Promise((resolve) => setTimeout(resolve, 60))
  await nextTick()
}
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((done, fail) => {
    resolve = done
    reject = fail
  })
  return { promise, resolve, reject }
}
async function mount(render: () => VNode) {
  const host = document.createElement('div')
  document.body.append(host)
  const app = createApp({ render })
  const pinia = createPinia()
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { render: () => null } }],
  })
  await router.push('/')
  app.use(pinia).use(router).use(ElementPlus)
  useSessionStore(pinia).user = {
    id: 'user-a',
    email: 'user@example.com',
    permissions: ['profile.create.owned'],
  }
  app.mount(host)
  const unmount = () => {
    app.unmount()
    host.remove()
  }
  cleanup.push(unmount)
  await settle()
  return { host, unmount }
}
function dialog() {
  const result = [...document.querySelectorAll<HTMLElement>('.ui-dialog')].find(
    (element) => element.closest<HTMLElement>('.el-overlay')?.style.display !== 'none',
  )
  if (!result) throw new Error('No visible dialog')
  return result
}
function button(root: ParentNode, label: string) {
  const result = [...root.querySelectorAll<HTMLButtonElement>('button')].find(
    (element) => element.textContent?.trim() === label,
  )
  if (!result) throw new Error(`No button: ${label}`)
  return result
}
function clickMask() {
  const mask = dialog().closest('.el-overlay-dialog')!
  for (const type of ['mousedown', 'mouseup', 'click'])
    mask.dispatchEvent(new MouseEvent(type, { bubbles: true }))
}
async function input(element: HTMLInputElement | HTMLTextAreaElement, value: string) {
  element.value = value
  element.dispatchEvent(new Event('input', { bubbles: true }))
  await nextTick()
}

describe('independent and sensitive form lifecycles', () => {
  it('creates one role request, retains input on API failure, and starts empty after success', async () => {
    const pending = deferred<never>()
    api.createProfile.mockReturnValueOnce(pending.promise).mockResolvedValueOnce({ data: {} })
    const mounted = await mount(() => h(DashboardRoles))
    button(mounted.host, '新建角色').click()
    await settle()
    await input(dialog().querySelector('input')!, 'NewPlayer')
    button(dialog(), '创建').click()
    button(dialog(), '创建').click()
    await settle()
    expect(api.createProfile).toHaveBeenCalledExactlyOnceWith({ name: 'NewPlayer' })
    dialog().querySelector<HTMLButtonElement>('.el-dialog__headerbtn')!.click()
    await settle()
    expect(dialog().querySelector('input')!.value).toBe('NewPlayer')
    pending.reject(new Error('role unavailable'))
    await settle()
    expect(ElMessage.error).toHaveBeenLastCalledWith('创建失败: role unavailable')
    expect(dialog().querySelector('input')!.value).toBe('NewPlayer')
    button(dialog(), '创建').click()
    await settle()
    expect(api.createProfile).toHaveBeenCalledTimes(2)
    button(mounted.host, '新建角色').click()
    await settle()
    expect(dialog().querySelector('input')!.value).toBe('')
  })
  it('keeps a cancelled mask draft, discards on explicit cancel, and starts empty on reopen', async () => {
    const confirm = vi.spyOn(ElMessageBox, 'confirm').mockRejectedValue('cancel')
    const state = reactive({ open: true, name: '', creating: false })
    await mount(() =>
      h(CreateRoleDialog, {
        visible: state.open,
        name: state.name,
        creating: state.creating,
        'onUpdate:visible': (v) => {
          state.open = v
        },
        'onUpdate:name': (v) => {
          state.name = v
        },
      }),
    )
    await input(dialog().querySelector('input')!, 'NewPlayer')
    clickMask()
    await settle()
    expect(state).toEqual({ open: true, name: 'NewPlayer', creating: false })
    expect(confirm).toHaveBeenCalledTimes(1)
    button(dialog(), '取消').click()
    await settle()
    expect(state.name).toBe('')
    expect(state.open).toBe(false)
    expect(confirm).toHaveBeenCalledTimes(1)
    state.open = true
    await settle()
    expect(dialog().querySelector('input')!.value).toBe('')
  })

  it('guards role creation controls and preserves failed input', async () => {
    const state = reactive({ open: true, name: 'NewPlayer', creating: false })
    const create = vi.fn(() => {
      state.creating = true
    })
    await mount(() =>
      h(CreateRoleDialog, {
        visible: state.open,
        name: state.name,
        creating: state.creating,
        onCreate: create,
        'onUpdate:visible': (v) => {
          state.open = v
        },
      }),
    )
    button(dialog(), '创建').click()
    await nextTick()
    button(dialog(), '创建').click()
    button(dialog(), '取消').click()
    dialog().querySelector<HTMLButtonElement>('.el-dialog__headerbtn')!.click()
    await settle()
    expect(create).toHaveBeenCalledTimes(1)
    expect(state.open).toBe(true)
    expect(dialog().querySelector('input')!.disabled).toBe(true)
    state.creating = false
    await settle()
    expect(dialog().querySelector('input')!.value).toBe('NewPlayer')
  })

  it('protects password reset and clears both sensitive fields on close and unmount', async () => {
    const confirm = vi.spyOn(ElMessageBox, 'confirm').mockRejectedValue('cancel')
    const state = reactive({
      open: true,
      password: resetPasswords.password,
      confirmation: resetPasswords.confirmation,
      resetting: true,
    })
    const mounted = await mount(() =>
      h(ResetPasswordDialog, {
        visible: state.open,
        newPassword: state.password,
        confirmPassword: state.confirmation,
        resetting: state.resetting,
        'onUpdate:visible': (v) => {
          state.open = v
        },
        'onUpdate:newPassword': (v) => {
          state.password = v
        },
        'onUpdate:confirmPassword': (v) => {
          state.confirmation = v
        },
      }),
    )
    dialog().querySelector<HTMLButtonElement>('.el-dialog__headerbtn')!.click()
    await settle()
    expect(state.password).toBe(resetPasswords.password)
    expect(state.open).toBe(true)
    state.resetting = false
    await nextTick()
    clickMask()
    await settle()
    expect(confirm).toHaveBeenCalledTimes(1)
    expect(state.open).toBe(true)
    expect(state.password).toBe(resetPasswords.password)
    button(dialog(), '取消').click()
    await settle()
    expect(state.password).toBe('')
    expect(state.confirmation).toBe('')
    state.open = true
    state.password = resetPasswords.password
    state.confirmation = resetPasswords.confirmation
    await settle()
    mounted.unmount()
    cleanup.pop()
    expect(state.password).toBe('')
    expect(state.confirmation).toBe('')
  })

  it('retains media edits in the parent draft when finishing and switching records', async () => {
    const confirm = vi.spyOn(ElMessageBox, 'confirm')
    const state = reactive({ open: true, item: structuredClone(mediaItem) })
    const updated = vi.fn((item: typeof mediaItem) => {
      Object.assign(state.item, item)
    })
    await mount(() =>
      h(HomepageMediaDialog, {
        visible: state.open,
        item: state.item,
        previewUrl: '/village.png',
        'onUpdate:visible': (v) => {
          state.open = v
        },
        'onUpdate:item': updated,
      }),
    )
    expect(dialog().textContent).toContain('关闭后请在父页面保存更改')
    expect(dialog().textContent).toContain('媒体名称')
    expect(dialog().textContent).toContain('修改不会重命名文件')
    await input(dialog().querySelector('input')!, 'Edited Village')
    await settle()
    expect(updated).toHaveBeenLastCalledWith({ ...mediaItem, title: 'Edited Village' })
    button(dialog(), '完成').click()
    await settle()
    expect(state.open).toBe(false)
    expect(state.item.title).toBe('Edited Village')
    expect(confirm).not.toHaveBeenCalled()
    state.item = { ...mediaItem, id: 'media-b', title: 'Another image' }
    state.open = true
    await settle()
    expect(dialog().querySelector('input')!.value).toBe('Another image')
  })

  it('retains the media draft after save failure without reloading and allows retry', async () => {
    const pending = deferred<never>()
    api.patchHomepageMedia.mockReturnValueOnce(pending.promise)
    const mounted = await mount(() => h(AdminHomepageMedia))
    mounted.host.querySelector<HTMLElement>('[data-homepage-media-id="media-a"]')!.click()
    await settle()
    await input(dialog().querySelector('input')!, 'Edited Village')
    button(dialog(), '完成').click()
    await settle()
    button(mounted.host, '保存配置').click()
    button(mounted.host, '保存配置').click()
    await settle()
    expect(api.patchHomepageMedia).toHaveBeenCalledExactlyOnceWith('media-a', expectedMediaPatch)
    pending.reject(new Error('media save failed'))
    await settle()
    expect(ElMessage.error).toHaveBeenLastCalledWith('media save failed')
    expect(api.listHomepageMedia).toHaveBeenCalledTimes(1)
    expect(button(mounted.host, '保存配置').disabled).toBe(false)
    mounted.host.querySelector<HTMLElement>('[data-homepage-media-id="media-a"]')!.click()
    await settle()
    expect(dialog().querySelector('input')!.value).toBe('Edited Village')
    button(dialog(), '完成').click()
    await settle()
    button(mounted.host, '保存配置').click()
    await settle()
    expect(api.patchHomepageMedia).toHaveBeenCalledTimes(2)
    expect(api.patchHomepageMedia).toHaveBeenLastCalledWith('media-a', expectedMediaPatch)
    expect(button(mounted.host, '保存配置').disabled).toBe(true)
  })
})

describe('email changes', () => {
  async function mountEmail() {
    const state = reactive({ open: true })
    const changed = vi.fn()
    await mount(() =>
      h(EmailChangeDialog, {
        modelValue: state.open,
        'onUpdate:modelValue': (v) => {
          state.open = v
        },
        onChanged: changed,
      }),
    )
    return { state, changed }
  }
  async function fillEmail() {
    await input(
      dialog().querySelector<HTMLInputElement>('input[placeholder="请输入新邮箱地址"]')!,
      'new@example.com',
    )
    await input(
      dialog().querySelector<HTMLInputElement>('input[placeholder="请输入验证码"]')!,
      'CODE1234',
    )
  }

  it('asks on mask dismissal but discards and clears on explicit close', async () => {
    const confirm = vi.spyOn(ElMessageBox, 'confirm').mockRejectedValue('cancel')
    const mounted = await mountEmail()
    await fillEmail()
    clickMask()
    await settle()
    expect(confirm).toHaveBeenCalledTimes(1)
    expect(mounted.state.open).toBe(true)
    expect(
      dialog().querySelector<HTMLInputElement>('input[placeholder="请输入新邮箱地址"]')!.value,
    ).toBe('new@example.com')
    dialog().querySelector<HTMLButtonElement>('.el-dialog__headerbtn')!.click()
    await settle()
    expect(mounted.state.open).toBe(false)
    expect(confirm).toHaveBeenCalledTimes(1)
    mounted.state.open = true
    await settle()
    expect(
      dialog().querySelector<HTMLInputElement>('input[placeholder="请输入新邮箱地址"]')!.value,
    ).toBe('')
    expect(api.changeEmail).not.toHaveBeenCalled()
    expect(api.sendEmailChangeCode).not.toHaveBeenCalled()
  })

  it('ignores an old email policy response after closing and reopening', async () => {
    const pending = deferred<{
      data: { email_suffix_policy: { mode: 'allowlist'; suffixes: string[] } }
    }>()
    api.getPublicSettings.mockReturnValueOnce(pending.promise)
    const mounted = await mountEmail()
    expect(dialog().querySelector('.el-skeleton')).not.toBeNull()
    dialog().querySelector<HTMLButtonElement>('.el-dialog__headerbtn')!.click()
    await settle()
    mounted.state.open = true
    await settle()
    await fillEmail()
    pending.resolve({
      data: { email_suffix_policy: { mode: 'allowlist', suffixes: ['@old.example'] } },
    })
    await settle()
    expect(api.getPublicSettings).toHaveBeenCalledTimes(2)
    expect(dialog().querySelector('.el-select')).toBeNull()
    expect(
      dialog().querySelector<HTMLInputElement>('input[placeholder="请输入新邮箱地址"]')!.value,
    ).toBe('new@example.com')
  })

  it('keeps invalid input and unlocks controls after validation without sending a request', async () => {
    const mounted = await mountEmail()
    await input(
      dialog().querySelector<HTMLInputElement>('input[placeholder="请输入新邮箱地址"]')!,
      'invalid',
    )
    button(dialog(), '确认重设').click()
    await settle()
    expect(api.changeEmail).not.toHaveBeenCalled()
    expect(mounted.state.open).toBe(true)
    await vi.waitFor(() =>
      expect(dialog().querySelectorAll('.el-form-item__error')).toHaveLength(2),
    )
    expect(dialog().textContent).toContain('请输入有效的邮箱地址')
    expect(dialog().textContent).toContain('请输入验证码')
    expect(button(dialog(), '取消').disabled).toBe(false)
  })

  it('rejects invalid email before code delivery and unlocks retry controls', async () => {
    await mountEmail()
    await input(
      dialog().querySelector<HTMLInputElement>('input[placeholder="请输入新邮箱地址"]')!,
      'invalid',
    )
    button(dialog(), '发送验证码').click()
    await settle()
    expect(api.sendEmailChangeCode).not.toHaveBeenCalled()
    expect(ElMessage.warning).toHaveBeenLastCalledWith('请先输入有效的新邮箱地址')
    expect(button(dialog(), '发送验证码').disabled).toBe(false)
    expect(button(dialog(), '取消').disabled).toBe(false)
  })

  it('blocks duplicates and closing through validation/submission, retains failed input, then closes on success', async () => {
    const pending = deferred<{ data: undefined }>()
    api.changeEmail.mockReturnValueOnce(pending.promise)
    const mounted = await mountEmail()
    await fillEmail()
    button(dialog(), '确认重设').click()
    button(dialog(), '确认重设').click()
    await settle()
    expect(api.changeEmail).toHaveBeenCalledExactlyOnceWith({
      email: 'new@example.com',
      code: 'CODE1234',
    })
    dialog().querySelector<HTMLButtonElement>('.el-dialog__headerbtn')!.click()
    await settle()
    expect(mounted.state.open).toBe(true)
    pending.reject(new Error('email unavailable'))
    await settle()
    expect(ElMessage.error).toHaveBeenLastCalledWith('重设失败: email unavailable')
    expect(
      dialog().querySelector<HTMLInputElement>('input[placeholder="请输入新邮箱地址"]')!.value,
    ).toBe('new@example.com')
    expect(
      dialog().querySelector<HTMLInputElement>('input[placeholder="请输入验证码"]')!.value,
    ).toBe('CODE1234')
    button(dialog(), '确认重设').click()
    await settle()
    expect(mounted.state.open).toBe(false)
    expect(mounted.changed).toHaveBeenCalledTimes(1)
    expect(api.changeEmail).toHaveBeenCalledTimes(2)
  })

  it('keeps the resend cooldown across close/reopen while clearing inputs', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval', 'Date'] })
    vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm')
    const mounted = await mountEmail()
    await fillEmail()
    button(dialog(), '发送验证码').click()
    await settle()
    expect(api.sendEmailChangeCode).toHaveBeenCalledExactlyOnceWith({ email: 'new@example.com' })
    expect(button(dialog(), '60s').disabled).toBe(true)
    button(dialog(), '取消').click()
    await settle()
    mounted.state.open = true
    await settle()
    expect(
      dialog().querySelector<HTMLInputElement>('input[placeholder="请输入新邮箱地址"]')!.value,
    ).toBe('')
    expect(button(dialog(), '60s').disabled).toBe(true)
    await vi.advanceTimersByTimeAsync(60000)
    await nextTick()
    expect(button(dialog(), '发送验证码').disabled).toBe(false)
  })

  it('blocks closing during code delivery and preserves input after delivery failure', async () => {
    const pending = deferred<{ data: { ttl: number } }>()
    api.sendEmailChangeCode.mockReturnValueOnce(pending.promise)
    const mounted = await mountEmail()
    await fillEmail()
    button(dialog(), '发送验证码').click()
    button(dialog(), '发送验证码').click()
    await settle()
    dialog().querySelector<HTMLButtonElement>('.el-dialog__headerbtn')!.click()
    await settle()
    expect(mounted.state.open).toBe(true)
    expect(api.sendEmailChangeCode).toHaveBeenCalledTimes(1)
    pending.reject(new Error('mail unavailable'))
    await settle()
    expect(ElMessage.error).toHaveBeenLastCalledWith('发送失败: mail unavailable')
    expect(button(dialog(), '发送验证码').disabled).toBe(false)
    expect(
      dialog().querySelector<HTMLInputElement>('input[placeholder="请输入新邮箱地址"]')!.value,
    ).toBe('new@example.com')
  })
})

describe('notice flow drafts', () => {
  async function openNotice() {
    const mounted = await mount(() => h(AdminNotices))
    button(mounted.host, '新建公告').click()
    await settle()
    await input(
      dialog().querySelector<HTMLInputElement>(
        'input[placeholder="例如：OAuth 应用注册开放说明"]',
      )!,
      noticeInput.title,
    )
    await input(dialog().querySelector('textarea')!, noticeInput.summary)
    return mounted
  }

  it('preserves one draft across next/back and cancelled mask dismissal; explicit cancel resets it', async () => {
    const confirm = vi.spyOn(ElMessageBox, 'confirm').mockRejectedValue('cancel')
    const mounted = await openNotice()
    button(dialog(), '下一步：发布设置').click()
    await settle()
    expect(dialog().textContent).toContain('发布设置')
    expect(confirm).not.toHaveBeenCalled()
    button(dialog(), '返回编辑').click()
    await settle()
    expect(dialog().querySelector('input')!.value).toBe(noticeInput.title)
    expect(dialog().querySelector('textarea')!.value).toBe(noticeInput.summary)
    clickMask()
    await settle()
    expect(dialog().querySelector('input')!.value).toBe(noticeInput.title)
    button(dialog(), '取消').click()
    await settle()
    button(mounted.host, '新建公告').click()
    await settle()
    expect(dialog().querySelector('input')!.value).toBe('')
    expect(dialog().querySelector('textarea')!.value).toBe('')
    expect(api.createAdminNotice).not.toHaveBeenCalled()
  })

  it('keeps failed notice input, blocks back/close/duplicate save, and closes on successful retry', async () => {
    const pending = deferred<never>()
    api.createAdminNotice.mockReturnValueOnce(pending.promise).mockResolvedValueOnce({ data: {} })
    await openNotice()
    button(dialog(), '下一步：发布设置').click()
    await settle()
    button(dialog(), '创建公告').click()
    button(dialog(), '创建公告').click()
    await settle()
    button(dialog(), '返回编辑').click()
    dialog().querySelector<HTMLButtonElement>('.el-dialog__headerbtn')!.click()
    await settle()
    expect(api.createAdminNotice).toHaveBeenCalledExactlyOnceWith(expectedNoticeDraft)
    expect(dialog().textContent).toContain('发布设置')
    pending.reject(new Error('notice save failed'))
    await settle()
    expect(ElMessage.error).toHaveBeenLastCalledWith('notice save failed')
    button(dialog(), '返回编辑').click()
    await settle()
    expect(dialog().querySelector('input')!.value).toBe(noticeInput.title)
    button(dialog(), '下一步：发布设置').click()
    await settle()
    button(dialog(), '创建公告').click()
    await settle()
    expect(api.createAdminNotice).toHaveBeenCalledTimes(2)
    expect(
      [...document.querySelectorAll<HTMLElement>('.ui-dialog')].filter(
        (element) => element.closest<HTMLElement>('.el-overlay')?.style.display !== 'none',
      ),
    ).toHaveLength(0)
  })
})

describe('role binding confirmations and immediate preview', () => {
  async function openRole() {
    const mounted = await mount(() => h(AdminRolesList))
    button(mounted.host, '编辑').click()
    await settle()
    return mounted
  }
  it('cancel does not write or alter binding; confirmed skin clear updates the current preview', async () => {
    const confirm = vi
      .spyOn(ElMessageBox, 'confirm')
      .mockRejectedValueOnce('cancel')
      .mockResolvedValue('confirm')
    await openRole()
    button(dialog(), '清除皮肤').click()
    await settle()
    expect(api.patchProfileSkin).not.toHaveBeenCalled()
    expect(dialog().querySelector('[aria-label="皮肤绑定"]')?.textContent).toContain('skin-a')
    expect(ElMessage.error).not.toHaveBeenCalled()
    button(dialog(), '清除皮肤').click()
    await settle()
    expect(confirm).toHaveBeenLastCalledWith(
      '确定清除角色「PlayerA」的皮肤绑定吗？此操作只解除绑定，不删除材质。',
      '清除皮肤',
      {
        type: 'warning',
        confirmButtonText: '清除皮肤',
        cancelButtonText: '取消',
        closeOnClickModal: false,
        distinguishCancelAndClose: true,
      },
    )
    expect(api.patchProfileSkin).toHaveBeenCalledExactlyOnceWith('role-a', { hash: null })
    expect(dialog().querySelector('.test-skin-viewer')).toBeNull()
    expect(dialog().querySelector('[aria-label="皮肤绑定"]')?.textContent).toContain('未绑定')
    expect(button(dialog(), '清除皮肤').disabled).toBe(true)
  })

  it('preserves a failed cape binding and blocks duplicate commands until it settles', async () => {
    vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm')
    const pending = deferred<never>()
    api.patchProfileCape.mockReturnValueOnce(pending.promise)
    await openRole()
    button(dialog(), '清除披风').click()
    await settle()
    button(dialog(), '清除披风').click()
    dialog().querySelector<HTMLButtonElement>('.el-dialog__headerbtn')!.click()
    await settle()
    expect(api.patchProfileCape).toHaveBeenCalledExactlyOnceWith('role-a', { hash: null })
    expect(dialog().querySelector('[aria-label="披风绑定"]')?.textContent).toContain('cape-a')
    pending.reject(new Error('clear failed'))
    await settle()
    expect(ElMessage.error).toHaveBeenLastCalledWith('清除披风绑定失败')
    expect(dialog().querySelector('[aria-label="披风绑定"]')?.textContent).toContain('cape-a')
    expect(button(dialog(), '清除披风').disabled).toBe(false)
  })

  it('does not overwrite a different selected role when a prior clear succeeds', async () => {
    vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm')
    const pending = deferred<{ data: undefined }>()
    api.patchProfileCape.mockReturnValueOnce(pending.promise)
    const mounted = await openRole()
    button(dialog(), '清除披风').click()
    await settle()
    const edits = [...mounted.host.querySelectorAll<HTMLButtonElement>('button')].filter(
      (element) => element.textContent?.trim() === '编辑',
    )
    edits[1]!.click()
    await settle()
    pending.resolve({ data: undefined })
    await settle()
    expect(api.patchProfileCape).toHaveBeenCalledExactlyOnceWith('role-a', { hash: null })
    expect(dialog().querySelector('input')!.value).toBe('PlayerB')
    expect(dialog().querySelector('[aria-label="皮肤绑定"]')?.textContent).toContain('skin-b')
    expect(dialog().querySelector('[aria-label="披风绑定"]')?.textContent).toContain('未绑定')
  })
})
