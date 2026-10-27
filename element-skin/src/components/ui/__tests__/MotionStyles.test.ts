import { createApp, h, nextTick, ref } from 'vue'
import { createPinia } from 'pinia'
import { useSessionStore } from '@/stores/session'
import ElementPlus, { ElButton, ElMessage } from 'element-plus'
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { installMotionStyles, motionCss } from '@/__tests__/helpers/motionStyles'
import AdminRolesList from '@/components/admin/roles/AdminRolesList.vue'
import AdminTexturesList from '@/components/admin/textures/AdminTexturesList.vue'
import {
  cardMotionPage,
  contentEntrances,
  textureMotionCases,
  textureMotionPage,
} from './fixtures/motionCases'

const api = vi.hoisted(() => ({ getAdminProfiles: vi.fn(), getAdminTextures: vi.fn() }))
vi.mock('@/api/admin/profiles', () => ({
  getAdminProfiles: api.getAdminProfiles,
  patchAdminProfile: vi.fn(),
  deleteAdminProfile: vi.fn(),
  patchProfileSkin: vi.fn(),
  patchProfileCape: vi.fn(),
}))
vi.mock('@/components/SkinViewer.vue', () => ({ default: { render: () => h('div') } }))
vi.mock('@/api/admin/textures', () => ({
  getAdminTextures: api.getAdminTextures,
  patchAdminTexture: vi.fn(),
  deleteAdminTexture: vi.fn(),
}))

let styles: ReturnType<typeof installMotionStyles>
const cleanup: Array<() => void> = []

async function mountTextures(dark = false) {
  const host = document.createElement('div')
  document.body.append(host)
  const app = createApp(AdminTexturesList)
  const pinia = createPinia()
  app.use(pinia).use(ElementPlus)
  useSessionStore(pinia).user = {
    id: 'motion-user',
    email: 'motion@example.com',
    permissions: ['texture.read.any'],
  }
  app.provide('isDark', ref(dark)).mount(host)
  cleanup.push(() => {
    app.unmount()
    host.remove()
  })
  await nextTick()
  return host
}
beforeAll(() => {
  styles = installMotionStyles()
})
afterAll(() => styles.remove())
afterEach(() =>
  cleanup
    .splice(0)
    .reverse()
    .forEach((dispose) => dispose()),
)

