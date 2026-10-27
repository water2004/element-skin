import { createApp, defineComponent, h, nextTick, type Component } from 'vue'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter, RouterView } from 'vue-router'
import ElementPlus, { ElMessage, ElMessageBox } from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useSessionStore } from '@/stores/session'
import SkinLibraryView from '@/views/SkinLibraryView.vue'
import DashboardWardrobe from '@/components/dashboard/wardrobe/DashboardWardrobe.vue'
import DashboardRoles from '@/components/dashboard/roles/DashboardRoles.vue'
import AdminRolesList from '@/components/admin/roles/AdminRolesList.vue'
import AdminTexturesList from '@/components/admin/textures/AdminTexturesList.vue'
import { binding as officialBinding } from '@/components/dashboard/roles/__tests__/fixtures/officialBinding'
import {
  adminDeleteWarning,
  viewerProfile,
  viewerReadCases,
  viewerTexture,
  wardrobeDeleteWarning,
} from './fixtures/viewerActions'

const api = vi.hoisted(() => ({
  getPublicSkinLibrary: vi.fn(),
  getOfficialProfileBindings: vi.fn(),
  getExternalIdentities: vi.fn(),
  getTextures: vi.fn(),
  getTextureDetail: vi.fn(),
  addToWardrobe: vi.fn(),
  patchTexture: vi.fn(),
  deleteTexture: vi.fn(),
  applyTexture: vi.fn(),
  uploadTexture: vi.fn(),
  getProfiles: vi.fn(),
  createProfile: vi.fn(),
  patchProfile: vi.fn(),
  deleteProfile: vi.fn(),
  clearProfileSkin: vi.fn(),
  clearProfileCape: vi.fn(),
  getAdminProfiles: vi.fn(),
  patchAdminProfile: vi.fn(),
  deleteAdminProfile: vi.fn(),
  patchProfileSkin: vi.fn(),
  patchProfileCape: vi.fn(),
  getAdminTextures: vi.fn(),
  patchAdminTexture: vi.fn(),
  deleteAdminTexture: vi.fn(),
  getMe: vi.fn(),
  setAvatar: vi.fn(),
}))
vi.mock('@/api/identity', () => ({ getExternalIdentities: api.getExternalIdentities }))
vi.mock('@/api/official-profiles', () => ({
  getOfficialProfileBindings: api.getOfficialProfileBindings,
  createOfficialProfileBinding: vi.fn(),
  syncOfficialProfileBinding: vi.fn(),
  deleteOfficialProfileBinding: vi.fn(),
}))
vi.mock('@/api/public', () => ({ getPublicSkinLibrary: api.getPublicSkinLibrary }))
vi.mock('@/api/textures', () => ({
  getTextures: api.getTextures,
  getTextureDetail: api.getTextureDetail,
  addToWardrobe: api.addToWardrobe,
  patchTexture: api.patchTexture,
  deleteTexture: api.deleteTexture,
  applyTexture: api.applyTexture,
  uploadTexture: api.uploadTexture,
}))
vi.mock('@/api/profiles', () => ({
  getProfiles: api.getProfiles,
  createProfile: api.createProfile,
  patchProfile: api.patchProfile,
  deleteProfile: api.deleteProfile,
  clearProfileSkin: api.clearProfileSkin,
  clearProfileCape: api.clearProfileCape,
}))
vi.mock('@/api/admin/profiles', () => ({
  getAdminProfiles: api.getAdminProfiles,
  patchAdminProfile: api.patchAdminProfile,
  deleteAdminProfile: api.deleteAdminProfile,
  patchProfileSkin: api.patchProfileSkin,
  patchProfileCape: api.patchProfileCape,
}))
vi.mock('@/api/admin/textures', () => ({
  getAdminTextures: api.getAdminTextures,
  patchAdminTexture: api.patchAdminTexture,
  deleteAdminTexture: api.deleteAdminTexture,
}))
vi.mock('@/api/me', () => ({ getMe: api.getMe }))
vi.mock('@/composables/useAvatar', () => ({ useAvatar: () => ({ setAvatar: api.setAvatar }) }))
vi.mock('@/components/textures/textureAssets', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/components/textures/textureAssets')>()),
  cacheTextureDimensions: vi.fn().mockResolvedValue({ width: 64, height: 64 }),
  textureAssetUrl: (hash: string) => `/textures/${hash}.png`,
}))
vi.mock('@/components/SkinViewer.vue', () => ({ default: { render: () => h('div') } }))
vi.mock('@/components/CapeViewer.vue', () => ({ default: { render: () => h('div') } }))
// Keep the real detail components and their event handlers; avoid WebGL/card asset work.
vi.mock('@/components/textures/TextureCard.vue', () => ({
  default: defineComponent({
    props: ['texture'],
    emits: ['preview'],
    setup:
      (props, { emit, slots }) =>
      () =>
        h('div', [
          h('button', { onClick: () => emit('preview', props.texture) }, '打开材质'),
          slots.actions?.({ texture: props.texture }),
        ]),
  }),
}))

