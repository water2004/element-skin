import { createApp, h, nextTick, ref } from 'vue'
import ElementPlus, { ElDrawer, ElImageViewer, ElMessageBox } from 'element-plus'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import UiDialog from '../UiDialog.vue'
import {
  classicScrollbar,
  overlayThemes,
  scrollLockOverlays,
  nestedScrollOverlays,
} from './fixtures/overlayScrollCases'

const styles = document.createElement('style')
const cleanup: Array<() => void> = []
let originalWidth = ''
let originalClasses = ''

beforeAll(() => {
  styles.textContent = [
    'node_modules/element-plus/dist/index.css',
    'node_modules/element-plus/theme-chalk/dark/css-vars.css',
    'src/assets/styles/appearance.css',
    'src/assets/styles/tokens.css',
    'src/assets/styles/element-overrides.css',
  ]
    .map((file) => readFileSync(path.join(process.cwd(), file), 'utf8'))
    .join('\n')
  document.head.append(styles)
})
afterAll(() => styles.remove())
beforeEach(() => {
  originalWidth = document.body.style.width
  originalClasses = document.body.className
  vi.spyOn(document.documentElement, 'clientHeight', 'get').mockReturnValue(
    classicScrollbar.viewportHeight,
  )
  vi.spyOn(document.body, 'scrollHeight', 'get').mockReturnValue(classicScrollbar.contentHeight)
  vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockImplementation(function (
    this: HTMLElement,
  ) {
    if (this.classList.contains('el-scrollbar__wrap')) return classicScrollbar.outerWidth
    if (this.parentElement?.classList.contains('el-scrollbar__wrap'))
      return classicScrollbar.innerWidth
    return 0
  })
})
afterEach(async () => {
  cleanup
    .splice(0)
    .reverse()
    .forEach((dispose) => dispose())
  ElMessageBox.close()
  await settle(250)
  document.body.style.width = originalWidth
  document.body.className = originalClasses
  document.documentElement.classList.remove('dark')
})

async function settle(delay = 30) {
  await nextTick()
  await new Promise((done) => setTimeout(done, delay))
  await nextTick()
}

async function mountOverlay(kind: (typeof scrollLockOverlays)[number]['kind']) {
  const open = ref(false)
  const host = document.createElement('div')
  document.body.append(host)
  const app = createApp({
    render: () => {
      if (kind === 'message-box') return h('div')
      if (kind === 'image-viewer')
        return open.value
          ? h(ElImageViewer, {
              urlList: ['/test.png'],
              teleported: true,
              onClose: () => {
                open.value = false
              },
            })
          : null
      return h(
        kind === 'drawer' ? ElDrawer : UiDialog,
        {
          modelValue: open.value,
          title: '编辑',
          'onUpdate:modelValue': (value: boolean) => {
            open.value = value
          },
        },
        () => '内容',
      )
    },
  }).use(ElementPlus)
  app.mount(host)
  cleanup.push(() => {
    app.unmount()
    host.remove()
  })

  async function show() {
    if (kind === 'message-box')
      void ElMessageBox.confirm('离开会丢失修改', '离开页面').catch(() => {})
    else open.value = true
    await settle()
  }
  async function close() {
    if (kind === 'message-box') ElMessageBox.close()
    else open.value = false
    await settle(250)
  }
  return { show, close }
}

describe.each(overlayThemes)('%s overlay scroll locking', (theme) => {
  it.each(scrollLockOverlays)(
    'does not deduct the reserved gutter again for $kind and restores on close',
    async ({ kind, lockClass }) => {
      document.documentElement.classList.toggle('dark', theme === 'dark')
      const overlay = await mountOverlay(kind)
      const initialComputedWidth = getComputedStyle(document.body).width
      expect(getComputedStyle(document.documentElement).getPropertyValue('scrollbar-gutter')).toBe(
        'stable',
      )
      await overlay.show()
      expect(document.body.classList.contains(lockClass)).toBe(true)
      // 实际组件确实走到经典滚动条补偿分支；应用覆盖使布局仍沿用原有宽度。
      expect(document.body.style.width).toBe('calc(100% - 15px)')
      expect(getComputedStyle(document.body).width).toBe(initialComputedWidth)
      expect(getComputedStyle(document.body).overflow).toBe('hidden')
      await overlay.close()
      expect(document.body.classList.contains(lockClass)).toBe(false)
      expect(document.body.style.width).toBe(originalWidth)
      expect(getComputedStyle(document.body).width).toBe(initialComputedWidth)
    },
  )

  it.each(nestedScrollOverlays)(
    'keeps the parent locked after dismissing nested $kind and restores after both close',
    async ({ kind, lockClass }) => {
      document.documentElement.classList.toggle('dark', theme === 'dark')
      const parent = await mountOverlay('dialog')
      const nested = await mountOverlay(kind)
      const initialComputedWidth = getComputedStyle(document.body).width
      await parent.show()
      await nested.show()
      expect(document.body.classList.contains('el-popup-parent--hidden')).toBe(true)
      expect(document.body.classList.contains(lockClass)).toBe(true)
      expect(getComputedStyle(document.body).width).toBe(initialComputedWidth)
      await nested.close()
      expect(document.body.classList.contains(lockClass)).toBe(false)
      expect(document.body.classList.contains('el-popup-parent--hidden')).toBe(true)
      expect(getComputedStyle(document.body).overflow).toBe('hidden')
      expect(getComputedStyle(document.body).width).toBe(initialComputedWidth)
      await parent.close()
      expect(document.body.classList.contains('el-popup-parent--hidden')).toBe(false)
      expect(document.body.style.width).toBe(originalWidth)
      expect(getComputedStyle(document.body).width).toBe(initialComputedWidth)
    },
  )

  it('keeps non-overflowing pages at their original width without creating scrollbar compensation', async () => {
    document.documentElement.classList.toggle('dark', theme === 'dark')
    vi.spyOn(document.body, 'scrollHeight', 'get').mockReturnValue(classicScrollbar.viewportHeight)
    const overlay = await mountOverlay('message-box')
    const initialComputedWidth = getComputedStyle(document.body).width
    await overlay.show()
    expect(document.body.classList.contains('el-message-box-parent--hidden')).toBe(true)
    expect(document.body.style.width).toBe(originalWidth)
    expect(getComputedStyle(document.body).width).toBe(initialComputedWidth)
    await overlay.close()
    expect(document.body.classList.contains('el-message-box-parent--hidden')).toBe(false)
    expect(document.body.style.width).toBe(originalWidth)
  })
})
