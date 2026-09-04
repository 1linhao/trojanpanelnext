import { cp, mkdir, rm } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

export async function buildPackage(packageDirectory) {
  const root = path.resolve(packageDirectory)
  const source = path.join(root, 'src')
  const destination = path.join(root, 'dist')
  await rm(destination, { recursive: true, force: true })
  await mkdir(destination, { recursive: true })
  await cp(source, destination, { recursive: true })
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  await buildPackage(process.argv[2] || process.cwd())
}
