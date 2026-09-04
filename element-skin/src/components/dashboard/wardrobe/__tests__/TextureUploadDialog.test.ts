import { createApp, defineComponent, h, nextTick, reactive } from 'vue'
import ElementPlus from 'element-plus'
import { describe, expect, it, vi } from 'vitest'
import type { TextureUploadForm } from '@/components/dashboard/wardrobe/uploadForm'
import TextureUploadDialog from '../TextureUploadDialog.vue'

vi.mock('@/components/SkinViewer.vue', () => ({
  default: defineComponent({
    props: { skinUrl: { type: String, default: '' }, model: { type: String, default: '' } },
    emits: ['error'],
    render() {
      const self = this as unknown as {
        skinUrl: string
        model: string
        $emit: (event: string, payload: unknown) => void
      }
      return h('div', { 'data-testid': 'viewer-skin' }, [
        h('span', { 'data-testid': 'viewer-skin-url' }, self.skinUrl),
        h('span', { 'data-testid': 'viewer-skin-model' }, self.model),
        h(
          'button',
          {
            'data-testid': 'viewer-skin-fail',
            onClick: () => self.$emit('error', new Error('Bad skin size')),
          },
          'fail',
        ),
      ])
    },
  }),
}))

vi.mock('@/components/CapeViewer.vue', () => ({
  default: defineComponent({
    props: { capeUrl: { type: String, default: '' } },
    emits: ['error'],
    render() {
      const self = this as unknown as {
        capeUrl: string
        $emit: (event: string, payload: unknown) => void
      }
      return h('div', { 'data-testid': 'viewer-cape' }, [
        h('span', { 'data-testid': 'viewer-cape-url' }, self.capeUrl),
        h(
          'button',
          {
            'data-testid': 'viewer-cape-fail',
            onClick: () => self.$emit('error', new Error('Bad cape size')),
          },
          'fail',
        ),
      ])
    },
  }),
}))

const baseForm: TextureUploadForm = {
  texture_type: 'skin',
  model: 'default',
  note: '',
  is_public: false,
  file: null,
}

