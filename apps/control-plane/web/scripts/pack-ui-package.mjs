import { cp, mkdir, mkdtemp, readFile, rm } from 'node:fs/promises'
import { spawnSync } from 'node:child_process'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const packageDirectory = path.resolve(process.argv[2] || process.cwd())
const workspaceRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const temporaryDirectory = await mkdtemp(path.join(os.tmpdir(), 'tp-ui-pack-'))
try {
  const manifest = JSON.parse(
    await readFile(path.join(packageDirectory, 'package.json'), 'utf8')
  )
  const result = spawnSync(
    'npm',
    ['pack', '--json', '--pack-destination', temporaryDirectory],
    {
      cwd: packageDirectory,
      encoding: 'utf8'
    }
  )
  if (result.status !== 0) throw new Error(result.stderr || 'npm pack failed')
  const parsed = JSON.parse(result.stdout)
  const packed = Array.isArray(parsed)
    ? parsed[0]
    : parsed[manifest.name] || Object.values(parsed)[0]
  const files = new Set(packed.files.map((entry) => entry.path))
  const exportTargets = Object.values(manifest.exports).flatMap((entry) =>
    typeof entry === 'string'
      ? [entry]
      : Object.values(entry).filter((value) => typeof value === 'string')
  )
  for (const target of exportTargets) {
    const packedPath = target.replace(/^\.\//, '')
    if (!files.has(packedPath))
      throw new Error(`${manifest.name}: packed tarball misses ${packedPath}`)
  }
  const extractionDirectory = path.join(temporaryDirectory, 'consumer')
  await mkdir(extractionDirectory)
  const archive = path.join(temporaryDirectory, packed.filename)
  const extraction = spawnSync('tar', [
    '-xzf',
    archive,
    '-C',
    extractionDirectory
  ])
  if (extraction.status !== 0)
    throw new Error(`${manifest.name}: cannot unpack tarball`)
  for (const dependency of Object.keys(manifest.dependencies || {}).filter((name) => name.startsWith('@tp-ui/'))) {
    const dependencyDirectory = path.join(workspaceRoot, 'packages', dependency.replace('@tp-ui/', 'ui-'))
    const target = path.join(extractionDirectory, 'package', 'node_modules', ...dependency.split('/'))
    await mkdir(path.dirname(target), { recursive: true })
    await cp(dependencyDirectory, target, { recursive: true })
  }
  const rootExport = manifest.exports['.']
  const rootTarget =
    typeof rootExport === 'string' ? rootExport : rootExport.import
  await import(
    `${pathToFileURL(
      path.join(extractionDirectory, 'package', rootTarget)
    )}?pack-smoke`
  )
  process.stdout.write(
    `PASS pack smoke ${manifest.name} (${packed.files.length} files)\n`
  )
} finally {
  await rm(temporaryDirectory, { recursive: true, force: true })
}
