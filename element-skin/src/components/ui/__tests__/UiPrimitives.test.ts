import { createApp, h, nextTick, reactive, type VNode } from 'vue'
import { Refresh, Setting } from '@element-plus/icons-vue'
import { ElCollapse, ElCollapseItem } from 'element-plus'
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { compileStyle } from '@vue/compiler-sfc'
import UiButton from '../UiButton.vue'
import UiCard from '../UiCard.vue'
import SearchBar from '@/components/common/SearchBar.vue'
import {
  buttonAccents,
  buttonVariants,
  cardPaddings,
  launcherLink,
} from './fixtures/primitiveCases'

const cleanup: Array<() => void> = []
const styles = document.createElement('style')
const projectRoot = process.cwd()
beforeAll(() => {
  const elementCss = readFileSync(
    path.join(projectRoot, 'node_modules/element-plus/dist/index.css'),
    'utf8',
  )
  const appearanceCss = readFileSync(
    path.join(projectRoot, 'src/assets/styles/appearance.css'),
    'utf8',
  )
  const tokensCss = readFileSync(path.join(projectRoot, 'src/assets/styles/tokens.css'), 'utf8')
  const darkCss = readFileSync(
    path.join(projectRoot, 'node_modules/element-plus/theme-chalk/dark/css-vars.css'),
    'utf8',
  )
  const overridesCss = readFileSync(
    path.join(projectRoot, 'src/assets/styles/element-overrides.css'),
    'utf8',
  )
  const buttonSource = readFileSync(
    path.join(projectRoot, 'src/components/ui/UiButton.vue'),
    'utf8',
  )
  const cardSource = readFileSync(path.join(projectRoot, 'src/components/ui/UiCard.vue'), 'utf8')
  const searchSource = readFileSync(
    path.join(projectRoot, 'src/components/common/SearchBar.vue'),
    'utf8',
  )
  const componentCss = [
    { source: buttonSource, component: UiButton, filename: 'UiButton.vue' },
    { source: cardSource, component: UiCard, filename: 'UiCard.vue' },
    { source: searchSource, component: SearchBar, filename: 'SearchBar.vue' },
  ].map(({ source, component, filename }) => {
    const id = (component as { __scopeId?: string }).__scopeId
    if (!id) throw new Error(`Missing scope ID for ${filename}`)
    const result = compileStyle({
      source: source.match(/<style[^>]*>([\s\S]*?)<\/style>/)![1]!,
      id,
      filename,
      scoped: true,
    })
    if (result.errors.length) throw result.errors[0]
    return result.code
  })
  styles.textContent = [
    elementCss,
    darkCss,
    appearanceCss,
    tokensCss,
    overridesCss,
    ...componentCss,
  ].join('\n')
  document.head.append(styles)
})
afterAll(() => styles.remove())
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

function cssValue(element: Element, property: string): string {
  return getComputedStyle(element)
    .getPropertyValue(property)
    .trim()
    .replace(/var\((--[\w-]+)\)/g, (_, name: string) => cssValue(element, name))
    .replace(/,\s*/g, ',')
}

