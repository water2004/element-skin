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
})

interface MountOptions {
  form?: TextureUploadForm
  previewUrl?: () => string | null
  open?: () => boolean
  onSubmit?: () => void
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
