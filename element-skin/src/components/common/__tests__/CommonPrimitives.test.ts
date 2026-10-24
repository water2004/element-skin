import { createApp, h, nextTick, reactive, ref, type VNode } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import UiButton from '@/components/ui/UiButton.vue'
import ActionBar from '../ActionBar.vue'
import CardActions from '../CardActions.vue'
import PageHeader from '../PageHeader.vue'
import SearchBar from '../SearchBar.vue'

const cleanup: Array<() => void> = []
afterEach(() =>
  cleanup
    .splice(0)
    .reverse()
    .forEach((dispose) => dispose()),
)

function mount(render: () => VNode) {
  const root = document.createElement('div')
  document.body.append(root)
  const app = createApp({ render })
  app.mount(root)
  cleanup.push(() => {
    app.unmount()
    root.remove()
  })
  return root
}

describe('page headers and action groups', () => {
  it('renders an accessible heading and action group without an AppLayout ancestor', async () => {
    const state = reactive({
      title: '通知中心',
      subtitle: '查看站内消息',
      level: 1 as 1 | 2 | 3,
      actions: true,
    })
    const root = mount(() =>
      h(
        PageHeader,
        {
          title: state.title,
          subtitle: state.subtitle,
          level: state.level,
          actionsLabel: '通知操作',
        },
        state.actions ? { actions: () => h(UiButton, () => '刷新') } : {},
      ),
    )
    expect(root.querySelector('h1')?.textContent).toBe('通知中心')
    expect(root.querySelector('.ui-page-header-subtitle')?.textContent).toBe('查看站内消息')
    expect(root.querySelector('[role="group"]')?.getAttribute('aria-label')).toBe('通知操作')
    expect(root.querySelectorAll('button')).toHaveLength(1)
    state.level = 2
    state.title = '更长的页面标题'.repeat(12)
    state.subtitle = ''
    state.actions = false
    await nextTick()
    expect(root.querySelector('h1')).toBeNull()
    expect(root.querySelector('h2')?.textContent).toBe(state.title)
    expect(root.querySelector('.ui-page-header-subtitle')).toBeNull()
    expect(root.querySelector('.ui-page-header-actions')).toBeNull()
  })

  it('keeps desktop and mobile alignment independent and updates them together', async () => {
    const state = reactive({
      align: 'end' as 'start' | 'center' | 'end',
      mobileAlign: 'start' as 'start' | 'center' | 'end',
      full: false,
    })
    const root = mount(() =>
      h(ActionBar, { ...state, label: '列表操作' }, () => h(UiButton, () => '导入')),
    )
    const group = root.querySelector('[role="group"]')!
    expect(group.classList.contains('align-end')).toBe(true)
    expect(group.classList.contains('mobile-align-start')).toBe(true)
    expect(group.getAttribute('aria-label')).toBe('列表操作')
    Object.assign(state, { align: 'start', mobileAlign: 'center', full: true })
    await nextTick()
    expect(group.classList.contains('align-end')).toBe(false)
    expect(group.classList.contains('align-start')).toBe(true)
    expect(group.classList.contains('mobile-align-center')).toBe(true)
    expect(group.classList.contains('is-full')).toBe(true)
  })

  it('emits one card action without activating the card around it', () => {
    const preview = vi.fn()
    const remove = vi.fn()
    const root = mount(() =>
      h('div', { onClick: preview }, [
        h(CardActions, { label: '角色操作' }, () =>
          h(UiButton, { tone: 'danger', onClick: () => remove('role-a') }, () => '删除'),
        ),
      ]),
    )
    root.querySelector('button')!.click()
    expect(remove).toHaveBeenCalledExactlyOnceWith('role-a')
    expect(preview).not.toHaveBeenCalled()
    expect(root.querySelector('[role="group"]')?.getAttribute('aria-label')).toBe('角色操作')
  })
})

describe('SearchBar event boundaries', () => {
  function mountSearch() {
    const query = ref('')
    const state = reactive({ loading: false, disabled: false })
    const search = vi.fn()
    const clear = vi.fn()
    const root = mount(() =>
      h(SearchBar, {
        ...state,
        modelValue: query.value,
        'onUpdate:modelValue': (value: string) => {
          query.value = value
        },
        onSearch: search,
        onClear: clear,
      }),
    )
    return {
      root,
      query,
      state,
      search,
      clear,
      input: root.querySelector('input')!,
      button: root.querySelector('button')!,
    }
  }

  it('updates the query without searching and emits exactly once for each explicit search', async () => {
    const mounted = mountSearch()
    mounted.input.value = '新角色'
    mounted.input.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    expect(mounted.query.value).toBe('新角色')
    expect(mounted.input.getAttribute('aria-label')).toBe('搜索内容')
    expect(mounted.search).not.toHaveBeenCalled()
    mounted.input.dispatchEvent(
      new KeyboardEvent('keyup', { key: 'Enter', bubbles: true, isComposing: true }),
    )
    expect(mounted.search).not.toHaveBeenCalled()
    mounted.input.dispatchEvent(new KeyboardEvent('keyup', { key: 'Enter', bubbles: true }))
    expect(mounted.search.mock.calls).toEqual([[]])
    mounted.button.click()
    expect(mounted.search.mock.calls).toEqual([[], []])
    expect(mounted.clear).not.toHaveBeenCalled()
  })

  it.each(['loading', 'disabled'] as const)(
    'blocks click and Enter while %s without losing the query',
    async (stateName) => {
      const mounted = mountSearch()
      mounted.query.value = '保留内容'
      mounted.state[stateName] = true
      await nextTick()
      mounted.button.click()
      mounted.input.dispatchEvent(new KeyboardEvent('keyup', { key: 'Enter', bubbles: true }))
      expect(mounted.search).not.toHaveBeenCalled()
      expect(mounted.button.disabled).toBe(true)
      expect(mounted.input.disabled).toBe(stateName === 'disabled')
      expect(mounted.query.value).toBe('保留内容')
      mounted.state[stateName] = false
      await nextTick()
      mounted.button.click()
      expect(mounted.search).toHaveBeenCalledExactlyOnceWith()
    },
  )

  it('clears the model and emits clear without implicitly searching', async () => {
    const mounted = mountSearch()
    mounted.query.value = '待清空'
    mounted.input.focus()
    await nextTick()
    mounted.root.querySelector<HTMLElement>('.el-input__clear')!.click()
    await nextTick()
    expect(mounted.query.value).toBe('')
    expect(mounted.input.value).toBe('')
    expect(mounted.clear).toHaveBeenCalledExactlyOnceWith()
    expect(mounted.search).not.toHaveBeenCalled()
  })
})
