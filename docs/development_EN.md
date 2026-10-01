# Development guide

[Documentation index](README_EN.md) · [简体中文](development.md)

## Contents

- [Source layout](#source-layout)
- [API and Node Agent](#go-services)
- [Web UI](#web-ui)
- [Browser tests](#browser-tests)
- [Documentation site](#documentation-site)
- [Script library](#script-library)

<a id="source-layout"></a>
## Source layout

| Path | Content |
| --- | --- |
| `apps/control-plane/api/` | Web API, business data, and mTLS identity |
| `apps/control-plane/web/` | Vue 2.7 / Vite Web UI |
| `apps/control-plane/web/packages/` | Internal UI contracts, components, materials, layouts, icons, and motion |
| `apps/control-plane/web/examples/` | Runnable UI composition examples and automated checks |
| `apps/node-agent/` | Node Agent, proxy runtimes, and host maintenance |
| `apps/docs-site/` | VuePress documentation site |
| `scripts/` | User deployment entrypoint, commands, and templates |
| `tests/deploy/` | Deployment script tests |
| `tools/` | Repository, documentation, and release checks |
| `docs/` | Current deployment, operation, certificate, API, and development documentation |

Keep test artifacts, temporary deployment configuration, and investigation notes in the ignored `.local/` directory. Generated Web, UI package, and documentation output is not committed.

<a id="go-services"></a>
## API and Node Agent

CI uses Go 1.23. Run in each of `apps/control-plane/api/` and `apps/node-agent/`:

```bash
go test ./...
go build ./...
```

API server-removal integration tests require a disposable MySQL/MariaDB instance. Set `TP_REMOVAL_TEST_DSN` in the current shell, then run in `apps/control-plane/api/`:

```bash
go test ./dao -run '^TestNodeRemovalIntegration$' -count=1
```

The DSN user needs permission to create and drop test databases. Each subtest uses and cleans up its own database, leaving the database named in the DSN unchanged. Tests skip when the variable is unset. These DAO tests share a connection and must not run in parallel. Coverage includes Web-only deletion, both remote uninstallation data modes, transaction rollback, repeated deletion, and isolation of shared records across servers.

The host maintenance entrypoint is `apps/node-agent/cmd/host-agent/`. Running services requires databases, Redis, configuration, and certificates; see the [deployment guide](deployment_EN.md).

<a id="web-ui"></a>
## Web UI

Use Node.js `^20.19.0 || >=22.12.0` (CI uses Node 22) and Yarn Classic 1.22. Run in `apps/control-plane/web/`:

```bash
npx --yes yarn@1.22.22 install --frozen-lockfile
npm run serve
```

The default URL is `http://127.0.0.1:8888/`, with API requests proxied to `http://127.0.0.1:8081/`. To use mock data, start the API in one terminal and the UI in another:

```bash
MOCK_API_PORT=18081 node tests/mock-api-server.js
```

```bash
MOCK_API_TARGET=http://127.0.0.1:18081 npm run serve -- --port 18888
```

`MOCK_API_TARGET` is used only by development and preview servers. `VITE_BASE_API` sets the browser API prefix and defaults to `/api`.

```bash
npm run lint -- --no-fix
npm run test:ui-libraries
npm run test:ui-cleanup
npm run test:vite-proxy
npm run build
```

`serve` and `build` build internal UI packages first. The Web output is written to `dist/`. Private workspace package versions follow their own `package.json` and are independent of product release tags. `yarn.lock` is the dependency lock source.

<a id="browser-tests"></a>
## Browser tests

Install Chromium and ChromeDriver before running browser checks.

`npm run test:server-delete:e2e` starts isolated mock API, UI, and ChromeDriver services using ports `18081`, `18082`, `18888`, and `9518`. It checks Cancel, Delete, Uninstall, and Uninstall completely; API dispatch for Web-only deletion and remote uninstallation; cancellation without a request; failed offline uninstallation retaining the row followed by explicit deletion; and desktop/mobile layout. Artifacts go to `.local/server-delete-dialog/`.

This browser test uses a mock API request recorder. It verifies UI behavior and request parameters without performing real Node host uninstallation or SQL cleanup. API `dao/node_removal_integration_test.go` covers transactional cleanup, protocol configuration removal, and isolation of other servers’ data; see [API and Node Agent](#go-services) for running it.

`npm run test:live-stack:e2e` uses the mock API and UI at port `18888` to check login, settings, subscription templates, and mobile navigation. Preview a production build with `MOCK_API_TARGET=http://127.0.0.1:18081 npm run preview -- --port 18889`, then run the same test with `LIVE_WEB_URL=http://127.0.0.1:18889`.

`npm run build:ui-labs` builds the examples in `examples/`. Run it after the main build to retain both in `dist/`, because the main build clears the directory. `npm run test:ui-labs:e2e` checks component, material, and layout composition.

<a id="documentation-site"></a>
## Documentation site

Run in `apps/docs-site/`:

```bash
npx --yes pnpm@8.15.9 install --frozen-lockfile
NODE_OPTIONS=--openssl-legacy-provider npx --yes pnpm@8.15.9 run docs:build
```

VuePress 1 needs this OpenSSL setting; the Vite Web build does not. Output is written to `apps/docs-site/docs/`. The maintained source for complete deployment instructions is `docs/deployment_EN.md`.

<a id="script-library"></a>
## Script library

User commands live in `scripts/`; see the [library reference](../scripts/README_EN.md). Run from the repository root:

```bash
node tools/check-readme-languages.mjs
node tools/check-installer-release.mjs
bash tests/deploy/installer_cli_test.sh
bash tests/deploy/entrypoint_test.sh
bash tests/deploy/update_test.sh
bash tests/deploy/uninstall_test.sh
bash tests/deploy/persistence_test.sh
bash tests/deploy/external_certificate_test.sh
bash tests/deploy/quick_deploy_test.sh
```

The tests use Bash, Docker test doubles, and mikefarah/yq v4. Run ShellCheck against Bash files in `scripts/` and `tests/deploy/`. Release tags, script declarations, templates, and product images must use the same release version.
