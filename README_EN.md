# TrojanPanel Next

[简体中文](README.md) | English

TrojanPanel Next is a multi-user Web administration panel supporting Xray, Hysteria2, and NaiveProxy. It provides account and node management, dashboards, certificate management, and distributed deployment.

| Component | Description |
| --- | --- |
| `apps/control-plane/api` | Control-plane API |
| `apps/control-plane/web` | Web administration interface |
| `apps/node-agent` | Node Agent and proxy runtime management |
| `deploy/installer` | Installation and deployment tool |
| `apps/docs-site` | User and installation documentation |

## Get started

The current version is `0.1.0-rc.8`. First follow the [dependency instructions](deploy/installer/README_EN.md#system-and-software-dependencies) to install Docker Engine, mikefarah/yq v4, and the other tools, then start Docker. The installer does not install dependencies. Node hosts also require systemd.

Download the command entrypoint on each Web or Node server:

```bash
curl -fsSL --connect-timeout 10 --max-time 60 \
  https://raw.githubusercontent.com/1linhao/trojanpanelnext/v0.1.0-rc.8/deploy/installer/tp.sh \
  -o tp.sh
chmod +x tp.sh
./tp.sh --version
```

The entrypoint downloads the required scripts from the matching release. Deploy Web first:

```bash
./tp.sh config web --output ./web.yaml
nano web.yaml
./tp.sh validate --config ./web.yaml
sudo ./tp.sh install --config ./web.yaml
```

Set the domain and email, and prepare DNS and firewall rules. Generated database and Redis passwords are written back to the YAML. Visit `https://<hostname>`; the initial account is `sysadmin` / `123456`. Change the password after signing in.

Create a node server in Web, record its actual ID (at least `1`), and securely copy Web's public `client-ca.crt` to Node's `pki_bundle_dir` (default `/tpdata/trojanpanelnext-pki`). Deploy on the Node host:

```bash
./tp.sh config node --output ./node.yaml
nano node.yaml
./tp.sh validate --config ./node.yaml
sudo ./tp.sh install --config ./node.yaml
```

Set the Node domain, Web database/Redis addresses and credentials, and actual `node_server_id`. Copy only the public CA to Node; keep private keys and the Web client identity on Web. Restrict Node's gRPC port (default `8100`) and host maintenance HTTPS port (`grpc_port + 1`, default `8101`) to Web sources. Node must also reach Web HTTPS (TCP 443) to confirm removal results. See the [full installation guide](deploy/installer/README_EN.md) for network preparation. `validate` checks only YAML and fields.

Recreate application containers after updating their image configuration. When upgrading to this version, update all Nodes before Web:

```bash
sudo ./tp.sh install --config ./node.yaml --force
sudo ./tp.sh install --config ./web.yaml --force
```

Uninstall while retaining data (on the corresponding host):

```bash
sudo ./tp.sh remove --config ./node.yaml --keep-data
sudo ./tp.sh remove --config ./web.yaml --keep-data
```

Uninstall completely, deleting data, PKI, camouflage content, custom kernel runtime, and deployment YAML (`--purge` is an alias for `--purge-data`):

```bash
sudo ./tp.sh remove --config ./node.yaml --purge-data
sudo ./tp.sh remove --config ./web.yaml --purge-data
```

Both modes remove the corresponding containers, anonymous volumes, and all local tags in the image repositories they use, including historical tags in the default GHCR repositories. Shared images used by other containers are retained with a message; no global prune runs. Full removal is refused while containers for the other deployment purpose remain on the same host. Both modes ultimately remove the installed host maintenance service. The downloaded `tp.sh` and Docker Engine remain.

Local removal does not remove Web registration; initiate deletion directly on Web's node server page to coordinate Node removal and registration cleanup.

The server delete button opens a dialog with Cancel (取消), Delete (删除), and Delete completely (彻底删除). Delete retains data and Web history; Delete completely also removes the corresponding history. Offline nodes, older versions without the maintenance service, or removal failure retain registration and return an error. Deleting one proxy node only deletes that proxy. See the [removal guide](deploy/installer/README_EN.md#removal) for details.

[Web administration interface](apps/control-plane/web/README_EN.md)

[Node Agent](apps/node-agent/README_EN.md)

[Documentation source](apps/docs-site/vpress/README_EN.md)

## Project origin

TrojanPanel Next is an independently maintained continuation of the [original TrojanPanel project](https://github.com/trojanpanel), not an official release from the original organization. See [NOTICE.md](NOTICE.md) for project origin and history information.
