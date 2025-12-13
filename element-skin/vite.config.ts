import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

// https://vite.dev/config/
export default defineConfig({
  base: process.env.VITE_BASE_PATH || '/',
  plugins: [
    vue(),
    vueDevTools(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    },
  },
  server: {
    // 开发时将常用后端路由代理到本地后端，避免跨域或错发到 Vite dev server
    proxy: {
      // Yggdrasil / auth APIs
      '^/authserver': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
        rewrite: (path) => path,
      },
      // Session APIs
      '^/sessionserver': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
        rewrite: (path) => path,
      },
      // Register, admin, me, textures etc
      '^/register': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
      },
      '^/admin': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
      },
      '^/textures': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
      },
      '^/static/textures': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
      },
      '^/me': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
      },
      '^/public': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
      },
      // 注意: 不代理根路径 '/'，以免覆盖 Vite 的 index.html
    }
  }
})
