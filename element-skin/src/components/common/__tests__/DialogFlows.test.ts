import { createApp, defineComponent, h, nextTick, reactive, type VNode } from 'vue'
import ElementPlus, { ElMessage, ElMessageBox } from 'element-plus'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter, RouterView } from 'vue-router'
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
import DashboardWardrobe from '@/components/dashboard/wardrobe/DashboardWardrobe.vue'
import AdminSettings from '@/components/admin/settings/AdminSettings.vue'
import AdminEmail from '@/components/admin/settings/AdminEmail.vue'
import {
  boundProfile,
  otherProfile,
  emailPolicyResponse,
  resetPasswords,
  mediaItem,
  noticeInput,
  expectedNoticeDraft,
  expectedMediaPatch,
  uploadFileData,
  siteSettingsGroups,
  dialogFlowPermissions,
  adminEmailSettings,
  adminEmailPolicy,
  emailFieldChanges,
  emailDraftChanges,
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
  uploadHomepageImage: vi.fn(),
  uploadHomepagePanorama: vi.fn(),
  deleteHomepageMedia: vi.fn(),
  getTextures: vi.fn(),
  uploadTexture: vi.fn(),
  getAdminSettingsGroup: vi.fn(),
  saveAdminSettingsGroup: vi.fn(),
  getAdminEmailSuffixPolicy: vi.fn(),
  putAdminEmailSuffixPolicy: vi.fn(),
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
  uploadHomepageImage: api.uploadHomepageImage,
  uploadHomepagePanorama: api.uploadHomepagePanorama,
  deleteHomepageMedia: api.deleteHomepageMedia,
}))
vi.mock('@/api/textures', () => ({
  getTextures: api.getTextures,
  uploadTexture: api.uploadTexture,
  getTextureDetail: vi.fn(),
  patchTexture: vi.fn(),
  deleteTexture: vi.fn(),
  applyTexture: vi.fn(),
}))
vi.mock('@/api/admin/settings', () => ({
  getAdminSettingsGroup: api.getAdminSettingsGroup,
  saveAdminSettingsGroup: api.saveAdminSettingsGroup,
  getAdminEmailSuffixPolicy: api.getAdminEmailSuffixPolicy,
  putAdminEmailSuffixPolicy: api.putAdminEmailSuffixPolicy,
}))
vi.mock('@/components/CapeViewer.vue', () => ({ default: { render: () => h('div', 'cape') } }))
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
  api.saveAdminSettingsGroup.mockReset().mockResolvedValue({ data: {} })
  api.getPublicSettings.mockResolvedValue(structuredClone(emailPolicyResponse))
  api.getMe.mockResolvedValue({
    data: { id: 'user-a', email: 'user@example.com', permissions: [...dialogFlowPermissions] },
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
  api.getTextures.mockResolvedValue({
    data: { items: [], has_next: false, next_cursor: null, page_size: 20 },
  })
  api.getAdminSettingsGroup.mockImplementation(
    async (group: keyof typeof siteSettingsGroups | 'email') => ({
      data: structuredClone(group === 'email' ? adminEmailSettings : siteSettingsGroups[group]),
    }),
  )
  api.getAdminEmailSuffixPolicy
    .mockReset()
    .mockResolvedValue({ data: structuredClone(adminEmailPolicy) })
  api.putAdminEmailSuffixPolicy.mockReset().mockResolvedValue({ data: {} })
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
  const app = createApp({ render: () => h(RouterView) })
  const pinia = createPinia()
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: defineComponent({ render }) }],
  })
  await router.push('/')
  app.use(pinia).use(router).use(ElementPlus)
  useSessionStore(pinia).user = {
    id: 'user-a',
    email: 'user@example.com',
    permissions: [...dialogFlowPermissions],
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
  // 两步弹窗切换时，旧层可仍在离场；操作浏览器实际最上面的可见层。
  const result = [...document.querySelectorAll<HTMLElement>('.ui-dialog')]
    .filter((element) => element.closest<HTMLElement>('.el-overlay')?.style.display !== 'none')
    .sort(
      (a, b) =>
        Number(b.closest<HTMLElement>('.el-overlay')?.style.zIndex || 0) -
        Number(a.closest<HTMLElement>('.el-overlay')?.style.zIndex || 0),
    )[0]
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

async function selectUpload(root: ParentNode) {
  const file = new File([uploadFileData.body], uploadFileData.name, { type: uploadFileData.type })
  const picker = root.querySelector<HTMLInputElement>('input[type="file"]')!
  Object.defineProperty(picker, 'files', { value: [file], configurable: true })
  picker.dispatchEvent(new Event('change', { bubbles: true }))
  await settle()
  return file
}

function emailField(root: ParentNode, label: string) {
  const field = [...root.querySelectorAll<HTMLElement>('.el-form-item')].find(
    (item) => item.querySelector('.el-form-item__label > span')?.textContent?.trim() === label,
  )
  if (!field) throw new Error(`No email field: ${label}`)
  return field
}

function dirtyEmailLabels(root: ParentNode) {
  return [...root.querySelectorAll('.field-change-hint')].map((hint) =>
    hint
      .closest('.el-form-item')
      ?.querySelector('.el-form-item__label > span')
      ?.textContent?.trim(),
  )
}

describe('email settings field hints and grouped save baselines', () => {
  it.each(emailFieldChanges)('marks only $label and clears it when restored', async (change) => {
    const mounted = await mount(() => h(AdminEmail))
    const smtp = mounted.host.querySelector<HTMLElement>('.ui-card')!
    const field = emailField(smtp, change.label)
    expect(smtp.querySelectorAll('.dirty-form-item')).toHaveLength(8)
    expect(dirtyEmailLabels(smtp)).toEqual([])
    if (change.kind === 'switch') {
      const control = field.querySelector<HTMLElement>('.el-switch')!
      control.click()
      await settle()
      expect(dirtyEmailLabels(smtp)).toEqual([change.label])
      control.click()
    } else {
      const control = field.querySelector<HTMLInputElement>('input')!
      const original = control.value
      if (change.label === '密码 / 授权码') {
        expect(original).toBe('')
        expect(control.type).toBe('password')
      }
      await input(control, change.value)
      control.dispatchEvent(new Event('change', { bubbles: true }))
      await settle()
      expect(dirtyEmailLabels(smtp)).toEqual([change.label])
      await input(control, original)
      control.dispatchEvent(new Event('change', { bubbles: true }))
    }
    await settle()
    expect(dirtyEmailLabels(smtp)).toEqual([])
    expect(smtp.querySelector('.el-card__header .el-tag')).toBeNull()
    expect(api.saveAdminSettingsGroup).not.toHaveBeenCalled()
  })

  it('retains failed SMTP secrets and policy drafts, then clears only each successfully saved group', async () => {
    const pending = deferred<void>()
    const requests: Array<{ group: string; settings: Record<string, unknown> }> = []
    api.saveAdminSettingsGroup
      .mockImplementationOnce((group: string, settings: Record<string, unknown>) => {
        requests.push({ group, settings: { ...settings } })
        return pending.promise
      })
      .mockImplementationOnce(async (group: string, settings: Record<string, unknown>) => {
        requests.push({ group, settings: { ...settings } })
        return { data: {} }
      })
    api.putAdminEmailSuffixPolicy.mockRejectedValueOnce(new Error('policy save failed'))
    const mounted = await mount(() => h(AdminEmail))
    const cards = mounted.host.querySelectorAll<HTMLElement>('.ui-card')
    const smtp = cards[0]!,
      policy = cards[1]!
    const host = emailField(smtp, '服务器地址').querySelector<HTMLInputElement>('input')!
    const password = emailField(smtp, '密码 / 授权码').querySelector<HTMLInputElement>('input')!
    expect(password.value).toBe('')
    expect(mounted.host.textContent).not.toContain(adminEmailSettings.smtp_password)
    await input(host, emailDraftChanges.smtp_host)
    await input(password, emailDraftChanges.smtp_password)
    await input(
      policy.querySelector<HTMLInputElement>('input[placeholder="例如 @qq.com"]')!,
      emailDraftChanges.policySuffix,
    )
    button(policy, '添加').click()
    await settle()
    expect(dirtyEmailLabels(smtp)).toEqual(['服务器地址', '密码 / 授权码'])
    expect(smtp.querySelector('.el-card__header .el-tag')).toBeNull()
    expect(policy.querySelector('.el-card__header .el-tag')?.textContent?.trim()).toBe('未保存')
    expect(policy.querySelectorAll('.field-change-hint')).toHaveLength(0)

    button(smtp, '保存配置').click()
    button(smtp, '保存配置').click()
    await settle()
    expect(api.saveAdminSettingsGroup).toHaveBeenCalledTimes(1)
    expect(password.disabled).toBe(true)
    expect(button(mounted.host, '重新加载').disabled).toBe(true)
    button(policy, '保存策略').click()
    await settle()
    expect(ElMessage.error).toHaveBeenLastCalledWith('保存邮箱后缀策略失败')
    expect(policy.querySelector('.el-card__header .el-tag')?.textContent?.trim()).toBe('未保存')

    pending.reject(new Error('SMTP save failed'))
    await settle()
    expect(ElMessage.error).toHaveBeenLastCalledWith('保存失败')
    expect(host.value).toBe(emailDraftChanges.smtp_host)
    expect(password.value).toBe(emailDraftChanges.smtp_password)
    expect(dirtyEmailLabels(smtp)).toEqual(['服务器地址', '密码 / 授权码'])
    const confirm = vi.spyOn(ElMessageBox, 'confirm').mockRejectedValue('cancel')
    button(mounted.host, '重新加载').click()
    await settle()
    expect(api.getAdminSettingsGroup).toHaveBeenCalledExactlyOnceWith('email')
    expect(api.getAdminEmailSuffixPolicy).toHaveBeenCalledTimes(1)
    expect(password.value).toBe(emailDraftChanges.smtp_password)

    button(smtp, '保存配置').click()
    await settle()
    const expectedSettings = {
      ...adminEmailSettings,
      smtp_host: emailDraftChanges.smtp_host,
      smtp_password: emailDraftChanges.smtp_password,
    }
    expect(requests).toEqual([
      { group: 'email', settings: expectedSettings },
      { group: 'email', settings: expectedSettings },
    ])
    expect(password.value).toBe('')
    expect(host.value).toBe(emailDraftChanges.smtp_host)
    expect(dirtyEmailLabels(smtp)).toEqual([])
    expect(policy.querySelector('.el-card__header .el-tag')?.textContent?.trim()).toBe('未保存')
    button(mounted.host, '重新加载').click()
    await settle()
    expect(confirm).toHaveBeenCalledTimes(2)
    expect(api.getAdminSettingsGroup).toHaveBeenCalledTimes(1)

    const expectedPolicy = {
      ...adminEmailPolicy,
      allowlist: [...adminEmailPolicy.allowlist, emailDraftChanges.policySuffix],
    }
    api.getAdminEmailSuffixPolicy.mockResolvedValueOnce({ data: expectedPolicy })
    button(policy, '保存策略').click()
    await settle()
    expect(api.putAdminEmailSuffixPolicy).toHaveBeenNthCalledWith(1, expectedPolicy)
    expect(api.putAdminEmailSuffixPolicy).toHaveBeenNthCalledWith(2, expectedPolicy)
    expect(api.putAdminEmailSuffixPolicy).toHaveBeenCalledTimes(2)
    expect(policy.querySelector('.el-card__header .el-tag')).toBeNull()
    expect(dirtyEmailLabels(smtp)).toEqual([])
  })
})

describe('independent and sensitive form lifecycles', () => {
  it('retains failed fields and clears only a successfully saved settings group', async () => {
    api.saveAdminSettingsGroup.mockImplementation(async (group: string) => {
      if (group === 'site') throw new Error('site save failed')
      return { data: {} }
    })
    const mounted = await mount(() => h(AdminSettings))
    const cards = mounted.host.querySelectorAll<HTMLElement>('.ui-card')
    const name = cards[0]!.querySelector<HTMLInputElement>('input[placeholder="皮肤站"]')!
    const days = cards[2]!.querySelector<HTMLInputElement>('input')!
    await input(name, 'unsaved site')
    await input(days, '14')
    days.dispatchEvent(new Event('change', { bubbles: true }))
    await settle()
    expect(cards[0]!.querySelectorAll('.field-change-hint')).toHaveLength(1)
    expect(cards[2]!.querySelectorAll('.field-change-hint')).toHaveLength(1)
    button(cards[0]!, '保存').click()
    button(cards[2]!, '保存').click()
    await settle()
    expect(api.saveAdminSettingsGroup).toHaveBeenNthCalledWith(1, 'site', {
      ...siteSettingsGroups.site,
      site_name: 'unsaved site',
    })
    expect(api.saveAdminSettingsGroup).toHaveBeenNthCalledWith(2, 'auth', {
      ...siteSettingsGroups.auth,
      jwt_expire_days: 14,
    })
    expect(name.value).toBe('unsaved site')
    expect(cards[0]!.querySelectorAll('.field-change-hint')).toHaveLength(1)
    expect(cards[2]!.querySelectorAll('.field-change-hint')).toHaveLength(0)
    expect(cards[2]!.querySelector('.el-card__header .el-tag')).toBeNull()
    expect(cards[0]!.querySelector('.el-card__header .el-tag')).toBeNull()
    expect(cards[0]!.querySelector('.field-change-hint')?.textContent).toBe('未保存')
    await input(name, siteSettingsGroups.site.site_name)
    expect(cards[0]!.querySelectorAll('.field-change-hint')).toHaveLength(0)
  })
  it('keeps site settings explicit, protects grouped writes, and confirms reload without losing a cancelled draft', async () => {
    const pending = deferred<void>()
    api.saveAdminSettingsGroup
      .mockReturnValueOnce(pending.promise)
      .mockResolvedValueOnce({ data: {} })
    const mounted = await mount(() => h(AdminSettings))
    const site = mounted.host.querySelector<HTMLElement>('.ui-card')!
    const name = site.querySelector<HTMLInputElement>('input[placeholder="皮肤站"]')!
    await input(name, 'Updated Site')
    expect(api.saveAdminSettingsGroup).not.toHaveBeenCalled()
    expect(site.querySelector('.field-change-hint')?.textContent).toBe('未保存')
    expect(site.querySelector('.el-card__header .el-tag')).toBeNull()
    const confirm = vi
      .spyOn(ElMessageBox, 'confirm')
      .mockRejectedValueOnce('cancel')
      .mockResolvedValueOnce('confirm')
    button(mounted.host, '重新加载').click()
    await settle()
    expect(name.value).toBe('Updated Site')
    expect(api.getAdminSettingsGroup).toHaveBeenCalledTimes(3)
    button(site, '保存').click()
    button(site, '保存').click()
    await settle()
    expect(api.saveAdminSettingsGroup).toHaveBeenCalledExactlyOnceWith('site', {
      ...siteSettingsGroups.site,
      site_name: 'Updated Site',
    })
    expect(name.disabled).toBe(true)
    expect(button(mounted.host, '重新加载').disabled).toBe(true)
    pending.reject(new Error('save failed'))
    await settle()
    expect(name.value).toBe('Updated Site')
    expect(site.querySelector('.field-change-hint')?.textContent).toBe('未保存')
    expect(site.querySelector('.el-card__header .el-tag')).toBeNull()
    button(mounted.host, '重新加载').click()
    await settle()
    expect(confirm).toHaveBeenCalledTimes(2)
    expect(api.getAdminSettingsGroup).toHaveBeenCalledTimes(6)
    expect(name.value).toBe(siteSettingsGroups.site.site_name)
    await vi.waitFor(() => expect(site.querySelectorAll('.field-change-hint')).toHaveLength(0))
  })

  it('protects texture upload options and files on indirect exit, then clears the abandoned selection and preview', async () => {
    const revoke = vi.fn()
    vi.stubGlobal(
      'URL',
      class extends URL {
        static createObjectURL() {
          return 'blob:texture-test'
        }
        static revokeObjectURL = revoke
      },
    )
    const confirm = vi.spyOn(ElMessageBox, 'confirm').mockRejectedValue('cancel')
    const mounted = await mount(() => h(DashboardWardrobe))
    button(mounted.host, '上传纹理').click()
    await settle()
    await input(
      dialog().querySelector<HTMLInputElement>('input[placeholder="给这个纹理添加备注（可选）"]')!,
      'Upload draft',
    )
    clickMask()
    await settle()
    expect(confirm).toHaveBeenCalledTimes(1)
    expect(
      dialog().querySelector<HTMLInputElement>('input[placeholder="给这个纹理添加备注（可选）"]')!
        .value,
    ).toBe('Upload draft')
    await selectUpload(dialog())
    expect(dialog().textContent).toContain(uploadFileData.name)
    button(dialog(), '取消').click()
    await settle()
    expect(revoke).toHaveBeenCalledExactlyOnceWith('blob:texture-test')
    button(mounted.host, '上传纹理').click()
    await settle()
    expect(dialog().textContent).not.toContain(uploadFileData.name)
    expect(
      dialog().querySelector<HTMLInputElement>('input[placeholder="给这个纹理添加备注（可选）"]')!
        .value,
    ).toBe('')
    expect(dialog().textContent).toContain('选择文件后即可在此预览')
    expect(api.uploadTexture).not.toHaveBeenCalled()
    expect(confirm).toHaveBeenCalledTimes(1)
  })

  it('uploads once, blocks closure and field edits while writing, retains failure inputs, and cleans up after success', async () => {
    const revoke = vi.fn()
    vi.stubGlobal(
      'URL',
      class extends URL {
        static createObjectURL() {
          return 'blob:texture-test'
        }
        static revokeObjectURL = revoke
      },
    )
    const confirm = vi.spyOn(ElMessageBox, 'confirm')
    const pending = deferred<never>()
    api.uploadTexture.mockReturnValueOnce(pending.promise).mockResolvedValueOnce({ data: {} })
    const mounted = await mount(() => h(DashboardWardrobe))
    button(mounted.host, '上传纹理').click()
    await settle()
    const file = await selectUpload(dialog())
    button(dialog(), '确认上传').click()
    button(dialog(), '确认上传').click()
    await settle()
    expect(api.uploadTexture).toHaveBeenCalledTimes(1)
    const payload = api.uploadTexture.mock.calls[0]![0] as FormData
    expect([...payload.keys()]).toEqual(['file', 'texture_type', 'model', 'note', 'is_public'])
    expect(payload.get('file')).toBe(file)
    expect(payload.get('texture_type')).toBe('skin')
    expect(payload.get('model')).toBe('default')
    expect(payload.get('note')).toBe('')
    expect(payload.get('is_public')).toBe('false')
    expect(button(dialog(), '取消').disabled).toBe(true)
    expect([...dialog().querySelectorAll('input')].every((el) => el.disabled)).toBe(true)
    document.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape', code: 'Escape', bubbles: true }),
    )
    clickMask()
    await settle()
    expect(dialog().textContent).toContain(uploadFileData.name)
    expect(confirm).not.toHaveBeenCalled()
    pending.reject(new Error('upload failed'))
    await settle()
    expect(ElMessage.error).toHaveBeenLastCalledWith('上传失败: upload failed')
    expect(dialog().textContent).toContain(uploadFileData.name)
    expect(revoke).not.toHaveBeenCalled()
    button(dialog(), '确认上传').click()
    await settle()
    expect(api.uploadTexture).toHaveBeenCalledTimes(2)
    expect(revoke).toHaveBeenCalledExactlyOnceWith('blob:texture-test')
    button(mounted.host, '上传纹理').click()
    await settle()
    expect(dialog().textContent).not.toContain(uploadFileData.name)
  })

  it('discards only the homepage draft without an API call and requires handling it before resource commands', async () => {
    const confirm = vi.spyOn(ElMessageBox, 'confirm')
    const mounted = await mount(() => h(AdminHomepageMedia))
    const configuration = mounted.host.querySelector('[aria-label="配置更改"]')!
    const status = configuration.querySelector('[role="status"] .el-tag')!
    expect(status.textContent?.trim()).toBe('未保存')
    expect(status.classList.contains('invisible')).toBe(true)
    expect(
      [...configuration.querySelectorAll('button')].map((el) => el.textContent?.trim()),
    ).toEqual(['放弃更改', '保存配置'])
    const card = mounted.host.querySelector<HTMLElement>('[data-homepage-media-id="media-a"]')!
    card.click()
    await settle()
    await input(dialog().querySelector('input')!, 'Edited Village')
    button(dialog(), '返回配置').click()
    await settle()
    expect(status.classList.contains('invisible')).toBe(false)
    card.querySelector<HTMLButtonElement>('button[aria-label="删除媒体"]')!.click()
    await selectUpload(mounted.host)
    expect(ElMessage.warning).toHaveBeenCalledTimes(2)
    expect(ElMessage.warning).toHaveBeenLastCalledWith('请先保存或放弃当前配置，再上传或删除媒体')
    expect(api.deleteHomepageMedia).not.toHaveBeenCalled()
    expect(api.uploadHomepageImage).not.toHaveBeenCalled()
    expect(confirm).not.toHaveBeenCalled()
    button(mounted.host, '放弃更改').click()
    await settle()
    expect(configuration.querySelector('[role="status"] .el-tag')).toBe(status)
    expect(status.classList.contains('invisible')).toBe(true)
    expect(button(mounted.host, '保存配置').disabled).toBe(true)
    card.click()
    await settle()
    expect(dialog().querySelector('input')!.value).toBe(mediaItem.title)
    expect(api.patchHomepageMedia).not.toHaveBeenCalled()
    expect(api.listHomepageMedia).toHaveBeenCalledTimes(1)
  })

  it('locks media configuration until upload and its reload settle, and reports delete failures', async () => {
    const upload = deferred<void>()
    const reload = deferred<{ data: { items: (typeof mediaItem)[] } }>()
    api.uploadHomepageImage.mockReturnValueOnce(upload.promise)
    api.listHomepageMedia
      .mockReturnValueOnce({ data: { items: [structuredClone(mediaItem)] } })
      .mockReturnValueOnce(reload.promise)
    const mounted = await mount(() => h(AdminHomepageMedia))
    await selectUpload(mounted.host)
    const card = mounted.host.querySelector<HTMLElement>('[data-homepage-media-id="media-a"]')!
    expect(card.draggable).toBe(false)
    expect(card.querySelector<HTMLButtonElement>('button')!.disabled).toBe(true)
    expect(button(mounted.host, '上传图片').disabled).toBe(true)
    upload.resolve(undefined)
    await settle()
    expect(button(mounted.host, '上传图片').disabled).toBe(true)
    reload.resolve({ data: { items: [structuredClone(mediaItem)] } })
    await settle()
    expect(card.draggable).toBe(true)
    const confirm = vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm')
    api.deleteHomepageMedia.mockRejectedValueOnce(new Error('delete failed'))
    card.querySelector<HTMLButtonElement>('button[aria-label="删除媒体"]')!.click()
    await settle()
    expect(confirm).toHaveBeenCalledExactlyOnceWith(
      '确定删除首页媒体“Village”吗？此操作立即生效，无法通过放弃配置恢复。',
      '删除媒体',
      {
        type: 'warning',
        confirmButtonText: '删除媒体',
        cancelButtonText: '取消',
        closeOnClickModal: false,
      },
    )
    expect(api.deleteHomepageMedia).toHaveBeenCalledExactlyOnceWith('media-a')
    expect(ElMessage.error).toHaveBeenLastCalledWith('delete failed')
    expect(api.listHomepageMedia).toHaveBeenCalledTimes(2)
    expect(button(mounted.host, '上传图片').disabled).toBe(false)
  })

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
    expect(dialog().textContent).toContain('返回后点击“保存配置”才会生效')
    expect(dialog().textContent).toContain('媒体名称')
    expect(dialog().textContent).toContain('修改不会重命名文件')
    await input(dialog().querySelector('input')!, 'Edited Village')
    await settle()
    expect(updated).toHaveBeenLastCalledWith({ ...mediaItem, title: 'Edited Village' })
    button(dialog(), '返回配置').click()
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
    button(dialog(), '返回配置').click()
    await settle()
    button(mounted.host, '保存配置').click()
    button(mounted.host, '保存配置').click()
    await settle()
    expect(api.patchHomepageMedia).toHaveBeenCalledExactlyOnceWith('media-a', expectedMediaPatch)
    pending.reject(new Error('media save failed'))
    await settle()
    expect(ElMessage.error).toHaveBeenLastCalledWith('保存未完成：media save failed')
    expect(api.listHomepageMedia).toHaveBeenCalledTimes(1)
    expect(button(mounted.host, '保存配置').disabled).toBe(false)
    mounted.host.querySelector<HTMLElement>('[data-homepage-media-id="media-a"]')!.click()
    await settle()
    expect(dialog().querySelector('input')!.value).toBe('Edited Village')
    button(dialog(), '返回配置').click()
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
    mounted.host.querySelector<HTMLElement>('.admin-role-card [role="button"]')!.click()
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
    expect(dialog().querySelector('.ui-viewer-info')?.textContent).toContain('skin-a')
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
    expect(dialog().querySelector('.ui-viewer-info')?.textContent).not.toContain('皮肤 Hash')
    expect(
      [...dialog().querySelectorAll('button')].map((item) => item.textContent?.trim()),
    ).not.toContain('清除皮肤')
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
    expect(dialog().querySelector('.ui-viewer-info')?.textContent).toContain('cape-a')
    pending.reject(new Error('clear failed'))
    await settle()
    expect(ElMessage.error).toHaveBeenLastCalledWith(
      'clear failed；未能确认写入结果，请核对后再重试',
    )
    expect(dialog().querySelector('.ui-viewer-info')?.textContent).toContain('cape-a')
    expect(button(dialog(), '清除披风').disabled).toBe(false)
  })

  it('blocks switching roles during a write and allows switching after it settles', async () => {
    vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm')
    const pending = deferred<{ data: undefined }>()
    api.patchProfileCape.mockReturnValueOnce(pending.promise)
    const mounted = await openRole()
    button(dialog(), '清除披风').click()
    await settle()
    const cards = mounted.host.querySelectorAll<HTMLElement>('.admin-role-card [role="button"]')
    cards[1]!.click()
    await settle()
    expect(dialog().querySelector('h2')?.textContent?.trim()).toBe('PlayerA')
    pending.resolve({ data: undefined })
    await settle()
    expect(api.patchProfileCape).toHaveBeenCalledExactlyOnceWith('role-a', { hash: null })
    expect(dialog().textContent).not.toContain('披风 Hash')
    cards[1]!.click()
    await settle()
    expect(dialog().querySelector('h2')?.textContent?.trim()).toBe('PlayerB')
    expect(dialog().textContent).toContain('skin-b')
    expect(dialog().textContent).not.toContain('披风 Hash')
  })
})
