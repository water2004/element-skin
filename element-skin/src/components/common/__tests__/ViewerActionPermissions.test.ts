import { createApp, defineComponent, h, nextTick, type Component } from 'vue'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import ElementPlus, { ElMessage, ElMessageBox } from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useSessionStore } from '@/stores/session'
import SkinLibraryView from '@/views/SkinLibraryView.vue'
import DashboardWardrobe from '@/components/dashboard/wardrobe/DashboardWardrobe.vue'
import DashboardRoles from '@/components/dashboard/roles/DashboardRoles.vue'
import AdminRolesList from '@/components/admin/roles/AdminRolesList.vue'
import AdminTexturesList from '@/components/admin/textures/AdminTexturesList.vue'
import {
  adminDeleteWarning,
  viewerProfile,
  viewerReadCases,
  viewerTexture,
  wardrobeDeleteWarning,
} from './fixtures/viewerActions'

const api = vi.hoisted(() => ({
  getPublicSkinLibrary: vi.fn(),
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
vi.mock('@/components/textures/textureAssets', () => ({
  cacheSkinTextureWidths: vi.fn(),
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
async function openPage(page: Page, permissions: string[], loggedIn = true) {
  const host = document.createElement('div')
  document.body.append(host)
  const pinia = createPinia()
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { render: () => h('div') } }],
  })
  await router.push('/')
  const app = createApp(pages[page] as Component)
    .use(pinia)
    .use(router)
    .use(ElementPlus)
  const session = useSessionStore(pinia)
  session.user = loggedIn ? { id: 'viewer-user', email: 'viewer@example.com', permissions } : null
  app.mount(host)
  cleanup.push(() => {
    app.unmount()
    host.remove()
  })
  await flushUI()
  if (page === 'roles') host.querySelector<HTMLElement>('.role-card')!.click()
  else if (page === 'adminRoles') button(host, '查看详情').click()
  else button(host, '打开材质').click()
  await flushUI()
  return { host, session, detail: dialog() }
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
  const input = detail.querySelector<HTMLInputElement>('input')!
  input.value = name
  input.dispatchEvent(new Event('input', { bubbles: true }))
  await nextTick()
  input.dispatchEvent(new Event(event, { bubbles: true }))
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

  it('shows a login hint to guests without a collectible command', async () => {
    const { detail } = await openPage('library', [], false)
    expect(detail.textContent).toContain('登录后可按账号权限收藏此纹理')
    expect(
      [...detail.querySelectorAll('button')].map((el) => el.textContent?.trim()),
    ).not.toContain('添加到我的衣柜')
  })
  it('requires wardrobe add permission even for a logged-in user or a stale button', async () => {
    const { detail, session } = await openPage('library', ['wardrobe_entry.add.owned'])
    button(detail, '添加到我的衣柜').click()
    await flushUI()
    expect(api.addToWardrobe).toHaveBeenCalledExactlyOnceWith('viewer-skin', 'skin')
    const stale = button(detail, '添加到我的衣柜')
    session.user!.permissions = ['texture.read.public']
    stale.click()
    await flushUI()
    expect(api.addToWardrobe).toHaveBeenCalledTimes(1)
    expect(detail.textContent).toContain('当前账号没有添加到衣柜的权限')
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
      { type: 'warning', confirmButtonText: '确认清除', cancelButtonText: '取消' },
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
    expect(detail.textContent).not.toContain('删除此角色')
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
  it('shows metadata impact and uses only the metadata admin permission', async () => {
    const { detail } = await openPage('adminTextures', [
      'texture.read.any',
      'texture.update_metadata.any',
    ])
    expect(detail.textContent).toContain('名称与模型会同步到所有用户对应的衣柜记录')
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
    expect(detail.querySelector('[aria-label="披风绑定"]')?.textContent).toContain('未绑定')
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
    button(detail, '删除此角色').click()
    await flushUI()
    expect(ElMessageBox.confirm).toHaveBeenCalledExactlyOnceWith(
      '确定删除此角色？角色记录将永久删除，衣柜中的材质仍会保留。',
      '删除角色',
      { type: 'error', confirmButtonText: '确认删除', cancelButtonText: '取消' },
    )
    expect(api.deleteProfile).toHaveBeenCalledExactlyOnceWith('viewer-profile')
    expect(api.deleteTexture).not.toHaveBeenCalled()
  })
})
