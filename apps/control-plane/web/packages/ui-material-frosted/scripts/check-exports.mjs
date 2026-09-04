import { checkPackageExports } from '../../../scripts/check-ui-exports.mjs'
await checkPackageExports(new URL('..', import.meta.url).pathname)
