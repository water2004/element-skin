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
    // 让表单校验库经过 Vite 的模块互操作，与浏览器构建保持一致。
    server: { deps: { inline: ['element-plus'] } },
  },
})
