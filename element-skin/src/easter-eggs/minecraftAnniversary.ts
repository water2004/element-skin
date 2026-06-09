import type { EasterEggCleanup } from './index'
import { combineCleanups, injectEasterEggStyle } from './domEffects'
import cjkPixelFontUrl from '@/assets/fonts/minecraft-anniversary/Cubic_11.woff2?url'

const splashes = [
  'Also try Element Skin!',
  'Achievement get!',
  'Now with skins!',
  'Happy birthday, Minecraft!',
  'Punching trees since 2009!',
  'Not affiliated with Mojang!',
]

export function start(): EasterEggCleanup {
  const style = injectEasterEggStyle('minecraft-anniversary', `
    @font-face {
      font-family: "ElementSkin Minecraft Pixel";
      src: url("${cjkPixelFontUrl}") format("woff2");
      font-display: block;
    }

    html.easter-egg-minecraft-anniversary .is-home-layout .hero-content {
      position: relative;
    }

    html.easter-egg-minecraft-anniversary .is-home-layout .hero-title,
    html.easter-egg-minecraft-anniversary .is-home-layout .hero-subtitle,
    html.easter-egg-minecraft-anniversary .is-home-layout .canvas-glass-button {
      image-rendering: pixelated;
      font-family: "ElementSkin Minecraft Pixel", "Courier New", Consolas, monospace;
      text-shadow:
        3px 3px 0 rgba(0, 0, 0, 0.42),
        0 0 14px rgba(255, 255, 255, 0.14);
      letter-spacing: 0.02em;
    }

    html.easter-egg-minecraft-anniversary .is-home-layout .hero-title {
      text-transform: uppercase;
      color: #fffdf0;
      -webkit-font-smoothing: none;
      font-smooth: never;
      -webkit-text-stroke: 1px rgba(255, 255, 255, 0.18);
      text-shadow:
        4px 4px 0 rgba(0, 0, 0, 0.5),
        -2px -2px 0 rgba(255, 255, 255, 0.28),
        0 0 18px rgba(255, 255, 255, 0.2);
    }

    html.easter-egg-minecraft-anniversary .is-home-layout .hero-subtitle {
      color: rgba(255, 255, 245, 0.96);
    }

    .minecraft-splash {
      position: absolute;
      top: -34px;
      left: calc(50% + 150px);
      z-index: 4;
      color: #ffff55;
      font-family: "ElementSkin Minecraft Pixel", "Courier New", Consolas, monospace;
      font-size: clamp(17px, 2vw, 25px);
      font-weight: 400;
      letter-spacing: 0.01em;
      white-space: nowrap;
      pointer-events: none;
      -webkit-font-smoothing: none;
      font-smooth: never;
      text-shadow:
        2px 2px 0 #3f2a00,
        -1px -1px 0 rgba(255, 255, 190, 0.55),
        0 0 10px rgba(255, 238, 70, 0.46);
      transform: rotate(-18deg) scale(1);
      transform-origin: 50% 50%;
      will-change: transform, filter;
      animation: minecraft-splash-pulse 820ms ease-in-out infinite;
    }

    @keyframes minecraft-splash-pulse {
      0%, 100% {
        filter: brightness(1);
        transform: rotate(-18deg) scale(1);
      }
      50% {
        filter: brightness(1.32);
        transform: rotate(-18deg) scale(1.045);
      }
    }

    .minecraft-achievement {
      position: fixed;
      top: 76px;
      right: 28px;
      z-index: 2147483002;
      isolation: isolate;
      display: grid;
      grid-template-columns: 38px minmax(0, 1fr);
      align-items: center;
      gap: 10px;
      width: min(346px, calc(100vw - 32px));
      min-height: 64px;
      padding: 8px 13px 8px 11px;
      color: #fff;
      pointer-events: none;
      background: #8f8f8f;
      font-family: "ElementSkin Minecraft Pixel", "Courier New", Consolas, monospace;
      -webkit-font-smoothing: none;
      font-smooth: never;
      box-shadow:
        0 0 0 2px #2b2b2b,
        0 10px 24px rgba(0, 0, 0, 0.34);
      clip-path: polygon(
        6px 0,
        calc(100% - 6px) 0,
        calc(100% - 6px) 3px,
        100% 3px,
        100% calc(100% - 3px),
        calc(100% - 6px) calc(100% - 3px),
        calc(100% - 6px) 100%,
        6px 100%,
        6px calc(100% - 3px),
        0 calc(100% - 3px),
        0 3px,
        6px 3px
      );
      image-rendering: pixelated;
      will-change: transform, opacity;
      animation:
        minecraft-achievement-in 360ms cubic-bezier(0.16, 1, 0.3, 1),
        minecraft-achievement-out 320ms cubic-bezier(0.7, 0, 0.84, 0) 4.8s forwards;
    }

    .minecraft-achievement::before {
      content: "";
      position: absolute;
      inset: 5px;
      z-index: 0;
      background: #202020;
      box-shadow:
        inset 0 0 0 2px #111,
        inset 0 3px 0 rgba(255, 255, 255, 0.08);
      clip-path: polygon(
        4px 0,
        calc(100% - 4px) 0,
        calc(100% - 4px) 2px,
        100% 2px,
        100% calc(100% - 2px),
        calc(100% - 4px) calc(100% - 2px),
        calc(100% - 4px) 100%,
        4px 100%,
        4px calc(100% - 2px),
        0 calc(100% - 2px),
        0 2px,
        4px 2px
      );
    }

    .minecraft-achievement-icon {
      position: relative;
      z-index: 1;
      width: 32px;
      height: 32px;
      box-shadow:
        inset 0 -12px 0 #7a4b2a,
        inset 0 -17px 0 #8f623a,
        inset 0 0 0 2px rgba(0, 0, 0, 0.28);
      background:
        linear-gradient(90deg, rgba(255, 255, 255, 0.14) 0 5px, transparent 5px 11px),
        linear-gradient(180deg, #66ac3f 0 12px, #4d8d34 12px 16px, #7a4b2a 16px);
      border: 2px solid #130707;
    }

    .minecraft-achievement-icon::before,
    .minecraft-achievement-icon::after {
      content: "";
      position: absolute;
      width: 5px;
      height: 5px;
      background: rgba(255, 255, 255, 0.18);
    }

    .minecraft-achievement-icon::before {
      left: 6px;
      top: 4px;
    }

    .minecraft-achievement-icon::after {
      right: 6px;
      bottom: 9px;
      background: rgba(68, 38, 21, 0.34);
    }

    .minecraft-achievement-title {
      position: relative;
      z-index: 1;
      margin: 0 0 3px;
      color: #ffff55;
      font-size: 15px;
      font-weight: 400;
      line-height: 1.18;
      text-shadow: 2px 2px 0 #000;
    }

    .minecraft-achievement-text {
      position: relative;
      z-index: 1;
      color: #ffffff;
      font-size: 13px;
      font-weight: 400;
      line-height: 1.18;
      text-shadow: 2px 2px 0 #000;
    }

    @keyframes minecraft-achievement-in {
      from {
        opacity: 0;
        transform: translate3d(22px, 0, 0);
      }
      to {
        opacity: 1;
        transform: translate3d(0, 0, 0);
      }
    }

    @keyframes minecraft-achievement-out {
      to {
        opacity: 0;
        transform: translate3d(22px, 0, 0);
      }
    }

    @media (max-width: 768px) {
      .minecraft-splash {
        top: -22px;
        left: 50%;
        transform: translateX(-50%) rotate(-12deg) scale(1);
        font-size: 17px;
      }

      @keyframes minecraft-splash-pulse {
        0%, 100% {
          filter: brightness(1);
          transform: translateX(-50%) rotate(-12deg) scale(1);
        }
        50% {
          filter: brightness(1.32);
          transform: translateX(-50%) rotate(-12deg) scale(1.045);
        }
      }

      .minecraft-achievement {
        top: 70px;
        right: 16px;
      }
    }
  `)

  const home = startHomeEffects()
  return combineCleanups(home, () => style.remove())
}

