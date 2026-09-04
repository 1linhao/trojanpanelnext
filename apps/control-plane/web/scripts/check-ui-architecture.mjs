import { readFile, readdir } from 'node:fs/promises'
import path from 'node:path'
import {
  MATERIAL_CUSTOM_PROPERTIES,
  UI_CUSTOM_PROPERTIES
} from '@tp-ui/contracts'

const packagesRoot = path.resolve('packages')
const packageNames = (await readdir(packagesRoot, { withFileTypes: true }))
  .filter((entry) => entry.isDirectory())
  .map((entry) => entry.name)

async function walk(directory) {
  const entries = await readdir(directory, { withFileTypes: true })
  const files = []
  for (const entry of entries) {
    const fullPath = path.join(directory, entry.name)
    if (entry.isDirectory()) files.push(...(await walk(fullPath)))
    else files.push(fullPath)
  }
  return files
}

const failures = []
const dependencyGraph = new Map()
const requiredReadmeSections = [
  '## 职责',
  '## 非职责',
  '## 安装',
  '## 组合示例',
  '## 兼容矩阵'
]
for (const packageName of packageNames) {
  const packageRoot = path.join(packagesRoot, packageName)
  const manifest = JSON.parse(
    await readFile(path.join(packageRoot, 'package.json'), 'utf8')
  )
  const readme = await readFile(path.join(packageRoot, 'README.md'), 'utf8')
  for (const section of requiredReadmeSections) {
    if (!readme.includes(section)) {
      failures.push(`${packageRoot}/README.md: 缺少 ${section}`)
    }
  }
  dependencyGraph.set(
    manifest.name,
    Object.keys(manifest.dependencies || {}).filter((name) =>
      name.startsWith('@tp-ui/')
    )
  )
  const files = (await walk(path.join(packageRoot, 'src'))).filter((file) =>
    /\.(?:js|css|svg)$/.test(file)
  )
  for (const file of files) {
    const source = await readFile(file, 'utf8')
    if (/#app\b/.test(source)) failures.push(`${file}: 禁止 #app 选择器`)
    if (/!important\b/.test(source)) failures.push(`${file}: 禁止 !important`)
    if (/from\s+['"][^'"]+\/src\//.test(source))
      failures.push(`${file}: 禁止跨包 /src/ 深层导入`)
    if (
      packageName.startsWith('ui-material-') &&
      /\.(?:prototype|account|dashboard|server|node)-/.test(source)
    ) {
      failures.push(`${file}: 材质包包含疑似业务选择器`)
    }
    if (
      !packageName.startsWith('ui-material-') &&
      /#[0-9a-fA-F]{3,8}\b|rgba?\s*\(/.test(source)
    ) {
      failures.push(`${file}: 硬编码色值只能出现在材质包`)
    }
  }
}

const visiting = new Set()
const visited = new Set()
function visit(name, trail = []) {
  if (visiting.has(name)) {
    failures.push(
      `UI package dependency cycle: ${[...trail, name].join(' -> ')}`
    )
    return
  }
  if (visited.has(name)) return
  visiting.add(name)
  for (const dependency of dependencyGraph.get(name) || [])
    visit(dependency, [...trail, name])
  visiting.delete(name)
  visited.add(name)
}
for (const name of dependencyGraph.keys()) visit(name)

const publicProperties = new Set(UI_CUSTOM_PROPERTIES)
const definedProperties = new Set()
const definitionsByPackage = new Map()
for (const packageName of packageNames) {
  const packageDefinitions = new Set()
  const files = (
    await walk(path.join(packagesRoot, packageName, 'src'))
  ).filter((file) => /\.(?:js|css)$/.test(file))
  for (const file of files) {
    const source = await readFile(file, 'utf8')
    for (const property of source.match(/--ui-[a-z0-9-]+/g) || []) {
      if (!publicProperties.has(property))
        failures.push(`${file}: ${property} 未登记在 @tp-ui/contracts`)
    }
    for (const declaration of source.matchAll(/(--ui-[a-z0-9-]+)\s*:/g)) {
      definedProperties.add(declaration[1])
      packageDefinitions.add(declaration[1])
    }
  }
  definitionsByPackage.set(packageName, packageDefinitions)
}
for (const property of publicProperties) {
  if (!definedProperties.has(property)) {
    failures.push(
      `@tp-ui/contracts: ${property} 缺少资源包 fallback 或材质定义`
    )
  }
}
for (const packageName of packageNames.filter((name) =>
  name.startsWith('ui-material-')
)) {
  const definitions = definitionsByPackage.get(packageName)
  for (const property of MATERIAL_CUSTOM_PROPERTIES) {
    if (!definitions.has(property)) {
      failures.push(`packages/${packageName}: ${property} 缺少材质实现`)
    }
  }
}

const viteSource = await readFile(path.resolve('vite.config.mjs'), 'utf8')
if (/@tp-ui\/[^'"\s]+['"]\s*:\s*resolve\(['"]\.\/packages\//.test(viteSource)) {
  failures.push('vite.config.mjs: 生产不得将 @tp-ui 别名指向 packages/*/src')
}
for (const file of (await walk(path.resolve('src'))).filter((entry) =>
  /\.(?:js|vue|css|scss)$/.test(entry)
)) {
  const source = await readFile(file, 'utf8')
  if (/from\s+['"]@tp-ui\/[^'"]+\/src\//.test(source))
    failures.push(`${file}: 应用禁止深层导入资源包实现`)
  if (/\.(?:tp-ui-[a-z0-9_-]+)/.test(source))
    failures.push(`${file}: 应用样式禁止选择资源包内部类名`)
  if (file.endsWith('.scss') && /@import\s+(?!url\s*\()/.test(source))
    failures.push(`${file}: Sass 禁止新增 @import，请使用 @use 或 JS CSS 入口`)
}

if (failures.length) throw new Error(failures.join('\n'))
process.stdout.write(
  `PASS architecture boundaries (${packageNames.length} packages)\n`
)
