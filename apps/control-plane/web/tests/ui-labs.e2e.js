const { spawn, spawnSync } = require('child_process')
const http = require('http')

const root = process.cwd()
const webUrl = 'http://127.0.0.1:4173'
const driverUrl = 'http://127.0.0.1:9517/wd/hub'
const children = []

function start(command, args, options = {}) {
  const child = spawn(command, args, {
    cwd: root,
    stdio: ['ignore', 'pipe', 'pipe'],
    ...options
  })
  children.push(child)
  return child
}

function request(url, method = 'GET', body) {
  return new Promise((resolve, reject) => {
    const payload = body ? JSON.stringify(body) : ''
    const target = new URL(url)
    const req = http.request(
      target,
      {
        method,
        headers: payload
          ? {
              'Content-Type': 'application/json',
              'Content-Length': Buffer.byteLength(payload)
            }
          : {}
      },
      (response) => {
        let text = ''
        response.on('data', (chunk) => {
          text += chunk
        })
        response.on('end', () =>
          response.statusCode >= 400
            ? reject(new Error(`${response.statusCode} ${text}`))
            : resolve(text ? JSON.parse(text) : null)
        )
      }
    )
    req.on('error', reject)
    if (payload) req.write(payload)
    req.end()
  })
}

async function waitFor(check, label, timeout = 30000) {
  const deadline = Date.now() + timeout
  let lastError
  while (Date.now() < deadline) {
    try {
      const result = await check()
      if (result) return result
    } catch (error) {
      lastError = error
    }
    await new Promise((resolve) => setTimeout(resolve, 150))
  }
  throw new Error(`Timed out waiting for ${label}: ${lastError?.message || ''}`)
}

async function main() {
  const build = spawnSync(process.execPath, ['scripts/build-ui-labs.mjs'], {
    cwd: root,
    stdio: 'inherit'
  })
  if (build.status !== 0) throw new Error('UI lab build failed')
  start(process.execPath, ['scripts/serve-ui-labs.mjs'])
  start('chromedriver', [
    '--port=9517',
    '--url-base=/wd/hub',
    '--log-level=WARNING'
  ])
  await waitFor(() => request(`${driverUrl}/status`), 'ChromeDriver')
  await waitFor(
    () =>
      new Promise((resolve, reject) =>
        http
          .get(`${webUrl}/examples/integration-lab/`, (response) =>
            resolve(response.statusCode === 200)
          )
          .on('error', reject)
      ),
    'UI lab server'
  )
  const session = (
    await request(`${driverUrl}/session`, 'POST', {
      capabilities: {
        alwaysMatch: {
          browserName: 'chrome',
          'goog:chromeOptions': {
            binary: '/usr/bin/chromium',
            args: [
              '--headless=new',
              '--no-sandbox',
              '--disable-dev-shm-usage',
              '--window-size=1440,1000'
            ]
          }
        }
      }
    })
  ).value
  const base = `${driverUrl}/session/${session.sessionId}`
  const command = async (path, method = 'GET', body) =>
    (await request(`${base}${path}`, method, body))?.value
  const execute = (script) =>
    command('/execute/sync', 'POST', { script, args: [] })
  try {
    await command('/url', 'POST', {
      url: `${webUrl}/examples/integration-lab/`
    })
    await waitFor(
      () =>
        execute(
          "return document.body.innerText.includes('组件、布局、材质和动画通过契约组合')"
        ),
      'integration lab render'
    )
    if (
      (await execute(
        "return document.documentElement.getAttribute('data-ui-material')"
      )) !== 'frosted'
    )
      throw new Error('frosted material was not composed')
    await execute(
      "document.querySelectorAll('.tp-ui-shell__actions .tp-ui-button')[0].click()"
    )
    if (
      (await execute(
        "return document.documentElement.getAttribute('data-ui-material')"
      )) !== 'flat-test'
    )
      throw new Error('flat material switch failed')
    await execute(
      "document.querySelectorAll('.tp-ui-shell__actions .tp-ui-button')[2].click()"
    )
    if (
      (await execute(
        "return document.documentElement.getAttribute('data-ui-motion')"
      )) !== 'none'
    )
      throw new Error('motion switch failed')
    await command('/url', 'POST', { url: `${webUrl}/examples/minimal-lab/` })
    await waitFor(
      () =>
        execute(
          "return document.body.innerText.includes('无 AppShell 的第二布局消费者')"
        ),
      'minimal lab render'
    )
    if (
      (await execute(
        "return Boolean(document.querySelector('.tp-ui-shell'))"
      )) !== false
    )
      throw new Error('minimal lab unexpectedly depends on AppShell')
    process.stdout.write(
      'PASS UI labs: frosted/flat, motion seam, and minimal layout\n'
    )
  } finally {
    await request(base, 'DELETE').catch(() => {})
  }
}

main()
  .catch((error) => {
    process.stderr.write(`FAIL ${error.stack || error.message}\n`)
    process.exitCode = 1
  })
  .finally(() => children.reverse().forEach((child) => child.kill('SIGTERM')))
