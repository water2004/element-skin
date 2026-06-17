import type { EasterEggCleanup } from './index'

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