describe('texture upload dialog preview stage', () => {
  it('shows an empty stage until a file is selected', async () => {
    const mounted = mountDialog({ previewUrl: () => null })
    await flushUI()

    const dialog = document.body.querySelector<HTMLElement>('.ui-dialog--viewer')
    expect(dialog?.textContent).toContain('选择文件后即可在此预览')
    expect(dialog?.querySelector('[data-testid="viewer-skin"]')).toBeNull()
    expect(dialog?.querySelector('[data-testid="viewer-cape"]')).toBeNull()
    mounted.unmount()
  })

  it('renders the selected file as an interactive skin or cape preview', async () => {
    const form = reactive({ ...baseForm })
    const state = reactive({ previewUrl: 'blob:first-preview' })
    const mounted = mountDialog({ form, previewUrl: () => state.previewUrl })
    await flushUI()

    let dialog = document.body.querySelector<HTMLElement>('.ui-dialog--viewer')!
    expect(dialog.querySelector('[data-testid="viewer-skin"]')).not.toBeNull()
    expect(text(dialog, 'viewer-skin-url')).toBe('blob:first-preview')
    expect(text(dialog, 'viewer-skin-model')).toBe('default')
    expect(dialog.querySelector('[data-testid="viewer-cape"]')).toBeNull()

    form.texture_type = 'cape'
    await flushUI()
    dialog = document.body.querySelector<HTMLElement>('.ui-dialog--viewer')!
    expect(dialog.querySelector('[data-testid="viewer-cape"]')).not.toBeNull()
    expect(text(dialog, 'viewer-cape-url')).toBe('blob:first-preview')
    expect(dialog.querySelector('[data-testid="viewer-skin"]')).toBeNull()
    mounted.unmount()
  })

  it('falls back when rendering fails and recovers when the preview url changes', async () => {
    const form = reactive({ ...baseForm })
    const state = reactive({ previewUrl: 'blob:first-preview' })
    const mounted = mountDialog({ form, previewUrl: () => state.previewUrl })
    await flushUI()

    let dialog = document.body.querySelector<HTMLElement>('.ui-dialog--viewer')!
    button(dialog, 'viewer-skin-fail').click()
    await flushUI()
    expect(document.body.textContent).toContain('无法渲染此纹理，仍可尝试上传')
    expect(dialog.querySelector('[data-testid="viewer-skin"]')).toBeNull()

    state.previewUrl = 'blob:second-preview'
    await flushUI()
    dialog = document.body.querySelector<HTMLElement>('.ui-dialog--viewer')!
    expect(dialog.textContent).not.toContain('无法渲染此纹理')
    expect(text(dialog, 'viewer-skin-url')).toBe('blob:second-preview')
    mounted.unmount()
  })

  it('emits submit and closes on cancel', async () => {
    const submitted: number[] = []
    const openState = reactive({ value: true })
    const mounted = mountDialog({
      form: reactive({ ...baseForm }),
      previewUrl: () => 'blob:preview',
      open: () => openState.value,
      onSubmit: () => submitted.push(1),
      'onUpdate:modelValue': (value: boolean) => {
        openState.value = value
      },
    })
    await flushUI()

    const dialog = document.body.querySelector<HTMLElement>('.ui-dialog--viewer')!
    findButton(dialog, '确认上传').click()
    await flushUI()
    expect(submitted).toEqual([1])

    findButton(dialog, '取消').click()
    await flushUI()
    expect(openState.value).toBe(false)
    mounted.unmount()
  })

  it('emits fileRemove when the selected file is removed from the list', async () => {
    const removed: number[] = []
    const mounted = mountDialog({
      form: reactive({ ...baseForm }),
      previewUrl: () => 'blob:preview',
      onFileRemove: () => removed.push(1),
    })
    try {
      await flushUI()
      const dialog = document.body.querySelector<HTMLElement>('.ui-dialog--viewer')!
      const input = dialog.querySelector<HTMLInputElement>('input[type="file"]')
      expect(input).not.toBeNull()
      const selectedFiles: File[] = [new File(['a'], 'a.png', { type: 'image/png' })]
      Object.defineProperty(input!, 'files', { configurable: true, get: () => selectedFiles })
      input!.dispatchEvent(new Event('change'))
      await flushUI()
      expect(dialog.querySelector('.el-upload-list__item')).not.toBeNull()

      dialog.querySelector<HTMLElement>('.el-upload-list__item .el-icon--close')!.click()
      await settle()
      expect(removed).toEqual([1])
      expect(dialog.querySelector('.el-upload-list__item')).toBeNull()
    } finally {
      mounted.unmount()
    }
  })

  it('emits fileExceed and keeps the first file when the limit is reached', async () => {
    const exceeded: number[] = []
    const mounted = mountDialog({
      form: reactive({ ...baseForm }),
      previewUrl: () => 'blob:preview',
      onFileExceed: () => exceeded.push(1),
    })
    try {
      await flushUI()
      const dialog = document.body.querySelector<HTMLElement>('.ui-dialog--viewer')!
      const input = dialog.querySelector<HTMLInputElement>('input[type="file"]')!
      let selectedFiles: File[] = [new File(['a'], 'a.png', { type: 'image/png' })]
      Object.defineProperty(input, 'files', { configurable: true, get: () => selectedFiles })
      input.dispatchEvent(new Event('change'))
      await flushUI()
      expect(dialog.querySelectorAll('.el-upload-list__item')).toHaveLength(1)

      selectedFiles = [new File(['b'], 'b.png', { type: 'image/png' })]
      input.dispatchEvent(new Event('change'))
      await flushUI()

      expect(exceeded).toEqual([1])
      expect(dialog.querySelectorAll('.el-upload-list__item')).toHaveLength(1)
      expect(
        dialog.querySelector('.el-upload-list__item-file-name')?.textContent?.trim(),
      ).toBe('a.png')
    } finally {
      mounted.unmount()
    }
  })
})

interface MountOptions {
  form?: TextureUploadForm
  previewUrl?: () => string | null
  open?: () => boolean
  onSubmit?: () => void
  onFileRemove?: () => void
  onFileExceed?: () => void
  'onUpdate:modelValue'?: (value: boolean) => void
}

function mountDialog(options: MountOptions) {
  const host = document.createElement('div')
  document.body.appendChild(host)

  const app = createApp({
    render: () =>
      h(TextureUploadDialog, {
        key: 'upload-dialog-under-test',
        modelValue: options.open ? options.open() : true,
        form: options.form ?? { ...baseForm },
        previewUrl: options.previewUrl ? options.previewUrl() : null,
        onSubmit: options.onSubmit,
        onFileRemove: options.onFileRemove,
        onFileExceed: options.onFileExceed,
        'onUpdate:modelValue': options['onUpdate:modelValue'],
      }),
  })
  app.use(ElementPlus)
  app.mount(host)
  return {
    host,
    unmount: () => {
      app.unmount()
      host.remove()
    },
  }
}

function button(root: HTMLElement, testId: string): HTMLButtonElement {
  const result = root.querySelector<HTMLButtonElement>(`[data-testid="${testId}"]`)
  if (!result) throw new Error(`button not found: ${testId}`)
  return result
}

function text(root: HTMLElement, testId: string): string {
  return root.querySelector(`[data-testid="${testId}"]`)?.textContent?.trim() || ''
}

function findButton(root: HTMLElement, label: string) {
  const button = [...root.querySelectorAll('button')].find((item) =>
    item.textContent?.includes(label),
  )
  if (!button) throw new Error(`button not found: ${label}`)
  return button
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
