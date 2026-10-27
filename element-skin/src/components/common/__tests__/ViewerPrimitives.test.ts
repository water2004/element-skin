import { createApp, h, nextTick, ref } from 'vue'
import ElementPlus, { ElMessage } from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import IdentifierValue from '@/components/common/IdentifierValue.vue'
import ProfileCard from '@/components/common/ProfileCard.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import TextureCard from '@/components/textures/TextureCard.vue'
import TextureModelTag from '@/components/textures/TextureModelTag.vue'
import TextureResolutionTag from '@/components/textures/TextureResolutionTag.vue'
import UiButton from '@/components/ui/UiButton.vue'
import { viewerProfile, viewerTexture } from './fixtures/viewerActions'
import { texturePrecisionCases } from '@/components/textures/__tests__/fixtures/texturePrecision'

const dimensions = vi.hoisted(() => vi.fn())
vi.mock('@/components/textures/textureAssets', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/components/textures/textureAssets')>()),
  cacheTextureDimensions: dimensions,
}))
vi.mock('@/components/SkinViewer.vue', () => ({
  default: { render: () => h('div', 'Skin preview') },
}))
vi.mock('@/components/CapeViewer.vue', () => ({
  default: { render: () => h('div', 'Cape preview') },
}))
const cleanup: Array<() => void> = []
const fullHash = 'b6e5ac15' + '0123456789abcdef'.repeat(3)
const writeText = vi.fn()
beforeEach(() => {
  dimensions.mockResolvedValue({ width: 64, height: 64 })
  writeText.mockReset().mockResolvedValue(undefined)
  const original = Object.getOwnPropertyDescriptor(navigator, 'clipboard')
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
  cleanup.push(() => {
    if (original) Object.defineProperty(navigator, 'clipboard', original)
    else Reflect.deleteProperty(navigator, 'clipboard')
  })
  vi.spyOn(ElMessage, 'success').mockReturnValue({ close: vi.fn() })
  vi.spyOn(ElMessage, 'error').mockReturnValue({ close: vi.fn() })
})
afterEach(() => {
  cleanup
    .splice(0)
    .reverse()
    .forEach((fn) => fn())
  vi.restoreAllMocks()
})
async function flush() {
  await nextTick()
  await new Promise((resolve) => setTimeout(resolve, 30))
  await nextTick()
}
async function mount(render: () => ReturnType<typeof h>) {
  const host = document.createElement('div')
  document.body.append(host)
  const app = createApp({ render }).use(ElementPlus)
  app.mount(host)
  cleanup.push(() => {
    app.unmount()
    host.remove()
  })
  await flush()
  return host
}
function visiblePopup() {
  return [...document.querySelectorAll<HTMLElement>('[role="tooltip"]')].find(
    (el) => el.style.display !== 'none' && el.textContent?.includes(fullHash),
  )!
}

describe('compact identifiers', () => {
  it('opens the first eight characters without copying and copies the exact full value', async () => {
    const host = await mount(() => h(IdentifierValue, { label: 'Hash', value: fullHash }))
    const trigger = host.querySelector('button')!
    expect(trigger.textContent?.trim()).toBe('b6e5ac15...')
    expect(host.textContent).not.toContain(fullHash)
    trigger.click()
    await flush()
    expect(writeText).not.toHaveBeenCalled()
    expect(trigger.getAttribute('aria-expanded')).toBe('true')
    const popup = visiblePopup()
    expect(popup.textContent).toContain('点击复制')
    popup.querySelector('button')!.click()
    await flush()
    expect(writeText).toHaveBeenCalledExactlyOnceWith(fullHash)
    expect(ElMessage.success).toHaveBeenCalledExactlyOnceWith('已复制')
    expect(trigger.getAttribute('aria-expanded')).toBe('true')
    const hint = [...popup.querySelectorAll<HTMLElement>('span')].find(
      (el) => el.textContent?.trim() === '点击复制',
    )!
    hint.click()
    await flush()
    expect(writeText).toHaveBeenNthCalledWith(2, fullHash)
  })

  it('consumes popover Esc and outside click before dismissing the main dialog', async () => {
    const open = ref(true)
    await mount(() =>
      h(
        UiDialog,
        {
          modelValue: open.value,
          'onUpdate:modelValue': (value) => {
            open.value = value
          },
          variant: 'viewer',
        },
        () => h(IdentifierValue, { label: 'UUID', value: fullHash }),
      ),
    )
    const dialog = document.querySelector<HTMLElement>('.ui-dialog')!
    const trigger = dialog.querySelector<HTMLButtonElement>('[aria-expanded]')!
    trigger.click()
    await flush()
    visiblePopup()
      .querySelector('button')!
      .dispatchEvent(
        new KeyboardEvent('keydown', {
          key: 'Escape',
          code: 'Escape',
          bubbles: true,
          cancelable: true,
        }),
      )
    await flush()
    expect(open.value).toBe(true)
    expect(trigger.getAttribute('aria-expanded')).toBe('false')
    expect(document.activeElement).toBe(trigger)
    trigger.click()
    await flush()
    document.querySelector<HTMLElement>('.el-overlay-dialog')!.click()
    await flush()
    expect(open.value).toBe(true)
    expect(trigger.getAttribute('aria-expanded')).toBe('false')
    dialog.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape', code: 'Escape', bubbles: true }),
    )
    await flush()
    expect(open.value).toBe(false)
  })

  it('keeps an absent value plain and reports clipboard failure without a false success', async () => {
    const host = await mount(() =>
      h('div', [
        h(IdentifierValue, { label: '皮肤 Hash', value: null }),
        h(IdentifierValue, { label: 'Hash', value: fullHash }),
      ]),
    )
    expect(host.querySelectorAll('button')).toHaveLength(1)
    expect(host.textContent).toContain('未绑定')
    writeText.mockRejectedValueOnce(new Error('denied'))
    host.querySelector('button')!.click()
    await flush()
    visiblePopup().querySelector('button')!.click()
    await flush()
    expect(ElMessage.success).not.toHaveBeenCalled()
    expect(ElMessage.error).toHaveBeenCalledExactlyOnceWith('复制失败，请选择完整值手动复制')
  })
})