const pages = {
  library: SkinLibraryView,
  wardrobe: DashboardWardrobe,
  roles: DashboardRoles,
  adminRoles: AdminRolesList,
  adminTextures: AdminTexturesList,
}
type Page = keyof typeof pages
const cleanup: Array<() => void> = []
beforeEach(() => {
  vi.resetAllMocks()
  const texturePage = () => ({
    data: {
      items: [structuredClone(viewerTexture)],
      has_next: false,
      next_cursor: null,
      page_size: 20,
    },
  })
  const profilePage = () => ({
    data: {
      items: [structuredClone(viewerProfile)],
      has_next: false,
      next_cursor: null,
      page_size: 20,
    },
  })
  api.getOfficialProfileBindings.mockResolvedValue({ data: { items: [] } })
  api.getExternalIdentities.mockResolvedValue({ data: { items: [] } })
  api.getPublicSkinLibrary.mockImplementation(texturePage)
  api.getTextures.mockImplementation(texturePage)
  api.getAdminTextures.mockImplementation(texturePage)
  api.getTextureDetail.mockImplementation(() => ({ data: structuredClone(viewerTexture) }))
  api.getProfiles.mockImplementation(profilePage)
  api.getAdminProfiles.mockImplementation(profilePage)
  api.getMe.mockResolvedValue({
    data: { id: 'viewer-user', email: 'viewer@example.com', permissions: [] },
  })
  for (const method of [
    api.addToWardrobe,
    api.patchTexture,
    api.deleteTexture,
    api.applyTexture,
    api.patchProfile,
    api.deleteProfile,
    api.clearProfileSkin,
    api.clearProfileCape,
    api.patchAdminProfile,
    api.deleteAdminProfile,
    api.patchProfileSkin,
    api.patchProfileCape,
    api.patchAdminTexture,
    api.deleteAdminTexture,
  ]) {
    method.mockResolvedValue({ data: undefined })
  }
  api.patchTexture.mockImplementation((_hash, _type, patch) =>
    Promise.resolve({ data: { ...structuredClone(viewerTexture), ...patch } }),
  )
  api.setAvatar.mockResolvedValue(true)
  vi.spyOn(ElMessage, 'success').mockImplementation(() => ({ close: vi.fn() }))
  vi.spyOn(ElMessage, 'error').mockImplementation(() => ({ close: vi.fn() }))
  vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm')
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
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  ElMessageBox.close()
})

async function flushUI() {
  await nextTick()
  await new Promise((resolve) => setTimeout(resolve, 40))
  await nextTick()
}
async function openPage(
  page: Page,
  permissions: string[],
  loggedIn = true,
  shouldOpen = true,
  authReady = true,
) {
  const host = document.createElement('div')
  document.body.append(host)
  const pinia = createPinia()
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: pages[page] as Component },
      { path: '/other', component: { render: () => h('div', 'Other page') } },
    ],
  })
  await router.push('/')
  const app = createApp({ render: () => h(RouterView) })
    .use(pinia)
    .use(router)
    .use(ElementPlus)
  const session = useSessionStore(pinia)
  session.user = loggedIn ? { id: 'viewer-user', email: 'viewer@example.com', permissions } : null
  api.getMe.mockImplementation(() =>
    Promise.resolve({
      data: { ...session.user, permissions: [...(session.user?.permissions || [])] },
    }),
  )
  session.authReady = authReady
  app.mount(host)
  cleanup.push(() => {
    app.unmount()
    host.remove()
  })
  await flushUI()
  if (shouldOpen) {
    if (page === 'roles') host.querySelector<HTMLElement>('.role-card')!.click()
    else if (page === 'adminRoles') host.querySelector<HTMLElement>('.admin-role-card')!.click()
    else button(host, '打开材质').click()
  }
  await flushUI()
  return { host, session, router, detail: dialog() }
}
function dialog() {
  return [...document.querySelectorAll<HTMLElement>('.ui-dialog--viewer')].find(
    (el) => el.closest<HTMLElement>('.el-overlay')?.style.display !== 'none',
  )!
}
function button(root: ParentNode, text: string) {
  const result = [...root.querySelectorAll<HTMLButtonElement>('button')].find(
    (el) => el.textContent?.trim() === text,
  )
  if (!result) throw new Error(`Missing button: ${text}`)
  return result
}
async function rename(detail: HTMLElement, name: string, event: 'blur' | 'change') {
  detail.querySelector<HTMLButtonElement>('h2 button')!.click()
  await nextTick()
  const input = detail.querySelector<HTMLInputElement>('input')!
  input.value = name
  input.dispatchEvent(new Event('input', { bubbles: true }))
  await nextTick()
  input.dispatchEvent(new Event(event, { bubbles: true }))
  await nextTick()
  button(detail, '保存名称').click()
  await flushUI()
}

