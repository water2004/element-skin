import { fileURLToPath, URL } from 'node:url'
import path from 'node:path'
import fs from 'node:fs'

import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

import { isPathInside, resolveStaticAssetRequest } from './vite/staticAssets'

const isLowMemory = process.env.BUILD_MODE === 'low-memory'
const appVersion = 'v3.0.1'

// https://vite.dev/config/
export default defineConfig({
  base: process.env.VITE_BASE_PATH || '/',
  define: {
    __APP_VERSION__: JSON.stringify(appVersion),
  },
  plugins: [
    tailwindcss(),
    vue(),
    vueDevTools(),
    {
      name: 'serve-static-assets',
      configureServer(server) {
        server.middlewares.use((req, res, next) => {
          const base = process.env.VITE_BASE_PATH || '/'
          const url = req.url || ''
          const asset = resolveStaticAssetRequest(
            path.resolve(__dirname, '../skin-backend'),
            base,
            url,
          )

          if (asset && fs.existsSync(asset.filePath) && fs.statSync(asset.filePath).isFile()) {
            const realRoot = fs.realpathSync(asset.rootPath)
            const realFile = fs.realpathSync(asset.filePath)
            if (!isPathInside(realRoot, realFile)) return next()

            res.setHeader('Content-Type', asset.contentType)
            res.end(fs.readFileSync(realFile))
            return
          }
          next()
        })
      },
    },
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    proxy: {
      '^/v1': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
      },
      '^/(authserver|sessionserver|api|users|minecraft)': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
      },
      '^/oauth': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
        bypass: (req) => {
          if (req.headers.accept?.includes('text/html')) return '/index.html'
        },
      },
    },
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
        },
      },
    },
  },
})
