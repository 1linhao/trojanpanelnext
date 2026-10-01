# TrojanPanel Next v1.0.2-rc.1 deployment guide

[简体中文](deployment.md) | English

## Contents

- [System and software dependencies](#dependencies)
  - [Install dependencies](#dependency-install)
  - [Prepare dependencies manually](#dependency-manual)
  - [Remove dependencies](#dependency-removal)
- [Release binding and entrypoint](#versions)
- [DNS, ports, and networking](#network)
- [One-command Web installation](#web)
- [Register a node server](#node-registration)
- [One-command Node installation](#node)
- [Existing Node certificates](#external-certificates)
- [Configuration deployment and fields](#configuration)
  - [Download and edit templates](#configuration-download)
  - [Minimum Web edits](#configuration-web-minimum)
  - [Minimum Node edits](#configuration-node-minimum)
  - [Prepare Node public CA](#configuration-node-ca)
  - [Validate and install](#configuration-validation)
  - [Configuration field reference](#configuration-fields)
- [Recreate current-release services](#recreate)
- [Local removal](#removal)
- [Delete a node server from Web](#web-removal)
- [Reconnect after deletion](#reconnect)
- [Automatic certificate maintenance](#certificate-maintenance)
- [Operations](#operations)
- [Troubleshooting](#troubleshooting)

<a id="dependencies"></a>
## System and software dependencies

Linux `amd64` and `arm64` are supported. Run commands in Bash; installation, recreation, and removal require root. Node hosts must run systemd. At least 1 GiB of memory per server is recommended. Web and Node can run on separate servers connected through controlled network access.

The `web`, `node`, and `install` deployment commands only check dependencies. Run `deps install` separately to prepare the required software, or install it manually:

| Operation | Dependencies |
| --- | --- |
| Entrypoint, help, template download | Bash, curl, CA certificates, grep, coreutils |
| Automatic dependency installation and removal | Entrypoint dependencies, util-linux (`flock`), root, supported Debian/Ubuntu, systemd, apt-get, dpkg, dpkg-query |
| One-command deployment and validation | Entrypoint dependencies and **mikefarah/yq v4** |
| Installation and recreation | Entrypoint dependencies, yq, a running Docker Engine, OpenSSL, tar, findutils, awk; Node also needs systemd |
| Removal | Bash, curl, CA certificates, grep, coreutils including realpath/rmdir, Docker Engine, yq; Node maintenance cleanup needs systemctl |

<a id="dependency-install"></a>
### Install dependencies

Supported systems are **Debian 12/13 and Ubuntu 22.04/24.04** on `amd64` or `arm64`, with root access and running systemd. Both Web and Node hosts can use this command. Follow the [manual instructions](#dependency-manual) for other Linux distributions.

Run in a root Bash session:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) deps install
```

The command prepares Bash, curl, CA certificates, grep, coreutils, OpenSSL, tar, findutils, awk (installing gawk if awk is missing), and the distribution's Docker Engine packages (`docker.io`, `containerd`, and `runc`; Debian 13 also needs `docker-cli`), then starts Docker. If compatible yq is missing, it downloads the architecture-specific **mikefarah/yq v4.53.6** binary and installs it after SHA256 verification. Existing compatible Docker and mikefarah/yq v4 are reused without replacing the original tools.

The entrypoint itself needs Bash, curl, CA certificates, grep, and coreutils; `deps` also needs `flock` from util-linux to prevent concurrent operations. If these minimal tools are missing, bootstrap them through apt in a root Bash session:

```bash
apt-get update
apt-get install -y bash curl ca-certificates grep coreutils util-linux
```

Dependency commands use the same release entrypoint as deployment commands. Select a release explicitly with:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) --version 1.0.2-rc.1 deps install
```

Installation records are stored in `/var/lib/trojanpanelnext-dependencies` (directory mode `0700`). They track Docker packages added by this command and the installed yq file's SHA256 for [dependency removal](#dependency-removal). Repeated installation checks and fills missing dependencies. If installation was interrupted, rerun `deps install` to repair it before removal.

<a id="dependency-manual"></a>
### Prepare dependencies manually

On other Linux distributions, prepare dependencies using the distribution's package manager. Debian/Ubuntu dependencies can also be prepared manually:

```bash
apt-get update
apt-get install -y bash curl ca-certificates grep coreutils openssl tar findutils gawk docker.io
systemctl enable --now docker
```

Debian 13 also requires `docker-cli`. Alternatively, follow the [Docker Engine installation guide](https://docs.docker.com/engine/install/). Install the architecture-appropriate v4 binary following [mikefarah/yq installation instructions](https://github.com/mikefarah/yq#install) and verify the release download. The Python package named `yq` is incompatible. Manually installed software is outside the scope of `deps remove`.

Check before deployment:

```bash
yq --version
docker info
openssl version
```

`yq --version` must identify mikefarah/yq v4; `docker info` must connect to the daemon. The one-command examples below run in a root Bash session and download scripts from a fixed release tag.

<a id="dependency-removal"></a>
### Remove dependencies

First [remove the project](#removal) on that host and remove containers belonging to other Docker services. After deleting a Node from Web, also wait for host maintenance cleanup. Then run:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) deps remove
```

| Target | Behavior |
| --- | --- |
| Docker packages added by `deps install` and listed in the record | Removed; Docker removal is refused if any Docker container exists (including stopped containers), shared containerd has containers in other namespaces, Node maintenance remains, or runtime state cannot be verified |
| yq installed by this command | Removed only if its current SHA256 matches the record; a replaced or modified file remains |
| Pre-existing Docker/yq and basic tools such as Bash, curl, CA certificates, and OpenSSL | Retained |
| Docker data such as `/var/lib/docker`, retained project data, and YAML | Retained |
| External certificates, Nginx, and Certbot | Retained |

Without an installation record, removal is a no-op. If dependency installation was interrupted, rerun `deps install` to repair it before running `deps remove`. The command does not run `apt autoremove` or global Docker prune, and does not remove software installed through other methods. Removing Docker packages does not erase Docker data. To delete project data, run `remove --purge-data` while Docker is still available. `deps remove` requires the same supported system and minimal entrypoint dependencies as automatic installation.

<a id="versions"></a>
## Release binding and entrypoint

This guide covers pre-release **v1.0.2-rc.1 (Pre-release)**. Version `1.0.2-rc.1` maps to Git tag `v1.0.2-rc.1`, configuration field `trojanpanelnext.release: "1.0.2-rc.1"`, and these product images:

| Component | Image |
| --- | --- |
| Web API | `ghcr.io/1linhao/trojanpanelnext-api:1.0.2-rc.1` |
| Web interface | `ghcr.io/1linhao/trojanpanelnext-web:1.0.2-rc.1` |
| Node Agent | `ghcr.io/1linhao/trojanpanelnext-node-agent:1.0.2-rc.1` |

All product images provide `linux/amd64` and `linux/arm64`. Caddy, MariaDB, and Redis use the separate upstream versions in the templates.

The shared entrypoint, `scripts/tp.sh`, fetches the script library and templates from the selected release tag, checks script versions, and executes the selected command. Temporary downloaded scripts are removed when the command ends. Configuration and product images must match the selected release. Scripts do not convert configuration from other versions; only the current release is maintained.

Select a version explicitly when installing Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) --version 1.0.2-rc.1 web
```

`--version <version>` is accepted before or after the command. `--entry-version` prints the entrypoint's default release. Use `--help` or `<command> --help` for usage. `--version` without a value does not query the version.

| Command | Behavior |
| --- | --- |
| `deps install` | Prepare dependencies on supported systems |
| `deps remove` | Remove added dependencies using the installation record |
| `web [options]` | Prompt for Web YAML, validate, and install |
| `node [options]` | Prompt for Node YAML, validate, and install |
| `config web\|node [--output <file>]` | Download a release-specific configuration template |
| `validate --config <file>` | Validate YAML, release, and fields |
| `install --config <file> [--force]` | Deploy the purpose defined in YAML |
| `remove --config <file> [--keep-data\|--purge-data]` | Remove the purpose defined in YAML |

One-command deployment reads terminal input; database and Redis passwords are not supplied as command-line arguments. `--config` immediately installs using an existing configuration without prompting. Templates and generated configuration use mode `0600`; existing paths and symlinks are never overwritten. `--config` can be combined only with `--force`; set all other values in YAML.

| One-command option | Purpose |
| --- | --- |
| `--hostname <domain>` | Server domain; prompted if omitted |
| `--email <address>` | ACME contact for Web or Node Caddy |
| `--output <file>` | New configuration path; defaults to `./web.yaml` or `./node.yaml` |
| `--config <file>` | Install using completed configuration |
| `--force` | Recreate application containers, retaining service data |
| `--web-host <host>` | Node Web database/Redis address; use YAML for separate addresses |
| `--node-id <id>` | Node server ID already registered in Web |
| `--client-ca <file>` | Local Web public CA file for Node |
| `--certificate-mode caddy\|external` | Node certificate mode; defaults to `caddy` |
| `--certificate <file>`, `--private-key <file>` | Node external fullchain and private key paths |

<a id="network"></a>
## DNS, ports, and networking

Prepare separate domains for Web and Node, such as `panel.example.com` and `node.example.com`, and point A/AAAA records at the respective servers. Published AAAA records require working IPv6 access. Automatic issuance needs publicly reachable validation ports without conflicting listeners.

| Direction | Default port and purpose |
| --- | --- |
| Browser or certificate authority → Web | TCP 80/443: Caddy and HTTPS panel |
| Node → Web | TCP 9507: MariaDB; TCP 6378: Redis; restrict to trusted Node sources |
| Node host maintenance service → Web | TCP 443: removal result confirmation |
| Certificate authority → Node, Caddy mode | TCP 80: HTTP domain validation |
| Web → Node | TCP 8100: mTLS gRPC control; restrict to Web sources |
| Web → Node host maintenance service | TCP 8101: mTLS HTTPS removal; always `grpc_port + 1`, restrict to Web sources |
| Camouflage site visitors → Node, Caddy mode | TCP 8863: Caddy HTTPS |
| Proxy clients → Node | Actual proxy TCP/UDP ports configured in the panel |

API 8081, UI 8888, and Node API 8082 listen using host networking. Restrict access to these internal services instead of exposing every port publicly. Connect MariaDB and Redis through firewall rules or a trusted private network. Configuration does not create VPN connections or firewall rules.

When changing ports, update configuration, Web server registration, and firewall rules together. The maintenance port is not independently configurable; `grpc_port` must be at most `65534`. External certificate mode does not run Node Caddy or use its HTTP/HTTPS ports. Proxies still need available listening ports.

<a id="web"></a>
## One-command Web installation

Run on the Web host:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) web
```

Enter the Web domain and certificate contact email when prompted. Public parameters can also be supplied directly:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) web --hostname panel.example.com --email admin@example.com --output ./web.yaml
```

The default output is `./web.yaml`. First installation generates MariaDB and Redis passwords and writes them to the configuration, creates internal Web mTLS credentials, and deploys MariaDB, Redis, API, UI, and Web Caddy.

Visit `https://panel.example.com`. The initial username is `sysadmin`, with password `123456`; change the password after signing in. Panel credentials are separate from database and Redis passwords in YAML. Existing deployments use the password set for that account.

Back up `web.yaml` and the complete `pki_bundle_dir`. Configuration contains sensitive credentials and should be readable only by administrators; never commit it to a public repository.

<a id="node-registration"></a>
## Register a node server

Sign into Web with the `sysadmin` role and click **Add Node server** on the dashboard, or open **Server management** in the sidebar and click **Add Node server**. This registers the entire Node host. Create individual proxy nodes separately after Node installation. Only the `sysadmin` role can add servers; ordinary users and the `admin` role do not see the add action.

You can also open [Server management](https://panel.example.com/#/server-manage/server-list) directly, replacing `panel.example.com` with the actual Web domain.

Enter a server name (2–20 characters), the Node address reachable from Web, its gRPC port (default `8100`), and TLS server name (the Node certificate domain, such as `node.example.com`). Save and note the actual **Server ID** shown below the server name in each row. Set Node YAML's `node_server_id` to this ID rather than the template example. Offline status is expected before Node is installed; check its online status after installation and connectivity are established.

<a id="node"></a>
## One-command Node installation

Deploy Web first, [register the node server](#node-registration), and record its actual ID, at least `1`.

Securely transfer Web's current public `client-ca.crt` to the Node host, for example `/root/client-ca.crt`. Its default Web location is `/tpdata/trojanpanelnext-pki/client-ca.crt`. Transfer only the public CA. Keep `client-ca.key`, `client.key`, and `client.crt` on Web.

Run on the Node host:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) node
```

Provide the Node domain, Web address, registered server ID, local public CA path, and Web database/Redis credentials. Caddy mode also asks for a certificate email. Administrators can obtain connection passwords from Web YAML; sensitive input is not echoed to the terminal.

Public parameters can be supplied directly, with remaining fields requested interactively:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) node --hostname node.example.com --email admin@example.com --web-host panel.example.com --node-id 1 --client-ca /root/client-ca.crt
```

`--client-ca` points to a public PEM CA already securely transferred to this host. The file must contain only unexpired CA certificates; multiple old/new CAs are accepted during rotation. Leaf certificates, expired CAs, private keys, and other mixed content are rejected before new YAML or CA files are published. After validation, the script copies it to `/tpdata/trojanpanelnext-pki/client-ca.crt`. Omit the option if that default file already exists. The same CA can be reused; a different existing trust file is not overwritten. Follow [certificate maintenance](certificates.md) for trust changes.

The default output is `./node.yaml`. Node needs ongoing access to Web's MariaDB, Redis, and HTTPS. Web needs access to Node control and maintenance ports. Scripts do not register servers or retrieve the CA automatically.

Installation deploys the Agent, Node Caddy in the default certificate mode, and `trojanpanelnext-host.service`. The host maintenance service uses mTLS HTTPS and accepts authorized Web clients for remote removal. The Agent container has no `docker.sock` mount.

After installation, check that the server is online in Web, then create the required Xray, Hysteria2, or NaiveProxy proxies.

<a id="external-certificates"></a>
## Existing Node certificates

If Nginx and Certbot or another tool manage host certificates, issue a valid certificate first and select external mode:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) node --certificate-mode external
```

Enter absolute paths to the full chain and its unencrypted PEM private key. Paths can also be supplied directly:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) node --certificate-mode external --certificate /etc/letsencrypt/live/node.example.com/fullchain.pem --private-key /etc/letsencrypt/live/node.example.com/privkey.pem
```

| Field | Meaning |
| --- | --- |
| `node_certificate_mode: caddy` | Default; run Node Caddy to issue and renew certificates |
| `node_certificate_mode: external` | Read existing PEM files; skip Node Caddy and issuance |
| `node_certificate_path` | Absolute host path to the full certificate chain |
| `node_private_key_path` | Absolute host path to the matching unencrypted PEM private key |

External-mode installation checks readability, validity dates, hostname coverage, and key pairing. Agent, host maintenance, and proxies using default paths share the certificate. If proxies use other domains, the certificate must cover those too, through a multi-domain SAN or wildcard certificate.

Certificate directories and symlink target directories are mounted read-only. Certbot `live/<domain>` → `archive/<domain>` link updates are supported. Use dedicated certificate directories separate from writable mTLS PKI. External certificates must be outside project data, PKI, camouflage content, runtime, and maintenance directories. Broad mounts such as `/`, `/etc`, or `/root` are rejected.

The host manages issuance, renewal, and Nginx reload. Check Certbot webroot renewal with:

```bash
certbot renew --cert-name node.example.com --dry-run --no-random-sleep-on-renew
```

Renewal pre/post hooks must match the validation method; keep Nginx running for webroot validation. If Nginx reads the certificate, configure a deploy hook to validate and reload Nginx.

Multiple TLS services can share a host TCP port using host Nginx stream SNI routing to separate local TCP ports, with distinct domains for each service. The stream module, listeners, TLS termination, and PROXY protocol settings must match the backends. Scripts do not configure SNI routing. TCP and UDP listeners are separate; TCP SNI routing does not replace UDP proxy port planning.

When changing an installed Node's certificate mode, paths, or symlink target directories, use `--force` to recreate mounts. Proxy references matching the previous certificate/key pair are updated, preserving proxy accounts and other settings. Both local removal modes and remote Web removal preserve external certificates, Nginx, Certbot, and their configuration.

<a id="configuration"></a>
## Configuration deployment and fields

For configuration deployment, follow: download a template → edit required fields → prepare Node's CA → validate → install. Dependencies, DNS, and networking still need to be prepared as described above.

<a id="configuration-download"></a>
### Download and edit configuration templates

Download the Web template on the Web host:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) config web --output ./web.yaml
```

Download the Node template on the Node host:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) config node --output ./node.yaml
```

Templates are saved with `0600` permissions. Existing files and symbolic links are never overwritten. Edit the original configuration for an existing deployment; use a different `--output` path if another file is needed for a new deployment.

Use an installed editor, such as `nano ./web.yaml`, `nano ./node.yaml`, `vi ./web.yaml`, or `vi ./node.yaml`. The YAML below shows fields to edit, rather than a replacement for the complete template. Keep the `trojanpanelnext:` mapping and two-space field indentation. Quote strings; within a YAML single-quoted password, write a literal single quote twice, for example `'pass''word'`. Do not commit credentials to a public repository.

<a id="configuration-web-minimum"></a>
### Minimum Web edits

For a first installation using the default ports and directories, edit at least these fields:

| Field inside `trojanpanelnext` | Required value |
| --- | --- |
| `hostname` | A real domain pointing to the Web host, such as `panel.example.com` |
| `email` | A real email address for certificate notices |

Edit the corresponding values in the original template:

```yaml
trojanpanelnext:
  hostname: 'panel.example.com'
  email: 'admin@example.com'
```

Keep the template's `release`, `schema_version`, `purpose`, images, ports, and paths. On the first installation, `mariadb_password` and `redis_password` may remain empty; the installer generates passwords and writes them back to `web.yaml`. Back up the file after installation. Once MariaDB and Redis are initialized, editing YAML alone cannot change their passwords; do not clear the values to generate replacements. Node must use Web's current actual credentials.

After installing and signing into Web, [register the node server](#node-registration) before filling out Node configuration.

<a id="configuration-node-minimum"></a>
### Minimum Node edits

In the default Caddy certificate mode, edit or confirm at least the following fields. Example domains, password placeholders, and ID `1` are not actual deployment settings:

| Field inside `trojanpanelnext` | Required value |
| --- | --- |
| `hostname` | A real domain pointing to the Node host |
| `email` | A real contact email for Caddy certificate issuance |
| `mariadb_host`, `redis_host` | Addresses of Web's database and Redis reachable from Node; these may share a domain or use trusted private addresses |
| `mariadb_password`, `redis_password` | The current actual database and Redis passwords from Web's `web.yaml` |
| `node_server_id` | The actual ID shown after [registration in Web](#node-registration), at least `1` |
| `grpc_tls_server_name` | A domain covered by Node's server certificate, matching the TLS server name registered in Web |

Edit the corresponding values in the original template:

```yaml
trojanpanelnext:
  hostname: 'node.example.com'
  email: 'admin@example.com'
  mariadb_host: 'panel.example.com'
  mariadb_password: 'actual current Web database password'
  redis_host: 'panel.example.com'
  redis_password: 'actual current Web Redis password'
  node_server_id: 3
  grpc_tls_server_name: 'node.example.com'
```

Replace example ID `3` with the ID of the server just registered in Web. Keep `mariadb_user`, `database`, and `account_table` at their defaults unless Web's database actually uses different settings. If Web uses non-default database or Redis ports, update Node's `mariadb_port` and `redis_port` as well. A changed `grpc_port` must match the gRPC port registered in Web and firewall rules; the host maintenance port is always `grpc_port + 1`.

**For existing certificates**, also edit these fields in the Node template:

```yaml
trojanpanelnext:
  node_certificate_mode: 'external'
  node_certificate_path: '/etc/letsencrypt/live/node.example.com/fullchain.pem'
  node_private_key_path: '/etc/letsencrypt/live/node.example.com/privkey.pem'
```

These absolute paths must already exist and be readable. The certificate must cover `hostname` and `grpc_tls_server_name`; its matching PEM private key must be unencrypted. External mode does not require `email` and does not run Node Caddy. All other required Node settings and public CA preparation still apply. See [external certificates](#external-certificates) for directory requirements and renewal.

<a id="configuration-node-ca"></a>
### Prepare Web's public CA before installing Node

From the Web host, securely copy the current public `client-ca.crt` from its configured `pki_bundle_dir` to Node. The default source is `/tpdata/trojanpanelnext-pki/client-ca.crt`. For example, run this on Web, substituting the Node SSH address, user, and port as needed:

```bash
scp /tpdata/trojanpanelnext-pki/client-ca.crt root@node.example.com:/root/client-ca.crt
```

Copy only the public `client-ca.crt`, never `client-ca.key`, `client.key`, or `client.crt`. Then prepare the default bootstrap location on the Node host:

```bash
install -d -m 0700 /tpdata/trojanpanelnext-pki
install -m 0644 /root/client-ca.crt /tpdata/trojanpanelnext-pki/client-ca.crt
```

For a custom Node `pki_bundle_dir`, the destination must be `client-ca.crt` inside that directory. This bootstrap location differs from `grpc_client_ca_path`: the default runtime file is `/tpdata/trojan-panel-core/pki/client-ca.crt`, and the installer copies the bootstrap file there. Keep the template's runtime path. Existing deployments prefer the live trust file; follow [certificate maintenance](certificates.md) for CA rotation rather than overwriting existing trust arbitrarily.

<a id="configuration-validation"></a>
### Validate and install

After editing and preparation, validate on each corresponding host.

Validate Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) validate --config ./web.yaml
```

Install Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) install --config ./web.yaml
```

Validate Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) validate --config ./node.yaml
```

Install Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) install --config ./node.yaml
```

Alternatively, use `web --config ./web.yaml` or `node --config ./node.yaml` to check the purpose and install. `validate` checks YAML, the release, and fields; it does not verify DNS, certificate contents, network connections, or service health. After installation, check Node's status in [Server management](#node-registration).

<a id="configuration-fields"></a>
### Configuration field reference

Complete editable templates: [Web](../scripts/deploy/templates/web.yaml) · [Node](../scripts/deploy/templates/node.yaml). All fields belong to the `trojanpanelnext` mapping:

| Shared field | Description |
| --- | --- |
| `release` | Required string; currently `"1.0.2-rc.1"`, matching the selected scripts |
| `schema_version` | Configuration structure version; currently `1` |
| `purpose` | `web` or `node`; determines deployment and removal targets |
| `hostname`, `email` | Server domain and certificate email; Node external mode does not require email |
| `pki_bundle_dir` | PKI directory; defaults to `/tpdata/trojanpanelnext-pki` |
| `caddy_image` | Caddy image; Node external mode does not start Caddy |
| `force` | `0` or `1`; recreate application containers, normally leave `0` and use `--force` |
| `purge_data` | `0` or `1`; default removal mode without an explicit data option, normally `0` |

| Web field | Description |
| --- | --- |
| `panel_image`, `ui_image` | API and Web images for this release |
| `mariadb_image`, `redis_image` | Upstream database/cache images |
| `mariadb_port`, `redis_port` | Database/cache host ports; defaults `9507`, `6378` |
| `panel_port`, `ui_port` | Internal API/UI ports; defaults `8081`, `8888` |
| `mariadb_password`, `redis_password` | May be empty for first-install generation; retain existing credentials on subsequent installs |
| `grpc_client_cert_path`, `grpc_client_key_path` | Runtime paths for Web's mTLS client identity |
| `grpc_server_ca_path` | Optional CA file for Node server certificates; empty uses system trust |

| Node field | Description |
| --- | --- |
| `core_image` | Node Agent image for this release |
| `mariadb_host`, `mariadb_port`, `mariadb_user`, `mariadb_password` | Web database connection; must match actual Web settings |
| `database`, `account_table` | Database/account table; defaults `trojan_panel_db`, `account` |
| `redis_host`, `redis_port`, `redis_password` | Web Redis connection |
| `node_server_id` | Actual registered Web server ID, at least `1` |
| `grpc_port`, `core_port` | Agent gRPC/API ports; defaults `8100`, `8082` |
| `grpc_tls_mode` | Must be `mtls` |
| `grpc_tls_server_name` | Node certificate domain; must match Web registration |
| `grpc_client_ca_path` | Absolute path to Node's runtime public CA trust file |
| `kernel_runtime_path` | Proxy runtime directory; defaults to `/tpdata/trojan-panel-core/runtime` |
| `node_certificate_mode`, `node_certificate_path`, `node_private_key_path` | [Certificate mode and paths](#external-certificates) |
| `node_caddy_http_port`, `node_caddy_https_port` | Caddy-mode ports; defaults `80`, `8863` |

Node template passwords and server ID are placeholders to replace, rather than actual deployment credentials. For configuration deployment, place Web's current public `client-ca.crt` in Node's `pki_bundle_dir`.

<a id="recreate"></a>
## Recreate current-release services

After changing the current release's domain, application ports, or certificate mounts, recreate the corresponding host using its configuration:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) install --config ./node.yaml --force
```

Use `./web.yaml` for Web. `--force` pulls configured images and recreates API, UI, Agent, or Caddy containers. Existing MariaDB and Redis containers are retained. Changing an initialized database password requires updating the database itself and all clients; editing YAML alone is insufficient.

Operations across releases must use the target release's scripts, configuration specification, and images. The entrypoint does not infer or convert other versions' configurations. Back up data and PKI before maintenance.

<a id="removal"></a>
## Local removal

Run on the corresponding host. Retain data:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) remove --config ./node.yaml --keep-data
```

Delete project data:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) remove --config ./node.yaml --purge-data
```

Use `./web.yaml` for Web. `--keep-data` and `--purge-data` are mutually exclusive. Without either, YAML `purge_data` selects the mode; templates retain data by default.

| Target | `--keep-data` | `--purge-data` |
| --- | --- | --- |
| Corresponding containers and anonymous volumes | Removed | Removed |
| Eligible local images in the corresponding repositories | Removed | Removed |
| Service data, PKI, camouflage content, proxy runtime configuration | Retained | Removed |
| Original deployment YAML | Retained | Removed |
| Installed host maintenance service and working copies | Removed | Removed |
| External certificates, host Nginx, Certbot | Retained | Retained |
| Docker Engine and manually saved entrypoint scripts | Retained | Retained |

After project removal, run [`deps remove`](#dependency-removal) separately if you want to remove Docker and yq installed by this entrypoint.

Web removal targets MariaDB, Redis, API, UI, and Web Caddy. Node removal targets Agent and Node Caddy in Caddy mode. Image cleanup covers all local tags in repositories named by configuration and actual containers. Images referenced by other containers remain with a message. No global prune runs.

Full removal clears project service directories, `pki_bundle_dir`, camouflage content, custom `kernel_runtime_path`, configured identity files, and original YAML. Maintenance directories are `/etc/trojanpanelnext-host` and `/usr/local/lib/trojanpanelnext-host`; the systemd unit is `/etc/systemd/system/trojanpanelnext-host.service`. Full removal is refused while the other project purpose still runs on the same host, protecting shared data. External certificate paths overlapping cleanup targets also cause refusal.

Local removal does not delete Web server registration. Delete the server in Web to coordinate removal and registration cleanup.

<a id="web-removal"></a>
## Delete a node server from Web

The node server deletion dialog offers Cancel (取消), Delete (删除), and Delete completely (彻底删除):

| Action | Node host | Web |
| --- | --- | --- |
| Cancel | No action | No action |
| Delete | Uninstall while retaining data | Delete server and associated proxy configuration; retain traffic/task history |
| Delete completely | Uninstall and delete project data | Delete server, associated proxies, and corresponding traffic/task history |

Web calls the Node host maintenance service first and commits deletion after receiving success. Offline nodes, failed mTLS/network connections, and failed removal return an error and retain registration and proxy configuration. Shared task records involving other servers remain. Deleting an individual proxy removes only that proxy.

Remote removal temporarily retains TLS identity and a removal receipt. Maintenance files and the service are cleaned after Web commits and confirms the result. When the UI reports pending cleanup (`cleanupPending`), main services and Web records have been removed, while maintenance files await confirmation. Failed confirmation and cleanup retry in the background and recover after restart. Keep Web access to the Node maintenance port and Node access to Web HTTPS available until completion.

<a id="reconnect"></a>
## Reconnect after deletion

Delete retains Node data, PKI, certificates, and YAML, while removing server/proxy registration. Reconnect as follows:

1. Wait for host maintenance cleanup, then register the server again in Web. Its name/address may be reused; obtain the new ID.
2. Update `node_server_id` in the retained `node.yaml` and check database, Redis, and TLS settings.
3. Redeploy Node with `install --config ./node.yaml`.
4. Check that the server is online in Web and recreate the required proxies.

Retained history belongs to the original server ID and does not automatically migrate to the new ID. After Delete completely, prepare YAML and public CA following first-install instructions. External certificates remain managed and retained by the host.

<a id="certificate-maintenance"></a>
## Automatic certificate maintenance

In Caddy mode, each Web/Node Caddy issues and renews public certificates. Preserve certificate data and keep DNS and ACME validation reachable. External Node certificates are issued and renewed by host tools.

| Service | Loading behavior after renewal |
| --- | --- |
| Agent gRPC, host maintenance, Hysteria2 | Read certificate files on new TLS handshakes |
| NaiveProxy | Check every minute and briefly restart affected instances after validation, preserving users and runtime configuration |
| Xray | File certificate reload defaults to every hour; restart the proxy in the panel for immediate loading |
| Host Nginx | Validate and reload using the administrator-configured renewal deploy hook |

Web API checks internal mTLS identity every five minutes and reissues client certificates when fewer than 90 days remain. With fewer than 365 days remaining on the CA, it distributes old/new trust and switches identity after every registered mTLS Node acknowledges. The previous identity remains for at least 24 hours. Offline Nodes block rotation progress, which retries when connectivity recovers.

First connection still requires manual transfer of the current public CA. Back up the complete Web PKI directory, including `state.json`, `generations`, and symlinks. Nodes offline beyond their original trust validity or restored from expired backups need trust bootstrap again. See [certificate maintenance](certificates.md) for the complete mechanism.

<a id="operations"></a>
## Operations

Host data defaults to `/tpdata`. Deployment YAML remains at the selected output path. Backups should include databases, Redis, API/Agent settings, proxy runtime configuration, and complete PKI. Follow the certificate manager's backup requirements for external certificates. Database backup requires a consistent export or stopped writes.

Inspect containers and host maintenance:

```bash
docker ps
docker logs --tail 100 trojan-panel
docker logs --tail 100 trojan-panel-core
systemctl status trojanpanelnext-host.service
journalctl -u trojanpanelnext-host.service -n 100 --no-pager
```

Inspect API on Web and Agent/maintenance on Node. Before sharing logs, remove passwords, access tokens, private keys, and user connection information.

<a id="troubleshooting"></a>
## Troubleshooting

| Symptom | Checks |
| --- | --- |
| Missing dependencies | Run `deps install` on supported systems or prepare dependencies manually; verify mikefarah/yq v4, running Docker, and systemd on Node |
| Dependency removal refuses to remove Docker | Check all Docker containers (`docker ps -a`, including stopped containers), shared containerd's other namespaces, Node maintenance, and runtime state; remove the project and other services first, and rerun `deps install` to repair any interrupted installation |
| Release/image mismatch | Use the selected release's template; check `release` and product image tags |
| Web issuance fails | Check A/AAAA, reachable/free 80/443, and Caddy logs |
| Login request times out | Check API, MariaDB, Redis, API logs, and Caddy-to-API access; a timeout does not establish a password error |
| Invalid login credentials | Use the initial account for a fresh install or the administrator-set password for an existing account |
| Node offline | Check actual server ID, Web database/Redis access, gRPC firewall, certificate domain, and public CA |
| External certificate rejected | Check absolute paths, full chain, unencrypted matching key, validity, hostname coverage, and dedicated directories |
| Certificate appears unchanged after renewal | Wait for the service's reload interval or restart that proxy; check symlink targets and Nginx reload |
| Web server deletion fails | Check Node maintenance, Web mTLS access to `grpc_port + 1`, and Node HTTPS access to Web |
| Removal retains a shared image | Another container still references it; identify that consumer before manual cleanup |

When reporting issues, include the release, command with sensitive arguments removed, error, and sanitized logs. Do not upload actual deployment YAML, private keys, database passwords, or access tokens.
