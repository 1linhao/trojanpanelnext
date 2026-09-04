const { spawn } = require('child_process')
const { writeFileSync } = require('fs')
const http = require('http')

const webUrl = process.env.LIVE_WEB_URL || 'http://127.0.0.1:18888'
const driverUrl = 'http://127.0.0.1:9518/wd/hub'
const screenshotPath =
  process.env.LIVE_SCREENSHOT || '/tmp/tp-composable-ui-live.png'

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
    req.setTimeout(15000, () => req.destroy(new Error(`Request timed out: ${url}`)))
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
  const driver = spawn(
    'chromedriver',
    ['--port=9518', '--url-base=/wd/hub', '--log-level=WARNING'],
    { stdio: 'ignore' }
  )
  try {
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
      (await request(`${base}${path}`, method, body))?.value
    const execute = (script) =>
      command('/execute/sync', 'POST', { script, args: [] })
    const find = (selector) =>
      command('/element', 'POST', { using: 'css selector', value: selector })
    const elementId = (element) =>
      element['element-6066-11e4-a52e-4f735466cecf']
    const assertNavigationControls = async (label, selectors) => {
      const controls = await execute(
        `const groups=${JSON.stringify(selectors)}; return groups.flatMap(({selector,kind}) => [...document.querySelectorAll(selector)].map((element) => { const style=getComputedStyle(element); const painted=style.backgroundImage !== 'none' || !['rgba(0, 0, 0, 0)','transparent'].includes(style.backgroundColor); return {selector,kind,text:element.textContent.trim(),display:style.display,painted,selected:element.matches('.on,.is-selected'),borderWidth:parseFloat(style.borderTopWidth),radius:parseFloat(style.borderTopLeftRadius),shadow:style.boxShadow,backdrop:style.backdropFilter || style.webkitBackdropFilter,interaction:element.dataset.uiInteraction,transition:style.transitionProperty}; }));`
      )
      const failures = controls.filter((control) => {
        if (control.kind === 'shell') {
          return !control.display.includes('flex') || control.painted ||
            control.borderWidth < 1 || control.radius < 16 ||
            control.shadow !== 'none' || control.backdrop !== 'none'
        }
        const invalidStateSkin = control.selected
          ? !control.painted || control.shadow === 'none'
          : control.painted || control.shadow !== 'none'
        return invalidStateSkin || control.borderWidth !== 0 ||
          control.radius < 10 ||
          control.interaction !== 'nav-lift' ||
          !control.transition.includes('transform')
      })
      if (!controls.length || failures.length) {
        throw new Error(
          `${label} navigation small-control skin failed: ${JSON.stringify({
            controls,
            failures
          })}`
        )
      }
      return controls.length
    }
    try {
      await command('/url', 'POST', { url: webUrl })
      const username = await waitFor(
        () => find('input[autocomplete="username"]'),
        'login form'
      )
      const password = await find('input[autocomplete="current-password"]')
      const loginButtonCoverage = await execute(
        "const items=[...document.querySelectorAll('button,[role=button]')]; return {total:items.length,covered:items.filter((item)=>item.dataset.uiInteraction==='nav-lift').length,motion:document.documentElement.dataset.uiMotion,transition:getComputedStyle(document.querySelector('.auth-submit.primary')).transitionProperty}"
      )
      if (
        !loginButtonCoverage.total ||
        loginButtonCoverage.covered !== loginButtonCoverage.total ||
        !loginButtonCoverage.transition.includes('transform') ||
        !['full', 'reduced'].includes(loginButtonCoverage.motion)
      ) {
        throw new Error(
          `Login button interaction coverage failed: ${JSON.stringify(
            loginButtonCoverage
          )}`
        )
      }
      await command(`/element/${elementId(username)}/value`, 'POST', {
        text: 'sysadmin'
      })
      await command(`/element/${elementId(password)}/value`, 'POST', {
        text: '123456'
      })
      // The local mock accepts any non-empty captcha fixture; no real challenge
      // is solved here. Supply it when the mock exposes the optional field.
      const captcha = await execute("return Boolean(document.querySelector('.captcha-row input'))")
      if (captcha) {
        const field = await find('.captcha-row input')
        await command(`/element/${elementId(field)}/value`, 'POST', { text: 'mock' })
      }
      const submit = await find('.auth-submit.primary')
      await command(`/element/${elementId(submit)}/click`, 'POST', {})
      await waitFor(
        () =>
          execute(
            "return location.hash.includes('/dashboard/index') && document.body.innerText.includes('仪表板')"
          ),
        'authenticated dashboard'
      )
      if (
        await execute(
          "return Boolean(document.querySelector('.liquid-message.is-error'))"
        )
      )
        throw new Error('UI displayed an error message')
      const dashboardButtonCoverage = await execute(
        "const items=[...document.querySelectorAll('button,[role=button]')]; return {total:items.length,covered:items.filter((item)=>item.dataset.uiInteraction==='nav-lift').length}"
      )
      if (
        !dashboardButtonCoverage.total ||
        dashboardButtonCoverage.covered !== dashboardButtonCoverage.total
      ) {
        throw new Error(
          `Dashboard button interaction coverage failed: ${JSON.stringify(
            dashboardButtonCoverage
          )}`
        )
      }
      const clickText = async (selector, label) => {
        const element = await command('/element', 'POST', {
          using: 'xpath', value: `//${selector}[normalize-space(.)='${label}']`
        })
        await command('/execute/sync', 'POST', {
          script:
            "arguments[0].scrollIntoView({ block: 'nearest', inline: 'center' })",
          args: [element]
        })
        await command(`/element/${elementId(element)}/click`, 'POST', {})
      }
      await clickText('aside/button', '系统配置')
      await waitFor(() => find('.prototype-config-card'), 'system settings route')
      await clickText('button', '面板设置')
      await waitFor(() => find('.panel-config input'), 'panel settings form')
      await clickText('button', '订阅模板')
      for (const [client, content] of [
        ['Clash.Meta', 'rules: []'],
        ['sing-box', '{"outbounds":[]}'],
        ['Xray', '{"routing":{"rules":[]}}']
      ]) {
        await clickText('button', client)
        const editor = await waitFor(() => find('.subscription-config-editor textarea'), `${client} editor`)
        await command(`/element/${elementId(editor)}/value`, 'POST', { text: '\uE009a\uE000' + content })
        const formatButton = await waitFor(() => find('.liquid-code-editor__toolbar button'), `${client} format button`)
        await command(`/element/${elementId(formatButton)}/click`, 'POST', {})
        await waitFor(
          () => execute(
            client === 'Clash.Meta'
              ? `const field=document.querySelector('.subscription-config-editor textarea'); return field.getAttribute('aria-invalid') !== 'true' && field.value.trim() === ${JSON.stringify(content)}`
              : `const field=document.querySelector('.subscription-config-editor textarea'); try { return field.getAttribute('aria-invalid') !== 'true' && JSON.stringify(JSON.parse(field.value)) === ${JSON.stringify(content)} } catch (_) { return false }`
          ),
          `${client} format`
        )
        const value = await execute("return document.querySelector('.subscription-config-editor textarea').value")
        if (client === 'Clash.Meta' ? value.trim() !== content : JSON.stringify(JSON.parse(value)) !== content) {
          throw new Error(`${client} editor lost input: ${value}`)
        }
      }
      await clickText('button', 'Clash.Meta')
      await waitFor(() => execute("return document.querySelector('.subscription-config-editor textarea').value.trim() === 'rules: []'"), 'template state across clients')
      const settingsSmallControls = await waitFor(
        () => assertNavigationControls(
          'system settings',
          [
            { selector: '.prototype-config-card .liquid-tabs', kind: 'shell' },
            { selector: '.prototype-config-card .liquid-tabs button', kind: 'action' },
            {
              selector: '.prototype-config-card .liquid-code-editor__toolbar button',
              kind: 'action'
            }
          ]
        ),
        'settled system settings navigation controls'
      )
      await clickText('button', 'sing-box')
      const selectTrigger = await waitFor(
        () => find('.template-config-editor__template-select .liquid-select__trigger'),
        'template selector'
      )
      await command(`/element/${elementId(selectTrigger)}/click`, 'POST', {})
      await waitFor(() => find('.liquid-select__option'), 'template selector options')
      const selectSmallControls = await waitFor(
        () => assertNavigationControls(
          'system settings select',
          [{ selector: '.liquid-select__option', kind: 'action' }]
        ),
        'settled select navigation controls'
      )
      const profileEntry = await find('.prototype-profile-entry')
      await command(`/element/${elementId(profileEntry)}/click`, 'POST', {})
      await waitFor(() => find('.profile-card'), 'profile route')
      await clickText('button', '颜色主题')
      await waitFor(
        () => find('.profile-card .liquid-palette-picker__menu.is-inline'),
        'profile palette'
      )
      const profileSmallControls = await waitFor(
        () => assertNavigationControls(
          'profile',
          [
            { selector: '.profile-card .liquid-tabs', kind: 'shell' },
            { selector: '.profile-card .liquid-tabs button', kind: 'action' },
            {
              selector: '.profile-card .liquid-palette-picker__menu button',
              kind: 'action'
            }
          ]
        ),
        'settled profile navigation controls'
      )
      await command('/window/rect', 'POST', { width: 390, height: 844 })
      await waitFor(
        () =>
          execute(
            "const nav=document.querySelector('.tp-ui-shell__mobile-nav'); return nav && getComputedStyle(nav).display !== 'none'"
          ),
        'mobile navigation'
      )
      const navigationParity = await execute(
        `const read=(element,keys)=>{const style=getComputedStyle(element);return Object.fromEntries(keys.map((key)=>[key,style[key]]))}; const itemKeys=['backgroundColor','backgroundImage','borderTopColor','borderTopWidth','borderTopLeftRadius','boxShadow','color','fontWeight']; const activeNav=document.querySelector('.tp-ui-shell__mobile-nav .tp-ui-shell__nav-item.is-active'); const inactiveNav=document.querySelector('.tp-ui-shell__mobile-nav .tp-ui-shell__nav-item:not(.is-active)'); const pairs=[['inactive tab',document.querySelector('.profile-card .liquid-tabs button:not(.on)'),inactiveNav,itemKeys],['selected tab',document.querySelector('.profile-card .liquid-tabs button.on'),activeNav,itemKeys],['inactive palette action',document.querySelector('.profile-card .liquid-palette-picker__menu button:not(.is-selected)'),inactiveNav,itemKeys],['selected palette action',document.querySelector('.profile-card .liquid-palette-picker__menu button.is-selected'),activeNav,itemKeys]]; return pairs.map(([kind,control,navigation,keys])=>({kind,control:control&&read(control,keys),navigation:navigation&&read(navigation,keys)}));`
      )
      const parityFailures = navigationParity.filter(
        ({ control, navigation }) =>
          !control || !navigation || JSON.stringify(control) !== JSON.stringify(navigation)
      )
      if (parityFailures.length) {
        throw new Error(
          `Small controls differ from mobile navigation: ${JSON.stringify(parityFailures)}`
        )
      }
      await clickText('nav/button', '首页')
      await waitFor(() => execute("return location.hash.includes('/dashboard/index')"), 'mobile route navigation')
      const errors = (await command('/log', 'POST', { type: 'browser' }))
        .filter((entry) => entry.level === 'SEVERE' || /multiple instances of Vue|Failed to resolve|Unknown custom element/i.test(entry.message))
      if (errors.length) throw new Error(`Browser errors: ${JSON.stringify(errors)}`)
      const screenshot = await command('/screenshot')
      writeFileSync(screenshotPath, Buffer.from(screenshot, 'base64'))
      process.stdout.write(
        `PASS live stack login, dashboard, system settings, three template editors, ${settingsSmallControls + selectSmallControls + profileSmallControls} navigation-style small controls without nested surface tint, mobile navigation item parity, clean browser console, ${dashboardButtonCoverage.covered} interactive buttons, responsive navigation; screenshot ${screenshotPath}\n`
      )
    } catch (error) {
      const state = await execute("return {hash:location.hash, editor:document.querySelector('.subscription-config-editor textarea')?.value, alerts:[...document.querySelectorAll('[role=alert]')].map(e=>e.textContent)}").catch(() => null)
      process.stderr.write(`UI failure state: ${JSON.stringify(state)}\n`)
      throw error
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