describe('viewer action permissions and scope', () => {
  it.each(viewerReadCases)(
    'keeps $page readable without write controls',
    async ({ page, permission, title, forbidden }) => {
      const { detail } = await openPage(page, [permission])
      expect(detail.querySelector('h2')?.textContent?.trim()).toBe(title)
      expect(detail.querySelectorAll('input')).toHaveLength(0)
      for (const label of forbidden) {
        expect(
          [...detail.querySelectorAll('button')].map((el) => el.textContent?.trim()),
        ).not.toContain(label)
        expect(detail.querySelector(`[aria-label="${label}"]`)).toBeNull()
      }
      for (const write of [
        api.addToWardrobe,
        api.patchTexture,
        api.deleteTexture,
        api.applyTexture,
        api.patchProfile,
        api.deleteProfile,
        api.clearProfileSkin,
        api.clearProfileCape,
        api.patchAdminProfile,
        api.deleteAdminProfile,
        api.patchProfileSkin,
        api.patchProfileCape,
        api.patchAdminTexture,
        api.deleteAdminTexture,
        api.setAvatar,
      ]) {
        expect(write).not.toHaveBeenCalled()
      }
    },
  )

  it('keeps guest details read-only without a collectible command', async () => {
    const { detail } = await openPage('library', [], false)
    expect(detail.querySelector('h2')?.textContent).toBe('Ocean')
    expect(
      [...detail.querySelectorAll('button')].map((el) => el.textContent?.trim()),
    ).not.toContain('添加到衣柜')
  })
  it('requires wardrobe add permission even for a logged-in user or a stale button', async () => {
    const { detail, session } = await openPage('library', ['wardrobe_entry.add.owned'])
    button(detail, '添加到衣柜').click()
    await flushUI()
    expect(api.addToWardrobe).toHaveBeenCalledExactlyOnceWith('viewer-skin', 'skin')
    const stale = button(detail, '添加到衣柜')
    session.user!.permissions = ['texture.read.public']
    stale.click()
    await flushUI()
    expect(api.addToWardrobe).toHaveBeenCalledTimes(1)
    expect(
      [...detail.querySelectorAll('button')].map((el) => el.textContent?.trim()),
    ).not.toContain('添加到衣柜')
  })
  it('allows wardrobe metadata without granting visibility, apply, or delete', async () => {
    const { detail } = await openPage('wardrobe', [
      'texture.read.owned',
      'texture.update_metadata.owned',
    ])
    await rename(detail, 'My note', 'blur')
    expect(api.patchTexture).toHaveBeenCalledExactlyOnceWith('viewer-skin', 'skin', {
      note: 'My note',
    })
    expect(detail.querySelector('.el-radio-group')).not.toBeNull()
    expect(detail.querySelector('.el-switch')).toBeNull()
    expect(detail.textContent).not.toContain('应用到角色')
    expect(detail.textContent).not.toContain('删除纹理')
  })
  it('allows visibility alone without exposing metadata inputs', async () => {
    const { detail } = await openPage('wardrobe', [
      'texture.read.owned',
      'texture.update_visibility.owned',
    ])
    detail.querySelector<HTMLElement>('.el-switch')!.click()
    await flushUI()
    expect(api.patchTexture).toHaveBeenCalledExactlyOnceWith('viewer-skin', 'skin', {
      is_public: true,
    })
    expect(detail.querySelector('.title-input-field')).toBeNull()
    expect(detail.querySelector('.el-radio-group')).toBeNull()
  })
  it.each([
    { permissions: ['texture.read.owned', 'texture.apply.owned'], expected: false },
    { permissions: ['texture.read.owned', 'profile.read.owned'], expected: false },
    {
      permissions: ['texture.read.owned', 'texture.apply.owned', 'profile.read.owned'],
      expected: true,
    },
  ])(
    'requires both apply ability and readable targets: $permissions',
    async ({ permissions, expected }) => {
      const { detail } = await openPage('wardrobe', permissions)
      expect(detail.textContent?.includes('应用到角色')).toBe(expected)
      if (expected) expect(api.getProfiles).toHaveBeenCalledExactlyOnceWith({ limit: 100 })
      else expect(api.getProfiles).not.toHaveBeenCalled()
    },
  )
  it('warns about both existing deletion scopes without guessing uploader identity', async () => {
    const { detail } = await openPage('wardrobe', ['texture.read.owned', 'texture.delete.owned'])
    button(detail, '删除纹理').click()
    await flushUI()
    expect(ElMessageBox.confirm).toHaveBeenCalledExactlyOnceWith(
      wardrobeDeleteWarning,
      '删除纹理',
      {
        confirmButtonText: '确定删除',
        cancelButtonText: '取消',
        type: 'warning',
        confirmButtonClass: 'el-button--danger',
        closeOnClickModal: false,
        distinguishCancelAndClose: true,
      },
    )
    expect(api.deleteTexture).toHaveBeenCalledExactlyOnceWith('viewer-skin', 'skin')
  })
  it.each(['cancel', 'close'])('does not delete or report an error after %s', async (action) => {
    vi.mocked(ElMessageBox.confirm).mockRejectedValueOnce(action)
    const { detail } = await openPage('wardrobe', ['texture.read.owned', 'texture.delete.owned'])
    button(detail, '删除纹理').click()
    await flushUI()
    expect(api.deleteTexture).not.toHaveBeenCalled()
    expect(ElMessage.error).not.toHaveBeenCalled()
  })
  it('rechecks delete permission after confirmation resolves', async () => {
    let confirm!: (value: 'confirm') => void
    vi.mocked(ElMessageBox.confirm).mockReturnValueOnce(
      new Promise((resolve) => {
        confirm = resolve
      }),
    )
    const { detail, session } = await openPage('wardrobe', [
      'texture.read.owned',
      'texture.delete.owned',
    ])
    button(detail, '删除纹理').click()
    await nextTick()
    session.user!.permissions = ['texture.read.owned']
    confirm('confirm')
    await flushUI()
    expect(api.deleteTexture).not.toHaveBeenCalled()
  })
  it('allows personal rename alone', async () => {
    const { detail } = await openPage('roles', ['profile.read.owned', 'profile.update.owned'])
    await rename(detail, 'Renamed', 'change')
    expect(api.patchProfile).toHaveBeenCalledExactlyOnceWith('viewer-profile', { name: 'Renamed' })
    expect(api.deleteProfile).not.toHaveBeenCalled()
    expect(api.clearProfileSkin).not.toHaveBeenCalled()
  })
  it('distinguishes clearing a binding from deleting a texture', async () => {
    const { detail } = await openPage('roles', ['profile.read.owned', 'texture.clear.owned'])
    button(detail, '清除皮肤').click()
    await flushUI()
    expect(ElMessageBox.confirm).toHaveBeenCalledExactlyOnceWith(
      '确定清除该角色的皮肤绑定？此操作只解除绑定，不删除衣柜或皮肤库中的材质。',
      '清除皮肤',
      {
        type: 'warning',
        confirmButtonText: '确认清除',
        cancelButtonText: '取消',
        closeOnClickModal: false,
        distinguishCancelAndClose: true,
      },
    )
    expect(api.clearProfileSkin).toHaveBeenCalledExactlyOnceWith('viewer-profile')
    expect(api.deleteTexture).not.toHaveBeenCalled()
    expect(api.deleteProfile).not.toHaveBeenCalled()
  })
  it('uses the account update ability for avatar selection', async () => {
    const { detail } = await openPage('roles', ['profile.read.owned', 'account.update.self'])
    button(detail, '用作头像').click()
    await flushUI()
    expect(api.setAvatar).toHaveBeenCalledExactlyOnceWith('viewer-skin', 'default')
    expect(detail.querySelector('.title-input-field')).toBeNull()
    expect(detail.textContent).not.toContain('清除皮肤')
    expect(detail.textContent).not.toContain('删除角色')
  })
  it('does not substitute owned permissions for administrative operations', async () => {
    const { detail } = await openPage('adminTextures', [
      'texture.read.any',
      'texture.update_metadata.owned',
      'texture.update_visibility.owned',
      'texture.delete.owned',
    ])
    expect(detail.querySelectorAll('input')).toHaveLength(0)
    expect(detail.textContent).not.toContain('强制下架')
    expect(api.patchAdminTexture).not.toHaveBeenCalled()
  })
  it('uses metadata admin permission without explanatory copy in the viewer', async () => {
    const { detail } = await openPage('adminTextures', [
      'texture.read.any',
      'texture.update_metadata.any',
    ])
    expect(detail.textContent).not.toContain('名称与模型会同步到所有用户对应的衣柜记录')
    await rename(detail, 'Global note', 'blur')
    expect(api.patchAdminTexture).toHaveBeenCalledExactlyOnceWith('viewer-skin', {
      type: 'skin',
      note: 'Global note',
    })
    expect(detail.querySelector('.el-switch')).toBeNull()
    expect(detail.textContent).not.toContain('强制下架')
  })
  it('keeps admin takedown scope accurate and separate from metadata ability', async () => {
    const { detail } = await openPage('adminTextures', ['texture.read.any', 'texture.delete.any'])
    expect(detail.querySelectorAll('input')).toHaveLength(0)
    button(detail, '强制下架').click()
    await flushUI()
    expect(ElMessageBox.confirm).toHaveBeenCalledExactlyOnceWith(adminDeleteWarning, '危险操作', {
      confirmButtonText: '确认下架',
      cancelButtonText: '取消',
      type: 'warning',
      closeOnClickModal: false,
      distinguishCancelAndClose: true,
    })
    expect(api.deleteAdminTexture).toHaveBeenCalledExactlyOnceWith('viewer-skin', {
      force: true,
      type: 'skin',
    })
    expect(api.patchAdminTexture).not.toHaveBeenCalled()
  })
  it('allows administrative role deletion without granting rename or clear', async () => {
    const { detail } = await openPage('adminRoles', ['profile.read.any', 'profile.delete.any'])
    expect(detail.querySelectorAll('input')).toHaveLength(0)
    expect(detail.textContent).not.toContain('清除皮肤')
    button(detail, '删除角色').click()
    await flushUI()
    expect(api.deleteAdminProfile).toHaveBeenCalledExactlyOnceWith('viewer-profile')
    expect(api.patchProfileSkin).not.toHaveBeenCalled()
    expect(api.patchAdminProfile).not.toHaveBeenCalled()
  })

  it('uses administrative update permission for role names and binding changes', async () => {
    const { detail } = await openPage('adminRoles', ['profile.read.any', 'profile.update.any'])
    await rename(detail, 'AdminRenamed', 'blur')
    expect(api.patchAdminProfile).toHaveBeenCalledExactlyOnceWith('viewer-profile', {
      name: 'AdminRenamed',
    })
    expect(detail.textContent).not.toContain('删除角色')
    button(detail, '清除披风').click()
    await flushUI()
    expect(api.patchProfileCape).toHaveBeenCalledExactlyOnceWith('viewer-profile', { hash: null })
    expect(detail.textContent).not.toContain('披风 Hash')
    expect(api.deleteAdminProfile).not.toHaveBeenCalled()
  })

  it('allows administrative visibility without granting metadata editing', async () => {
    const { detail } = await openPage('adminTextures', [
      'texture.read.any',
      'texture.update_visibility.any',
    ])
    detail.querySelector<HTMLElement>('.el-switch')!.click()
    await flushUI()
    expect(api.patchAdminTexture).toHaveBeenCalledExactlyOnceWith('viewer-skin', {
      type: 'skin',
      is_public: true,
    })
    expect(detail.querySelector('.el-input')).toBeNull()
    expect(detail.querySelector('.el-radio-group')).toBeNull()
    expect(detail.textContent).not.toContain('强制下架')
  })

  it('requires owned delete permission for personal role deletion', async () => {
    const { detail } = await openPage('roles', ['profile.read.owned', 'profile.delete.owned'])
    button(detail, '删除角色').click()
    await flushUI()
    expect(ElMessageBox.confirm).toHaveBeenCalledExactlyOnceWith(
      '确定删除此角色？角色记录将永久删除，衣柜中的材质仍会保留。',
      '删除角色',
      {
        type: 'error',
        confirmButtonText: '确认删除',
        cancelButtonText: '取消',
        closeOnClickModal: false,
        distinguishCancelAndClose: true,
      },
    )
    expect(api.deleteProfile).toHaveBeenCalledExactlyOnceWith('viewer-profile')
    expect(api.deleteTexture).not.toHaveBeenCalled()
  })
})

