/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

declare const __APP_VERSION__: string

// matter-js ships no type declarations and we don't depend on its API surface
// beyond the easter egg in assets/scripts/meow.ts.
declare module 'matter-js' {
  const Matter: any
  export default Matter
}

interface Window {
  meowCleanup?: () => void
  meowReinit?: () => void
}
