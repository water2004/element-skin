import Matter from 'matter-js'
import type { CompositeType, EngineType } from 'matter-js'
import type { EasterEggCleanup } from './index'

type Tracked = [HTMLElement, number, number]

interface ListenerEntry {
  element: EventTarget
  event: string
  handler: EventListenerOrEventListenerObject
  options?: AddEventListenerOptions | boolean
}

export function start(): EasterEggCleanup {
  let engine: EngineType | null = null
  let bodyMap: Map<number, Tracked> | null = null
  let walls: CompositeType | null = null
  let overlayDiv: HTMLDivElement | null = null
  let listeners: ListenerEntry[] = []
  let bootstrapped = false

  function cleanup(): void {
    if (engine) {
      Matter.Engine.clear(engine)
      engine = null
    }
    if (bodyMap) {
      bodyMap.clear()
      bodyMap = null
    }

    document.querySelectorAll<HTMLElement>('.meow-floating').forEach((el) => {
      el.classList.remove('meow-floating')
      el.style.transform = ''
    })

    if (overlayDiv && overlayDiv.parentNode) {
      overlayDiv.parentNode.removeChild(overlayDiv)
      overlayDiv = null
    }

    listeners.forEach(({ element, event, handler, options }) => {
      element.removeEventListener(event, handler, options)
    })
    listeners = []
    walls = null
    bootstrapped = false
  }

  function addListener(
    element: EventTarget,
    event: string,
    handler: EventListenerOrEventListenerObject,
    options?: AddEventListenerOptions | boolean,
  ): void {
    element.addEventListener(event, handler, options)
    listeners.push({ element, event, handler, options })
  }

  function findNearestBlock(node: Element | null): HTMLElement | null {
    let current = node
    while (current) {
      if (current === document.body) return null
      const style = getComputedStyle(current)
      if (current.tagName !== 'svg' && style.display !== 'inline') {
        return current as HTMLElement
      }
      current = current.parentNode as Element | null
    }
    return null
  }

  function bootstrap(): void {
    if (bootstrapped) return
    bootstrapped = true

    let lastTime = 0
    let lastScrollX = 0
    let lastScrollY = 0
    let lastScreenX = 0
    let lastScreenY = 0
    let startTime = 0

    function createWalls(): void {
      if (!engine) return
      const t = 1e4
      const top = Matter.Bodies.rectangle(window.innerWidth / 2 + window.scrollX, -t / 2 + window.scrollY, window.innerWidth + 2 * t, t, { isStatic: true, restitution: 0.8 })
      const bottom = Matter.Bodies.rectangle(window.innerWidth / 2 + window.scrollX, window.innerHeight + t / 2 + window.scrollY, window.innerWidth + 2 * t, t, { isStatic: true, restitution: 0.8 })
      const left = Matter.Bodies.rectangle(-t / 2 + window.scrollX, window.innerHeight / 2 + window.scrollY, t, window.innerHeight + 2 * t, { isStatic: true, restitution: 0.8 })
      const right = Matter.Bodies.rectangle(window.innerWidth + t / 2 + window.scrollX, window.innerHeight / 2 + window.scrollY, t, window.innerHeight + 2 * t, { isStatic: true, restitution: 0.8 })
      if (walls) Matter.Composite.remove(engine.world, walls)
      walls = Matter.Composite.create()
      Matter.Composite.add(walls, [top, bottom, left, right])
      Matter.Composite.add(engine.world, walls)
    }

    function renderLoop(now: number): void {
      if (!engine || !bodyMap) return
      const delta = Math.min(now - lastTime, 40)
      lastTime = now
      const scrollX = window.scrollX
      const scrollY = window.scrollY

      if (startTime + 100 < now) {
        if (walls) Matter.Composite.translate(walls, { x: scrollX - lastScrollX, y: scrollY - lastScrollY })
        lastScrollX = scrollX
        lastScrollY = scrollY
        const dx = window.screenX - lastScreenX
        const dy = window.screenY - lastScreenY
        for (const body of engine.world.bodies) {
          Matter.Body.translate(body, { x: -dx, y: -dy })
        }
        lastScreenX = window.screenX
        lastScreenY = window.screenY
      }

      Matter.Engine.update(engine, delta)

      for (const body of Matter.Composite.allBodies(engine.world)) {
        const tracked = bodyMap.get(body.id)
        if (!tracked) continue
        const [el, width, height] = tracked
        if (el && el.offsetParent) {
          const rect = (el.offsetParent as HTMLElement).getBoundingClientRect()
          const centerX = rect.left + el.offsetLeft + width / 2
          const centerY = rect.top + el.offsetTop + height / 2
          const x = body.position.x - scrollX
          const y = body.position.y - scrollY
          el.style.transform = `translate(${x - centerX}px, ${y - centerY}px) rotate(${body.angle}rad)`
        } else {
          bodyMap.delete(body.id)
          Matter.Composite.remove(engine.world, body)
        }
      }

      requestAnimationFrame(renderLoop)
    }

    engine = Matter.Engine.create({ gravity: { x: 0, y: -0.1 } })
    bodyMap = new Map<number, Tracked>()
    walls = null
    createWalls()

    addListener(window, 'resize', createWalls)

    overlayDiv = document.createElement('div')
    overlayDiv.style.position = 'fixed'
    overlayDiv.style.left = '0'
    overlayDiv.style.top = '0'
    overlayDiv.style.width = '100vw'
    overlayDiv.style.height = '100vh'
    overlayDiv.style.pointerEvents = 'none'
    document.body.appendChild(overlayDiv)

    lastTime = performance.now()
    lastScrollX = window.scrollX
    lastScrollY = window.scrollY
    lastScreenX = window.screenX
    lastScreenY = window.screenY
    startTime = lastTime
    requestAnimationFrame(renderLoop)

    const clickListener = (e: Event): boolean => {
      if (!engine || !bodyMap) return true
      const target = findNearestBlock(e.target as Element)
      if (!target) return true
      if (target.classList.contains('meow-floating')) return true
      if (target.querySelector('.meow-floating')) return true

      const rect = target.getBoundingClientRect()
      const width = rect.width
      const height = rect.height
      if (height > 600) return true

      e.preventDefault()
      e.stopImmediatePropagation()

      const left = window.scrollX + rect.left
      const top = window.scrollY + rect.top
      const body = Matter.Bodies.rectangle(left + width / 2, top + height / 2, width, height, { restitution: 0.8 })
      const id = body.id
      const vx = 0.6 * (2 * Math.random() - 1)
      const vy = 0.6 * (2 * Math.random() - 1)
      const va = 0.008 * Math.random() - 0.004
      Matter.Body.setVelocity(body, { x: vx, y: vy })
      Matter.Body.setAngularVelocity(body, va)

      target.style.setProperty('--real-width', `${width}px`)
      target.style.setProperty('--real-height', `${height}px`)
      target.classList.add('meow-floating')

      target.querySelectorAll('a[data-toggle=popover]').forEach((anchor) => {
        anchor.addEventListener('mouseover', (ev) => ev.stopImmediatePropagation(), true)
      })
      target.querySelectorAll('.popover').forEach((p) => p.remove())

      bodyMap.set(id, [target, width, height])
      Matter.Composite.add(engine.world, body)
      return false
    }
    addListener(document.body, 'click', clickListener, true)

    const motionListener = (e: Event): void => {
      if (!engine) return
      const motion = e as DeviceMotionEvent

      if (motion.accelerationIncludingGravity) {
        const ax = motion.accelerationIncludingGravity.x
        const ay = motion.accelerationIncludingGravity.y
        if (ax === null || ay === null) return
        const vec = Matter.Vector.create(2e-4 * ax, 2e-4 * -ay)
        const magnitude = Matter.Vector.magnitude(vec)
        const normalised = Matter.Vector.normalise(vec)
        engine.gravity.x = normalised.x
        engine.gravity.y = normalised.y
        engine.gravity.scale = magnitude
      }

      if (motion.rotationRate) {
        const gamma = motion.rotationRate.gamma
        if (gamma === null) return
        const cx = window.innerWidth / 2 + window.scrollX
        const cy = window.innerHeight / 2 + window.scrollY
        for (const body of engine.world.bodies) {
          Matter.Body.rotate(body, 2e-4 * gamma, { x: cx, y: cy })
        }
      }
    }
    addListener(window, 'devicemotion', motionListener)
  }

  if (document.readyState === 'loading') {
    addListener(document, 'DOMContentLoaded', bootstrap)
  } else {
    bootstrap()
  }

  return cleanup
}
