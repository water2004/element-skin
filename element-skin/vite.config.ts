import { fileURLToPath, URL } from 'node:url'
import path from 'node:path'
import fs from 'node:fs'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

const isLowMemory = process.env.BUILD_MODE === 'low-memory'
const appVersion = 'v1.3.1'

// https://vite.dev/config/
export default defineConfig({
  base: process.env.VITE_BASE_PATH || '/',
  define: {
    __APP_VERSION__: JSON.stringify(appVersion),
  },
  plugins: [
    vue(),
    vueDevTools(),
    {
      name: 'serve-static-assets',
      configureServer(server) {
        server.middlewares.use((req, res, next) => {
          // 获取不带 base 前缀的路径
          const base = process.env.VITE_BASE_PATH || '/'
          const url = req.url || ''
          if (!url.startsWith(base)) return next()
          
          const relativeUrl = url.slice(base.length - 1) // 保持以 / 开头
          const staticMatch = relativeUrl.match(/^\/static\/(textures|carousel)\/(.+)$/)
          
          if (staticMatch) {
            const [, type, filename] = staticMatch
            // 映射到后端目录
            const filePath = path.resolve(__dirname, `../skin-backend/${type}/${filename.split('?')[0]}`)
            
            if (fs.existsSync(filePath)) {
              res.setHeader('Content-Type', type === 'textures' ? 'image/png' : 'image/jpeg')
              res.end(fs.readFileSync(filePath))
              return
            }
          }
          next()
        })
      }
    }
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
      // API routes that might conflict with frontend routes
      // When a browser refreshes on these paths, it should serve index.html instead of proxying to the backend
      '^/(admin|register|reset-password|site-login|me|public|microsoft|send-verification-code)': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
        bypass: (req) => {
          if (req.headers.accept?.indexOf('text/html') !== -1) {
            return '/index.html';
          }
        }
      },
    }
  },
  build: {
    sourcemap: !isLowMemory,
    chunkSizeWarningLimit: 1500,
    rollupOptions: {
      // 在低内存模式下将 maxParallelFileOps 设为 1，以极大地减小内存占用
      ...(isLowMemory ? { maxParallelFileOps: 1 } : {}),
      output: {
        manualChunks: {
          'vendor-element': ['element-plus'],
          'vendor-utils': ['axios', 'vue', 'vue-router', 'pinia'],
          'vendor-render': ['three', 'skinview3d'],
        }
      }
    }
  }
})
