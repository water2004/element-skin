import type { EasterEggCleanup } from './index'
import { combineCleanups, injectEasterEggStyle, randomBetween, startClickBurst } from './domEffects'

interface FireworkParticle {
  x: number
  y: number
  previousX: number
  previousY: number
  vx: number
  vy: number
  life: number
  maxLife: number
  color: string
}

interface Firework {
  particles: FireworkParticle[]
}

const fireworkColors: [string, ...string[]] = ['#fff1a8', '#ffd166', '#ffb347', '#ff8f70']

export function start(): EasterEggCleanup {
  const style = injectEasterEggStyle(
    'spring-festival',
    `
    html.easter-egg-spring-festival {
      --el-color-primary: #e86b3c;
      --el-color-primary-light-3: #f08a5c;
      --el-color-primary-light-5: #f6aa78;
      --el-color-primary-light-7: #ffd3a0;
      --el-color-primary-light-8: #ffe3bd;
      --el-color-primary-light-9: #fff4df;
      --el-color-primary-dark-2: #c94f2d;
    }

    .spring-festival-fireworks {
      position: fixed;
      inset: 0;
      z-index: 1;
      pointer-events: none;
      opacity: 0.86;
    }

    .spring-festival-burst-layer {
      position: fixed;
      inset: 0;
      z-index: 2147483000;
      pointer-events: none;
      overflow: hidden;
    }

    .spring-festival-spark {
      position: absolute;
      width: 4px;
      height: 4px;
      border-radius: 50%;
      background: #ffe08a;
      box-shadow: 0 0 10px rgba(255, 224, 138, 0.72);
      transform: translate(-50%, -50%);
      animation: spring-festival-pop 560ms ease-out forwards;
      --dx: 0px;
      --dy: 0px;
    }

    @keyframes spring-festival-pop {
      0% {
        opacity: 0.95;
        transform: translate(-50%, -50%) scale(1);
      }
      100% {
        opacity: 0;
        transform: translate(calc(-50% + var(--dx)), calc(-50% + var(--dy))) scale(0.2);
      }
    }

    html.easter-egg-spring-festival .layout-header-wrap {
      border-bottom-color: rgba(255, 197, 87, 0.24);
      box-shadow: 0 1px 8px rgba(232, 107, 60, 0.08);
    }

    html.easter-egg-spring-festival .desktop-nav .el-menu-item.is-active,
    html.easter-egg-spring-festival .desktop-nav .el-sub-menu__title.is-active,
    html.easter-egg-spring-festival .desktop-nav .el-menu-item:hover,
    html.easter-egg-spring-festival .desktop-nav .el-sub-menu__title:hover,
    html.easter-egg-spring-festival .logo:hover {
      color: #e86b3c !important;
    }

    html.easter-egg-spring-festival .desktop-nav .el-menu-item.is-active {
      border-bottom-color: #f4a62a !important;
    }

    html.easter-egg-spring-festival .page-header-content h1,
    html.easter-egg-spring-festival .page-header-text h2,
    html.easter-egg-spring-festival .hero-title {
      background: linear-gradient(135deg, var(--color-heading) 0%, #e86b3c 54%, #f4a62a 100%);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
    }

    html.easter-egg-spring-festival .el-button--primary,
    html.easter-egg-spring-festival .btn-gradient-primary,
    html.easter-egg-spring-festival .is-home-layout .home-fixed-button.home-fixed-primary,
    html.easter-egg-spring-festival .is-home-layout .header-actions .el-button--primary {
      background-image:
        linear-gradient(135deg, rgba(255, 214, 102, 0.2), transparent 42%),
        linear-gradient(135deg, #e86b3c, #f4a62a) !important;
      border-color: rgba(255, 214, 102, 0.42) !important;
      color: #fff !important;
      box-shadow:
        0 8px 22px rgba(232, 107, 60, 0.14),
        inset 0 1px 0 rgba(255, 232, 150, 0.32) !important;
    }

    html.easter-egg-spring-festival .btn-gradient-primary:hover:not(:disabled),
    html.easter-egg-spring-festival .el-button--primary:hover,
    html.easter-egg-spring-festival .is-home-layout .home-fixed-button.home-fixed-primary:hover {
      box-shadow:
        0 8px 24px rgba(232, 107, 60, 0.2),
        0 0 0 1px rgba(255, 214, 102, 0.26) !important;
    }

    html.easter-egg-spring-festival .search-bar-container .el-input-group__append {
      background-image:
        linear-gradient(135deg, rgba(255, 214, 102, 0.2), transparent 42%),
        linear-gradient(135deg, #e86b3c, #f4a62a) !important;
      border-color: rgba(255, 214, 102, 0.42) !important;
      color: #fff !important;
      box-shadow:
        0 8px 22px rgba(232, 107, 60, 0.14),
        inset 0 1px 0 rgba(255, 232, 150, 0.32) !important;
    }

    html.easter-egg-spring-festival .search-bar-container .el-input-group__append:hover {
      box-shadow:
        0 8px 24px rgba(232, 107, 60, 0.2),
        0 0 0 1px rgba(255, 214, 102, 0.26) !important;
    }

    html.easter-egg-spring-festival .search-bar-container .el-input-group__append .el-button,
    html.easter-egg-spring-festival .search-bar-container .el-input-group__append .el-button:hover,
    html.easter-egg-spring-festival .search-bar-container .el-input-group__append .el-button:focus {
      background: transparent !important;
      background-image: none !important;
      border-color: transparent !important;
      box-shadow: none !important;
      color: inherit !important;
    }

    html.easter-egg-spring-festival .is-home-layout .home-fixed-button.home-fixed-secondary,
    html.easter-egg-spring-festival .hero-register-btn {
      border-color: rgba(255, 214, 102, 0.34) !important;
      background: rgba(232, 107, 60, 0.14) !important;
      color: #fff !important;
    }

    html.easter-egg-spring-festival .capsule-radio .el-radio-button.is-active .el-radio-button__inner,
    html.easter-egg-spring-festival .modern-radio .el-radio-button.is-active .el-radio-button__inner,
    html.easter-egg-spring-festival .mobile-drawer .el-menu-item.is-active {
      background-color: rgba(232, 107, 60, 0.1) !important;
      border-color: #e86b3c !important;
      color: #e86b3c !important;
    }

    html.easter-egg-spring-festival .group-title,
    html.easter-egg-spring-festival .code-preview-box {
      border-color: #e86b3c !important;
    }

    html.easter-egg-spring-festival .code-preview-box span,
    html.easter-egg-spring-festival .title-edit-btn:hover,
    html.easter-egg-spring-festival .footer-link-item:hover {
      color: #e86b3c !important;
    }

    html.easter-egg-spring-festival .item-card-preview {
      background-image:
        repeating-linear-gradient(135deg, transparent 0 18px, rgba(232, 107, 60, 0.045) 19px 20px, transparent 21px 38px),
        var(--festival-preview-background, none);
    }
  `,
  )

  const bursts = startClickBurst({
    className: 'spring-festival-burst-layer',
    count: 14,
    create: () => {
      const spark = document.createElement('span')
      spark.className = 'spring-festival-spark'
      const angle = randomBetween(0, Math.PI * 2)
      const distance = randomBetween(20, 62)
      spark.style.setProperty('--dx', `${Math.cos(angle) * distance}px`)
      spark.style.setProperty('--dy', `${Math.sin(angle) * distance}px`)
      return spark
    },
  })

  const fireworks = startFireworks()

  return combineCleanups(fireworks, bursts, () => style.remove())
}

