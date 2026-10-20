import { execFileSync } from 'node:child_process'
import { copyFile, mkdir, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { build } from 'vite'

const root = fileURLToPath(new URL('../../', import.meta.url))
const output = new URL('./dist/', import.meta.url)
const revision = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }).trim()

await build({
  root,
  configFile: fileURLToPath(new URL('../../vite.config.ts', import.meta.url)),
  base: '/',
  envDir: false,
  define: { 'import.meta.env.VITE_API_BASE': JSON.stringify('') },
  plugins: [
    {
      name: 'hero-resize-verification-entry',
      transformIndexHtml: {
        order: 'pre',
        handler(html) {
          const entry = 'src="/src/main.ts"'
          if (!html.includes(entry)) throw new Error('Application entry not found')
          return html.replace(entry, 'src="/verification/hero-resize/entry.ts"')
        },
      },
    },
  ],
  build: {
    outDir: fileURLToPath(new URL('./app/', output)),
    emptyOutDir: true,
    sourcemap: false,
  },
})

await mkdir(output, { recursive: true })
for (const name of ['server.mjs', 'assets.mjs', 'index.html', 'README.md', 'start.ps1']) {
  await copyFile(new URL(name, import.meta.url), new URL(name, output))
}
await copyFile(new URL('../../../LICENSE.txt', import.meta.url), new URL('LICENSE.txt', output))
await writeFile(
  new URL('build-info.json', output),
  JSON.stringify({ issue: 19, revision, built_at: new Date().toISOString() }, null, 2) + '\n',
)
console.log(`Verification package: ${fileURLToPath(output)} (${revision})`)