describe('UiButton public behavior', () => {
  it.each(buttonAccents)(
    'uses the $tone accent for opt-in glow without changing button position',
    ({ tone, color }) => {
      const root = mount(() => h(UiButton, { tone, hoverEffect: true }, () => '操作'))
      const button = root.querySelector('button')!
      expect(cssValue(button, '--ui-button-accent')).toBe(color)
      expect(cssValue(button, '--shadow-button-hover')).toBe('0 4px 12px')
      expect(button.classList.contains('ui-button--hover')).toBe(true)
      expect(cssValue(button, 'transform')).toBe('none')
    },
  )

  it('keeps glow opt-in and leaves the disabled/loading guards intact', async () => {
    const state = reactive({ hoverEffect: false, disabled: false, loading: false })
    const click = vi.fn()
    const root = mount(() => h(UiButton, { ...state, onClick: click }, () => '保存'))
    const button = root.querySelector('button')!
    expect(button.classList.contains('ui-button--hover')).toBe(false)
    state.hoverEffect = true
    await nextTick()
    expect(button.classList.contains('ui-button--hover')).toBe(true)
    button.click()
    expect(click).toHaveBeenCalledTimes(1)
    state.loading = true
    await nextTick()
    expect(button.classList.contains('is-loading')).toBe(true)
    button.click()
    expect(click).toHaveBeenCalledTimes(1)
    state.loading = false
    state.disabled = true
    await nextTick()
    expect(button.classList.contains('is-disabled')).toBe(true)
    button.click()
    expect(click).toHaveBeenCalledTimes(1)
  })

  it('replaces only the leading icon while retaining the label and blocking repeat activation', async () => {
    const state = reactive({ loading: false })
    const click = vi.fn()
    const root = mount(() =>
      h(UiButton, { ...state, icon: Refresh, loadingMode: 'icon', onClick: click }, () => '刷新'),
    )
    const button = root.querySelector('button')!
    const content = button.querySelector('.ui-button-content')!
    state.loading = true
    await nextTick()
    expect(button.querySelector('.ui-button-content')).toBe(content)
    expect(content.textContent?.trim()).toBe('刷新')
    expect(cssValue(content, 'opacity')).toBe('1')
    expect(button.querySelectorAll('.el-icon')).toHaveLength(1)
    expect(button.querySelectorAll('.is-loading')).toHaveLength(1)
    expect(button.querySelector('.ui-button-spinner')).toBeNull()
    expect(button.disabled).toBe(true)
    button.click()
    expect(click).not.toHaveBeenCalled()
    state.loading = false
    await nextTick()
    expect(content.querySelector('.is-loading')).toBeNull()
    button.click()
    expect(click).toHaveBeenCalledTimes(1)
  })

  it('keeps a swap icon decorative and preserves the action name and command', async () => {
    const state = reactive({ disabled: false })
    const click = vi.fn()
    const root = mount(() =>
      h(
        UiButton,
        { ...state, swapIcon: Setting, 'aria-label': '设为头像', onClick: click },
        () => '头像',
      ),
    )
    const button = root.querySelector('button')!
    expect(button.getAttribute('aria-label')).toBe('设为头像')
    expect(button.querySelector('.ui-button-content')?.textContent).toBe('头像')
    expect(button.querySelector('.ui-button-swap')?.getAttribute('aria-hidden')).toBe('true')
    button.click()
    expect(click).toHaveBeenCalledTimes(1)
    state.disabled = true
    await nextTick()
    button.click()
    expect(click).toHaveBeenCalledTimes(1)
  })

  it('uses the same overshoot transition when swapping in and returning, respecting disabled state', async () => {
    const state = reactive({ disabled: false })
    const root = mount(() =>
      h(UiButton, { ...state, swapIcon: Setting, 'aria-label': '设为头像' }, () => '头像'),
    )
    const button = root.querySelector('button')!
    const content = button.querySelector('.ui-button-content')!
    const icon = button.querySelector('.ui-button-swap')!
    const swapTransition = 'transform 300ms cubic-bezier(0.34,1.56,0.64,1),opacity 300ms ease'
    expect(cssValue(button, 'overflow')).toBe('hidden')
    expect(cssValue(content, 'transition')).toBe(swapTransition)
    expect(cssValue(icon, 'transition')).toBe(swapTransition)
    expect(cssValue(content, 'transform')).toBe('translateY(0) scale(1)')
    expect(cssValue(icon, 'transform')).toBe('translateY(100%) rotate(-90deg)')
    expect(cssValue(icon, 'opacity')).toBe('0')

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }))
    button.focus()
    button.dispatchEvent(new KeyboardEvent('keyup', { key: 'Tab', bubbles: true }))
    expect(document.activeElement).toBe(button)
    expect(button.matches(':focus-visible')).toBe(true)
    // JSDOM 的焦点变化不使已缓存的 computed style 失效，用无样式 class 触发重算。
    button.classList.add('focus-style-probe')
    expect(cssValue(content, 'transform')).toBe('translateY(-100%) scale(0.8)')
    expect(cssValue(content, 'opacity')).toBe('0')
    expect(cssValue(icon, 'transform')).toBe('translateY(0) rotate(0deg)')
    expect(cssValue(icon, 'opacity')).toBe('1')
    expect(button.getAttribute('aria-label')).toBe('设为头像')

    button.blur()
    button.classList.remove('focus-style-probe')
    expect(cssValue(content, 'transform')).toBe('translateY(0) scale(1)')
    expect(cssValue(content, 'opacity')).toBe('1')
    expect(cssValue(icon, 'transform')).toBe('translateY(100%) rotate(-90deg)')
    expect(cssValue(icon, 'opacity')).toBe('0')
    expect(cssValue(content, 'transition')).toBe(swapTransition)
    expect(cssValue(icon, 'transition')).toBe(swapTransition)

    button.focus()
    button.classList.add('focus-style-probe')
    state.disabled = true
    await nextTick()
    expect(cssValue(content, 'transform')).toBe('translateY(0) scale(1)')
    expect(cssValue(content, 'opacity')).toBe('1')
    expect(cssValue(icon, 'opacity')).toBe('0')
    expect(content.textContent).toBe('头像')
  })

  it.each(buttonVariants)('renders $variant / $tone independently', (testCase) => {
    const root = mount(() =>
      h(UiButton, { variant: testCase.variant, tone: testCase.tone }, () => '操作'),
    )
    const button = root.querySelector('button')!
    expect(button.classList.contains(`el-button--${testCase.type}`)).toBe(true)
    expect(button.classList.contains('is-plain')).toBe(testCase.plain)
    expect(button.classList.contains('is-text')).toBe(testCase.text)
    expect(button.textContent?.trim()).toBe('操作')
    expect(button.type).toBe('button')
  })

  it.each(['small', 'default', 'large'] as const)(
    'renders a %s icon button without an empty label wrapper',
    (size) => {
      const root = mount(() =>
        h(UiButton, { icon: Refresh, iconOnly: true, size, 'aria-label': '刷新' }),
      )
      const button = root.querySelector('button')!
      expect(button.getAttribute('aria-label')).toBe('刷新')
      expect(button.querySelectorAll(':scope > .el-icon')).toHaveLength(1)
      expect(button.querySelector(':scope > span')).toBeNull()
      expect(button.querySelector('.ui-button-content')).toBeNull()
      expect(button.classList.contains('ui-button--icon-only')).toBe(true)
      expect(cssValue(button, 'width')).toBe(
        size === 'small' ? '24px' : size === 'large' ? '40px' : '32px',
      )
    },
  )

  it('keeps normal content mounted while loading and blocks duplicate clicks', async () => {
    const state = reactive({ loading: false, disabled: false })
    const click = vi.fn()
    const root = mount(() => h(UiButton, { ...state, icon: Refresh, onClick: click }, () => '刷新'))
    const button = root.querySelector('button')!
    const content = button.querySelector('.ui-button-content')!
    button.click()
    expect(click).toHaveBeenCalledTimes(1)
    expect(click.mock.calls[0]![0]).toBeInstanceOf(MouseEvent)
    state.loading = true
    await nextTick()
    expect(button.disabled).toBe(true)
    expect(button.getAttribute('aria-busy')).toBe('true')
    expect(button.querySelector('.ui-button-content')).toBe(content)
    expect(content.textContent?.trim()).toBe('刷新')
    expect(cssValue(content, 'opacity')).toBe('0')
    expect(content.getAttribute('aria-hidden')).toBeNull()
    expect(button.querySelectorAll('.ui-button-spinner')).toHaveLength(1)
    button.click()
    button.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    expect(click).toHaveBeenCalledTimes(1)
    state.loading = false
    state.disabled = true
    await nextTick()
    expect(button.querySelector('.ui-button-content')).toBe(content)
    expect(button.querySelector('.ui-button-spinner')).toBeNull()
    button.click()
    expect(click).toHaveBeenCalledTimes(1)
    state.disabled = false
    await nextTick()
    button.click()
    expect(click).toHaveBeenCalledTimes(2)
  })

  it('updates native attributes and preserves the semantic hierarchy after rerender', async () => {
    const attrs = reactive({ class: 'first', title: '初始说明', 'aria-label': '初始操作' })
    const root = mount(() => h(UiButton, attrs, () => '操作'))
    const button = root.querySelector('button')!
    Object.assign(attrs, { class: 'second', title: '更新说明', 'aria-label': '更新操作' })
    await nextTick()
    expect(button.classList.contains('first')).toBe(false)
    expect(button.classList.contains('second')).toBe(true)
    expect(button.classList.contains('ui-button')).toBe(true)
    expect(button.title).toBe('更新说明')
    expect(button.getAttribute('aria-label')).toBe('更新操作')
  })

  it('forwards an explicit native submit type without emitting duplicate form submissions', () => {
    const submit = vi.fn()
    const root = mount(() =>
      h(
        'form',
        {
          onSubmit: (event: SubmitEvent) => {
            event.preventDefault()
            submit()
          },
        },
        [h(UiButton, { 'native-type': 'submit', variant: 'primary' }, () => '保存')],
      ),
    )
    const button = root.querySelector('button')!
    expect(button.type).toBe('submit')
    button.click()
    expect(submit).toHaveBeenCalledExactlyOnceWith()
  })

  it('keeps primary button colors inside an input append group', async () => {
    const state = reactive({ disabled: false })
    const root = mount(() => h(SearchBar, { ...state, modelValue: '查询' }))
    const button = root.querySelector('button')!
    expect(cssValue(button, 'background-color')).toBe('#409eff')
    state.disabled = true
    await nextTick()
    expect(button.disabled).toBe(true)
    expect(cssValue(button, 'background-color')).toBe('#a0cfff')
  })

  it.each(['disabled', 'loading'] as const)(
    'removes link activation while %s and restores it afterwards',
    async (stateName) => {
      const state = reactive({ disabled: false, loading: false })
      const click = vi.fn()
      const root = mount(() =>
        h(UiButton, { ...state, tag: 'a', href: '#target', onClick: click }, () => '查看'),
      )
      const link = root.querySelector('a')!
      expect(link.getAttribute('href')).toBe('#target')
      state[stateName] = true
      await nextTick()
      expect(link.hasAttribute('href')).toBe(false)
      expect(link.getAttribute('aria-disabled')).toBe('true')
      expect(link.tabIndex).toBe(-1)
      link.click()
      expect(click).not.toHaveBeenCalled()
      state[stateName] = false
      await nextTick()
      expect(link.getAttribute('href')).toBe('#target')
      expect(link.hasAttribute('aria-disabled')).toBe(false)
      link.dispatchEvent(new MouseEvent('click', { bubbles: true }))
      expect(click).toHaveBeenCalledTimes(1)
    },
  )

  it('preserves a draggable launcher link and the copy handler preventing protocol navigation', () => {
    const click = vi.fn((event: MouseEvent) => event.preventDefault())
    const root = mount(() =>
      h(
        UiButton,
        {
          tag: 'a',
          href: launcherLink.href,
          draggable: true,
          variant: 'primary',
          size: 'large',
          hoverEffect: true,
          onClick: click,
        },
        () => launcherLink.label,
      ),
    )
    const link = root.querySelector('a')!
    expect(root.querySelector('button')).toBeNull()
    expect(link.getAttribute('href')).toBe(launcherLink.href)
    expect(link.draggable).toBe(true)
    expect(link.textContent?.trim()).toBe(launcherLink.label)
    expect(cssValue(link, 'height')).toBe('40px')
    const event = new MouseEvent('click', { bubbles: true, cancelable: true })
    expect(link.dispatchEvent(event)).toBe(false)
    expect(event.defaultPrevented).toBe(true)
    expect(click).toHaveBeenCalledExactlyOnceWith(event)
  })
})