describe('viewer name drafts and write protection', () => {
  async function edit(detail: HTMLElement, name: string) {
    detail.querySelector<HTMLButtonElement>('h2 button')!.click()
    await nextTick()
    const input = detail.querySelector<HTMLInputElement>('input')!
    input.value = name
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    return input
  }

  it('keeps blur local, blocks other writes with a draft and cancels only the name', async () => {
    const { detail } = await openPage('roles', [
      'profile.read.owned',
      'profile.update.owned',
      'texture.clear.owned',
      'profile.delete.owned',
      'account.update.self',
    ])
    expect(detail.querySelector('form')).toBeNull()
    const input = await edit(detail, 'Draft')
    input.dispatchEvent(new Event('change', { bubbles: true }))
    input.dispatchEvent(new Event('blur', { bubbles: true }))
    await flushUI()
    expect(api.patchProfile).not.toHaveBeenCalled()
    for (const label of ['用作头像', '清除皮肤', '清除披风', '删除角色'])
      expect(button(detail, label).disabled).toBe(true)
    button(detail, '取消').click()
    await flushUI()
    expect(detail.querySelector('h2')?.textContent?.trim()).toBe('Player')
    expect(detail.querySelector('form')).toBeNull()
    expect(button(detail, '清除皮肤').disabled).toBe(false)
    expect(dialog()).toBe(detail)
  })

  it('does not dirty or send an unchanged name and ignores IME Enter', async () => {
    const { detail } = await openPage('roles', [
      'profile.read.owned',
      'profile.update.owned',
      'texture.clear.owned',
    ])
    const input = await edit(detail, 'Player')
    expect(button(detail, '保存名称').disabled).toBe(true)
    expect(button(detail, '清除皮肤').disabled).toBe(false)
    input.value = 'NewName'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    input.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, isComposing: true }),
    )
    await flushUI()
    expect(api.patchProfile).not.toHaveBeenCalled()
    input.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }),
    )
    await flushUI()
    expect(detail.querySelector('form')).toBeNull()
    expect(dialog()).toBe(detail)
    expect(ElMessageBox.confirm).not.toHaveBeenCalled()
  })

  it('saves once, locks close and navigation while pending, and updates the captured role', async () => {
    let resolve!: (result: { data: void }) => void
    api.patchProfile.mockReturnValueOnce(
      new Promise((done) => {
        resolve = done
      }),
    )
    const { detail, router, host } = await openPage('roles', [
      'profile.read.owned',
      'profile.update.owned',
      'texture.clear.owned',
    ])
    const input = await edit(detail, 'Saved')
    button(detail, '保存名称').click()
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    await nextTick()
    expect(api.patchProfile).toHaveBeenCalledExactlyOnceWith('viewer-profile', { name: 'Saved' })
    expect(input.disabled).toBe(true)
    expect(button(detail, '取消').disabled).toBe(true)
    detail.querySelector<HTMLButtonElement>('[aria-label="关闭此对话框"]')!.click()
    await router.push('/other')
    expect(router.currentRoute.value.path).toBe('/')
    expect(dialog()).toBe(detail)
    resolve({ data: undefined })
    await flushUI()
    expect(detail.querySelector('form')).toBeNull()
    expect(detail.querySelector('h2')?.textContent?.trim()).toBe('Saved')
    expect(host.querySelector('.role-card')?.textContent).toContain('Saved')
    expect(button(detail, '清除皮肤').disabled).toBe(false)
  })

  it('retains a failed draft and releases the lock for repair or retry', async () => {
    api.patchAdminProfile.mockRejectedValueOnce({
      response: {
        status: 409,
        data: { error: { object: 'profile_name', operation: 'reserve', reason: 'conflict' } },
      },
    })
    const { detail } = await openPage('adminRoles', ['profile.read.any', 'profile.update.any'])
    const input = await edit(detail, 'Taken')
    button(detail, '保存名称').click()
    await flushUI()
    expect(input.value).toBe('Taken')
    expect(input.disabled).toBe(false)
    expect(detail.querySelector('[role="alert"]')?.textContent).toContain('名称未保存')
    expect(button(detail, '清除皮肤').disabled).toBe(true)
    expect(ElMessage.error).toHaveBeenCalledExactlyOnceWith('角色名已被占用')
    button(detail, '保存名称').click()
    await flushUI()
    expect(api.patchAdminProfile).toHaveBeenCalledTimes(2)
    expect(detail.querySelector('h2')?.textContent?.trim()).toBe('Taken')
  })

  it('asks on outer Esc but discards directly on main X', async () => {
    const { detail } = await openPage('roles', ['profile.read.owned', 'profile.update.owned'])
    await edit(detail, 'Discard')
    button(detail, '保存名称').focus()
    // Move outside the local editor so this is an outer dismissal.
    detail.focus()
    vi.mocked(ElMessageBox.confirm).mockRejectedValueOnce('cancel')
    detail.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape', code: 'Escape', bubbles: true }),
    )
    await flushUI()
    expect(ElMessageBox.confirm).toHaveBeenCalledWith(
      '当前修改尚未保存，确定放弃吗？',
      '放弃修改',
      expect.objectContaining({ cancelButtonText: '继续编辑', closeOnClickModal: false }),
    )
    expect(detail.querySelector<HTMLInputElement>('input')?.value).toBe('Discard')
    detail.querySelector<HTMLButtonElement>('[aria-label="关闭此对话框"]')!.click()
    await flushUI()
    expect(ElMessageBox.confirm).toHaveBeenCalledTimes(1)
    await vi.waitFor(() => expect(dialog()).toBeUndefined())
  })

  it('hides confirmed absent bindings and clear actions without deleting the role or closing', async () => {
    const { detail } = await openPage('roles', ['profile.read.owned', 'texture.clear.owned'])
    button(detail, '清除皮肤').click()
    await flushUI()
    expect(detail.textContent).not.toContain('皮肤 Hash')
    expect([...detail.querySelectorAll('button')].map((b) => b.textContent?.trim())).not.toContain(
      '清除皮肤',
    )
    expect(dialog()).toBe(detail)
    expect(api.deleteProfile).not.toHaveBeenCalled()
    expect(api.deleteTexture).not.toHaveBeenCalled()
  })

  it('does not enable writes when the wardrobe detail read fails', async () => {
    api.getTextureDetail.mockRejectedValueOnce(new Error('offline'))
    const { detail } = await openPage('wardrobe', [
      'texture.read.owned',
      'texture.update_metadata.owned',
      'texture.update_visibility.owned',
      'texture.delete.owned',
    ])
    expect(detail.querySelector('[role="alert"]')?.textContent).toContain('获取详情失败')
    expect(detail.querySelector('h2 button')).toBeNull()
    expect(detail.querySelector('.el-switch')).toBeNull()
    expect([...detail.querySelectorAll('button')].map((b) => b.textContent?.trim())).not.toContain(
      '删除纹理',
    )
    expect(api.patchTexture).not.toHaveBeenCalled()
  })

  it('ignores the wardrobe read from a closed earlier session', async () => {
    let resolve!: (result: { data: typeof viewerTexture }) => void
    api.getTextureDetail.mockReturnValueOnce(
      new Promise((done) => {
        resolve = done
      }),
    )
    const { detail, host } = await openPage('wardrobe', [
      'texture.read.owned',
      'texture.update_metadata.owned',
    ])
    detail.querySelector<HTMLButtonElement>('[aria-label="关闭此对话框"]')!.click()
    await flushUI()
    button(host, '打开材质').click()
    await flushUI()
    const next = dialog()
    expect(next.querySelector('h2')?.textContent?.trim()).toBe('Ocean')
    resolve({ data: { ...viewerTexture, note: 'Stale' } })
    await flushUI()
    expect(next.querySelector('h2')?.textContent?.trim()).toBe('Ocean')
  })
})

