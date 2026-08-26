import { createApp, defineComponent, h, nextTick } from 'vue'
import ElementPlus from 'element-plus'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Texture } from '@/api/types'
import DashboardWardrobe from '../DashboardWardrobe.vue'

const textureApiMocks = vi.hoisted(() => ({
  applyTexture: vi.fn(),
  deleteTexture: vi.fn(),
  getTextureDetail: vi.fn(),
  getTextures: vi.fn(),
  patchTexture: vi.fn(),
  uploadTexture: vi.fn(),
}))
const profileApiMocks = vi.hoisted(() => ({
  getProfiles: vi.fn(),
}))

vi.mock('@/api/textures', () => textureApiMocks)
vi.mock('@/api/profiles', () => profileApiMocks)
vi.mock('@/components/textures/textureAssets', () => ({
  cacheSkinTextureWidths: vi.fn(),
  textureAssetUrl: vi.fn((hash: string) => `/static/textures/${hash}.png`),
}))
vi.mock('@/components/common/CursorPager.vue', () => ({
  default: defineComponent({ render: () => h('div') }),
}))
vi.mock('@/components/textures/TextureCard.vue', () => ({
  default: defineComponent({
    props: { texture: { type: Object, required: true } },
    emits: ['preview'],
    render() {
      return h(
        'button',
        {
          'data-testid': 'open-texture',
          onClick: () => this.$emit('preview', this.texture),
        },
        '打开材质',
      )
    },
  }),
}))
vi.mock('@/components/dashboard/wardrobe/TextureDetailDialog.vue', () => ({
  default: defineComponent({
    props: { texture: { type: Object, default: null } },
    emits: ['updatePublic'],
    render() {
      const texture = this.texture as Texture | null
      return h('div', [
        h('span', { 'data-testid': 'visibility' }, String(texture?.is_public ?? 'none')),
        h(
          'button',
          {
            'data-testid': 'make-public',
            onClick: () => this.$emit('updatePublic', 1),
          },
          '公开',
        ),
        h(
          'button',
          {
            'data-testid': 'make-private',
            onClick: () => this.$emit('updatePublic', 0),
          },
          '私有',
        ),
      ])
    },
  }),
}))

const texture: Texture = {
  hash: 'texture-hash',
  type: 'skin',
  model: 'default',
  note: '测试材质',
  is_public: 0,
  created_at: 1000,
}

beforeEach(() => {
  vi.clearAllMocks()
  textureApiMocks.getTextures.mockResolvedValue({
    data: {
      items: [{ ...texture }],
      has_next: false,
      next_cursor: null,
      page_size: 20,
    },
  })
  textureApiMocks.getTextureDetail.mockResolvedValue({ data: { ...texture } })
  textureApiMocks.patchTexture.mockResolvedValue({ data: undefined })
  profileApiMocks.getProfiles.mockResolvedValue({
    data: {
      items: [],
      has_next: false,
      next_cursor: null,
      page_size: 100,
    },
  })
})

describe('DashboardWardrobe texture visibility', () => {
  it('updates the controlled visibility state across consecutive public and private changes', async () => {
    const host = document.createElement('div')
    document.body.appendChild(host)
    const app = createApp({ render: () => h(DashboardWardrobe) })
    app.use(ElementPlus)
    app.mount(host)
    await flushUI()

    button(host, 'open-texture').click()
    await flushUI()
    expect(text(host, 'visibility')).toBe('0')

    button(host, 'make-public').click()
    await flushUI()
    expect(textureApiMocks.patchTexture).toHaveBeenNthCalledWith(1, texture.hash, 'skin', {
      is_public: true,
    })
    expect(text(host, 'visibility')).toBe('1')

    button(host, 'make-private').click()
    await flushUI()
    expect(textureApiMocks.patchTexture).toHaveBeenNthCalledWith(2, texture.hash, 'skin', {
      is_public: false,
    })
    expect(textureApiMocks.patchTexture).toHaveBeenCalledTimes(2)
    expect(text(host, 'visibility')).toBe('0')

    app.unmount()
    host.remove()
  })

  it('discards the pending upload and preview when the dialog closes without submitting', async () => {
    vi.stubGlobal('URL', {
      createObjectURL: vi.fn(() => 'blob:test'),
      revokeObjectURL: vi.fn(),
    })
    const host = document.createElement('div')
    document.body.appendChild(host)
    const app = createApp(DashboardWardrobe)
    app.use(ElementPlus)
    app.mount(host)
    await flushUI()

    const setup = app._instance?.setupState as {
      showUploadDialog: boolean
      uploadForm: { file: File | null; texture_type: string; model: string; note: string; is_public: boolean }
      previewUrl: string | null
      handleFileChange: (file: { raw: File }) => void
    }

    // Select a file and confirm the pending state is populated.
    setup.showUploadDialog = true
    setup.handleFileChange({ raw: new File(['x'], 'a.png', { type: 'image/png' }) })
    await flushUI()
    expect(setup.uploadForm.file).toBeInstanceOf(File)
    expect(setup.previewUrl).toBe('blob:test')

    // Closing the dialog without submitting discards the pending upload.
    setup.showUploadDialog = false
    await flushUI()
    expect(setup.uploadForm.file).toBeNull()
    expect(setup.previewUrl).toBeNull()

    app.unmount()
    host.remove()
    vi.unstubAllGlobals()
  })
})

function button(root: HTMLElement, testId: string): HTMLButtonElement {
  const result = root.querySelector<HTMLButtonElement>(`[data-testid="${testId}"]`)
  expect(result).not.toBeNull()
  return result!
}

function buttonByText(root: HTMLElement, label: string): HTMLButtonElement {
  const result = [...root.querySelectorAll<HTMLButtonElement>('button')].find((item) =>
    item.textContent?.includes(label),
  )
  expect(result).not.toBeNull()
  return result!
}

function text(root: HTMLElement, testId: string): string | null {
  return root.querySelector(`[data-testid="${testId}"]`)?.textContent ?? null
}

async function flushUI() {
  await Promise.resolve()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
}