describe('UiCard public behavior', () => {
  it.each(['light', 'dark'])(
    'keeps collapsed and expanded sections inside the %s card theme',
    async (theme) => {
      document.documentElement.classList.toggle('dark', theme === 'dark')
      cleanup.push(() => document.documentElement.classList.remove('dark'))
      const state = reactive({ expanded: [] as string[] })
      const root = mount(() =>
        h(UiCard, {}, () =>
          h(
            ElCollapse,
            {
              modelValue: state.expanded,
              'onUpdate:modelValue': (value) => {
                state.expanded = value as string[]
              },
            },
            () => h(ElCollapseItem, { name: 'regulatory', title: '监管信息' }, () => '备案信息'),
          ),
        ),
      )
      const header = root.querySelector<HTMLButtonElement>('.el-collapse-item__header')!
      const wrap = root.querySelector('.el-collapse-item__wrap')!
      const content = root.querySelector('.el-collapse-item__content')!
      expect(header.getAttribute('aria-expanded')).toBe('false')
      expect(cssValue(header, 'background-color')).toBe('transparent')
      expect(cssValue(wrap, 'background-color')).toBe('transparent')
      expect(cssValue(header, 'color')).toBe(cssValue(header, '--color-heading'))
      expect(cssValue(content, 'color')).toBe(cssValue(content, '--color-text'))
      expect(cssValue(header, '--el-collapse-border-color')).toBe(
        cssValue(header, '--color-border'),
      )
      header.click()
      await nextTick()
      expect(state.expanded).toEqual(['regulatory'])
      expect(header.getAttribute('aria-expanded')).toBe('true')
      expect(cssValue(header, 'background-color')).toBe('transparent')
      expect(cssValue(wrap, 'background-color')).toBe('transparent')
      header.click()
      await nextTick()
      expect(state.expanded).toEqual([])
      expect(header.getAttribute('aria-expanded')).toBe('false')
    },
  )

  it.each(['never', 'hover', 'always'] as const)(
    'honors %s shadows independently of interactive motion',
    (shadow) => {
      const root = mount(() => h(UiCard, { shadow, hoverable: true }, () => '内容'))
      const card = root.querySelector('.ui-card')!
      expect(card.classList.contains(`is-${shadow}-shadow`)).toBe(true)
      expect(cssValue(card, 'box-shadow')).toBe(
        shadow === 'always' ? '0 4px 12px rgba(0,0,0,0.05)' : 'none',
      )
      expect(card.textContent?.trim()).toBe('内容')
    },
  )

  it.each(cardPaddings)(
    'uses $padding padding for the body rather than the outer card',
    (testCase) => {
      const root = mount(() => h(UiCard, { padding: testCase.padding }, () => '内容'))
      const card = root.querySelector('.ui-card')!
      const body = card.querySelector('.el-card__body')!
      expect(cssValue(body, '--el-card-padding')).toBe(testCase.pixels)
      expect(cssValue(card, 'box-shadow')).toBe('none')
    },
  )

  it('forwards changing attributes and keeps header and body slots separate', async () => {
    const attrs = reactive({ class: 'old-card', title: '旧说明' })
    const root = mount(() => h(UiCard, attrs, { header: () => '标题', default: () => '正文' }))
    const card = root.querySelector('.ui-card')!
    Object.assign(attrs, { class: 'new-card', title: '新说明' })
    await nextTick()
    expect(card.classList.contains('old-card')).toBe(false)
    expect(card.classList.contains('new-card')).toBe(true)
    expect(card.getAttribute('title')).toBe('新说明')
    expect(card.querySelector('.el-card__header')?.textContent).toBe('标题')
    expect(card.querySelector('.el-card__body')?.textContent).toBe('正文')
    expect(card.classList.contains('is-hoverable')).toBe(false)
  })
})
