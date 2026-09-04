const { spawn } = require('child_process')
const http = require('http')

const ROOT = process.cwd()
const WEB_URL = 'http://127.0.0.1:8890'
const DRIVER_URL = 'http://127.0.0.1:9516/wd/hub'
const children = []

function start(command, args, options = {}) {
  const child = spawn(command, args, {
    cwd: ROOT,
    stdio: ['ignore', 'pipe', 'pipe'],
    ...options
  })
  child.output = ''
  child.stdout.on('data', (chunk) => {
    child.output += chunk
    process.stdout.write(chunk)
  })
  child.stderr.on('data', (chunk) => {
    child.output += chunk
    process.stderr.write(chunk)
  })
  children.push(child)
  return child
}

function step(message) {
  process.stdout.write(`[traffic-date-e2e] ${message}\n`)
}

function request(url, options = {}, body) {
  return new Promise((resolve, reject) => {
    const parsed = new URL(url)
    const payload = body ? JSON.stringify(body) : ''
    const req = http.request(
      parsed,
      {
        method: options.method || 'GET',
        headers: payload
          ? {
              'Content-Type': 'application/json; charset=utf-8',
              'Content-Length': Buffer.byteLength(payload)
            }
          : undefined
      },
      (res) => {
        let text = ''
        res.setEncoding('utf8')
        res.on('data', (chunk) => {
          text += chunk
        })
        res.on('end', () => {
          if (res.statusCode >= 400) {
            reject(new Error(`${res.statusCode} ${text}`))
            return
          }
          const contentType = res.headers['content-type'] || ''
          resolve(
            text && contentType.includes('application/json')
              ? JSON.parse(text)
              : text
          )
        })
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
    await new Promise((resolve) => setTimeout(resolve, 200))
  }
  throw new Error(
    `等待 ${label} 超时${lastError ? `：${lastError.message}` : ''}`
  )
}

async function webdriver(path, method = 'GET', body) {
  let response
  try {
    response = await request(`${DRIVER_URL}${path}`, { method }, body)
  } catch (error) {
    throw new Error(`${method} ${path}：${error.message}`)
  }
  return response && Object.prototype.hasOwnProperty.call(response, 'value')
    ? response.value
    : response
}

async function main() {
  start(process.execPath, ['tests/mock-api-server.js'], {
    env: { ...process.env, MOCK_API_PORT: '18081' }
  })
  const web = start('npm', ['run', 'serve', '--', '--port', '8890'], {
    env: {
      ...process.env,
      MOCK_API_TARGET: 'http://127.0.0.1:18081'
    }
  })
  start('chromedriver', [
    '--port=9516',
    '--url-base=/wd/hub',
    '--log-level=WARNING'
  ])

  step('等待隔离服务')
  await waitFor(
    () => request('http://127.0.0.1:18081/auth/setting'),
    'mock API'
  )
  await waitFor(() => web.output.includes('Local:'), 'Vue 编译', 60000)
  await waitFor(() => request(WEB_URL), '开发服务器', 60000)
  await waitFor(() => request(`${DRIVER_URL}/status`), 'ChromeDriver')

  step('创建无头浏览器会话')
  const session = await webdriver('/session', 'POST', {
    capabilities: {
      alwaysMatch: {
        browserName: 'chrome',
        'goog:chromeOptions': {
          binary: '/usr/bin/chromium',
          args: [
            '--headless=new',
            '--no-sandbox',
            '--disable-dev-shm-usage',
            '--window-size=1440,1200'
          ]
        }
      }
    }
  })
  const sessionId = session.sessionId
  const sessionPath = `/session/${sessionId}`
  const execute = (script, args = []) =>
    webdriver(`${sessionPath}/execute/sync`, 'POST', { script, args })
  const navigate = (url) => webdriver(`${sessionPath}/url`, 'POST', { url })
  const find = (using, value) =>
    webdriver(`${sessionPath}/element`, 'POST', { using, value })
  const click = (element) =>
    webdriver(
      `${sessionPath}/element/${element['element-6066-11e4-a52e-4f735466cecf']}/click`,
      'POST',
      {}
    )

  try {
    step('注入管理员预览令牌并打开首页')
    await navigate(WEB_URL)
    await execute("document.cookie='Authorization=Bearer mock-token; path=/'")
    await navigate(`${WEB_URL}/#/dashboard/index`)
    step('等待全部排行数据')
    await waitFor(
      async () =>
        await execute("return document.body.innerText.includes('rank-total')"),
      '初始排行数据',
      10000
    )

    step('切换按月并选择 2026-07')
    const monthButton = await find(
      'css selector',
      ".dashboard-detail-grid > [data-ui-component='panel']:nth-child(1) .dashboard-segment button:nth-child(2)"
    )
    await click(monthButton)
    const monthTrigger = await waitFor(
      () =>
        find(
          'css selector',
          ".dashboard-detail-grid > [data-ui-component='panel']:nth-child(1) .traffic-period-filter__date .liquid-date-picker__trigger"
        ),
      '月份选择器'
    )
    await click(monthTrigger)
    const selectedMonth = await execute(
      "const input = document.querySelector(\".dashboard-detail-grid > [data-ui-component='panel']:nth-child(1) .traffic-period-filter__date .liquid-date-picker__manual input\"); input.value = '2026-07'; input.dispatchEvent(new Event('input', { bubbles: true })); return input.value"
    )
    if (selectedMonth !== '2026-07')
      throw new Error(`月份输入未更新：${selectedMonth}`)
    const monthConfirm = await find(
      'css selector',
      ".dashboard-detail-grid > [data-ui-component='panel']:nth-child(1) .traffic-period-filter__date .liquid-date-picker__popover footer .is-primary"
    )
    await click(monthConfirm)

    step('等待新月份排行数据')
    await waitFor(
      async () =>
        await execute(
          "return document.body.innerText.includes('rank-2026-07')"
        ),
      '切换月份后的排行数据',
      10000
    )

    step('切换按日并选择 2026-07-15')
    const dayButton = await find(
      'css selector',
      ".dashboard-detail-grid > [data-ui-component='panel']:nth-child(1) .dashboard-segment button:nth-child(3)"
    )
    await click(dayButton)
    const dayTrigger = await waitFor(
      () =>
        find(
          'css selector',
          ".dashboard-detail-grid > [data-ui-component='panel']:nth-child(1) .traffic-period-filter__date .liquid-date-picker__trigger"
        ),
      '日期选择器'
    )
    await click(dayTrigger)
    const selectedDay = await execute(
      "const input = document.querySelector('.liquid-date-picker__manual input'); input.value = '2026-07-15'; input.dispatchEvent(new Event('input', { bubbles: true })); return input.value"
    )
    if (selectedDay !== '2026-07-15')
      throw new Error(`日期输入未更新：${selectedDay}`)
    const dayConfirm = await find(
      'css selector',
      ".dashboard-detail-grid > [data-ui-component='panel']:nth-child(1) .traffic-period-filter__date .liquid-date-picker__popover footer .is-primary"
    )
    await click(dayConfirm)
    await waitFor(
      async () =>
        await execute(
          "return document.body.innerText.includes('rank-2026-07-15')"
        ),
      '切换日期后的排行数据',
      10000
    )

    step('检查日期选择器与普通下拉框使用相同的尾部间距')
    const serverDayButton = await find(
      'css selector',
      ".dashboard-detail-grid > [data-ui-component='panel']:nth-child(2) .dashboard-segment button:nth-child(3)"
    )
    await click(serverDayButton)
    await waitFor(
      () =>
        find(
          'css selector',
          ".dashboard-detail-grid > [data-ui-component='panel']:nth-child(2) .liquid-date-picker__trigger"
        ),
      '服务器日期选择器'
    )
    const tailGaps = await execute(`
      const panel = document.querySelector(".dashboard-detail-grid > [data-ui-component='panel']:nth-child(2)")
      const measure = (triggerSelector, arrowSelector) => {
        const trigger = panel.querySelector(triggerSelector).getBoundingClientRect()
        const arrow = panel.querySelector(arrowSelector).getBoundingClientRect()
        return Math.round(trigger.right - arrow.right)
      }
      return {
        select: measure('.liquid-select__trigger', '.liquid-select__arrow'),
        date: measure('.liquid-date-picker__trigger', '.liquid-date-picker__arrow')
      }
    `)
    if (Math.abs(tailGaps.date - tailGaps.select) > 1) {
      throw new Error(
        `日期与普通下拉框尾部间距不一致：${JSON.stringify(tailGaps)}`
      )
    }

    step('检查服务器汇总筛选、列结构与用户详情弹窗')
    const serverContract = await execute(`
      const panel = document.querySelector(".dashboard-detail-grid > [data-ui-component='panel']:nth-child(2)")
      const rankPanel = document.querySelector(".dashboard-detail-grid > [data-ui-component='panel']:nth-child(1)")
      const labels = [...panel.querySelectorAll('.dashboard-segment button')].map((button) => button.textContent.trim())
      const rankLabels = [...rankPanel.querySelectorAll('.dashboard-segment button')].map((button) => button.textContent.trim())
      const table = panel.querySelector('.dashboard-table-block > .liquid-table')
      const headers = [...table.querySelectorAll('th')].map((cell) => cell.textContent.trim())
      return { labels, rankLabels, headers, rowCount: table.querySelectorAll('tbody tr').length }
    `)
    if (
      JSON.stringify(serverContract.labels) !==
      JSON.stringify(serverContract.rankLabels)
    ) {
      throw new Error(
        `服务器周期按钮未对齐：${JSON.stringify(serverContract.labels)}`
      )
    }
    if (
      serverContract.headers.includes('用户名') ||
      serverContract.headers.includes('Username')
    ) {
      throw new Error(
        `服务器汇总仍包含用户名列：${JSON.stringify(serverContract.headers)}`
      )
    }
    if (serverContract.rowCount !== 5) {
      throw new Error(`服务器汇总行数异常：${serverContract.rowCount}`)
    }
    const detailButton = await find(
      'css selector',
      ".dashboard-detail-grid > [data-ui-component='panel']:nth-child(2) .dashboard-table-block > .liquid-table .cap.small"
    )
    await click(detailButton)
    await waitFor(
      async () =>
        await execute(
          "return document.querySelector('.tp-ui-dialog')?.innerText.includes('glassdemo')"
        ),
      '服务器用户详情',
      10000
    )
    const surfaces = await execute(`
      const panel = document.querySelector(".dashboard-detail-grid > [data-ui-component='panel']:nth-child(2)")
      const dialog = document.querySelector('.tp-ui-dialog')
      const panelStyle = getComputedStyle(panel)
      const dialogStyle = getComputedStyle(dialog)
      return {
        panelBackground: panelStyle.background,
        dialogBackground: dialogStyle.background
      }
    `)
    if (surfaces.panelBackground !== surfaces.dialogBackground) {
      throw new Error(`弹窗与内容面板背景不一致：${JSON.stringify(surfaces)}`)
    }
    process.stdout.write(
      'PASS dashboard traffic filters, summaries, and details share the UI contract\n'
    )
  } finally {
    await webdriver(`${sessionPath}`, 'DELETE').catch(() => {})
  }
}

main()
  .catch((error) => {
    process.stderr.write(`FAIL ${error.stack || error.message}\n`)
    process.exitCode = 1
  })
  .finally(() => {
    for (const child of children.reverse()) child.kill('SIGTERM')
  })
