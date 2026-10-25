import { createApp, h, nextTick, ref } from 'vue'
import ElementPlus from 'element-plus'
import { afterEach, describe, expect, it, vi } from 'vitest'
import OAuthWebhookEndpointEditor from '../OAuthWebhookEndpointEditor.vue'
import type { WebhookEndpointForm } from '../oauthAppFormState'
import { endpointFixtures, eventFixtures } from './fixtures/webhookEditor'

const cleanups: Array<() => void> = []

afterEach(() => {
  cleanups.splice(0).forEach((cleanup) => cleanup())
})

async function mountEditor(disabled = false, withEvents = true) {
  const endpoints = ref(structuredClone(endpointFixtures))
  const update = vi.fn((index: number, patch: Partial<WebhookEndpointForm>) => {
    endpoints.value[index] = { ...endpoints.value[index]!, ...patch }
  })
  const remove = vi.fn((index: number) => endpoints.value.splice(index, 1))
  const add = vi.fn()
  const root = document.createElement('div')
  document.body.appendChild(root)
  const app = createApp({
    render: () =>
      h(OAuthWebhookEndpointEditor, {
        endpoints: endpoints.value,
        events: withEvents ? eventFixtures : [],
        disabled,
        maxEndpoints: 2,
        onAdd: add,
        onRemove: remove,
        'onUpdate-endpoint': update,
      }),
  })
  app.use(ElementPlus)
  app.mount(root)
  cleanups.push(() => {
    app.unmount()
    root.remove()
  })
  await nextTick()
  return { root, endpoints, update, remove, add }
}

describe('Webhook endpoint editor', () => {
  it('routes URL, event and enabled changes to the selected endpoint without mutating its neighbour', async () => {
    const { root, endpoints, update } = await mountEditor()
    const forms = root.querySelectorAll('form')
    expect(forms).toHaveLength(2)
    expect([...forms].map((form) => form.querySelector('input')!.value)).toEqual(
      endpointFixtures.map((endpoint) => endpoint.url),
    )
    const url = forms[1]!.querySelector('input')!
    url.value = 'https://updated.example/events'
    url.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    const checkbox = forms[1]!.querySelector<HTMLInputElement>('input[type="checkbox"]')!
    checkbox.click()
    await nextTick()
    root.querySelectorAll<HTMLButtonElement>('[role="switch"]')[1]!.click()
    await nextTick()

    expect(update.mock.calls).toEqual([
      [1, { url: 'https://updated.example/events' }],
      [1, { events: ['profile.created'] }],
      [1, { enabled: true }],
    ])
    expect(endpoints.value).toEqual([
      endpointFixtures[0],
      {
        ...endpointFixtures[1],
        url: 'https://updated.example/events',
        events: ['profile.created'],
        enabled: true,
      },
    ])
  })

  it('uses the new index after removal and enforces the endpoint limit', async () => {
    const { root, endpoints, update, remove, add } = await mountEditor()
    const addButton = [...root.querySelectorAll('button')].find((b) =>
      b.textContent?.includes('添加 endpoint'),
    )!
    expect(addButton.disabled).toBe(true)
    addButton.click()
    expect(add).not.toHaveBeenCalled()
    const removeButton = [...root.querySelectorAll('button')].find((b) =>
      b.textContent?.includes('移除'),
    )!
    removeButton.click()
    await nextTick()
    expect(remove.mock.calls).toEqual([[0]])
    expect(endpoints.value).toEqual([endpointFixtures[1]])
    expect(root.textContent).toContain('Endpoint 1')
    expect(root.textContent).not.toContain('Endpoint 2')
    const url = root.querySelector<HTMLInputElement>('form input')!
    url.value = 'https://remaining.example/events'
    url.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    expect(update.mock.calls).toEqual([[0, { url: 'https://remaining.example/events' }]])
    expect(endpoints.value).toEqual([
      { ...endpointFixtures[1], url: 'https://remaining.example/events' },
    ])
    expect(addButton.disabled).toBe(false)
    addButton.click()
    expect(add.mock.calls).toEqual([[]])
  })

  it('disables all editing controls without emitting changes', async () => {
    const { root, endpoints, update, remove, add } = await mountEditor(true)
    const controls = root.querySelectorAll<HTMLInputElement | HTMLButtonElement>('input, button')
    expect(controls).toHaveLength(9)
    for (const control of controls) {
      expect(control.disabled).toBe(true)
      control.click()
    }
    await nextTick()
    expect(update).not.toHaveBeenCalled()
    expect(remove).not.toHaveBeenCalled()
    expect(add).not.toHaveBeenCalled()
    expect(endpoints.value).toEqual(endpointFixtures)
  })

  it('shows the permission guidance instead of event selectors when no events are available', async () => {
    const { root, update } = await mountEditor(false, false)
    expect([...root.querySelectorAll('.el-alert__title')].map((el) => el.textContent)).toEqual([
      '请先申请支持 Webhook 事件的读取权限',
      '请先申请支持 Webhook 事件的读取权限',
    ])
    expect(root.querySelectorAll('form input[type="checkbox"]')).toHaveLength(0)
    expect(update).not.toHaveBeenCalled()
  })
})
