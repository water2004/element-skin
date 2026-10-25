import { readFile, realpath } from 'node:fs/promises'
import { createServer } from 'node:http'
import { extname, isAbsolute, relative, resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'
import { landscape, cubeFace, mediaFor, settings, user } from './assets.mjs'

const portIndex = process.argv.indexOf('--port')
const port = portIndex === -1 ? 5174 : Number(process.argv[portIndex + 1])
if (!Number.isInteger(port) || port < 1 || port > 65535) {
  throw new Error('--port must be an integer between 1 and 65535')
}
const appRoot = await realpath(fileURLToPath(new URL('./app/', import.meta.url)))
const hub = await readFile(new URL('./index.html', import.meta.url), 'utf8')
const buildInfo = JSON.parse(await readFile(new URL('./build-info.json', import.meta.url), 'utf8'))
const types = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
  '.png': 'image/png',
  '.woff2': 'font/woff2',
}

function inside(root, target) {
  const path = relative(root, target)
  return path !== '..' && !path.startsWith(`..${sep}`) && !isAbsolute(path)
}

function send(req, res, status, body, type = 'application/json; charset=utf-8') {
  res.writeHead(status, {
    'Content-Type': type,
    'Cache-Control': 'no-store',
    'X-Content-Type-Options': 'nosniff',
  })
  res.end(
    req.method === 'HEAD'
      ? undefined
      : typeof body === 'string' || Buffer.isBuffer(body)
        ? body
        : JSON.stringify(body),
  )
}

function fail(req, res, status, reason, object = 'local_fixture', operation = 'read') {
  send(req, res, status, { error: { object, operation, reason } })
}

const server = createServer(async (req, res) => {
  try {
    const url = new URL(req.url, `http://127.0.0.1:${port}`)
    if (url.pathname === '/v2/auth/session/refresh' && req.method === 'POST') {
      return fail(req, res, 401, 'unauthenticated', 'session', 'refresh')
    }
    if (req.method !== 'GET' && req.method !== 'HEAD') {
      res.setHeader('Allow', 'GET, HEAD')
      return fail(req, res, 405, 'method_not_allowed')
    }
    if (url.pathname === '/__verify/health') {
      return send(req, res, 200, {
        status: 'ready',
        service: 'issue-19-verification',
        ...buildInfo,
      })
    }
    if (url.pathname === '/__verify/' || url.pathname === '/__verify') {
      return send(req, res, 200, hub, types['.html'])
    }
    if (url.pathname === '/v2/public/settings') return send(req, res, 200, settings)
    const referer = new URL(req.headers.referer || '/', url)
    if (url.pathname === '/v2/public/homepage-media') {
      return send(req, res, 200, { items: mediaFor(referer.searchParams.get('media')) })
    }
    if (url.pathname === '/v2/users/me') {
      return referer.searchParams.get('signedIn') === '1'
        ? send(req, res, 200, user)
        : fail(req, res, 401, 'unauthenticated', 'session')
    }
    if (/^\/static\/carousel\/local-(day|dusk)\.svg$/.test(url.pathname)) {
      return send(req, res, 200, landscape(url.pathname.includes('dusk')), types['.svg'])
    }
    const face = url.pathname.match(/^\/static\/carousel\/local-panorama\/panorama_([0-5])\.png$/)
    if (face) return send(req, res, 200, cubeFace(Number(face[1])), types['.svg'])

    // Serve only built frontend files. Never expose the package directory or proxy to a backend.
    const pathname = decodeURIComponent(url.pathname)
    if (pathname !== '/' && pathname !== '/favicon.ico' && !pathname.startsWith('/assets/')) {
      return fail(req, res, 404, 'not_configured')
    }
    const file = resolve(appRoot, pathname === '/' ? 'index.html' : `.${pathname}`)
    if (!inside(appRoot, file)) return fail(req, res, 404, 'not_configured')
    const canonicalFile = await realpath(file)
    if (!inside(appRoot, canonicalFile)) return fail(req, res, 404, 'not_configured')
    const contentType = types[extname(canonicalFile)]
    if (!contentType) return fail(req, res, 404, 'not_configured')
    send(req, res, 200, await readFile(canonicalFile), contentType)
  } catch (error) {
    if (error instanceof URIError || error instanceof TypeError) {
      return fail(req, res, 400, 'invalid_request')
    }
    if (['ENOENT', 'ENOTDIR', 'EISDIR'].includes(error.code)) {
      return fail(req, res, 404, 'not_configured')
    }
    console.error(error)
    fail(req, res, 500, 'read_failed')
  }
})
server.on('error', (error) => {
  console.error(
    error.code === 'EADDRINUSE'
      ? `Port ${port} is already in use. Try: node server.mjs --port ${port + 1}`
      : error.message,
  )
  process.exitCode = 1
})
server.listen(port, '127.0.0.1', () => {
  console.log(`Issue #19 verification (${buildInfo.revision})`)
  console.log(`Open http://127.0.0.1:${port}/__verify/`)
  console.log('Press Ctrl+C to stop.')
})
