# TrojanPanel Next Documentation

[简体中文](README.md) | English

TrojanPanel Next is a multi-user Web administration panel for Xray, Hysteria2, and NaiveProxy.

## Installation

The current version is `0.1.0-rc.7`. Preinstall Docker Engine, [mikefarah/yq v4](https://github.com/mikefarah/yq), Bash, curl, CA certificates, grep, OpenSSL, tar, coreutils, findutils, and awk, then start Docker. Node hosts must run systemd. The installer does not install dependencies. The [installer guide](https://github.com/1linhao/trojanpanelnext/blob/main/deploy/installer/README_EN.md) includes complete Debian/Ubuntu commands and manual checksum-verified yq installation instructions.

Download the command entrypoint on each server:

```bash
curl -fsSL --connect-timeout 10 --max-time 60 \
  https://raw.githubusercontent.com/1linhao/trojanpanelnext/v0.1.0-rc.7/deploy/installer/tp.sh \
  -o tp.sh
chmod +x tp.sh
./tp.sh --version
```

The entrypoint downloads `common.sh` and the selected command from the same release; installation also downloads `uninstall.sh`. No local checkout is needed. Deploy Web first:

```bash
./tp.sh config web --output ./web.yaml
nano web.yaml
./tp.sh validate --config ./web.yaml
sudo ./tp.sh install --config ./web.yaml
```

Set the domain and email, and prepare DNS and ports. Generated database/Redis passwords are written back to `web.yaml`. Visit `https://<hostname>`; the initial account is `sysadmin` / `123456`. Change the password after signing in.

Create a node server in Web first and obtain its actual ID (at least `1`). Securely copy Web's public `client-ca.crt` to Node's `pki_bundle_dir` (default `/tpdata/trojanpanelnext-pki`), then deploy Node:

```bash
./tp.sh config node --output ./node.yaml
nano node.yaml
./tp.sh validate --config ./node.yaml
sudo ./tp.sh install --config ./node.yaml
```

Set the Node domain, Web database/Redis addresses and credentials, and actual `node_server_id`. Keep private keys and the Web client identity on Web. Node installation also provisions `trojanpanelnext-host.service`; its maintenance endpoint uses mTLS HTTPS at `grpc_port + 1` (default `8101`). Allow only Web sources to gRPC (default `8100`) and maintenance ports. Node must also reach Web HTTPS (TCP 443) to confirm removal results. The Agent has no `docker.sock` mount. `validate` checks only YAML and fields. See the [full installer guide](https://github.com/1linhao/trojanpanelnext/blob/main/deploy/installer/README_EN.md) for network and configuration preparation.

## Recreate containers and remove services

To upgrade, update application image tags in existing YAML to `0.1.0-rc.7`, update every Node first, and then Web. Forced Node installation provisions the host maintenance service:

```bash
sudo ./tp.sh install --config ./node.yaml --force
sudo ./tp.sh install --config ./web.yaml --force
```

`--force` recreates application and Caddy containers while retaining existing MariaDB and Redis containers.

Uninstall on the corresponding host while retaining data:

```bash
sudo ./tp.sh remove --config ./node.yaml --keep-data
sudo ./tp.sh remove --config ./web.yaml --keep-data
```

Uninstall completely (`--purge` is an alias for `--purge-data`):

```bash
sudo ./tp.sh remove --config ./node.yaml --purge-data
sudo ./tp.sh remove --config ./web.yaml --purge-data
```

Both modes remove the corresponding containers, anonymous volumes, and all local tags in their image repositories, including historical tags in the default GHCR repositories. Images still referenced by other containers are retained with a message; no global prune runs. Ordinary removal retains service data, PKI, camouflage content, runtime, and YAML. Full removal also deletes all project service directories, these paths, and YAML. Full removal is refused while project containers for the other purpose remain on the same host. Both modes ultimately remove the installed host maintenance service and working copies. The manually downloaded `tp.sh` and Docker Engine remain.

Deleting a node server in Web first uninstalls it remotely; after success, Web transactionally deletes the server and associated proxy configurations while retaining service data and traffic/kernel task history. Deletion with data purge also removes data and the corresponding Web history. Offline nodes, missing older services, or removal failure produce an error and retain registration. Maintenance TLS files remain until both sides confirm the result. Result confirmation and maintenance service cleanup retry in the background on failure, resume after restart, and report `cleanupPending`. Keep maintenance connectivity and Node access to Web HTTPS (TCP 443) available until cleanup completes. Deleting a single proxy node only deletes that proxy. Local `remove` does not clean up Web registration; initiate server deletion in Web to coordinate both operations.

`TP_SCRIPT_REF` selects the script source (default `v0.1.0-rc.7`), and templates follow that ref. `TP_CONFIG_REF` can override only templates. Direct scripts require matching adjacent `common.sh` and no command prefix. See the [installer guide](https://github.com/1linhao/trojanpanelnext/blob/main/deploy/installer/README_EN.md) for complete parameters.

## Support

[Original TrojanPanel project on GitHub](https://github.com/trojanpanel)
