import { createApp, h } from 'vue'
import { ElButton } from 'element-plus'
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest'
import { installMotionStyles, motionCss } from '@/__tests__/helpers/motionStyles'
import { contentEntrances } from './fixtures/motionCases'

let styles: ReturnType<typeof installMotionStyles>
const cleanup: Array<() => void> = []
beforeAll(() => {
  styles = installMotionStyles()
})
afterAll(() => styles.remove())
afterEach(() =>
  cleanup
    .splice(0)
    .reverse()
    .forEach((dispose) => dispose()),
)

describe('shared motion contracts', () => {
  it.each(contentEntrances)(
    'keeps $className visible without delay or displacement under reduced motion',
    ({ className, duration }) => {
      const content = document.createElement('div')
      content.className = className
      content.style.setProperty('--delay-index', '20')
      document.body.append(content)
      cleanup.push(() => content.remove())
      expect(motionCss(content, 'animation')).toContain('contentEnter')
      expect(motionCss(content, 'animation')).toContain(duration)
      expect(motionCss(content, 'animation')).toContain('backwards')
      expect(motionCss(document.documentElement, '--default-transition-duration')).toBe('180ms')
      cleanup.push(styles.reduce())
      expect(motionCss(content, 'animation')).toBe('none')
      expect(motionCss(content, 'opacity')).toBe('1')
      expect(motionCss(content, 'translate')).toBe('none')
      expect(motionCss(content, '--motion-stagger')).toBe('0ms')
      expect(motionCss(content, '--motion-offset-hover')).toBe('0px')
      expect(motionCss(document.documentElement, '--default-transition-duration')).toBe('0ms')
    },
  )

  it('limits ordinary button feedback to paint properties and retains its loading protection', () => {
    const host = document.createElement('div')
    document.body.append(host)
    const app = createApp({ render: () => h(ElButton, { loading: true }, () => '保存') })
    app.mount(host)
    cleanup.push(() => {
      app.unmount()
      host.remove()
    })
    const button = host.querySelector('button')!
    expect(motionCss(button, 'transition')).toContain('background-color 120ms')
    expect(motionCss(button, 'transition')).not.toMatch(/(?:^|,\s*)(all|transform|width|height)\s/)
    cleanup.push(styles.reduce())
    expect(button.disabled).toBe(true)
    expect(button.classList.contains('is-loading')).toBe(true)
    expect(button.querySelectorAll('.el-icon.is-loading')).toHaveLength(1)
    expect(motionCss(button, 'transition')).toContain('background-color 0ms')
    expect(motionCss(button, '--control-height-default')).toBe('32px')
  })

  it('does not reset transforms that position layout elements', () => {
    const positioned = document.createElement('div')
    positioned.style.transform = 'translateX(-50%)'
    document.body.append(positioned)
    cleanup.push(() => positioned.remove())
    cleanup.push(styles.reduce())
    expect(motionCss(positioned, 'transform')).toBe('translateX(-50%)')
  })

  it('does not stack a card entrance inside an entering page', () => {
    const page = document.createElement('div')
    page.className = 'animate-fade-in'
    const card = document.createElement('div')
    card.className = 'animate-card-slide'
    page.append(card)
    document.body.append(page)
    cleanup.push(() => page.remove())
    expect(motionCss(page, 'animation')).toContain('contentEnter')
    expect(motionCss(card, 'animation')).toBe('none')
    expect(motionCss(card, 'opacity')).toBe('1')
  })
})
