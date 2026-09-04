import { readdir } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { buildPackage } from './build-ui-package.mjs'

export async function buildUiPackages() {
  const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
  const packagesRoot = path.join(root, 'packages')
  const packageDirectories = (
    await readdir(packagesRoot, { withFileTypes: true })
  )
    .filter((entry) => entry.isDirectory())
    .map((entry) => path.join(packagesRoot, entry.name))
    .sort()

  for (const packageDirectory of packageDirectories) {
    await buildPackage(packageDirectory)
  }

  process.stdout.write(`PASS built ${packageDirectories.length} UI packages\n`)
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  await buildUiPackages()
}
