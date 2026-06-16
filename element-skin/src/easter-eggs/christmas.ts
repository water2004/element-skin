import type { EasterEggCleanup } from './index'

interface Flake {
  x: number
  y: number
  radius: number
  speed: number
  sway: number
  phase: number
}

interface SnowCapBinding {
  element: HTMLElement
  cap: HTMLSpanElement
  spec?: SnowCapSpec
  mode: 'hosted' | 'floating'
  fading?: boolean
}

interface SnowCapSpec {
  xRatio: number
  topOffset: number
  capWidthRatio: number
  maxWidth: number
  height: number
  className: string
  vars: Record<string, string>
}

const snowTargetSelector = [
  '.surface-card',
  '.el-card',
  '.el-table',
  '.el-dialog',
  '.texture-card',
  '.role-card',
  '.home-fixed-button',
].join(',')

const floatingSnowTargetSelector = '.home-fixed-button'

export function start(): EasterEggCleanup {
  const style = document.createElement('style')
  style.dataset.easterEgg = 'christmas-snow-caps'
  style.textContent = `
    .christmas-snow-cap-layer {
      position: fixed;
      inset: 0;
      z-index: 90;
      pointer-events: none;
      overflow: hidden;
    }

    .christmas-snow-cap-layer.is-floating {
      z-index: 2147483001;
    }

    .christmas-snow-cap-layer.is-overlay {
      z-index: 2147483002;
    }

    .christmas-snow-host {
      position: relative !important;
      overflow: visible !important;
    }

    .christmas-snow-cap {
      position: absolute;
      z-index: 3;
      height: var(--snow-height);
      left: var(--snow-left);
      top: var(--snow-top);
      width: var(--snow-width);
      pointer-events: none;
      opacity: var(--snow-opacity);
      background:
        radial-gradient(ellipse at 8% 64%, rgba(255,255,255,0.98) 0 var(--snow-lump-a), transparent calc(var(--snow-lump-a) + 1px)),
        radial-gradient(ellipse at 23% 42%, rgba(255,255,255,0.96) 0 var(--snow-lump-b), transparent calc(var(--snow-lump-b) + 1px)),
        radial-gradient(ellipse at 41% 57%, rgba(255,255,255,0.98) 0 var(--snow-lump-c), transparent calc(var(--snow-lump-c) + 1px)),
        radial-gradient(ellipse at 62% 38%, rgba(248,253,255,0.94) 0 var(--snow-lump-d), transparent calc(var(--snow-lump-d) + 1px)),
        radial-gradient(ellipse at 82% 60%, rgba(255,255,255,0.96) 0 var(--snow-lump-e), transparent calc(var(--snow-lump-e) + 1px)),
        linear-gradient(180deg, rgba(255,255,255,0.98) 0%, rgba(248,253,255,0.96) 48%, rgba(232,247,255,0.82) 100%);
      border-radius: 18px 22px 10px 10px;
      box-shadow:
        0 2px 8px rgba(104, 151, 188, 0.34),
        0 0 0 1px rgba(132, 181, 218, 0.28),
        0 0 10px rgba(255, 255, 255, 0.34),
        inset 0 -1px 0 rgba(111, 164, 202, 0.34);
    }

    .christmas-snow-cap::before,
    .christmas-snow-cap::after {
      content: "";
      position: absolute;
      pointer-events: none;
      background: rgba(255,255,255,0.9);
      box-shadow: 0 1px 3px rgba(170,205,230,0.16);
    }

    .christmas-snow-cap::before {
      left: var(--snow-drip-left);
      top: 58%;
      width: var(--snow-drip-width);
      height: var(--snow-drip-height);
      border-radius: 0 0 999px 999px;
      opacity: var(--snow-drip-opacity);
    }

    .christmas-snow-cap::after {
      right: var(--snow-chip-right);
      top: var(--snow-chip-top);
      width: var(--snow-chip-size);
      height: var(--snow-chip-size);
      border-radius: 50%;
      opacity: var(--snow-chip-opacity);
    }

    html.dark.easter-egg-christmas .christmas-snow-cap {
      box-shadow:
        0 2px 7px rgba(185, 216, 238, 0.2),
        0 0 10px rgba(255, 255, 255, 0.22),
        inset 0 -1px 0 rgba(164, 205, 232, 0.26);
    }

    .christmas-snow-cap.is-small {
      filter: blur(0.1px);
    }

    .christmas-snow-cap.is-fading {
      transition: opacity 0.2s ease;
      opacity: 0 !important;
    }

    html.easter-egg-christmas .el-button,
    html.easter-egg-christmas .el-input__wrapper,
    html.easter-egg-christmas .el-textarea__inner,
    html.easter-egg-christmas .el-select__wrapper,
    html.easter-egg-christmas .account-trigger {
      box-shadow:
        inset 0 1px 0 rgba(255,255,255,0.26),
        0 0 0 1px rgba(210,235,255,0.08);
    }

    html.dark.easter-egg-christmas .el-button,
    html.dark.easter-egg-christmas .el-input__wrapper,
    html.dark.easter-egg-christmas .el-textarea__inner,
    html.dark.easter-egg-christmas .el-select__wrapper,
    html.dark.easter-egg-christmas .account-trigger {
      box-shadow:
        inset 0 1px 0 rgba(255,255,255,0.16),
        0 0 0 1px rgba(210,235,255,0.06);
    }
  `
  document.head.appendChild(style)

  const floatingCapLayer = document.createElement('div')
  floatingCapLayer.className = 'christmas-snow-cap-layer is-floating'
  floatingCapLayer.dataset.easterEgg = 'christmas-floating-snow-caps'
  document.body.appendChild(floatingCapLayer)

  const overlayCapLayer = document.createElement('div')
  overlayCapLayer.className = 'christmas-snow-cap-layer is-overlay'
  overlayCapLayer.dataset.easterEgg = 'christmas-overlay-snow-caps'
  document.body.appendChild(overlayCapLayer)

  const canvas = document.createElement('canvas')
  const context = canvas.getContext('2d')
  if (!context) {
    return () => {
      style.remove()
      floatingCapLayer.remove()
      overlayCapLayer.remove()
      canvas.remove()
    }
  }
  const ctx = context

  canvas.style.position = 'fixed'
  canvas.style.inset = '0'
  canvas.style.width = '100vw'
  canvas.style.height = '100vh'
  canvas.style.pointerEvents = 'none'
  canvas.style.zIndex = '2147483000'
  canvas.dataset.easterEgg = 'christmas'
  document.body.appendChild(canvas)

  let raf = 0
  let width = 0
  let height = 0
  let dpr = 1
  const flakes: Flake[] = []
  const capBindings: SnowCapBinding[] = []
  const capSpecs = new WeakMap<HTMLElement, SnowCapSpec[]>()
  const hostedTargets = new Set<HTMLElement>()
  const fadingCaps = new Set<HTMLSpanElement>()
  let capTimer = 0
  let disposed = false

  function randomBetween(min: number, max: number): number {
    return min + Math.random() * (max - min)
  }

  function shouldSkipTarget(el: Element): boolean {
    return Boolean(el.closest('.christmas-snow-cap')) || el.getAttribute('aria-hidden') === 'true'
  }

  function targetOverlayIsLeaving(el: HTMLElement): boolean {
    const overlay = el.closest<HTMLElement>('.el-overlay')
    const dialog = el.closest<HTMLElement>('.el-dialog')
    const overlayClass = overlay?.className || ''
    const dialogClass = dialog?.className || ''
    return (
      overlayClass.includes('leave') ||
      overlayClass.includes('leaving') ||
      dialogClass.includes('leave') ||
      dialogClass.includes('leaving') ||
      overlay?.style.display === 'none' ||
      dialog?.style.display === 'none'
    )
  }

  function shouldUseFloatingSnow(target: HTMLElement): boolean {
    return Boolean(target.closest('.el-overlay')) || target.matches(floatingSnowTargetSelector)
  }

  function snowCapCount(width: number): number {
    return width > 520 ? 3 : width > 260 ? 2 : 1
  }

  function createSnowCapSpec(rect: DOMRect): SnowCapSpec {
    const capWidthRatio = randomBetween(0.16, 0.36)
    const maxWidth = randomBetween(54, 142)
    const capWidth = Math.min(rect.width * capWidthRatio, maxWidth)
    const maxRelativeLeft = Math.max(10, rect.width - capWidth - 10)
    const relativeLeft = randomBetween(10, maxRelativeLeft)
    const height = randomBetween(6, 12)
    return {
      xRatio: relativeLeft / rect.width,
      topOffset: randomBetween(3, 7),
      capWidthRatio,
      maxWidth,
      height,
      className: 'christmas-snow-cap' + (capWidth < 72 ? ' is-small' : ''),
      vars: {
        '--snow-height': `${height}px`,
        '--snow-opacity': String(randomBetween(0.66, 0.92)),
        '--snow-lump-a': `${randomBetween(4, 8)}px`,
        '--snow-lump-b': `${randomBetween(5, 10)}px`,
        '--snow-lump-c': `${randomBetween(4, 9)}px`,
        '--snow-lump-d': `${randomBetween(5, 10)}px`,
        '--snow-lump-e': `${randomBetween(4, 8)}px`,
        '--snow-drip-left': `${randomBetween(16, 72)}%`,
        '--snow-drip-width': `${randomBetween(4, 10)}px`,
        '--snow-drip-height': `${randomBetween(3, 10)}px`,
        '--snow-drip-opacity': String(randomBetween(0.32, 0.74)),
        '--snow-chip-right': `${randomBetween(8, 34)}%`,
        '--snow-chip-top': `${randomBetween(-2, 5)}px`,
        '--snow-chip-size': `${randomBetween(2, 5)}px`,
        '--snow-chip-opacity': String(randomBetween(0.38, 0.8)),
      },
    }
  }

  function specsForTarget(target: HTMLElement, rect: DOMRect): SnowCapSpec[] {
    const count = snowCapCount(rect.width)
    const existing = capSpecs.get(target) || []
    while (existing.length < count) {
      existing.push(createSnowCapSpec(rect))
    }
    existing.length = count
    capSpecs.set(target, existing)
    return existing
  }

  function currentTargets(): HTMLElement[] {
    return Array.from(document.querySelectorAll<HTMLElement>(snowTargetSelector))
      .filter((el) => !shouldSkipTarget(el))
      .filter((el) => !targetOverlayIsLeaving(el))
      .filter((el) => {
        const rect = el.getBoundingClientRect()
        return (
          rect.width >= 120 && rect.height >= 34 && rect.bottom > 0 && rect.top < window.innerHeight
        )
      })
      .slice(0, 48)
  }

  function renderSnowCaps(): void {
    for (const binding of capBindings) {
      if (binding.mode === 'floating') {
        fadeOverlayCap(binding.cap)
      } else {
        binding.cap.remove()
      }
    }
    capBindings.length = 0

    const targets = currentTargets()
    for (const target of targets) {
      const rect = target.getBoundingClientRect()
      const floating = shouldUseFloatingSnow(target)
      if (!floating) {
        target.classList.add('christmas-snow-host')
        hostedTargets.add(target)
      }
      for (const spec of specsForTarget(target, rect)) {
        const cap = document.createElement('span')
        const capWidth = Math.min(rect.width * spec.capWidthRatio, spec.maxWidth)
        const relativeLeft = Math.min(
          Math.max(10, spec.xRatio * rect.width),
          Math.max(10, rect.width - capWidth - 10),
        )
        cap.className = spec.className
        cap.style.setProperty('--snow-width', `${capWidth}px`)
        for (const [key, value] of Object.entries(spec.vars)) {
          cap.style.setProperty(key, value)
        }
        if (floating) {
          cap.style.setProperty('--snow-left', '0px')
          cap.style.setProperty('--snow-top', '0px')
          const layer = target.closest('.el-overlay') ? overlayCapLayer : floatingCapLayer
          layer.appendChild(cap)
        } else {
          cap.style.setProperty('--snow-left', `${relativeLeft}px`)
          cap.style.setProperty('--snow-top', `${-spec.height + 1}px`)
          target.appendChild(cap)
        }
        capBindings.push({
          element: target,
          cap,
          spec,
          mode: floating ? 'floating' : 'hosted',
        })
      }
    }
    updateSnowCapVisibility()
  }

  function fadeOverlayCap(cap: HTMLSpanElement): void {
    if (disposed) {
      cap.remove()
      return
    }
    if (fadingCaps.has(cap)) return
    fadingCaps.add(cap)
    cap.classList.add('is-fading')
    window.setTimeout(() => {
      fadingCaps.delete(cap)
      cap.remove()
    }, 220)
  }

  function scheduleSnowCaps(): void {
    window.clearTimeout(capTimer)
    capTimer = window.setTimeout(renderSnowCaps, 80)
  }

  function resize(): void {
    dpr = Math.max(1, Math.min(window.devicePixelRatio || 1, 2))
    width = window.innerWidth
    height = window.innerHeight
    canvas.width = Math.floor(width * dpr)
    canvas.height = Math.floor(height * dpr)
    canvas.style.width = `${width}px`
    canvas.style.height = `${height}px`
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  }

  function resetFlake(flake: Flake, initial = false): void {
    flake.x = Math.random() * width
    flake.y = initial ? Math.random() * height : -12
    flake.radius = 1.2 + Math.random() * 2.8
    flake.speed = 0.35 + Math.random() * 0.9
    flake.sway = 0.4 + Math.random() * 1.4
    flake.phase = Math.random() * Math.PI * 2
  }

  function updateSnowCapVisibility(): void {
    for (const binding of capBindings) {
      const rect = binding.element.getBoundingClientRect()
      if (binding.mode === 'floating' && targetOverlayIsLeaving(binding.element)) {
        binding.fading = true
        fadeOverlayCap(binding.cap)
      }
      if (
        rect.width <= 0 ||
        rect.height <= 0 ||
        rect.bottom <= 0 ||
        rect.top >= window.innerHeight
      ) {
        if (binding.mode === 'floating') {
          binding.fading = true
          fadeOverlayCap(binding.cap)
        } else {
          binding.cap.style.display = 'none'
        }
        continue
      }

      binding.cap.style.display = ''
      if (binding.mode === 'floating' && binding.spec) {
        const capWidth = Math.min(rect.width * binding.spec.capWidthRatio, binding.spec.maxWidth)
        const relativeLeft = Math.min(
          Math.max(10, binding.spec.xRatio * rect.width),
          Math.max(10, rect.width - capWidth - 10),
        )
        binding.cap.style.setProperty('--snow-width', `${capWidth}px`)
        binding.cap.style.transform = `translate3d(${rect.left + relativeLeft}px, ${rect.top - binding.spec.height + 1}px, 0)`
      }
    }
  }

  function draw(now: number): void {
    ctx.clearRect(0, 0, width, height)
    ctx.fillStyle = 'rgba(255, 255, 255, 0.82)'
    for (const flake of flakes) {
      flake.y += flake.speed
      const x = flake.x + Math.sin(now / 900 + flake.phase) * flake.sway * 10
      if (flake.y - flake.radius > height) resetFlake(flake)
      ctx.beginPath()
      ctx.arc(x, flake.y, flake.radius, 0, Math.PI * 2)
      ctx.fill()
    }
    updateSnowCapVisibility()
    raf = requestAnimationFrame(draw)
  }

  resize()
  for (let i = 0; i < 80; i += 1) {
    const flake = {} as Flake
    resetFlake(flake, true)
    flakes.push(flake)
  }
  window.addEventListener('resize', resize)
  window.addEventListener('resize', scheduleSnowCaps)
  window.addEventListener('scroll', scheduleSnowCaps, true)
  window.visualViewport?.addEventListener('resize', scheduleSnowCaps)
  window.visualViewport?.addEventListener('scroll', updateSnowCapVisibility)
  renderSnowCaps()
  raf = requestAnimationFrame(draw)

  return () => {
    disposed = true
    cancelAnimationFrame(raf)
    window.clearTimeout(capTimer)
    window.removeEventListener('resize', resize)
    window.removeEventListener('resize', scheduleSnowCaps)
    window.removeEventListener('scroll', scheduleSnowCaps, true)
    window.visualViewport?.removeEventListener('resize', scheduleSnowCaps)
    window.visualViewport?.removeEventListener('scroll', updateSnowCapVisibility)
    for (const binding of capBindings) {
      binding.cap.remove()
    }
    for (const cap of fadingCaps) {
      cap.remove()
    }
    fadingCaps.clear()
    for (const target of hostedTargets) {
      target.classList.remove('christmas-snow-host')
    }
    style.remove()
    floatingCapLayer.remove()
    overlayCapLayer.remove()
    canvas.remove()
  }
}
