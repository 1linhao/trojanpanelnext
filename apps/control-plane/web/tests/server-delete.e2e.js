const { spawn } = require('child_process')
const { mkdirSync, writeFileSync } = require('fs')
const { resolve } = require('path')
const assert = require('assert/strict')
const http = require('http')

// Run real Web interactions against a local recorder. Never contact a live Node.
const webUrl = 'http://127.0.0.1:18888'
const driverUrl = 'http://127.0.0.1:9518/wd/hub'
const children = []
const deletions = []
const removed = new Set()
const artifacts = resolve(
  process.env.SERVER_DELETE_ARTIFACTS || '../../../.local/server-delete-dialog'
)
mkdirSync(artifacts, { recursive: true })
const start = (command, args, env = {}) =>
  children.push(
    spawn(command, args, { env: { ...process.env, ...env }, stdio: 'ignore' })
  )
function request(url, method = 'GET', body) {
  return new Promise((resolve, reject) => {
    const req = http.request(
      url,
      { method, headers: { 'Content-Type': 'application/json' } },
      (res) => {
        let text = ''
        res.on('data', (chunk) => {
          text += chunk
        })
        res.on('end', () =>
          res.statusCode >= 400
            ? reject(new Error(`${res.statusCode} ${text}`))
            : resolve(text ? JSON.parse(text) : null)
        )
      }
    )
    req.setTimeout(15000, () => req.destroy(new Error('Request timeout')))
    req.on('error', reject)
    req.end(body ? JSON.stringify(body) : '')
  })
}
async function waitFor(check, label) {
  const deadline = Date.now() + 45000
  let lastError
  while (Date.now() < deadline) {
    try {
      const value = await check()
      if (value) return value
    } catch (error) {
      lastError = error
    }
    await new Promise((resolve) => setTimeout(resolve, 100))
  }
  throw new Error(`Timed out: ${label} ${lastError?.message || ''}`)
}
const fixture = http.createServer((req, res) => {
  const path = new URL(req.url, webUrl).pathname
  if (path === '/api/nodeServer/deleteNodeServerById') {
    let body = ''
    req.on('data', (chunk) => {
      body += chunk
    })
    req.on('end', () => {
      const data = JSON.parse(body)
      deletions.push(data)
      setTimeout(() => {
        const failed = data.id === 4
        if (!failed) removed.add(data.id)
        res.setHeader('Content-Type', 'application/json')
        res.end(
          JSON.stringify(
            failed
              ? { code: 50000, message: '模拟目标机器卸载失败', data: null }
              : { code: 20000, data: { cleanupPending: true } }
          )
        )
      }, 600)
    })
    return
  }
  const upstream = http.request(
    `http://127.0.0.1:18081${req.url}`,
    { method: req.method, headers: req.headers },
    (response) => {
      if (path !== '/api/nodeServer/selectNodeServerPage') {
        res.writeHead(response.statusCode, response.headers)
        response.pipe(res)
        return
      }
      let body = ''
      response.on('data', (chunk) => {
        body += chunk
      })
      response.on('end', () => {
        const result = JSON.parse(body)
        result.data.nodeServers = result.data.nodeServers.filter(
          (row) => !removed.has(row.id)
        )
        result.data.total = result.data.nodeServers.length
        res.setHeader('Content-Type', 'application/json')
        res.end(JSON.stringify(result))
      })
    }
  )
  upstream.on('error', () => {
    res.statusCode = 502
    res.end()
  })
  req.pipe(upstream)
})
async function main() {
  start(process.execPath, ['tests/mock-api-server.js'], {
    MOCK_API_PORT: '18081'
  })
  await new Promise((resolve, reject) => {
    fixture.once('error', reject)
    fixture.listen(18082, '127.0.0.1', resolve)
  })
  start('npm', ['run', 'serve', '--', '--port', '18888'], {
    MOCK_API_TARGET: 'http://127.0.0.1:18082'
  })
  start('chromedriver', [
    '--port=9518',
    '--url-base=/wd/hub',
    '--log-level=WARNING'
  ])
  await waitFor(
    () => request('http://127.0.0.1:18081/api/auth/setting'),
    'fixture'
  )
  await waitFor(async () => (await fetch(webUrl)).ok, 'Web')
  await waitFor(() => request(`${driverUrl}/status`), 'ChromeDriver')
  const session = (
    await request(`${driverUrl}/session`, 'POST', {
      capabilities: {
        alwaysMatch: {
          browserName: 'chrome',
          'goog:loggingPrefs': { browser: 'ALL' },
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
    (await request(`${base}${path}`, method, body)).value
  const execute = (script) =>
    command('/execute/sync', 'POST', { script, args: [] })
  const find = (selector) =>
    command('/element', 'POST', { using: 'css selector', value: selector })
  const id = (element) => element['element-6066-11e4-a52e-4f735466cecf']
  const click = async (selector) =>
    command(`/element/${id(await find(selector))}/click`, 'POST', {})
  const type = async (selector, text) =>
    command(`/element/${id(await find(selector))}/value`, 'POST', { text })
  const screenshot = async (name) =>
    writeFileSync(
      resolve(artifacts, name),
      Buffer.from(await command('/screenshot'), 'base64')
    )
  const dialog = '.server-delete-dialog'
  const open = async (name) => {
    const button = await command('/element', 'POST', {
      using: 'xpath',
      value: `//tr[td/strong[normalize-space(.)='${name}']]//button[@title='删除']`
    })
    await command(`/element/${id(button)}/click`, 'POST', {})
    await waitFor(
      () => execute(`return Boolean(document.querySelector('${dialog}'))`),
      'dialog'
    )
  }
  const closed = () =>
    waitFor(
      () => execute(`return !document.querySelector('${dialog}')`),
      'closed dialog'
    )
  try {
    await command('/url', 'POST', { url: webUrl })
    await waitFor(() => find('input[autocomplete="username"]'), 'login')
    await type('input[autocomplete="username"]', 'sysadmin')
    await type('input[autocomplete="current-password"]', '123456')
    if (
      await execute(
        "return Boolean(document.querySelector('.captcha-row input'))"
      )
    )
      await type('.captcha-row input', 'mock')
    await click('.auth-submit.primary')
    await waitFor(
      () => execute("return location.hash.includes('/dashboard/index')"),
      'dashboard'
    )
    await command('/url', 'POST', {
      url: webUrl + '/#/server-manage/server-list'
    })
    await waitFor(
      () =>
        execute(
          "return document.querySelectorAll('.row-actions button[title=删除]').length === 5"
        ),
      'server rows'
    )
    assert.equal(
      await execute(
        "return [...document.querySelectorAll('.row-actions button')].some(b => /无痕|彻底删除/.test(b.textContent + b.title))"
      ),
      false
    )
    await open('Tokyo')
    assert.deepEqual(
      await execute(
        `return [...document.querySelectorAll('${dialog} .dialog-footer button')].map(b => b.textContent.trim())`
      ),
      ['取消', '删除', '彻底删除']
    )
    assert.equal(
      await execute('return document.activeElement.textContent.trim()'),
      '取消'
    )
    assert.match(
      await execute(`return document.querySelector('${dialog}').textContent`),
      /此操作无法恢复/
    )
    await screenshot('desktop.png')
    await click(`${dialog} .dialog-footer button:first-child`)
    await closed()
    await open('Tokyo')
    await command('/actions', 'POST', {
      actions: [
        {
          type: 'key',
          id: 'keyboard',
          actions: [
            { type: 'keyDown', value: '\uE00C' },
            { type: 'keyUp', value: '\uE00C' }
          ]
        }
      ]
    })
    await closed()
    await open('Tokyo')
    await click(`${dialog} .tp-ui-dialog__close`)
    await closed()
    await open('Tokyo')
    await command('/actions', 'POST', {
      actions: [
        {
          type: 'pointer',
          id: 'mouse',
          parameters: { pointerType: 'mouse' },
          actions: [
            { type: 'pointerMove', duration: 0, x: 5, y: 5 },
            { type: 'pointerDown', button: 0 },
            { type: 'pointerUp', button: 0 }
          ]
        }
      ]
    })
    await closed()
    assert.equal(deletions.length, 0)
    console.log('PASS cancel, Escape, close icon and backdrop send no deletion')
    await open('Tokyo')
    await click(`${dialog} .dialog-footer button:nth-child(2)`)
    await waitFor(() => deletions.length === 1, 'ordinary request')
    assert.deepEqual(deletions[0], { id: 1, purge: false })
    assert.equal(
      await execute(
        "return [...document.querySelectorAll('.row-actions button[title=删除]')].every(b => b.disabled)"
      ),
      true
    )
    await waitFor(
      () =>
        execute(
          "return ![...document.querySelectorAll('td strong')].some(e => e.textContent === 'Tokyo')"
        ),
      'removed row'
    )
    console.log(
      'PASS 删除 sends purge=false; row actions disabled during request'
    )
    await open('Singapore')
    await click(`${dialog} .dialog-footer button:nth-child(3)`)
    await waitFor(() => deletions.length === 2, 'purge request')
    assert.deepEqual(deletions[1], { id: 2, purge: true })
    await waitFor(
      () =>
        execute(
          "return ![...document.querySelectorAll('td strong')].some(e => e.textContent === 'Singapore')"
        ),
      'purged row'
    )
    console.log('PASS 彻底删除 sends purge=true')
    await open('San Francisco')
    await click(`${dialog} .dialog-footer button:nth-child(2)`)
    await waitFor(
      () =>
        execute(
          "return document.body.innerText.includes('模拟目标机器卸载失败')"
        ),
      'failed request'
    )
    assert.equal(
      await execute(
        "return [...document.querySelectorAll('td strong')].some(e => e.textContent === 'San Francisco')"
      ),
      true
    )
    await waitFor(
      () =>
        execute(
          "return [...document.querySelectorAll('.row-actions button[title=删除]')].every(b => !b.disabled)"
        ),
      'retry enabled'
    )
    console.log('PASS failure keeps server row and enables retry')
    await waitFor(
      () => execute("return !document.querySelector('.liquid-message')"),
      'notifications dismissed'
    )
    await command('/window/rect', 'POST', { width: 390, height: 844 })
    await command('/refresh', 'POST', {})
    await waitFor(
      () =>
        execute(
          "return document.querySelectorAll('.row-actions button[title=删除]').length === 3"
        ),
      'mobile server rows'
    )
    await open('Frankfurt')
    // Capture settled layout after the viewport and overlay transitions.
    await new Promise((resolve) => setTimeout(resolve, 600))
    await screenshot('mobile.png')
    assert.equal(
      await execute(
        `return [...document.querySelectorAll('${dialog} .dialog-footer button')].every(b => { const r=b.getBoundingClientRect(); return r.left>=0 && r.right<=innerWidth && r.bottom<=innerHeight })`
      ),
      true
    )
    const logs = await command('/log', 'POST', { type: 'browser' })
    assert.deepEqual(
      logs.filter(
        (log) => log.level === 'SEVERE' && !log.message.includes('favicon')
      ),
      []
    )
    assert.equal(deletions.length, 3)
    console.log('PASS mobile buttons visible; no browser errors')
    console.log(`Screenshots: ${artifacts}`)
  } finally {
    await request(base, 'DELETE').catch(() => {})
  }
}
main()
  .catch((error) => {
    console.error(error)
    process.exitCode = 1
  })
  .finally(() => {
    fixture.close()
    children.forEach((child) => child.kill('SIGTERM'))
  })
