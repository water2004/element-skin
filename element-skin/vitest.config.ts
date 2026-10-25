import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    environment: 'jsdom',
    restoreMocks: true,
    server: {
      deps: {
        // Transform Element Plus so async-validator uses the same default export as the browser.
        inline: ['element-plus'],
      },
    },
  },
})
