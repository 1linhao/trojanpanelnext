import { createReadStream } from 'node:fs'
import { stat } from 'node:fs/promises'
import http from 'node:http'
import path from 'node:path'

const root = path.resolve(process.argv[2] || 'dist/ui-labs')
const port = Number(process.env.UI_LABS_PORT || 4173)
const types = {
  '.css': 'text/css',
  '.html': 'text/html',
  '.js': 'text/javascript',
  '.svg': 'image/svg+xml'
}

http
  .createServer(async (request, response) => {
    try {
      const urlPath = decodeURIComponent(
        new URL(request.url, 'http://localhost').pathname
      )
      let file = path.resolve(root, `.${urlPath}`)
      if (!file.startsWith(root)) throw new Error('invalid path')
      if ((await stat(file)).isDirectory()) file = path.join(file, 'index.html')
      response.setHeader(
        'Content-Type',
        `${
          types[path.extname(file)] || 'application/octet-stream'
        }; charset=utf-8`
      )
      createReadStream(file).pipe(response)
    } catch {
      response.statusCode = 404
      response.end('Not found')
    }
  })
  .listen(port, '127.0.0.1', () => {
    process.stdout.write(
      `UI labs: http://127.0.0.1:${port}/examples/integration-lab/\n`
    )
  })
