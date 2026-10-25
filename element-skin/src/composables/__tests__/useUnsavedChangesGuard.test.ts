import { createApp, defineComponent, h, nextTick, ref } from 'vue'
import { createMemoryHistory, createRouter, RouterView, isNavigationFailure } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useUnsavedChangesGuard } from '../useUnsavedChangesGuard'
import { leaveConfirmation, reloadConfirmation } from './fixtures/unsavedChangesCases'

const cleanup: Array<() => void> = []
beforeEach(() => {
  vi.spyOn(ElMessage, 'warning').mockImplementation(() => ({ close: vi.fn() }))
})
afterEach(() =>
  cleanup
    .splice(0)
    .reverse()
    .forEach((dispose) => dispose()),
)

function deferred() {
  let resolve!: (value: 'confirm') => void
  let reject!: (reason: string) => void
  const promise = new Promise<'confirm'>((done, fail) => {
    resolve = done
    reject = fail
  })
  return { promise, resolve, reject }
}

async function mountPage() {
  const dirty = ref(false)
  const busy = ref(false)
  let confirmReload!: () => Promise<boolean>
  const editor = defineComponent({
    setup() {
      const guard = useUnsavedChangesGuard({ dirty, busy })
      confirmReload = guard.confirmReload
      return () => h('input', { value: dirty.value ? 'draft' : 'saved' })
    },
  })
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/edit/:id', component: editor },
      { path: '/other', component: { render: () => h('p', 'other') } },
      { path: '/third', component: { render: () => h('p', 'third') } },
    ],
  })
  await router.push('/edit/one')
  const host = document.createElement('div')
  document.body.append(host)
  const app = createApp({ render: () => h(RouterView) }).use(router)
  app.mount(host)
  cleanup.push(() => {
    app.unmount()
    host.remove()
  })
  return { dirty, busy, router, host, reload: () => confirmReload() }
}

function unload() {
  const event = new Event('beforeunload', { cancelable: true })
  window.dispatchEvent(event)
  return event.defaultPrevented
}

describe('unsaved page navigation', () => {
  it('leaves a clean page without a confirmation', async () => {
    const confirm = vi.spyOn(ElMessageBox, 'confirm')
    const page = await mountPage()
    expect(unload()).toBe(false)
    await page.router.push('/other')
    expect(page.router.currentRoute.value.path).toBe('/other')
    expect(confirm).not.toHaveBeenCalled()
  })

  it.each(['cancel', 'close'])('keeps the route and draft after %s', async (reason) => {
    const confirm = vi.spyOn(ElMessageBox, 'confirm').mockRejectedValue(reason)
    const page = await mountPage()
    page.dirty.value = true
    const result = await page.router.push('/other')
    expect(isNavigationFailure(result)).toBe(true)
    expect(page.router.currentRoute.value.path).toBe('/edit/one')
    expect(page.host.querySelector('input')!.value).toBe('draft')
    expect(confirm).toHaveBeenCalledExactlyOnceWith(...leaveConfirmation)
    expect(unload()).toBe(true)
  })

  it('allows confirmed navigation and removes the native listener on unmount', async () => {
    vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm')
    const page = await mountPage()
    page.dirty.value = true
    expect(unload()).toBe(true)
    await page.router.push('/other')
    expect(page.router.currentRoute.value.path).toBe('/other')
    expect(unload()).toBe(false)
  })

  it('shares one pending confirmation for repeated navigation', async () => {
    const pending = deferred()
    const confirm = vi.spyOn(ElMessageBox, 'confirm').mockReturnValue(pending.promise)
    const page = await mountPage()
    page.dirty.value = true
    const first = page.router.push('/other')
    await nextTick()
    await new Promise((done) => setTimeout(done, 0))
    const second = page.router.push('/third')
    await new Promise((done) => setTimeout(done, 0))
    expect(confirm).toHaveBeenCalledTimes(1)
    pending.resolve('confirm')
    await Promise.all([first, second])
    expect(page.router.currentRoute.value.path).toBe('/third')
  })

  it('does not discard fields when a later navigation guard aborts', async () => {
    vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm')
    const page = await mountPage()
    page.dirty.value = true
    page.router.beforeEach(() => false)
    await page.router.push('/other')
    expect(page.router.currentRoute.value.path).toBe('/edit/one')
    expect(page.host.querySelector('input')!.value).toBe('draft')
    expect(page.dirty.value).toBe(true)
  })

  it('protects object changes but permits same-page query and anchor changes', async () => {
    const confirm = vi.spyOn(ElMessageBox, 'confirm').mockRejectedValue('cancel')
    const page = await mountPage()
    page.dirty.value = true
    await page.router.push('/edit/one?tab=details#fields')
    expect(page.router.currentRoute.value.fullPath).toBe('/edit/one?tab=details#fields')
    expect(confirm).not.toHaveBeenCalled()
    await page.router.push('/edit/two')
    expect(page.router.currentRoute.value.params.id).toBe('one')
    expect(confirm).toHaveBeenCalledExactlyOnceWith(...leaveConfirmation)
  })

  it('blocks app navigation and reload while a write is pending, even on a clean page', async () => {
    const confirm = vi.spyOn(ElMessageBox, 'confirm')
    const page = await mountPage()
    page.busy.value = true
    expect(unload()).toBe(true)
    await page.router.push('/other')
    expect(page.router.currentRoute.value.path).toBe('/edit/one')
    expect(await page.reload()).toBe(false)
    expect(ElMessage.warning).toHaveBeenCalledTimes(2)
    expect(ElMessage.warning).toHaveBeenLastCalledWith('操作正在进行，请完成后再离开或重新加载')
    expect(confirm).not.toHaveBeenCalled()
    page.busy.value = false
    expect(unload()).toBe(false)
  })

  it('requires confirmation again after a cancellation and permits only confirmed reload', async () => {
    const confirm = vi
      .spyOn(ElMessageBox, 'confirm')
      .mockRejectedValueOnce('cancel')
      .mockResolvedValueOnce('confirm')
    const page = await mountPage()
    page.dirty.value = true
    expect(await page.reload()).toBe(false)
    expect(await page.reload()).toBe(true)
    expect(confirm).toHaveBeenCalledTimes(2)
    expect(confirm).toHaveBeenLastCalledWith(...reloadConfirmation)
    expect(page.dirty.value).toBe(true)
    expect(page.router.currentRoute.value.path).toBe('/edit/one')
  })

  it('does not leave if a write starts during the confirmation', async () => {
    const pending = deferred()
    vi.spyOn(ElMessageBox, 'confirm').mockReturnValue(pending.promise)
    const page = await mountPage()
    page.dirty.value = true
    const result = page.reload()
    page.busy.value = true
    pending.resolve('confirm')
    expect(await result).toBe(false)
    expect(page.dirty.value).toBe(true)
  })

  it('attaches the browser warning only for dirty or pending writes and cleans up on disposal', async () => {
    const page = await mountPage()
    page.dirty.value = true
    expect(unload()).toBe(true)
    page.dirty.value = false
    expect(unload()).toBe(false)
    page.busy.value = true
    expect(unload()).toBe(true)
    cleanup.pop()!()
    expect(unload()).toBe(false)
    expect(await page.reload()).toBe(false)
  })
})
