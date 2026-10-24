import { createApp, h, nextTick, reactive, ref } from 'vue'
import ElementPlus, { ElMessageBox } from 'element-plus'
import { afterEach, describe, expect, it, vi } from 'vitest'
import UiDialog from '../UiDialog.vue'
import { closeSources } from './fixtures/dialogCases'

const cleanup: Array<() => void> = []
afterEach(() => {
  cleanup
    .splice(0)
    .reverse()
    .forEach((dispose) => dispose())
  ElMessageBox.close()
})

async function settle() {
  await nextTick()
  await new Promise((resolve) => setTimeout(resolve, 60))
  await nextTick()
}

function deferred() {
  let resolve!: (value: 'confirm') => void
  let reject!: (reason: string) => void
  const promise = new Promise<'confirm'>((done, fail) => {
    resolve = done
    reject = fail
  })
  return { promise, resolve, reject }
}

async function mountDialog(beforeClose?: (done: (cancel?: boolean) => void) => void) {
  const state = reactive({
    open: true,
    busy: false,
    dirty: false,
    title: '编辑记录',
    value: '草稿',
  })
  const updates = vi.fn((value: boolean) => {
    state.open = value
  })
  const closed = vi.fn()
  const instance = ref<InstanceType<typeof UiDialog>>()
  const host = document.createElement('div')
  document.body.append(host)
  const app = createApp({
    render: () =>
      h(
        UiDialog,
        {
          ref: instance,
          modelValue: state.open,
          busy: state.busy,
          dirty: state.dirty,
          beforeClose,
          title: state.title,
          class: state.title,
          'data-record': state.title,
          'onUpdate:modelValue': updates,
          onClosed: closed,
        },
        {
          default: () => h('input', { value: state.value, 'aria-label': '草稿' }),
          footer: ({ requestClose }: { requestClose: () => void }) =>
            h('button', { class: 'test-cancel', onClick: requestClose }, '取消'),
        },
      ),
  })
  app.use(ElementPlus)
  app.mount(host)
  const unmount = () => {
    app.unmount()
    host.remove()
  }
  cleanup.push(unmount)
  await settle()
  return { state, updates, closed, instance, unmount }
}

async function dismiss(source: (typeof closeSources)[number]) {
  const dialog = document.body.querySelector<HTMLElement>('.ui-dialog')!
  if (source === 'close-button')
    dialog.querySelector<HTMLButtonElement>('.el-dialog__headerbtn')!.click()
  if (source === 'cancel') dialog.querySelector<HTMLButtonElement>('.test-cancel')!.click()
  if (source === 'mask') {
    const mask = dialog.closest('.el-overlay-dialog')!
    for (const type of ['mousedown', 'mouseup', 'click'])
      mask.dispatchEvent(new MouseEvent(type, { bubbles: true }))
  }
  if (source === 'escape')
    document.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape', code: 'Escape', bubbles: true }),
    )
  await settle()
}

