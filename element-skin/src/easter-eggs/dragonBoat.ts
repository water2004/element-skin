import Matter from 'matter-js'
import type { BodyType, CompositeType, EngineType } from 'matter-js'
import type { EasterEggCleanup } from './index'

interface Zongzi {
  body: BodyType
  size: number
}

interface ListenerEntry {
  element: EventTarget
  event: string
  handler: EventListenerOrEventListenerObject
  options?: AddEventListenerOptions | boolean
}

const zongziSvg = `
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 96 96">
  <defs>
    <linearGradient id="leaf" x1="16" y1="20" x2="82" y2="82" gradientUnits="userSpaceOnUse">
      <stop offset="0" stop-color="#8fd16a"/>
      <stop offset="0.55" stop-color="#3f9b55"/>
      <stop offset="1" stop-color="#1f6f47"/>
    </linearGradient>
    <linearGradient id="leafDark" x1="72" y1="20" x2="22" y2="86" gradientUnits="userSpaceOnUse">
      <stop offset="0" stop-color="#58b35c"/>
      <stop offset="1" stop-color="#195c3d"/>
    </linearGradient>
    <filter id="shadow" x="-20%" y="-20%" width="140%" height="140%">
      <feDropShadow dx="0" dy="4" stdDeviation="4" flood-color="#0b2f23" flood-opacity="0.28"/>
    </filter>
  </defs>
  <path filter="url(#shadow)" d="M48 8 88 78H8L48 8Z" fill="url(#leaf)"/>
  <path d="M48 8 88 78 46 61Z" fill="url(#leafDark)" opacity="0.72"/>
  <path d="M48 8 8 78 46 61Z" fill="#6fbd63" opacity="0.62"/>
  <path d="M16 76 80 76" stroke="#d8c37a" stroke-width="5" stroke-linecap="round"/>
  <path d="M28 42 71 72" stroke="#e8d892" stroke-width="4" stroke-linecap="round"/>
  <path d="M69 42 28 72" stroke="#eadb9a" stroke-width="4" stroke-linecap="round"/>
  <path d="M48 14 47 61" stroke="#c4e7a5" stroke-width="2" opacity="0.45"/>
  <path d="M30 32 48 18 66 32" fill="none" stroke="#b9e090" stroke-width="2" opacity="0.38"/>
</svg>`