describe('viewer read and failed command boundaries', () => {
  it.each([
    {
      page: 'wardrobe' as const,
      permission: 'texture.update_metadata.owned',
      read: api.getTextures,
      label: '无权读取衣柜',
    },
    {
      page: 'roles' as const,
      permission: 'profile.update.owned',
      read: api.getProfiles,
      label: '无权读取角色列表',
    },
    {
      page: 'adminRoles' as const,
      permission: 'profile.update.any',
      read: api.getAdminProfiles,
      label: '无权读取角色列表',
    },
    {
      page: 'adminTextures' as const,
      permission: 'texture.update_metadata.any',
      read: api.getAdminTextures,
      label: '无权读取材质列表',
    },
  ])(
    'does not infer $page read access from write permission',
    async ({ page, permission, read, label }) => {
      const { host } = await openPage(page, [permission], true, false)
      expect(read).not.toHaveBeenCalled()
      expect(host.textContent).toContain(label)
      expect(dialog()).toBeUndefined()
    },
  )

  it('distinguishes a disabled public library from permission denial', async () => {
    api.getPublicSkinLibrary.mockRejectedValueOnce({
      response: {
        status: 403,
        data: { error: { object: 'permission', operation: 'check', reason: 'denied' } },
      },
    })
    const { host } = await openPage('library', [], true, false)
    expect(host.textContent).not.toContain('皮肤库已关闭')
    expect(host.textContent).toContain('没有执行此操作的权限')
    expect(host.textContent).not.toContain('库中暂无公开材质')
  })

  it('keeps a failed model change at its confirmed value', async () => {
    api.patchAdminTexture.mockRejectedValueOnce({
      response: {
        status: 400,
        data: { error: { object: 'texture', operation: 'update', reason: 'invalid' } },
      },
    })
    const { detail } = await openPage('adminTextures', [
      'texture.read.any',
      'texture.update_metadata.any',
    ])
    const slim = detail.querySelector<HTMLInputElement>('input[value="slim"]')!
    slim.click()
    await flushUI()
    expect(api.patchAdminTexture).toHaveBeenCalledExactlyOnceWith('viewer-skin', {
      type: 'skin',
      model: 'slim',
    })
    expect(detail.querySelector<HTMLInputElement>('input[value="default"]')!.checked).toBe(true)
    expect(slim.checked).toBe(false)
    expect(ElMessage.error).toHaveBeenCalledTimes(1)
  })

  it('reports a failed admin takedown and retains the original record and dialog', async () => {
    api.deleteAdminTexture.mockRejectedValueOnce({
      response: {
        status: 403,
        data: { error: { object: 'permission', operation: 'check', reason: 'denied' } },
      },
    })
    const { detail, host } = await openPage('adminTextures', [
      'texture.read.any',
      'texture.delete.any',
    ])
    button(detail, '强制下架').click()
    await flushUI()
    expect(api.deleteAdminTexture).toHaveBeenCalledExactlyOnceWith('viewer-skin', {
      force: true,
      type: 'skin',
    })
    expect(ElMessage.error).toHaveBeenCalledExactlyOnceWith('没有执行此操作的权限')
    expect(button(detail, '强制下架').disabled).toBe(false)
    expect(dialog()).toBe(detail)
    expect(button(host, '打开材质')).toBeDefined()
  })
})