describe('UiDialog closing contract', () => {
  it('coalesces synchronous clean close attempts into one model update', async () => {
    const mounted = await mountDialog()
    void mounted.instance.value!.requestDismiss()
    void mounted.instance.value!.requestDismiss()
    await settle()
    expect(mounted.updates).toHaveBeenCalledExactlyOnceWith(false)
    expect(mounted.closed).toHaveBeenCalledTimes(1)
  })

  it('preserves the native before-close callback for both footer and dismiss actions', async () => {
    let allowed = false
    const beforeClose = vi.fn((done: (cancel?: boolean) => void) => done(!allowed))
    const mounted = await mountDialog(beforeClose)
    await dismiss('cancel')
    expect(mounted.updates).not.toHaveBeenCalled()
    expect(beforeClose).toHaveBeenCalledTimes(1)
    allowed = true
    await dismiss('close-button')
    expect(beforeClose).toHaveBeenCalledTimes(2)
    expect(mounted.updates).toHaveBeenCalledExactlyOnceWith(false)
  })
  it.each(closeSources)('closes a clean dialog once from %s', async (source) => {
    const mounted = await mountDialog()
    await dismiss(source)
    expect(mounted.state.open).toBe(false)
    expect(mounted.updates).toHaveBeenCalledExactlyOnceWith(false)
    expect(mounted.closed).toHaveBeenCalledTimes(1)
  })

  it.each(closeSources)('blocks %s while busy, then permits it', async (source) => {
    const mounted = await mountDialog()
    mounted.state.busy = true
    await settle()
    await dismiss(source)
    expect(mounted.state.open).toBe(true)
    expect(mounted.updates).not.toHaveBeenCalled()
    expect(mounted.closed).not.toHaveBeenCalled()
    mounted.state.busy = false
    await settle()
    await dismiss(source)
    expect(mounted.updates).toHaveBeenCalledExactlyOnceWith(false)
  })

  it('keeps input on cancelled discard and coalesces repeated close attempts', async () => {
    const result = deferred()
    const confirm = vi.spyOn(ElMessageBox, 'confirm').mockReturnValue(result.promise)
    const mounted = await mountDialog()
    mounted.state.dirty = true
    await nextTick()
    void mounted.instance.value!.requestDismiss()
    void mounted.instance.value!.requestDismiss()
    expect(confirm).toHaveBeenCalledExactlyOnceWith('当前修改尚未保存，确定放弃吗？', '放弃修改', {
      type: 'warning',
      confirmButtonText: '放弃修改',
      cancelButtonText: '继续编辑',
      closeOnClickModal: false,
      distinguishCancelAndClose: true,
    })
    result.reject('cancel')
    await settle()
    expect(mounted.state.open).toBe(true)
    expect(document.querySelector<HTMLInputElement>('[aria-label="草稿"]')!.value).toBe('草稿')
    expect(mounted.updates).not.toHaveBeenCalled()
    confirm.mockResolvedValue('confirm')
    await mounted.instance.value!.requestDismiss()
    await settle()
    expect(mounted.updates).toHaveBeenCalledExactlyOnceWith(false)
  })

  it.each(['cancel', 'close-button'] as const)(
    'discards dirty input directly from %s',
    async (source) => {
      const confirm = vi.spyOn(ElMessageBox, 'confirm')
      const mounted = await mountDialog()
      mounted.state.dirty = true
      await nextTick()
      await dismiss(source)
      expect(confirm).not.toHaveBeenCalled()
      expect(mounted.updates).toHaveBeenCalledExactlyOnceWith(false)
      expect(mounted.closed).toHaveBeenCalledTimes(1)
    },
  )

  it.each(['mask', 'escape'] as const)(
    'asks before discarding dirty input from %s',
    async (source) => {
      const confirm = vi.spyOn(ElMessageBox, 'confirm').mockRejectedValue('cancel')
      const mounted = await mountDialog()
      mounted.state.dirty = true
      await nextTick()
      await dismiss(source)
      expect(confirm).toHaveBeenCalledTimes(1)
      expect(mounted.updates).not.toHaveBeenCalled()
      expect(mounted.state.open).toBe(true)
    },
  )

  it('does not close a reopened dialog when an old discard resolves', async () => {
    const result = deferred()
    vi.spyOn(ElMessageBox, 'confirm').mockReturnValue(result.promise)
    const mounted = await mountDialog()
    mounted.state.dirty = true
    await nextTick()
    const closing = mounted.instance.value!.requestDismiss()
    mounted.state.open = false
    await nextTick()
    mounted.state.open = true
    mounted.state.value = '新记录'
    await nextTick()
    result.resolve('confirm')
    await closing
    await settle()
    expect(mounted.state.open).toBe(true)
    expect(mounted.state.value).toBe('新记录')
    expect(mounted.updates).not.toHaveBeenCalled()
    expect(mounted.closed).not.toHaveBeenCalled()
  })

  it('ignores a pending discard after unmount', async () => {
    const result = deferred()
    vi.spyOn(ElMessageBox, 'confirm').mockReturnValue(result.promise)
    const mounted = await mountDialog()
    mounted.state.dirty = true
    await nextTick()
    const closing = mounted.instance.value!.requestDismiss()
    mounted.unmount()
    cleanup.pop()
    result.resolve('confirm')
    await closing
    expect(mounted.updates).not.toHaveBeenCalled()
  })

  it('allows parent success/step changes without discard and forwards updated attributes', async () => {
    const confirm = vi.spyOn(ElMessageBox, 'confirm')
    const mounted = await mountDialog()
    mounted.state.title = '另一条记录'
    mounted.state.busy = true
    mounted.state.dirty = true
    await nextTick()
    expect(document.querySelector('.ui-dialog')?.getAttribute('data-record')).toBe('另一条记录')
    expect(document.querySelector('.ui-dialog')?.classList.contains('另一条记录')).toBe(true)
    expect(document.querySelector('.ui-dialog')?.classList.contains('编辑记录')).toBe(false)
    mounted.state.open = false
    await settle()
    expect(confirm).not.toHaveBeenCalled()
    expect(mounted.closed).toHaveBeenCalledTimes(1)
  })

  it('lets Escape cancel the nested discard without closing its parent', async () => {
    const mounted = await mountDialog()
    mounted.state.dirty = true
    await nextTick()
    void mounted.instance.value!.requestDismiss()
    await settle()
    expect(document.querySelector('.el-message-box__title')?.textContent).toBe('放弃修改')
    document.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape', code: 'Escape', bubbles: true }),
    )
    await settle()
    expect(mounted.state.open).toBe(true)
    expect(mounted.updates).not.toHaveBeenCalled()
    expect(document.activeElement?.closest('.ui-dialog')).not.toBeNull()
  })
})