const zongziDataUrl = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(zongziSvg)}`

export function start(): EasterEggCleanup {
  const canvas = document.createElement('canvas')
  const context = canvas.getContext('2d')
  if (!context) return () => canvas.remove()
  const ctx = context

  canvas.style.position = 'fixed'
  canvas.style.inset = '0'
  canvas.style.width = '100vw'
  canvas.style.height = '100vh'
  canvas.style.pointerEvents = 'none'
  canvas.style.zIndex = '2147483000'
  canvas.dataset.easterEgg = 'dragon-boat-zongzi'
  document.body.appendChild(canvas)

  const image = new Image()
  image.src = zongziDataUrl

  let engine: EngineType | null = null
  let walls: CompositeType | null = null
  let mouseBody: BodyType | null = null
  let raf = 0
  let width = 0
  let height = 0
  let dpr = 1
  let lastTime = 0
  const zongzis: Zongzi[] = []
  const listeners: ListenerEntry[] = []

  function addListener(
    element: EventTarget,
    event: string,
    handler: EventListenerOrEventListenerObject,
    options?: AddEventListenerOptions | boolean,
  ): void {
    element.addEventListener(event, handler, options)
    listeners.push({ element, event, handler, options })
  }

  function resize(): void {
    width = window.innerWidth
    height = window.innerHeight
    dpr = Math.max(1, Math.min(window.devicePixelRatio || 1, 1.5))
    canvas.width = Math.floor(width * dpr)
    canvas.height = Math.floor(height * dpr)
    canvas.style.width = `${width}px`
    canvas.style.height = `${height}px`
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    createWalls()
  }

  function createWalls(): void {
    if (!engine) return
    if (walls) Matter.Composite.remove(engine.world, walls)

    const thickness = 80
    const options = { isStatic: true, restitution: 0.55, friction: 0.2 }
    walls = Matter.Composite.create()
    Matter.Composite.add(walls, [
      Matter.Bodies.rectangle(width / 2, height + thickness / 2, width + thickness * 2, thickness, options),
      Matter.Bodies.rectangle(width / 2, -thickness / 2, width + thickness * 2, thickness, options),
      Matter.Bodies.rectangle(-thickness / 2, height / 2, thickness, height + thickness * 2, options),
      Matter.Bodies.rectangle(width + thickness / 2, height / 2, thickness, height + thickness * 2, options),
    ])
    Matter.Composite.add(engine.world, walls)
  }

  function createZongzis(): void {
    if (!engine) return
    const count = Math.max(8, Math.min(18, Math.floor((width * height) / 90000)))
    for (let i = 0; i < count; i += 1) {
      const size = 42 + Math.random() * 24
      const body = Matter.Bodies.polygon(
        size + Math.random() * Math.max(1, width - size * 2),
        size + Math.random() * Math.max(1, height * 0.42),
        3,
        size * 0.55,
        {
          restitution: 0.42,
          friction: 0.35,
          frictionAir: 0.015,
          density: 0.0012,
        },
      )
      Matter.Body.rotate(body, Math.PI / 6)
      Matter.Body.setAngularVelocity(body, (Math.random() - 0.5) * 0.05)
      zongzis.push({ body, size })
      Matter.Composite.add(engine.world, body)
    }
  }

  function updateMouseBody(event: PointerEvent): void {
    if (!mouseBody) return
    Matter.Body.setPosition(mouseBody, { x: event.clientX, y: event.clientY })
  }

  function handlePointerMove(event: Event): void {
    updateMouseBody(event as PointerEvent)
  }

  function drawZongzi(item: Zongzi): void {
    const { body, size } = item
    ctx.save()
    ctx.translate(body.position.x, body.position.y)
    ctx.rotate(body.angle)
    if (image.complete && image.naturalWidth > 0) {
      ctx.drawImage(image, -size / 2, -size / 2, size, size)
    } else {
      ctx.fillStyle = '#49a35b'
      ctx.beginPath()
      ctx.moveTo(0, -size / 2)
      ctx.lineTo(size / 2, size / 2)
      ctx.lineTo(-size / 2, size / 2)
      ctx.closePath()
      ctx.fill()
    }
    ctx.restore()
  }

  function render(now: number): void {
    if (!engine) return
    const delta = Math.min(now - lastTime || 16, 33)
    lastTime = now
    Matter.Engine.update(engine, delta)
    ctx.clearRect(0, 0, width, height)
    for (const item of zongzis) drawZongzi(item)
    raf = requestAnimationFrame(render)
  }

  function handleMotion(event: Event): void {
    if (!engine) return
    const motion = event as DeviceMotionEvent
    const gravity = motion.accelerationIncludingGravity
    if (!gravity || gravity.x === null || gravity.y === null) return
    engine.gravity.x = Math.max(-1, Math.min(1, gravity.x / 9.8))
    engine.gravity.y = Math.max(-1, Math.min(1, -gravity.y / 9.8))
    engine.gravity.scale = 0.001
  }

  engine = Matter.Engine.create({ gravity: { x: 0, y: 1, scale: 0.001 } })
  mouseBody = Matter.Bodies.circle(-200, -200, 44, {
    isStatic: true,
    restitution: 0.8,
    friction: 0,
    render: { visible: false },
  })
  Matter.Composite.add(engine.world, mouseBody)

  addListener(window, 'resize', resize)
  addListener(window, 'pointermove', handlePointerMove, { passive: true })
  addListener(window, 'pointerleave', () => {
    if (mouseBody) Matter.Body.setPosition(mouseBody, { x: -200, y: -200 })
  })
  addListener(window, 'devicemotion', handleMotion)

  resize()
  createZongzis()
  lastTime = performance.now()
  raf = requestAnimationFrame(render)

  return () => {
    cancelAnimationFrame(raf)
    for (const { element, event, handler, options } of listeners) {
      element.removeEventListener(event, handler, options)
    }
    listeners.length = 0
    if (engine) Matter.Engine.clear(engine)
    zongzis.length = 0
    canvas.remove()
    engine = null
    walls = null
    mouseBody = null
  }
}
