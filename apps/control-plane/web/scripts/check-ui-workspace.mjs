import { spawnSync } from 'node:child_process'
import { readFile, readdir } from 'node:fs/promises'
import path from 'node:path'

function run(command, args, cwd = process.cwd()) {
  const result = spawnSync(command, args, {
    cwd,
    stdio: 'inherit',
    env: process.env
  })
  if (result.status !== 0)
    throw new Error(`${command} ${args.join(' ')} 执行失败`)
}

const packageRoot = path.resolve('packages')
const packageDirectories = (await readdir(packageRoot, { withFileTypes: true }))
  .filter((entry) => entry.isDirectory())
  .map((entry) => path.join(packageRoot, entry.name))
  .sort()

const packages = await Promise.all(
  packageDirectories.map(async (directory) => {
    const manifest = JSON.parse(
      await readFile(path.join(directory, 'package.json'), 'utf8')
    )
    return {
      directory,
      name: manifest.name,
      dependencies: Object.keys(manifest.dependencies || {}).filter((name) =>
        name.startsWith('@tp-ui/')
      )
    }
  })
)
const packagesByName = new Map(packages.map((entry) => [entry.name, entry]))
const orderedPackages = []
const visiting = new Set()
const visited = new Set()

function visit(entry, trail = []) {
  if (visiting.has(entry.name)) {
    throw new Error(
      `UI package dependency cycle: ${[...trail, entry.name].join(' -> ')}`
    )
  }
  if (visited.has(entry.name)) return
  visiting.add(entry.name)
  for (const dependencyName of entry.dependencies) {
    const dependency = packagesByName.get(dependencyName)
    if (dependency) visit(dependency, [...trail, entry.name])
  }
  visiting.delete(entry.name)
  visited.add(entry.name)
  orderedPackages.push(entry)
}

for (const entry of packages) visit(entry)
for (const { directory: packageDirectory } of orderedPackages) {
  run('npm', ['run', 'check'], packageDirectory)
}
run(process.execPath, ['scripts/check-ui-architecture.mjs'])
run(process.execPath, ['scripts/build-ui-labs.mjs'])
process.stdout.write('PASS composable UI workspace\n')
