# TrojanPanel Next Web UI

[简体中文](README.md) | English

The responsive Web administration interface for TrojanPanel Next provides administrator and user views with a shared frosted-glass theme on desktop and mobile browsers.

## Features

- Manage users, node servers, Xray / Hysteria2 / NaiveProxy proxies, and subscriptions.
- View account and server traffic and manage Xray and Hysteria2 kernel tasks.
- Choose Delete (Web records only), Uninstall (retain Node data), or Uninstall completely in the server removal dialog; Delete also works for offline servers.
- Follow browser light/dark preferences and select blue, violet, emerald, or amber palettes.
- Use shared desktop navigation, mobile navigation, tables, forms, dialogs, and loading states.

## Deployment

Install Web through the [v1.0.2-rc.4 script entrypoint](../../../scripts/README_EN.md). See the [deployment guide](../../../docs/deployment_EN.md#web) for complete instructions and [image updates](../../../docs/deployment_EN.md#updates) for existing deployments.

## Interface

![Login page](docs/screenshots/login.png)

![Nodes page](docs/screenshots/nodes.png)

![Profile page](docs/screenshots/profile.png)

## Local development

Use Node.js `^20.19.0 || >=22.12.0` and Yarn Classic 1.22. Run from this directory:

```bash
npx --yes yarn@1.22.22 install --frozen-lockfile
npm run serve
```

The development URL is `http://127.0.0.1:8888/`. API requests are proxied to `http://127.0.0.1:8081/` by default. The [development guide](../../../docs/development_EN.md) covers Vue 2.7, Vite, UI packages, mock API, and browser tests.

## Build

```bash
npm run build
```

Output is written to `dist/` and can be served by the project Web image or Nginx. `VITE_BASE_API` sets the client API prefix and defaults to `/api`.
