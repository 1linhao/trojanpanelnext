#!/usr/bin/env node

import fs from 'node:fs'
import path from 'node:path'
import process from 'node:process'

function fail(message) {
  process.stderr.write(`错误：${message}\n`)
  process.exit(1)
}

function countFiles(root) {
  if (!fs.existsSync(root)) return 0
  let count = 0
  for (const entry of fs.readdirSync(root, { withFileTypes: true })) {
    const entryPath = path.join(root, entry.name)
    if (entry.isDirectory()) count += countFiles(entryPath)
    else count += 1
  }
  return count
}

function replaceChecked(file, pattern, replacement, expected, label) {
  const original = fs.readFileSync(file, 'utf8')
  let replacements = 0
  const updated = original.replace(pattern, (...args) => {
    replacements += 1
    return typeof replacement === 'function' ? replacement(...args) : replacement
  })
  if (replacements !== expected) {
    fail(`${label} 预期替换 ${expected} 处，实际 ${replacements} 处`)
  }
  fs.writeFileSync(file, updated)
  return replacements
}

const destination = process.argv[2] ? path.resolve(process.argv[2]) : ''
if (!destination || !fs.existsSync(path.join(destination, '.git'))) {
  fail('必须传入已经初始化的新仓目录')
}

const apiFixture = path.join(
  destination,
  'apps/control-plane/api/testing/service/node_test.go'
)
const docsSource = path.join(destination, 'apps/docs-site/vpress/api/api.md')
const docsOutput = path.join(destination, 'apps/docs-site/docs')
const installer = path.join(destination, 'deploy/installer/install_script_standalone.sh')
for (const requiredFile of [apiFixture, docsSource, installer]) {
  if (!fs.existsSync(requiredFile)) fail(`缺少待处理文件：${requiredFile}`)
}

const jwtPattern = /eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/g
const apiFixtureJwt = replaceChecked(
  apiFixture,
  jwtPattern,
  'REDACTED_TEST_TOKEN',
  2,
  'API 测试 JWT'
)
const docsJwt = replaceChecked(
  docsSource,
  jwtPattern,
  'REDACTED_EXAMPLE_TOKEN',
  1,
  '文档 JWT'
)
const docsPrivateKeys = replaceChecked(
  docsSource,
  /("privateKey"\s*:\s*")[^"]+("\s*,?)/g,
  (_match, prefix, suffix) => `${prefix}REDACTED_EXAMPLE_PRIVATE_KEY${suffix}`,
  3,
  '文档 privateKey'
)
const docsPasswords = replaceChecked(
  docsSource,
  /("password"\s*:\s*")[^"]+("\s*,?)/g,
  (_match, prefix, suffix) => `${prefix}REDACTED_EXAMPLE_PASSWORD${suffix}`,
  1,
  '文档 password'
)
const installerAllowMarker = replaceChecked(
  installer,
  /^(\s*trojanGO_shadowsocks_password="")$/m,
  (_match, assignment) => `${assignment} # gitleaks:allow -- empty default`,
  1,
  '安装器空密码 allow marker'
)

const removedGeneratedDocs = countFiles(docsOutput)
if (removedGeneratedDocs !== 68) {
  fail(`文档编译产物预期 68 个文件，实际 ${removedGeneratedDocs} 个`)
}
fs.rmSync(docsOutput, { recursive: true, force: false })

process.stdout.write(
  `${JSON.stringify(
    {
      schemaVersion: 1,
      policy: 'security-sanitized-snapshot',
      replacements: {
        apiFixtureJwt,
        docsJwt,
        docsPrivateKeys,
        docsPasswords,
        installerAllowMarker
      },
      exclusions: {
        generatedDocsPath: 'apps/docs-site/docs',
        removedGeneratedDocs
      }
    },
    null,
    2
  )}\n`
)
