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
vi.mock('@/components/SkinViewer.vue', () => ({
  default: defineComponent({
    props: { skinUrl: { type: String, default: '' }, model: { type: String, default: '' } },
    render() {
      const self = this as unknown as { skinUrl: string; model: string }
      return h('div', { 'data-testid': 'viewer-skin' }, [
        h('span', { 'data-testid': 'viewer-skin-url' }, self.skinUrl),
        h('span', { 'data-testid': 'viewer-skin-model' }, self.model),
      ])
    },
  }),
}))
vi.mock('@/components/CapeViewer.vue', () => ({
  default: defineComponent({
    props: { capeUrl: { type: String, default: '' } },
    render() {
      const self = this as unknown as { capeUrl: string }
      return h('div', { 'data-testid': 'viewer-cape' }, [
        h('span', { 'data-testid': 'viewer-cape-url' }, self.capeUrl),
      ])
    },
  }),
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
    const mounted = mountWardrobe()
    try {
      const setup = mounted.setup

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
    } finally {
      mounted.cleanup()
      vi.unstubAllGlobals()
    }
  })

  it('clears the pending upload through the real remove flow and blocks uploading the removed file', async () => {
    const { revokeObjectURL } = stubBlobUrls()
    const mounted = mountWardrobe()
    try {
      const setup = mounted.setup

      setup.showUploadDialog = true
      await flushUI()

      const input = document.body.querySelector<HTMLInputElement>('input[type="file"]')
      expect(input).not.toBeNull()
      let selectedFiles: File[] = [new File(['a'], 'a.png', { type: 'image/png' })]
      Object.defineProperty(input!, 'files', { configurable: true, get: () => selectedFiles })
      input!.dispatchEvent(new Event('change'))
      await flushUI()

      expect(setup.uploadForm.file).toBe(selectedFiles[0])
      expect(setup.previewUrl).toBe('blob:a.png')
      expect(text(document.body, 'viewer-skin-url')).toBe('blob:a.png')

      const removeButton = document.body.querySelector<HTMLElement>(
        '.el-upload-list__item .el-icon--close',
      )
      expect(removeButton).not.toBeNull()
      removeButton!.click()
      await settle()

      expect(document.body.querySelector('.el-upload-list__item')).toBeNull()
      expect(setup.uploadForm.file).toBeNull()
      expect(setup.previewUrl).toBeNull()
      expect(revokeObjectURL).toHaveBeenCalledTimes(1)
      expect(revokeObjectURL).toHaveBeenCalledWith('blob:a.png')
      expect(document.body.textContent).toContain('选择文件后即可在此预览')

      buttonByText(document.body, '确认上传').click()
      await flushUI()

      expect(textureApiMocks.uploadTexture).not.toHaveBeenCalled()
      expect(document.body.textContent).toContain('请选择文件')
    } finally {
      mounted.cleanup()
      vi.unstubAllGlobals()
    }
  })

  it('re-selects a new file after removal without revoking the previous url twice', async () => {
    const { createObjectURL, revokeObjectURL } = stubBlobUrls()
    const mounted = mountWardrobe()
    try {
      const setup = mounted.setup

      setup.showUploadDialog = true
      await flushUI()

      const input = document.body.querySelector<HTMLInputElement>('input[type="file"]')!
      let selectedFiles: File[] = [new File(['a'], 'a.png', { type: 'image/png' })]
      Object.defineProperty(input, 'files', { configurable: true, get: () => selectedFiles })
      input.dispatchEvent(new Event('change'))
      await flushUI()

      document.body.querySelector<HTMLElement>('.el-upload-list__item .el-icon--close')!.click()
      await settle()
      expect(setup.uploadForm.file).toBeNull()
      expect(revokeObjectURL).toHaveBeenCalledTimes(1)

      selectedFiles = [new File(['b'], 'b.png', { type: 'image/png' })]
      input.dispatchEvent(new Event('change'))
      await flushUI()

      expect(setup.uploadForm.file).toBe(selectedFiles[0])
      expect(createObjectURL).toHaveBeenCalledTimes(2)
      expect(revokeObjectURL).toHaveBeenCalledTimes(1)
      expect(revokeObjectURL).toHaveBeenNthCalledWith(1, 'blob:a.png')
      expect(setup.previewUrl).toBe('blob:b.png')
      expect(text(document.body, 'viewer-skin-url')).toBe('blob:b.png')
    } finally {
      mounted.cleanup()
      vi.unstubAllGlobals()
    }
  })

  it('is idempotent when the removal fires repeatedly without a selection', async () => {
    const { revokeObjectURL } = stubBlobUrls()
    const mounted = mountWardrobe()
    try {
      const setup = mounted.setup
      setup.handleFileChange({ raw: new File(['a'], 'a.png', { type: 'image/png' }) })
      await flushUI()
      expect(setup.previewUrl).toBe('blob:a.png')

      setup.handleFileRemove()
      setup.handleFileRemove()
      await flushUI()

      expect(setup.uploadForm.file).toBeNull()
      expect(setup.previewUrl).toBeNull()
      expect(revokeObjectURL).toHaveBeenCalledTimes(1)
      expect(revokeObjectURL).toHaveBeenCalledWith('blob:a.png')
    } finally {
      mounted.cleanup()
      vi.unstubAllGlobals()
    }
  })
})

interface WardrobeSetupState {
  showUploadDialog: boolean
  uploadForm: { file: File | null; texture_type: string; model: string; note: string; is_public: boolean }
  previewUrl: string | null
  handleFileChange: (file: { raw: File }) => void
  handleFileRemove: () => void
  doUpload: () => Promise<unknown>
}

function stubBlobUrls() {
  const createObjectURL = vi.fn((file: File) => `blob:${file.name}` as `${string}:${string}`)
  const revokeObjectURL = vi.fn()
  vi.stubGlobal('URL', { createObjectURL, revokeObjectURL })
  return { createObjectURL, revokeObjectURL }
}

function mountWardrobe() {
  const host = document.createElement('div')
  document.body.appendChild(host)
  const app = createApp(DashboardWardrobe)
  app.use(ElementPlus)
  app.mount(host)
  return {
    setup: app._instance?.setupState as unknown as WardrobeSetupState,
    cleanup() {
      app.unmount()
      host.remove()
    },
  }
}

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

// transition-group 的离场动画依赖双 requestAnimationFrame，直接断言 DOM 消失前需要等待。
async function settle() {
  await flushUI()
  await new Promise((resolve) => setTimeout(resolve, 60))
  await flushUI()
}