describe('viewer card and model semantics', () => {
  it.each(texturePrecisionCases)(
    'shows the card px badge only at a known nonstandard density: $type $width×$height',
    async ({ type, width, height, badge }) => {
      dimensions.mockResolvedValueOnce({ width, height })
      const host = await mount(() =>
        h(TextureResolutionTag, { hash: 'precision-test', type, nonStandardOnly: true }),
      )
      expect(host.querySelector('.el-tag')?.textContent ?? null).toBe(badge)
      expect(host.textContent).not.toContain('清晰度加载中')
      expect(host.querySelector('.el-tooltip__trigger')).toBeNull()
    },
  )

  it.each([
    { type: 'skin', model: 'slim', label: 'Alex' },
    { type: 'skin', model: 'default', label: 'Steve' },
    { type: 'cape', model: 'slim', label: '披风' },
  ])('uses one Element tag for $label', async ({ type, model, label }) => {
    const host = await mount(() => h(TextureModelTag, { type, model }))
    expect(host.querySelector('.el-tag')?.textContent?.trim()).toBe(label)
    expect(host.textContent).not.toMatch(/slim|default|皮肤/)
    expect(host.querySelector('.el-tag')?.classList.contains('is-round')).toBe(true)
    expect(host.querySelector('.el-tag')?.classList.contains('el-tag--light')).toBe(true)
    expect(
      host
        .querySelector('.el-tag')
        ?.classList.contains(type === 'cape' ? 'el-tag--success' : 'el-tag--primary'),
    ).toBe(true)
  })

  it.each(['skin-hash', 'cape-hash'])('probes the real width for %s', async (hash) => {
    dimensions.mockResolvedValueOnce({ width: 128, height: 64 })
    const host = await mount(() => h(TextureResolutionTag, { hash }))
    expect(dimensions).toHaveBeenLastCalledWith(hash, expect.any(Map))
    expect(host.querySelector('.el-tag')?.textContent).toBe('128px')
    expect(host.querySelector('.el-tag')?.classList.contains('el-tag--dark')).toBe(true)
    expect(host.querySelector('.el-tooltip__trigger')).toBeNull()
  })

  it('does not fabricate a resolution after an image read fails', async () => {
    dimensions.mockResolvedValueOnce(null)
    const host = await mount(() => h(TextureResolutionTag, { hash: 'missing' }))
    expect(host.textContent).toBe('清晰度未知')
    expect(host.querySelector('.el-tag')).toBeNull()
  })

  it('opens cards by keyboard and keeps the library action from opening the viewer', async () => {
    const preview = vi.fn()
    const add = vi.fn()
    const host = await mount(() =>
      h(
        TextureCard,
        {
          texture: viewerTexture,
          title: 'Ocean',
          texturesUrl: (hash) => String(hash),
          onPreview: preview,
        },
        {
          actions: () =>
            h(
              UiButton,
              {
                onClick: (event: MouseEvent) => {
                  event.stopPropagation()
                  add()
                },
              },
              () => '添加到衣柜',
            ),
        },
      ),
    )
    const card = host.querySelector<HTMLElement>('[role="button"]')!
    card.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    expect(preview).toHaveBeenCalledExactlyOnceWith(viewerTexture)
    host.querySelector('button')!.click()
    expect(add).toHaveBeenCalledTimes(1)
    expect(preview).toHaveBeenCalledTimes(1)
    expect(card.querySelectorAll('.el-tag')).toHaveLength(1)
  })

  it('limits personal and admin role cards to their agreed information without commands', async () => {
    const host = await mount(() =>
      h('div', [
        h(ProfileCard, { profile: viewerProfile, texturesUrl: String, delayIndex: 0 }),
        h(ProfileCard, {
          profile: { ...viewerProfile, owner_display_name: 'Owner' },
          texturesUrl: String,
          delayIndex: 1,
          admin: true,
        }),
      ]),
    )
    expect(host.querySelector('.role-card')?.textContent).toBe('Skin previewPlayer')
    expect(host.querySelector('.admin-role-card')?.textContent).toBe('Skin previewPlayerOwner')
    expect(host.querySelectorAll('button')).toHaveLength(0)
    expect(host.querySelectorAll('.action-bar, .card-actions, .el-tag')).toHaveLength(0)
  })
})
