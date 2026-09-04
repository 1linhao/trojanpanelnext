import { cp, mkdir, rm } from 'node:fs/promises'
import path from 'node:path'
import { buildUiPackages } from './build-ui-packages.mjs'

await buildUiPackages()
const output = path.resolve('dist/ui-labs')
await rm(output, { recursive: true, force: true })
await mkdir(output, { recursive: true })
await cp(path.resolve('examples'), path.join(output, 'examples'), {
  recursive: true
})
for (const packageName of [
  'ui-contracts',
  'ui-components-vue2',
  'ui-material-frosted',
  'ui-material-flat-test',
  'ui-layout-app-shell-vue2',
  'ui-icons',
  'ui-motion-native'
]) {
  await cp(
    path.resolve('packages', packageName, 'dist'),
    path.join(output, 'packages', packageName, 'dist'),
    { recursive: true }
  )
}
await mkdir(path.join(output, 'vendor'), { recursive: true })
await cp(
  path.resolve('node_modules/vue/dist/vue.js'),
  path.join(output, 'vendor/vue.js')
)
process.stdout.write(`Built UI labs at ${output}\n`)