function startFireworks(): EasterEggCleanup {
  const canvas = document.createElement('canvas')
  const context = canvas.getContext('2d')
  if (!context) return () => canvas.remove()
  const ctx = context

  canvas.className = 'spring-festival-fireworks'
  canvas.dataset.easterEgg = 'spring-festival-fireworks'
  document.body.appendChild(canvas)

  let width = 0
  let height = 0
  let dpr = 1
  let raf = 0
  let nextLaunch = 0
  let lastFrame = 0
  const fireworks: Firework[] = []

  function resize(): void {
    dpr = Math.max(1, Math.min(window.devicePixelRatio || 1, 1.5))
    width = window.innerWidth
    height = window.innerHeight
    canvas.width = Math.floor(width * dpr)
    canvas.height = Math.floor(height * dpr)
    canvas.style.width = `${width}px`
    canvas.style.height = `${height}px`
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  }

  function launch(now: number): void {
    const cx = randomBetween(width * 0.15, width * 0.85)
    const cy = randomBetween(height * 0.12, height * 0.38)
    if (fireworks.length >= 4) {
      nextLaunch = now + randomBetween(900, 1500)
      return
    }
    const count = Math.round(randomBetween(24, 34))
    const particles: FireworkParticle[] = []
    for (let i = 0; i < count; i += 1) {
      const angle = (Math.PI * 2 * i) / count + randomBetween(-0.08, 0.08)
      const speed = randomBetween(0.45, 1.18)
      particles.push({
        x: cx,
        y: cy,
        previousX: cx,
        previousY: cy,
        vx: Math.cos(angle) * speed,
        vy: Math.sin(angle) * speed,
        life: 0,
        maxLife: randomBetween(72, 102),
        color:
          fireworkColors[Math.floor(Math.random() * fireworkColors.length)] || fireworkColors[0],
      })
    }
    fireworks.push({ particles })
    nextLaunch = now + randomBetween(900, 1550)
  }

  function draw(now: number): void {
    if (now - lastFrame < 33) {
      raf = requestAnimationFrame(draw)
      return
    }
    lastFrame = now
    ctx.clearRect(0, 0, width, height)
    if (now >= nextLaunch && width > 0 && height > 0) launch(now)

    for (let i = fireworks.length - 1; i >= 0; i -= 1) {
      const firework = fireworks[i]
      if (!firework) continue
      const alive: FireworkParticle[] = []
      for (const particle of firework.particles) {
        particle.life += 1
        particle.previousX = particle.x
        particle.previousY = particle.y
        particle.x += particle.vx
        particle.y += particle.vy
        particle.vy += 0.018

        const alpha = Math.max(0, 1 - particle.life / particle.maxLife)
        if (alpha <= 0) continue
        alive.push(particle)
        ctx.fillStyle = particle.color
        ctx.globalAlpha = alpha * 0.92
        ctx.strokeStyle = particle.color
        ctx.lineWidth = 1.35
        ctx.beginPath()
        ctx.moveTo(particle.previousX, particle.previousY)
        ctx.lineTo(particle.x, particle.y)
        ctx.stroke()
        ctx.beginPath()
        ctx.arc(particle.x, particle.y, 1.65, 0, Math.PI * 2)
        ctx.fill()
      }
      firework.particles = alive
      if (firework.particles.length === 0) fireworks.splice(i, 1)
    }
    ctx.globalAlpha = 1
    raf = requestAnimationFrame(draw)
  }

  resize()
  window.addEventListener('resize', resize)
  raf = requestAnimationFrame(draw)

  return () => {
    cancelAnimationFrame(raf)
    window.removeEventListener('resize', resize)
    canvas.remove()
  }
}
