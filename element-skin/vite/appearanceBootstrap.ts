import fs from 'node:fs/promises'
import path from 'node:path'
import { transformWithEsbuild, type Plugin } from 'vite'

export function appearanceBootstrap(): Plugin {
  let root = ''
  let base = '/'

  return {
    name: 'appearance-bootstrap',
    configResolved(config) {
      root = config.root
      base = config.base
    },
    transformIndexHtml: {
      order: 'pre',
      async handler(html) {
        const marker = '<!-- appearance-bootstrap -->'
        if (!html.includes(marker)) return html
        const [source, css] = await Promise.all([
          fs.readFile(path.join(root, 'src/storage/appearance.ts'), 'utf8'),
          fs.readFile(path.join(root, 'src/assets/styles/appearance.css'), 'utf8'),
        ])
        const result = await transformWithEsbuild(
          `${source}\ninitializeAppearance(${JSON.stringify(base)})`,
          'appearance.ts',
          { loader: 'ts', format: 'iife', target: 'es2018', minify: true },
        )
        const script = result.code.replace(/<\/script/gi, '<\\/script')
        return html.replace(
          marker,
          () => `<style id="initial-appearance">${css}</style><script>${script}</script>`,
        )
      },
    },
  }
}
