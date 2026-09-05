# TrojanPanel Next Installer

[简体中文](README.md) | English

The installer deploys a server non-interactively with one script, one YAML file, and one explicit server purpose.

## Purpose modes

| Mode | Purpose | Services |
| --- | --- | --- |
| `web` | Web control plane | API, Web UI, MariaDB, Redis, and Caddy |
| `node` | Node Agent | Node Agent, proxy runtimes, certificates, and camouflage site |

Install the Web control plane first. A Node Agent uses the MariaDB and Redis credentials generated in the Web configuration.

## Requirements

| Item | Requirement |
| --- | --- |
| OS | Ubuntu 20.04+, Debian 11+, or a comparable systemd Linux |
| Privileges | Installation and removal require `root` |
| CPU | `linux/amd64` or `linux/arm64` |
| Memory | At least 1 GiB |
| Network | DNS points to the target host and configured ports are open |

## Install the Web control plane

```bash
cp examples/web.yaml ./web.yaml
chmod 600 ./web.yaml
```

Set `hostname` and `email`, then validate and install:

```bash
./install.sh validate --mode web --config ./web.yaml
sudo ./install.sh install --mode web --config ./web.yaml
```

The first installation generates MariaDB and Redis passwords, writes them back to `web.yaml`, and changes its permissions to `600`. It also creates the control-plane mTLS identity in `pki_bundle_dir`; the CA private key stays on the Web control plane.

Before installing a Node Agent, transfer `/tpdata/trojanpanelnext-pki/client-ca.crt` from the Web control plane to the same path on the Node through a trusted file-transfer or secret-management channel. Copy only the public CA certificate; never copy `client-ca.key`, `client.key`, or `client.crt`.

## Install a Node Agent

```bash
cp examples/node-agent.yaml ./node-agent.yaml
chmod 600 ./node-agent.yaml
```

Set the node hostname, Web control-plane address, and the database and Redis passwords from the Web configuration. Confirm that the public CA certificate is present in `pki_bundle_dir`:

```bash
./install.sh validate --mode node --config ./node-agent.yaml
sudo ./install.sh install --mode node --config ./node-agent.yaml
```

`--mode` must match `trojanpanelnext.purpose`; the installer exits immediately when they differ.

## Recreate or remove

```bash
sudo ./install.sh install --mode web --config ./web.yaml --force
sudo ./install.sh remove --mode web --config ./web.yaml
sudo ./install.sh remove --mode node --config ./node-agent.yaml --purge-data
```

The first remove command keeps generated data. `--purge-data` deletes it.

## Configuration files

[Web control-plane template](examples/web.yaml) contains the hostname, images, ports, mTLS identity directory, and internal credentials.

[Node Agent template](examples/node-agent.yaml) contains the node hostname, control-plane database and Redis connections, gRPC settings, public CA directory, and certificate paths.

Passwords are never printed. Treat populated configuration files as secrets and do not commit them to Git.

## Support

Project origin: [the original TrojanPanel project](https://github.com/trojanpanel).