function startHomeEffects(): EasterEggCleanup {
  let splash: HTMLDivElement | null = null
  let achievement: HTMLDivElement | null = null
  let retryTimer = 0
  let achievementTimer = 0
  let disposed = false

  function cleanupNodes(): void {
    splash?.remove()
    achievement?.remove()
    splash = null
    achievement = null
  }

  function render(): void {
    if (disposed) return
    window.clearTimeout(retryTimer)
    window.clearTimeout(achievementTimer)
    cleanupNodes()
    if (window.location.pathname !== '/') return

    const hero = document.querySelector<HTMLElement>('.hero-content')
    if (!hero) {
      retryTimer = window.setTimeout(render, 120)
      return
    }

    splash = document.createElement('div')
    splash.className = 'minecraft-splash'
    splash.textContent = splashes[Math.floor(Math.random() * splashes.length)] ?? 'Achievement get!'
    hero.appendChild(splash)

    achievement = document.createElement('div')
    achievement.className = 'minecraft-achievement'
    achievement.innerHTML = `
      <span class="minecraft-achievement-icon" aria-hidden="true"></span>
      <span>
        <div class="minecraft-achievement-title">成就达成！</div>
        <div class="minecraft-achievement-text">进入皮肤站</div>
      </span>
    `
    document.body.appendChild(achievement)

    achievementTimer = window.setTimeout(() => {
      achievement?.remove()
      achievement = null
    }, 5200)
  }

  window.setTimeout(render, 0)
  window.addEventListener('popstate', render)

  return () => {
    disposed = true
    window.clearTimeout(retryTimer)
    window.clearTimeout(achievementTimer)
    window.removeEventListener('popstate', render)
    cleanupNodes()
  }
}
