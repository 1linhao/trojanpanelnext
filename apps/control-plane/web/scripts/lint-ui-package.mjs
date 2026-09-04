import { spawnSync } from 'node:child_process'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const packageDirectory = path.resolve(process.argv[2] || '.')
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')

function run(binary, args) {
  const result = spawnSync(binary, args, { cwd: root, stdio: 'inherit' })
  if (result.status !== 0) process.exit(result.status || 1)
}

run(path.join(root, 'node_modules/.bin/eslint'), [
  '--no-ignore',
  '--ext',
  '.js',
  path.join(packageDirectory, 'src')
])
run(path.join(root, 'node_modules/.bin/stylelint'), [
  `${path.join(packageDirectory, 'src')}/**/*.css`,
  '--config',
  path.join(root, 'stylelint.config.cjs'),
  '--allow-empty-input'
])
