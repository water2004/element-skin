import type { EasterEggCleanup } from './index'
import { combineCleanups, injectEasterEggStyle, randomBetween } from './domEffects'

export function start(): EasterEggCleanup {
  const style = injectEasterEggStyle('mid-autumn', `
    html.dark.easter-egg-mid-autumn {
      --mid-autumn-primary: #e5c468;
      --mid-autumn-primary-soft: #fff0b9;
      --mid-autumn-primary-pale: #fff7db;
      --mid-autumn-primary-ink: #241908;
      --mid-autumn-glow: rgba(255, 224, 145, 0.28);
      --mid-autumn-line: rgba(255, 232, 164, 0.42);
      --mid-autumn-line-soft: rgba(255, 232, 164, 0.24);
      --mid-autumn-title: linear-gradient(135deg, #fff9e6 0%, #e7cc7a 62%, #b98c2e 100%);
      --el-color-primary: var(--mid-autumn-primary);
      --el-color-primary-light-3: #edd48b;
      --el-color-primary-light-5: #f4e1aa;
      --el-color-primary-light-7: #f8ecc9;
      --el-color-primary-light-8: #fbf3dc;
      --el-color-primary-light-9: #fff9ed;
      --el-color-primary-dark-2: #c9a64d;
    }

    html.easter-egg-mid-autumn:not(.dark) .theme-toggle {
      position: relative;
      overflow: visible;
      color: #7e5608 !important;
      background-color: rgba(255, 235, 157, 0.42) !important;
      background-image: none !important;
      border: 1px solid rgba(181, 128, 22, 0.5) !important;
      box-shadow:
        0 0 0 1px rgba(181, 128, 22, 0.22),
        0 0 0 5px rgba(245, 205, 97, 0.08),
        0 8px 20px rgba(181, 128, 22, 0.18),
        inset 0 1px 0 rgba(255, 255, 245, 0.56) !important;
      transition:
        color 260ms ease,
        background-color 260ms ease,
        border-color 260ms ease,
        filter 260ms ease,
        transform 260ms ease;
      animation: mid-autumn-theme-float 3.2s ease-in-out infinite;
    }

    html.easter-egg-mid-autumn:not(.dark) .theme-toggle::before {
      content: '';
      position: absolute;
      inset: -11px;
      border-radius: 999px;
      background:
        radial-gradient(circle at 50% 50%, rgba(255, 254, 238, 0.5) 0 15%, transparent 17%),
        radial-gradient(circle, rgba(229, 181, 65, 0.36), rgba(245, 205, 97, 0.14) 42%, transparent 68%);
      opacity: 0.58;
      pointer-events: none;
      z-index: -1;
      transition: opacity 320ms ease, filter 320ms ease;
      animation: mid-autumn-theme-halo 2.6s ease-in-out infinite;
    }

    html.easter-egg-mid-autumn:not(.dark) .theme-toggle::after {
      content: '';
      position: absolute;
      top: 1px;
      right: 0;
      width: 6px;
      height: 6px;
      border-radius: 50%;
      background: #fff4b8;
      box-shadow:
        -20px 11px 0 -1px rgba(226, 170, 42, 0.98),
        0 0 10px rgba(181, 128, 22, 0.62),
        -20px 11px 9px rgba(181, 128, 22, 0.42);
      pointer-events: none;
      opacity: 0.72;
      transition: opacity 320ms ease, filter 320ms ease;
      animation: mid-autumn-theme-stars 2.2s ease-in-out infinite;
    }

    html.easter-egg-mid-autumn:not(.dark) .theme-toggle:hover {
      color: #6a4705 !important;
      background-color: rgba(255, 226, 124, 0.54) !important;
      border-color: rgba(181, 128, 22, 0.62) !important;
      filter: saturate(1.08) brightness(1.02);
    }

    html.easter-egg-mid-autumn:not(.dark) .theme-toggle:hover::before {
      filter: brightness(1.1) saturate(1.1);
      opacity: 0.96;
    }

    html.easter-egg-mid-autumn:not(.dark) .theme-toggle:hover::after {
      filter: brightness(1.12) saturate(1.08);
      opacity: 1;
    }

    @keyframes mid-autumn-theme-float {
      0%, 100% {
        transform: translateY(0);
      }
      50% {
        transform: translateY(-1.5px);
      }
    }

    @keyframes mid-autumn-theme-halo {
      0%, 100% {
        transform: scale(0.9);
      }
      50% {
        transform: scale(1.12);
      }
    }

    @keyframes mid-autumn-theme-stars {
      0%, 100% {
        transform: rotate(0deg) translateY(0);
      }
      40% {
        filter: brightness(1.08);
      }
      60% {
        transform: rotate(18deg) translateY(-1px);
      }
    }

    .mid-autumn-osmanthus-layer {
      position: fixed;
      inset: 0;
      z-index: 2147483000;
      pointer-events: none;
      overflow: hidden;
    }

    .mid-autumn-osmanthus {
      position: absolute;
      width: 13px;
      height: 13px;
      transform: translate(-50%, -50%);
      opacity: 0.96;
      background:
        radial-gradient(ellipse at 50% 12%, rgba(255, 225, 132, 0.98) 0 25%, transparent 27%),
        radial-gradient(ellipse at 88% 50%, rgba(255, 196, 82, 0.95) 0 25%, transparent 27%),
        radial-gradient(ellipse at 50% 88%, rgba(255, 215, 111, 0.96) 0 25%, transparent 27%),
        radial-gradient(ellipse at 12% 50%, rgba(255, 188, 72, 0.92) 0 25%, transparent 27%),
        radial-gradient(circle at 50% 50%, rgba(255, 245, 188, 1) 0 14%, transparent 16%);
      filter: drop-shadow(0 2px 5px rgba(166, 116, 20, 0.2));
      animation: mid-autumn-osmanthus-pop var(--duration) cubic-bezier(0.16, 0.84, 0.35, 1) forwards;
      --dx: 0px;
      --dy: 0px;
      --spin: 0deg;
      --scale: 1;
      --duration: 900ms;
    }

    @keyframes mid-autumn-osmanthus-pop {
      0% {
        opacity: 0;
        transform: translate(-50%, -50%) scale(0.35) rotate(0deg);
      }
      14% {
        opacity: 0.96;
      }
      100% {
        opacity: 0;
        transform: translate(calc(-50% + var(--dx)), calc(-50% + var(--dy))) scale(var(--scale)) rotate(var(--spin));
      }
    }

    html.dark.easter-egg-mid-autumn .layout-header-wrap {
      border-bottom-color: var(--mid-autumn-line-soft);
      box-shadow: 0 1px 10px var(--mid-autumn-glow);
    }

    html.dark.easter-egg-mid-autumn .is-home-layout .layout-header-wrap {
      border-bottom-color: transparent !important;
      box-shadow: none !important;
    }

    html.dark.easter-egg-mid-autumn .desktop-nav .el-menu-item.is-active,
    html.dark.easter-egg-mid-autumn .desktop-nav .el-sub-menu__title.is-active,
    html.dark.easter-egg-mid-autumn .desktop-nav .el-menu-item:hover,
    html.dark.easter-egg-mid-autumn .desktop-nav .el-sub-menu__title:hover,
    html.dark.easter-egg-mid-autumn .logo:hover,
    html.dark.easter-egg-mid-autumn .title-edit-btn:hover,
    html.dark.easter-egg-mid-autumn .footer-link-item:hover {
      color: var(--mid-autumn-primary) !important;
    }

    html.dark.easter-egg-mid-autumn .desktop-nav .el-menu-item.is-active {
      border-bottom-color: var(--mid-autumn-primary-soft) !important;
    }

    html.dark.easter-egg-mid-autumn .page-header-content h1,
    html.dark.easter-egg-mid-autumn .page-header-text h2,
    html.dark.easter-egg-mid-autumn .hero-title {
      background: var(--mid-autumn-title);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
    }

    html.dark.easter-egg-mid-autumn .el-button--primary,
    html.dark.easter-egg-mid-autumn .btn-gradient-primary,
    html.dark.easter-egg-mid-autumn .is-home-layout .header-actions .el-button--primary {
      background-image:
        linear-gradient(135deg, rgba(255, 250, 226, 0.34), transparent 44%),
        linear-gradient(135deg, var(--mid-autumn-primary-pale), var(--mid-autumn-primary)) !important;
      border-color: var(--mid-autumn-line) !important;
      color: var(--mid-autumn-primary-ink) !important;
      box-shadow:
        0 8px 22px var(--mid-autumn-glow),
        inset 0 1px 0 rgba(255, 255, 238, 0.52) !important;
    }

    html.dark.easter-egg-mid-autumn .el-button--primary:hover,
    html.dark.easter-egg-mid-autumn .btn-gradient-primary:hover:not(:disabled) {
      border-color: rgba(255, 239, 183, 0.78) !important;
      box-shadow:
        0 10px 26px var(--mid-autumn-glow),
        0 0 0 1px rgba(255, 232, 164, 0.34) !important;
    }

    html.dark.easter-egg-mid-autumn .search-bar-container .el-input-group__append {
      background-image:
        linear-gradient(135deg, rgba(255, 250, 226, 0.34), transparent 44%),
        linear-gradient(135deg, var(--mid-autumn-primary-pale), var(--mid-autumn-primary)) !important;
      border-color: var(--mid-autumn-line) !important;
      color: var(--mid-autumn-primary-ink) !important;
      box-shadow:
        0 8px 22px var(--mid-autumn-glow),
        inset 0 1px 0 rgba(255, 255, 238, 0.52) !important;
    }

    html.dark.easter-egg-mid-autumn .search-bar-container .el-input-group__append:hover {
      border-color: rgba(255, 239, 183, 0.78) !important;
      box-shadow:
        0 10px 26px var(--mid-autumn-glow),
        0 0 0 1px rgba(255, 232, 164, 0.34) !important;
    }

    html.dark.easter-egg-mid-autumn .search-bar-container .el-input-group__append .el-button,
    html.dark.easter-egg-mid-autumn .search-bar-container .el-input-group__append .el-button:hover,
    html.dark.easter-egg-mid-autumn .search-bar-container .el-input-group__append .el-button:focus {
      background: transparent !important;
      background-image: none !important;
      border-color: transparent !important;
      box-shadow: none !important;
      color: inherit !important;
    }

    html.dark.easter-egg-mid-autumn .search-bar-container .el-input__wrapper,
    html.dark.easter-egg-mid-autumn .sort-select .el-select__wrapper,
    html.dark.easter-egg-mid-autumn .capsule-radio .el-radio-button__inner,
    html.dark.easter-egg-mid-autumn .modern-radio .el-radio-button__inner {
      border-color: var(--mid-autumn-line-soft) !important;
      box-shadow: 0 0 0 1px var(--mid-autumn-line-soft) inset !important;
    }

    html.dark.easter-egg-mid-autumn .search-bar-container .el-input__wrapper:hover,
    html.dark.easter-egg-mid-autumn .search-bar-container .el-input__wrapper.is-focus,
    html.dark.easter-egg-mid-autumn .sort-select .el-select__wrapper:hover,
    html.dark.easter-egg-mid-autumn .sort-select .el-select__wrapper.is-focused,
    html.dark.easter-egg-mid-autumn .capsule-radio .el-radio-button__inner:hover,
    html.dark.easter-egg-mid-autumn .modern-radio .el-radio-button__inner:hover {
      border-color: var(--mid-autumn-line) !important;
      box-shadow: 0 0 0 1px var(--mid-autumn-line) inset !important;
    }

    html.dark.easter-egg-mid-autumn .capsule-radio .el-radio-button:first-child .el-radio-button__inner,
    html.dark.easter-egg-mid-autumn .modern-radio .el-radio-button:first-child .el-radio-button__inner {
      border-left-color: var(--mid-autumn-line-soft) !important;
    }

    html.dark.easter-egg-mid-autumn .capsule-radio .el-radio-button.is-active .el-radio-button__inner,
    html.dark.easter-egg-mid-autumn .modern-radio .el-radio-button.is-active .el-radio-button__inner,
    html.dark.easter-egg-mid-autumn .mobile-drawer .el-menu-item.is-active {
      background-color: rgba(229, 196, 104, 0.12) !important;
      border-color: var(--mid-autumn-primary) !important;
      box-shadow: 0 0 0 1px var(--mid-autumn-primary) inset !important;
      color: var(--mid-autumn-primary) !important;
    }

    html.dark.easter-egg-mid-autumn .sort-select .el-select__placeholder,
    html.dark.easter-egg-mid-autumn .sort-select .el-select__selected-item,
    html.dark.easter-egg-mid-autumn .capsule-radio .el-radio-button__inner,
    html.dark.easter-egg-mid-autumn .modern-radio .el-radio-button__inner {
      color: rgba(255, 247, 219, 0.72) !important;
    }

    html.dark.easter-egg-mid-autumn .capsule-radio .el-radio-button.is-active .el-radio-button__inner,
    html.dark.easter-egg-mid-autumn .modern-radio .el-radio-button.is-active .el-radio-button__inner {
      color: var(--mid-autumn-primary) !important;
    }

    html.dark.easter-egg-mid-autumn .is-home-layout .canvas-glass-button {
      box-shadow:
        0 0 0 1px rgba(255, 239, 183, 0.2),
        0 0 34px rgba(255, 241, 196, 0.18),
        0 18px 54px rgba(255, 224, 145, 0.26),
        inset 0 1px 0 rgba(255, 255, 238, 0.26);
    }

    html.dark.easter-egg-mid-autumn .is-home-layout .canvas-glass-button::after {
      content: '';
      position: absolute;
      inset: 0;
      z-index: 0;
      pointer-events: none;
      background:
        linear-gradient(105deg, transparent 0 24%, rgba(255, 251, 226, 0.2) 38%, transparent 54%),
        radial-gradient(ellipse at 50% -18%, rgba(255, 244, 199, 0.24), transparent 58%);
      opacity: 0.74;
    }

    html.dark.easter-egg-mid-autumn .canvas-glass-button.is-primary {
      border-color: var(--mid-autumn-line);
      background: rgba(229, 196, 104, 0.18);
    }

    html.dark.easter-egg-mid-autumn .canvas-glass-button.is-primary .glass-tint {
      background: rgba(255, 240, 185, 0.22);
    }

    html.dark.easter-egg-mid-autumn .canvas-glass-button.is-secondary,
    html.dark.easter-egg-mid-autumn .hero-register-btn {
      border-color: rgba(255, 232, 164, 0.36) !important;
      background: rgba(255, 240, 185, 0.13) !important;
      color: #fff !important;
    }

    html.dark.easter-egg-mid-autumn .canvas-glass-button.is-secondary .glass-tint {
      background: rgba(255, 248, 218, 0.14);
    }

    html.dark.easter-egg-mid-autumn .group-title,
    html.dark.easter-egg-mid-autumn .code-preview-box {
      border-color: var(--mid-autumn-primary) !important;
    }

    html.dark.easter-egg-mid-autumn .code-preview-box span {
      color: var(--mid-autumn-primary) !important;
    }

    html.dark.easter-egg-mid-autumn .item-card-preview {
      background-image:
        radial-gradient(circle at 78% 18%, rgba(255, 246, 210, 0.12), transparent 18%),
        linear-gradient(135deg, rgba(229, 196, 104, 0.05), transparent 42%),
        var(--festival-preview-background, none);
    }
  `)

  const osmanthus = startDarkOnlyOsmanthus()

  return combineCleanups(osmanthus, () => style.remove())
}