describe('session restoration on direct page refresh', () => {
  it.each([
    {
      page: 'wardrobe' as const,
      permission: 'texture.read.owned',
      read: api.getTextures,
      card: '打开材质',
    },
    {
      page: 'roles' as const,
      permission: 'profile.read.owned',
      read: api.getProfiles,
      card: 'Player',
    },
    {
      page: 'adminRoles' as const,
      permission: 'profile.read.any',
      read: api.getAdminProfiles,
      card: 'Player',
    },
    {
      page: 'adminTextures' as const,
      permission: 'texture.read.any',
      read: api.getAdminTextures,
      card: '打开材质',
    },
    {
      page: 'library' as const,
      permission: 'texture.read.public',
      read: api.getPublicSkinLibrary,
      card: '打开材质',
    },
  ])(
    'loads $page when the authenticated session arrives after mount',
    async ({ page, permission, read, card }) => {
      const { host, session } = await openPage(page, [], false, false, false)
      expect(read).not.toHaveBeenCalled()
      expect(host.textContent).not.toMatch(
        /还没有纹理|还没有角色|暂无材质数据|暂无角色数据|库中暂无公开材质/,
      )
      session.user = {
        id: 'restored-user',
        email: 'restored@example.com',
        permissions: [permission],
      }
      session.authReady = true
      await flushUI()
      expect(read).toHaveBeenCalledTimes(1)
      expect(host.textContent).toContain(card)
    },
  )
})

