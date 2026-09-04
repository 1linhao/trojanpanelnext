const { spawn } = require('child_process')
const http = require('http')

const webUrl = process.env.LIVE_WEB_URL || 'http://127.0.0.1:18888'
const driverUrl = 'http://127.0.0.1:9519/wd/hub'

function request(url, method = 'GET', body) {
  return new Promise((resolve, reject) => {
    const payload = body ? JSON.stringify(body) : ''
    const req = http.request(
      new URL(url),
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
    req.setTimeout(10000, () => req.destroy(new Error(`Timeout: ${url}`)))
    req.on('error', reject)
    if (payload) req.write(payload)
    req.end()
  })
}

async function waitFor(check, label, timeout = 15000) {
  const deadline = Date.now() + timeout
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
  throw new Error(`Timed out waiting for ${label}: ${lastError?.message || ''}`)
}

async function main() {
  const driver = spawn(
    'chromedriver',
    ['--port=9519', '--url-base=/wd/hub', '--log-level=WARNING'],
    { stdio: 'ignore' }
  )
  try {
    await waitFor(() => request(`${driverUrl}/status`), 'ChromeDriver')
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
                '--window-size=1280,1000'
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
    const find = (selector) =>
      command('/element', 'POST', { using: 'css selector', value: selector })
    const elementId = (element) =>
      element['element-6066-11e4-a52e-4f735466cecf']
    const clickText = async (selector, label) => {
      const element = await command('/element', 'POST', {
        using: 'xpath',
        value: `//${selector}[normalize-space(.)='${label}']`
      })
      await command(`/element/${elementId(element)}/click`, 'POST', {})
    }

    try {
      await command('/url', 'POST', { url: webUrl })
      const username = await waitFor(
        () => find('input[autocomplete="username"]'),
        'login form'
      )
      const logo = await execute(`
        const candidates = [...document.querySelectorAll('img, svg, [class*="logo"], [class*="brand"]')]
        return candidates.map((element) => {
          const style = getComputedStyle(element)
          const rect = element.getBoundingClientRect()
          return {
            tag: element.tagName,
            className: typeof element.className === 'string' ? element.className : element.className.baseVal,
            src: element.currentSrc || element.getAttribute('src'),
            naturalWidth: element.naturalWidth,
            naturalHeight: element.naturalHeight,
            width: rect.width,
            height: rect.height,
            display: style.display,
            background: style.background
          }
        }).filter((entry) => entry.width > 0 && entry.height > 0)
      `)
      const logoImage = logo.find(
        (entry) => entry.tag === 'IMG' && entry.src?.includes('/api/image/logo')
      )
      if (
        !logoImage ||
        !logoImage.naturalWidth ||
        !logoImage.naturalHeight ||
        logoImage.width < 40 ||
        logoImage.width > 64 ||
        logoImage.height < 40 ||
        logoImage.height > 64
      ) {
        throw new Error(`Login Logo skin failed: ${JSON.stringify(logoImage)}`)
      }
      if (process.env.STYLE_LOGIN_ONLY === '1') {
        process.stdout.write(`${JSON.stringify({ logo }, null, 2)}\n`)
        return
      }
      const password = await find('input[autocomplete="current-password"]')
      await command(`/element/${elementId(username)}/value`, 'POST', {
        text: 'sysadmin'
      })
      await command(`/element/${elementId(password)}/value`, 'POST', {
        text: '123456'
      })
      const captcha = await execute(
        "return Boolean(document.querySelector('.captcha-row input'))"
      )
      if (captcha) {
        const field = await find('.captcha-row input')
        await command(`/element/${elementId(field)}/value`, 'POST', {
          text: 'mock'
        })
      }
      const submit = await find('.auth-submit.primary')
      await command(`/element/${elementId(submit)}/click`, 'POST', {})
      await waitFor(
        () => execute("return location.hash.includes('/dashboard/index')"),
        'authenticated dashboard'
      )
      await command('/url', 'POST', {
        url: `${webUrl}/#/system/base-config`
      })
      await waitFor(
        () =>
          execute(
            "return Boolean(document.querySelector('.prototype-config-card .liquid-switch') && document.querySelector('.prototype-config-card .liquid-number-input'))"
          ),
        'system settings controls'
      )
      const controls = await execute(`
        const visible = (element) => {
          const rect = element.getBoundingClientRect()
          return rect.width > 0 && rect.height > 0
        }
        const describe = (element) => {
          const style = getComputedStyle(element)
          const rect = element.getBoundingClientRect()
          return {
            tag: element.tagName,
            type: element.getAttribute('type'),
            className: typeof element.className === 'string' ? element.className : '',
            text: (element.textContent || '').trim().slice(0, 40),
            width: rect.width,
            height: rect.height,
            background: style.background,
            border: style.border,
            borderRadius: style.borderRadius,
            padding: style.padding,
            appearance: style.appearance,
            right: rect.right,
            left: rect.left
          }
        }
        const selectors = [
          '[class*="switch"]',
          'input[type="number"]',
          '[class*="number"]',
          'input[type="file"]',
          '[class*="upload"]',
          '[class*="select"]'
        ]
        return [...new Set(selectors.flatMap((selector) =>
          [...document.querySelectorAll(selector)]
        ))].filter(visible).map(describe)
      `)

      const switchControl = controls.find(
        (control) => control.className.split(' ').includes('liquid-switch')
      )
      const numberControl = controls.find(
        (control) => control.className === 'liquid-number-input'
      )
      const controlFailures = []
      if (
        !switchControl ||
        switchControl.height < 36 ||
        switchControl.borderRadius === '0px' ||
        switchControl.border.includes('outset')
      ) {
        controlFailures.push({ kind: 'switch', actual: switchControl })
      }
      if (
        !numberControl ||
        numberControl.height < 40 ||
        numberControl.borderRadius === '0px' ||
        numberControl.background.startsWith('rgba(0, 0, 0, 0)')
      ) {
        controlFailures.push({ kind: 'number', actual: numberControl })
      }
      if (controlFailures.length) {
        throw new Error(
          `System control production skin failed: ${JSON.stringify(controlFailures)}`
        )
      }

      await clickText('button', 'Web 文件')
      await waitFor(
        () => execute("return Boolean(document.querySelector('.liquid-file-picker .liquid-button'))"),
        'file picker'
      )
      const fileButton = await execute(`
        const element = document.querySelector('.liquid-file-picker .liquid-button')
        const style = getComputedStyle(element)
        const rect = element.getBoundingClientRect()
        return {
          width: rect.width,
          height: rect.height,
          border: style.border,
          borderRadius: style.borderRadius,
          background: style.background,
          interaction: element.dataset.uiInteraction
        }
      `)
      if (
        fileButton.height < 36 ||
        fileButton.borderRadius === '0px' ||
        (fileButton.background.startsWith('rgba(0, 0, 0, 0)') &&
          fileButton.background.includes(' none ')) ||
        fileButton.interaction !== 'nav-lift'
      ) {
        throw new Error(`File button skin failed: ${JSON.stringify(fileButton)}`)
      }

      await clickText('button', '订阅模板')
      await clickText('button', 'sing-box')
      const selectGeometry = await waitFor(
        () => execute(`
          const trigger = document.querySelector('.template-config-editor__template-select .liquid-select__trigger')
          const arrow = trigger?.querySelector('.liquid-select__arrow')
          if (!trigger || !arrow) return null
          const triggerRect = trigger.getBoundingClientRect()
          const arrowRect = arrow.getBoundingClientRect()
          return {
            trailingGap: triggerRect.right - arrowRect.right,
            triggerWidth: triggerRect.width,
            arrowWidth: arrowRect.width
          }
        `),
        'template selector geometry'
      )
      if (selectGeometry.trailingGap < 8 || selectGeometry.trailingGap > 18) {
        throw new Error(
          `Select arrow trailing gap failed: ${JSON.stringify(selectGeometry)}`
        )
      }

      process.stdout.write(
        `PASS production control skins, file picker, select arrow geometry and login Logo\n`
      )
    } finally {
      await request(base, 'DELETE').catch(() => {})
    }
  } finally {
    driver.kill('SIGTERM')
  }
}

main().catch((error) => {
  process.stderr.write(`FAIL ${error.stack || error.message}\n`)
  process.exitCode = 1
})
