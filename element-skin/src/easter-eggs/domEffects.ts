import type { EasterEggCleanup } from './index'

interface FloatingItemOptions {
  className: string
  count: number
  create: (index: number) => HTMLElement
  intervalMs?: number
}

interface ClickBurstOptions {
  className: string
  count: number
  create: (index: number) => HTMLElement
}

export function injectEasterEggStyle(name: string, css: string): HTMLStyleElement {
  const style = document.createElement('style')
  style.dataset.easterEgg = name
  style.textContent = css
  document.head.appendChild(style)
  return style
}

export function randomBetween(min: number, max: number): number {
  return min + Math.random() * (max - min)
}

export function startFloatingItems(options: FloatingItemOptions): EasterEggCleanup {
  const layer = document.createElement('div')
  layer.className = options.className
  layer.dataset.easterEgg = options.className
  document.body.appendChild(layer)

  function appendItem(index: number): void {
    const item = options.create(index)
    layer.appendChild(item)
    item.addEventListener('animationend', () => item.remove(), { once: true })
  }

  for (let i = 0; i < options.count; i += 1) {
    window.setTimeout(() => appendItem(i), randomBetween(0, options.intervalMs || 1000))
  }

  const timer = window.setInterval(() => appendItem(0), options.intervalMs || 1800)

  return () => {
    window.clearInterval(timer)
    layer.remove()
  }
}

export function startClickBurst(options: ClickBurstOptions): EasterEggCleanup {
  const layer = document.createElement('div')
  layer.className = options.className
  layer.dataset.easterEgg = options.className
  document.body.appendChild(layer)

  function onPointerDown(event: PointerEvent): void {
    const target = event.target
    if (!(target instanceof Element)) return
    if (target.closest('input, textarea, select, [contenteditable="true"]')) return

    for (let i = 0; i < options.count; i += 1) {
      const particle = options.create(i)
      particle.style.left = `${event.clientX}px`
      particle.style.top = `${event.clientY}px`
      layer.appendChild(particle)
      particle.addEventListener('animationend', () => particle.remove(), { once: true })
    }
  }

  window.addEventListener('pointerdown', onPointerDown, true)

  return () => {
    window.removeEventListener('pointerdown', onPointerDown, true)
    layer.remove()
  }
}

export function combineCleanups(...cleanups: EasterEggCleanup[]): EasterEggCleanup {
  return () => {
    for (const cleanup of cleanups) cleanup()
  }
}