describe('personal role card shortcuts', () => {
  const rolePermissions = [
    'profile.read.owned',
    'account.update.self',
    'texture.clear.owned',
    'profile.delete.owned',
    'profile.create.owned',
  ]
  it('offers only usable shortcuts without a delete entry and does not open the detail when used', async () => {
    const { host } = await openPage('roles', rolePermissions, true, false)
    const card = host.querySelector<HTMLElement>('.role-card')!
    expect([...card.querySelectorAll('button')].map((el) => el.textContent?.trim())).toEqual([
      '头像',
      '皮肤',
      '披风',
    ])
    expect([...card.querySelectorAll('button')].map((el) => el.getAttribute('aria-label'))).toEqual(
      ['设为头像', '清除皮肤', '清除披风'],
    )
    const avatar = button(card, '头像')
    avatar.focus()
    await flushUI()
    expect(
      document.getElementById(avatar.getAttribute('aria-describedby')!)?.textContent?.trim(),
    ).toBe('设为头像')
    for (const code of ['Enter', 'Space']) {
      const key = new KeyboardEvent('keydown', {
        code,
        key: code === 'Space' ? ' ' : 'Enter',
        bubbles: true,
        cancelable: true,
      })
      avatar.dispatchEvent(key)
      expect(key.defaultPrevented).toBe(false)
    }
    avatar.click()
    await flushUI()
    expect(api.setAvatar).toHaveBeenCalledExactlyOnceWith('viewer-skin', 'default')
    expect(dialog()).toBeUndefined()
    const skin = button(card, '皮肤')
    skin.dispatchEvent(new MouseEvent('mouseenter'))
    await flushUI()
    expect(
      document.getElementById(skin.getAttribute('aria-describedby')!)?.textContent?.trim(),
    ).toBe('清除皮肤')
    skin.click()
    await flushUI()
    expect(api.clearProfileSkin).toHaveBeenCalledExactlyOnceWith('viewer-profile')
    expect([...card.querySelectorAll('button')].map((el) => el.textContent?.trim())).toEqual([
      '披风',
    ])
    expect(dialog()).toBeUndefined()
    expect(api.deleteProfile).not.toHaveBeenCalled()
    card.querySelector<HTMLElement>('[role="button"]')!.click()
    await flushUI()
    expect(button(dialog(), '删除角色')).toBeDefined()
  })

  it('keeps read-only cards free of shortcuts', async () => {
    const { host } = await openPage('roles', ['profile.read.owned'], true, false)
    expect(host.querySelector('.role-card .card-actions')).toBeNull()
    expect(host.querySelectorAll('.role-card button')).toHaveLength(0)
  })

  it('hides missing-material shortcuts even with all relevant permissions', async () => {
    api.getProfiles.mockResolvedValueOnce({
      data: {
        items: [{ ...viewerProfile, skin_hash: null, cape_hash: null }],
        has_next: false,
        next_cursor: null,
        page_size: 20,
      },
    })
    const { host } = await openPage('roles', rolePermissions, true, false)
    expect(host.querySelector('.role-card .card-actions')).toBeNull()
    expect(host.querySelectorAll('.role-card button')).toHaveLength(0)
  })

  it('serializes card writes, blocks opening another flow and restores actions after failure', async () => {
    let reject!: (reason: unknown) => void
    api.clearProfileCape.mockReturnValueOnce(
      new Promise((_, fail) => {
        reject = fail
      }),
    )
    const { host } = await openPage('roles', rolePermissions, true, false)
    const card = host.querySelector<HTMLElement>('.role-card')!
    button(card, '披风').click()
    await flushUI()
    expect(api.clearProfileCape).toHaveBeenCalledExactlyOnceWith('viewer-profile')
    for (const action of card.querySelectorAll<HTMLButtonElement>('button'))
      expect(action.disabled).toBe(true)
    expect(button(host, '新建角色').disabled).toBe(true)
    card.querySelector<HTMLElement>('[role="button"]')!.click()
    await flushUI()
    expect(dialog()).toBeUndefined()
    reject({
      response: {
        status: 403,
        data: { error: { object: 'permission', operation: 'check', reason: 'denied' } },
      },
    })
    await flushUI()
    expect(ElMessage.error).toHaveBeenCalledExactlyOnceWith('没有执行此操作的权限')
    expect(button(card, '披风').disabled).toBe(false)
    expect(button(host, '新建角色').disabled).toBe(false)
  })

  it('rechecks permission after the card clear confirmation and preserves the binding on cancellation', async () => {
    let confirm!: (value: 'confirm') => void
    vi.mocked(ElMessageBox.confirm).mockReturnValueOnce(
      new Promise((resolve) => {
        confirm = resolve
      }),
    )
    const { host, session } = await openPage('roles', rolePermissions, true, false)
    button(host, '皮肤').click()
    await nextTick()
    session.user!.permissions = ['profile.read.owned']
    confirm('confirm')
    await flushUI()
    expect(api.clearProfileSkin).not.toHaveBeenCalled()
    expect(api.deleteProfile).not.toHaveBeenCalled()
  })
})

