import { access, readFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

function collectExportTargets(exportsField) {
  return Object.values(exportsField).flatMap((entry) => {
    if (typeof entry === 'string') return [entry]
    return Object.values(entry).filter((value) => typeof value === 'string')
  })
}

export async function checkPackageExports(packageDirectory) {
  const root = path.resolve(packageDirectory)
  const manifest = JSON.parse(
    await readFile(path.join(root, 'package.json'), 'utf8')
  )
  const targets = collectExportTargets(manifest.exports || {})
  if (!targets.length)
    throw new Error(`${manifest.name}: package.json#exports 不能为空`)
  for (const target of targets) await access(path.join(root, target))
  const rootEntry = manifest.exports['.']
  const importTarget =
    typeof rootEntry === 'string' ? rootEntry : rootEntry.import
  await import(pathToFileURL(path.join(root, importTarget)))
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  await checkPackageExports(process.argv[2] || process.cwd())
}
