import type { EasterEggCleanup } from './index'
import { injectEasterEggStyle } from './domEffects'

export function start(): EasterEggCleanup {
  const style = injectEasterEggStyle(
    'halloween',
    `
    html.easter-egg-halloween {
      --halloween-pumpkin-face: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 64 38'%3E%3Cpath fill='%236f2b08' d='M7 8h16L15 22zM41 8h16l-8 14zM28 17h8l-4 8zM6 27l9-5 8 6 8-6 8 6 8-6 11 5-2 8-9-4-8 5-8-5-8 5-8-5-9 4z'/%3E%3C/svg%3E");
    }

    html.easter-egg-halloween .el-button,
    html.easter-egg-halloween .btn-gradient,
    html.easter-egg-halloween .btn-outline,
    html.easter-egg-halloween .home-fixed-button {
      position: relative;
      isolation: isolate;
      overflow: hidden;
      transition:
        top 0.25s cubic-bezier(0.4, 0, 0.2, 1),
        transform 0.25s cubic-bezier(0.4, 0, 0.2, 1),
        border-color 0.25s ease,
        box-shadow 0.25s ease,
        background-color 0.25s ease;
    }

    html.easter-egg-halloween .el-button::before,
    html.easter-egg-halloween .btn-gradient::before,
    html.easter-egg-halloween .btn-outline::before,
    html.easter-egg-halloween .home-fixed-button::before {
      content: "";
      position: absolute;
      inset: 0;
      z-index: 0;
      pointer-events: none;
      opacity: 0;
      background:
        radial-gradient(ellipse at 50% 50%, rgba(255, 151, 44, 0.24), transparent 44%),
        linear-gradient(135deg, rgba(255, 122, 24, 0.18), rgba(80, 35, 12, 0.1)),
        linear-gradient(135deg, #f08a24, #b85b13);
      transition: opacity 0.22s ease;
    }

    html.easter-egg-halloween .el-button::after,
    html.easter-egg-halloween .btn-gradient::after,
    html.easter-egg-halloween .btn-outline::after,
    html.easter-egg-halloween .home-fixed-button::after {
      content: "";
      position: absolute;
      left: 50%;
      top: 50%;
      z-index: 0;
      width: min(54px, calc(100% - 18px));
      height: 30px;
      pointer-events: none;
      opacity: 0;
      transform: translate(-50%, -50%) scale(0.92);
      background: var(--halloween-pumpkin-face) center / contain no-repeat;
      filter:
        drop-shadow(0 1px 0 rgba(255, 199, 101, 0.2))
        drop-shadow(0 0 5px rgba(73, 25, 4, 0.28));
      transition:
        opacity 0.18s ease,
        transform 0.22s cubic-bezier(0.34, 1.56, 0.64, 1);
    }

    html.easter-egg-halloween .el-button:not(.btn-icon-swap) > span,
    html.easter-egg-halloween .el-button .el-button__text,
    html.easter-egg-halloween .btn-gradient:not(.btn-icon-swap) > *,
    html.easter-egg-halloween .btn-outline:not(.btn-icon-swap) > *,
    html.easter-egg-halloween .home-fixed-button > .home-fixed-label {
      position: relative;
      z-index: 2;
    }

    html.easter-egg-halloween .el-button .el-icon {
      z-index: 2;
    }

    html.easter-egg-halloween .el-button:hover:not(:disabled),
    html.easter-egg-halloween .btn-gradient:hover:not(:disabled),
    html.easter-egg-halloween .btn-outline:hover:not(:disabled),
    html.easter-egg-halloween .home-fixed-button:hover:not(:disabled) {
      border-color: rgba(255, 145, 35, 0.58) !important;
      color: #fff !important;
      box-shadow:
        0 0 0 3px rgba(255, 145, 35, 0.12),
        0 8px 24px rgba(89, 42, 10, 0.22),
        0 0 18px rgba(255, 122, 24, 0.14) !important;
    }

    html.easter-egg-halloween .el-button:hover:not(:disabled)::before,
    html.easter-egg-halloween .btn-gradient:hover:not(:disabled)::before,
    html.easter-egg-halloween .btn-outline:hover:not(:disabled)::before,
    html.easter-egg-halloween .home-fixed-button:hover:not(:disabled)::before,
    html.easter-egg-halloween .el-button:focus-visible::before,
    html.easter-egg-halloween .btn-gradient:focus-visible::before,
    html.easter-egg-halloween .btn-outline:focus-visible::before,
    html.easter-egg-halloween .home-fixed-button:focus-visible::before {
      opacity: 1;
    }

    html.easter-egg-halloween .el-button:hover:not(:disabled)::after,
    html.easter-egg-halloween .btn-gradient:hover:not(:disabled)::after,
    html.easter-egg-halloween .btn-outline:hover:not(:disabled)::after,
    html.easter-egg-halloween .home-fixed-button:hover:not(:disabled)::after,
    html.easter-egg-halloween .el-button:focus-visible::after,
    html.easter-egg-halloween .btn-gradient:focus-visible::after,
    html.easter-egg-halloween .btn-outline:focus-visible::after,
    html.easter-egg-halloween .home-fixed-button:focus-visible::after {
      opacity: 0.58;
      transform: translate(-50%, -50%) scale(1);
    }

    html.easter-egg-halloween .el-button.is-circle::after,
    html.easter-egg-halloween .el-button.is-round::after {
      left: 50%;
      width: 30px;
      height: 18px;
      transform: translate(-50%, -50%) scale(0.92);
    }

    html.easter-egg-halloween .el-button.is-circle:hover:not(:disabled)::after,
    html.easter-egg-halloween .el-button.is-round:hover:not(:disabled)::after,
    html.easter-egg-halloween .el-button.is-circle:focus-visible::after,
    html.easter-egg-halloween .el-button.is-round:focus-visible::after {
      transform: translate(-50%, -50%) scale(1);
    }

    html.easter-egg-halloween .el-button.is-round.drag-btn::after {
      width: min(88px, calc(100% - 48px));
      height: 42px;
    }

    html.easter-egg-halloween .search-bar-container .el-input-group__append {
      position: relative;
      isolation: isolate;
      overflow: hidden;
      background: var(--el-color-primary) !important;
      border-color: var(--el-color-primary) !important;
      opacity: 1 !important;
      transition:
        border-color 0.25s ease,
        box-shadow 0.25s ease;
    }

    html.easter-egg-halloween .search-bar-container .el-input-group__append::before {
      content: "";
      position: absolute;
      inset: 0;
      z-index: 0;
      pointer-events: none;
      opacity: 0;
      background:
        radial-gradient(ellipse at 50% 50%, rgba(255, 151, 44, 0.24), transparent 44%),
        linear-gradient(135deg, rgba(255, 122, 24, 0.16), rgba(80, 35, 12, 0.08)),
        linear-gradient(135deg, #f08a24, #b85b13);
      transition: opacity 0.25s ease;
    }

    html.easter-egg-halloween .search-bar-container .el-input-group__append::after {
      content: "";
      position: absolute;
      left: 50%;
      top: 50%;
      z-index: 1;
      width: 46px;
      height: 28px;
      pointer-events: none;
      opacity: 0;
      transform: translate(-50%, -50%) scale(0.92);
      background: var(--halloween-pumpkin-face) center / contain no-repeat;
      filter:
        drop-shadow(0 1px 0 rgba(255, 199, 101, 0.2))
        drop-shadow(0 0 5px rgba(73, 25, 4, 0.28));
      transition:
        opacity 0.18s ease,
        transform 0.22s cubic-bezier(0.34, 1.56, 0.64, 1);
    }

    html.easter-egg-halloween .search-bar-container .el-input-group__append:hover {
      background: var(--el-color-primary) !important;
      border-color: rgba(255, 161, 42, 0.58) !important;
      opacity: 1 !important;
      box-shadow:
        0 8px 24px rgba(89, 42, 10, 0.22),
        0 0 0 1px rgba(255, 183, 74, 0.24) !important;
    }

    html.easter-egg-halloween .search-bar-container .el-input-group__append:hover::before,
    html.easter-egg-halloween .search-bar-container .el-input-group__append:focus-within::before {
      opacity: 1;
    }

    html.easter-egg-halloween .search-bar-container .el-input-group__append:hover::after,
    html.easter-egg-halloween .search-bar-container .el-input-group__append:focus-within::after {
      opacity: 0.58;
      transform: translate(-50%, -50%) scale(1);
    }

    html.easter-egg-halloween .search-bar-container .el-input-group__append .el-button,
    html.easter-egg-halloween .search-bar-container .el-input-group__append .el-button:hover,
    html.easter-egg-halloween .search-bar-container .el-input-group__append .el-button:focus {
      position: relative;
      z-index: 2;
      transform: none !important;
      background: transparent !important;
      background-image: none !important;
      border-color: transparent !important;
      box-shadow: none !important;
      color: inherit !important;
    }

    html.easter-egg-halloween .search-bar-container .el-input-group__append .el-button::before,
    html.easter-egg-halloween .search-bar-container .el-input-group__append .el-button::after {
      content: none !important;
    }

    html.easter-egg-halloween .is-home-layout .home-fixed-button {
      color: #fff !important;
      box-shadow:
        inset 0 0 0 1px var(--home-action-ring, rgba(255, 255, 255, 0.38)),
        inset 0 1px 0 rgba(255, 255, 255, 0.16) !important;
    }

    html.easter-egg-halloween .is-home-layout .home-fixed-button:hover {
      --home-action-ring: rgba(255, 145, 35, 0.58);
      box-shadow:
        0 0 0 3px rgba(255, 145, 35, 0.12),
        0 14px 28px rgba(89, 42, 10, 0.22),
        0 0 18px rgba(255, 122, 24, 0.14),
        inset 0 0 0 1px var(--home-action-ring),
        inset 0 1px 0 rgba(255, 199, 101, 0.18) !important;
    }
  `,
  )

  return () => style.remove()
}