describe('official role card status', () => {
  it('shows the official badge only after binding data is ready, and removes it when read access is lost', async () => {
    let resolve!: (result: { data: { items: (typeof officialBinding)[] } }) => void
    api.getOfficialProfileBindings.mockReturnValueOnce(
      new Promise((done) => {
        resolve = done
      }),
    )
    const { host, session } = await openPage(
      'roles',
      ['profile.read.owned', 'official_profile.read.owned'],
      true,
      false,
    )
    expect(host.querySelector('.role-card .el-tag--success')).toBeNull()
    resolve({
      data: {
        items: [{ ...officialBinding, profile_id: viewerProfile.id, profile: viewerProfile }],
      },
    })
    await flushUI()
    expect(host.querySelector('.role-card .el-tag--success')?.textContent).toBe('正版')
    session.user!.permissions = ['profile.read.owned']
    await flushUI()
    expect(host.querySelector('.role-card .el-tag--success')).toBeNull()
  })

  it('keeps a failed binding read distinct from an unbound role without showing an official badge', async () => {
    api.getOfficialProfileBindings.mockRejectedValueOnce({ response: { status: 403 } })
    const { host, detail } = await openPage('roles', [
      'profile.read.owned',
      'official_profile.read.owned',
    ])
    expect(host.querySelector('.role-card .el-tag--success')).toBeNull()
    const official = detail.querySelector('section[aria-label="正版绑定"]')!
    expect(official.textContent).toContain('正版绑定加载失败')
    expect(official.querySelector('.el-tag')).toBeNull()
    expect(official.querySelector('button')).toBeNull()
  })
})