describe('shared motion contracts', () => {
  it.each(contentEntrances)(
    'keeps $className visible without delay or displacement under reduced motion',
    ({ className, duration }) => {
      const content = document.createElement('div')
      content.className = className
      content.style.setProperty('--delay-index', '20')
      document.body.append(content)
      cleanup.push(() => content.remove())
      expect(motionCss(content, 'animation')).toContain('contentEnter')
      expect(motionCss(content, 'animation')).toContain(duration)
      expect(motionCss(content, 'animation')).toContain('backwards')
      expect(motionCss(document.documentElement, '--default-transition-duration')).toBe('180ms')
      cleanup.push(styles.reduce())
      expect(motionCss(content, 'animation')).toBe('none')
      expect(motionCss(content, 'opacity')).toBe('1')
      expect(motionCss(content, 'translate')).toBe('none')
      expect(motionCss(content, '--motion-stagger')).toBe('0ms')
      expect(motionCss(content, '--motion-offset-hover')).toBe('0px')
      expect(motionCss(document.documentElement, '--default-transition-duration')).toBe('0ms')
    },
  )

  it('limits ordinary button feedback to paint properties and retains its loading protection', () => {
    const host = document.createElement('div')
    document.body.append(host)
    const app = createApp({ render: () => h(ElButton, { loading: true }, () => '保存') })
    app.mount(host)
    cleanup.push(() => {
      app.unmount()
      host.remove()
    })
    const button = host.querySelector('button')!
    expect(motionCss(button, 'transition')).toContain('background-color 120ms')
    expect(motionCss(button, 'transition')).not.toMatch(/(?:^|,\s*)(all|transform|width|height)\s/)
    cleanup.push(styles.reduce())
    expect(button.disabled).toBe(true)
    expect(button.classList.contains('is-loading')).toBe(true)
    expect(button.querySelectorAll('.el-icon.is-loading')).toHaveLength(1)
    expect(motionCss(button, 'transition')).toContain('background-color 0ms')
    expect(motionCss(button, '--control-height-default')).toBe('32px')
  })

  it('does not reset transforms that position layout elements', () => {
    const positioned = document.createElement('div')
    positioned.style.transform = 'translateX(-50%)'
    document.body.append(positioned)
    cleanup.push(() => positioned.remove())
    cleanup.push(styles.reduce())
    expect(motionCss(positioned, 'transform')).toBe('translateX(-50%)')
  })

  it.each([false, true])(
    'gives late cards their own entrance and retains them through refresh (reduced: %s)',
    async (reduced) => {
      const requests: Array<(page: typeof cardMotionPage) => void> = []
      api.getAdminProfiles
        .mockReset()
        .mockImplementation(() => new Promise((resolve) => requests.push(resolve)))
      if (reduced) cleanup.push(styles.reduce())
      const host = document.createElement('div')
      document.body.append(host)
      const app = createApp(AdminRolesList)
      const pinia = createPinia()
      app.use(pinia).use(ElementPlus)
      useSessionStore(pinia).user = {
        id: 'motion-user',
        email: 'motion@example.com',
        permissions: ['profile.read.any'],
      }
      app.mount(host)
      cleanup.push(() => {
        app.unmount()
        host.remove()
      })
      await nextTick()

      const header = host.querySelector('.ui-page-header')!
      const headerGroup = header.closest('.animate-fade-in')!
      expect(motionCss(headerGroup, 'animation')).toBe(
        reduced ? 'none' : 'contentEnter 320ms cubic-bezier(0.4,0,0.2,1) backwards',
      )
      expect(host.querySelectorAll('.admin-role-card')).toHaveLength(0)
      expect(api.getAdminProfiles).toHaveBeenCalledExactlyOnceWith({ limit: 20, cursor: null })

      requests.shift()!(structuredClone(cardMotionPage))
      await vi.waitFor(() => expect(host.querySelectorAll('.admin-role-card')).toHaveLength(2))
      const cards = [...host.querySelectorAll('.admin-role-card')]
      expect(cards.map((card) => card.querySelector('[title]')?.textContent)).toEqual([
        'PlayerA',
        'PlayerB',
      ])
      expect(cards.map((card) => motionCss(card, '--delay-index'))).toEqual(['0', '1'])
      expect(motionCss(cards[0]!.parentElement!, '--motion-card-count')).toBe('2')
      for (const card of cards) {
        expect(card.parentElement!.closest('.animate-fade-in')).toBeNull()
        expect(motionCss(card, 'animation')).toBe(
          reduced ? 'none' : 'contentEnter 400ms cubic-bezier(0.4,0,0.2,1) backwards',
        )
        if (reduced) {
          expect(motionCss(card, 'opacity')).toBe('1')
          expect(motionCss(card, 'translate')).toBe('none')
          expect(motionCss(card, '--motion-stagger-limit')).toBe('0ms')
        }
      }

      const refresh = [...host.querySelectorAll<HTMLButtonElement>('button')].find(
        (button) => button.textContent?.trim() === '刷新',
      )!
      refresh.click()
      await nextTick()
      expect(api.getAdminProfiles).toHaveBeenCalledTimes(2)
      expect(host.querySelectorAll('.admin-role-card')[0]).toBe(cards[0])
      requests.shift()!(structuredClone(cardMotionPage))
      await nextTick()
      expect(host.querySelectorAll('.admin-role-card')[0]).toBe(cards[0])
      expect(host.querySelectorAll('.admin-role-card')[1]).toBe(cards[1])
      expect(host.querySelector('.ui-page-header')).toBe(header)
    },
  )

  it.each(textureMotionCases)(
    'loads late textures without replacing the list region (dark: $dark, reduced: $reduced)',
    async ({ dark, reduced }) => {
      let resolve!: (page: typeof textureMotionPage) => void
      api.getAdminTextures.mockReset().mockReturnValue(
        new Promise((done) => {
          resolve = done
        }),
      )
      const originalDark = document.documentElement.classList.contains('dark')
      document.documentElement.classList.toggle('dark', dark)
      cleanup.push(() => document.documentElement.classList.toggle('dark', originalDark))
      if (reduced) cleanup.push(styles.reduce())
      const host = await mountTextures(dark)
      const region = host.querySelector<HTMLElement>('[aria-label="材质列表"]')!

      expect(region.getAttribute('aria-busy')).toBe('true')
      expect(region.querySelector<HTMLElement>('.el-loading-mask')?.style.backgroundColor).toBe(
        'transparent',
      )
      expect(host.querySelector('.skeleton-card')).toBeNull()
      expect(host.textContent).not.toContain('暂无材质数据')
      expect(api.getAdminTextures).toHaveBeenCalledExactlyOnceWith({ limit: 20, cursor: null })

      resolve(structuredClone(textureMotionPage))
      await vi.waitFor(() => expect(region.querySelectorAll('.texture-card')).toHaveLength(2))
      expect(host.querySelector('[aria-label="材质列表"]')).toBe(region)
      expect(region.getAttribute('aria-busy')).toBe('false')
      expect(region.closest('.animate-fade-in')).toBeNull()
      expect(region.textContent).toContain('MotionSkinA')
      expect(region.textContent).toContain('MotionSkinB')
      const cards = [...region.querySelectorAll('.texture-card')]
      expect(cards.map((card) => motionCss(card, '--delay-index'))).toEqual(['0', '1'])
      for (const card of cards) {
        expect(motionCss(card, 'animation')).toBe(
          reduced ? 'none' : 'contentEnter 400ms cubic-bezier(0.4,0,0.2,1) backwards',
        )
        if (reduced) {
          expect(motionCss(card, 'opacity')).toBe('1')
          expect(motionCss(card, 'translate')).toBe('none')
        }
      }
    },
  )

  it('keeps existing textures and their list region when refresh fails', async () => {
    api.getAdminTextures.mockReset().mockResolvedValueOnce(structuredClone(textureMotionPage))
    const errorMessage = vi.spyOn(ElMessage, 'error').mockReturnValue({ close: vi.fn() })
    const host = await mountTextures()
    const region = host.querySelector<HTMLElement>('[aria-label="材质列表"]')!
    await vi.waitFor(() => expect(region.querySelectorAll('.texture-card')).toHaveLength(2))
    const cards = [...region.querySelectorAll('.texture-card')]
    let reject!: (error: Error) => void
    api.getAdminTextures.mockReturnValueOnce(
      new Promise((_, fail) => {
        reject = fail
      }),
    )
    const refresh = [...host.querySelectorAll<HTMLButtonElement>('button')].find(
      (button) => button.textContent?.trim() === '刷新',
    )!
    refresh.click()
    await nextTick()
    await vi.waitFor(() => expect(region.getAttribute('aria-busy')).toBe('true'))
    expect(region.querySelectorAll('.texture-card')[0]).toBe(cards[0])
    reject(new Error('refresh unavailable'))
    await vi.waitFor(() => expect(region.getAttribute('aria-busy')).toBe('false'))
    expect(api.getAdminTextures).toHaveBeenNthCalledWith(2, { limit: 20, cursor: null })
    expect(errorMessage).toHaveBeenCalledExactlyOnceWith('加载材质列表失败')
    expect(host.querySelector('[aria-label="材质列表"]')).toBe(region)
    expect(region.querySelectorAll('.texture-card')[0]).toBe(cards[0])
    expect(region.querySelectorAll('.texture-card')[1]).toBe(cards[1])
    expect(region.textContent).not.toContain('暂无材质数据')
  })

  it('shows the texture empty state only after the first response confirms it', async () => {
    let resolve!: (page: typeof textureMotionPage) => void
    api.getAdminTextures.mockReset().mockReturnValue(
      new Promise((done) => {
        resolve = done
      }),
    )
    const host = await mountTextures()
    const region = host.querySelector('[aria-label="材质列表"]')!
    expect(region.getAttribute('aria-busy')).toBe('true')
    expect(region.textContent).not.toContain('暂无材质数据')
    resolve({ data: { ...textureMotionPage.data, items: [] } })
    await vi.waitFor(() => expect(region.textContent).toContain('暂无材质数据'))
    expect(region.getAttribute('aria-busy')).toBe('false')
    expect(region.querySelectorAll('.texture-card')).toHaveLength(0)
    expect(host.querySelector('[aria-label="材质列表"]')).toBe(region)
  })
})