function startDarkOnlyOsmanthus(): EasterEggCleanup {
  const layer = document.createElement('div')
  layer.className = 'mid-autumn-osmanthus-layer'
  layer.dataset.easterEgg = 'mid-autumn-osmanthus-layer'
  document.body.appendChild(layer)

  function onPointerDown(event: PointerEvent): void {
    if (!document.documentElement.classList.contains('dark')) return
    const target = event.target
    if (!(target instanceof Element)) return
    if (target.closest('input, textarea, select, [contenteditable="true"]')) return

    for (let i = 0; i < 9; i += 1) {
      const flower = document.createElement('span')
      flower.className = 'mid-autumn-osmanthus'
      flower.style.left = `${event.clientX}px`
      flower.style.top = `${event.clientY}px`
      const angle = randomBetween(-Math.PI * 0.9, Math.PI * 0.1)
      const distance = randomBetween(18, 74)
      flower.style.setProperty('--dx', `${Math.cos(angle) * distance}px`)
      flower.style.setProperty('--dy', `${Math.sin(angle) * distance + randomBetween(-12, 28)}px`)
      flower.style.setProperty('--spin', `${randomBetween(-220, 220)}deg`)
      flower.style.setProperty('--scale', `${randomBetween(0.55, 1.08)}`)
      flower.style.setProperty('--duration', `${randomBetween(720, 1180)}ms`)
      layer.appendChild(flower)
      flower.addEventListener('animationend', () => flower.remove(), { once: true })
    }
  }

  window.addEventListener('pointerdown', onPointerDown, true)

  return () => {
    window.removeEventListener('pointerdown', onPointerDown, true)
    layer.remove()
  }
}
