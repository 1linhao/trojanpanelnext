# TrojanPanel Next v1.0.2-rc.11 deployment guide

[简体中文](deployment.md) | English

## Contents

- [System and software dependencies](#dependencies)
  - [Minimal bootstrap tools](#dependency-bootstrap)
  - [Install dependencies](#dependency-install)
  - [Prepare dependencies manually](#dependency-manual)
  - [Remove dependencies](#dependency-removal)
- [Release binding and entrypoint](#versions)
- [DNS, ports, and networking](#network)
- [Proxy external ports and forwarding](port-forwarding_EN.md)
- [One-command Web installation](#web)
- [Register a node server](#node-registration)
- [Install Node](#node)
  - [Web deployment package](#node-deployment-package)
  - [Interactive command-line installation](#node-interactive)
- [Existing Node certificates](#external-certificates)
- [Configuration deployment and fields](#configuration)
  - [Download and edit templates](#configuration-download)
  - [Minimum Web edits](#configuration-web-minimum)
  - [Minimum Node edits](#configuration-node-minimum)
  - [Prepare Node public CA](#configuration-node-ca)
  - [Validate and install](#configuration-validation)
  - [Configuration field reference](#configuration-fields)
- [Recreate current-release services](#recreate)
- [Update Web and Node images](#updates)
- [Local removal](#removal)
- [Uninstall or delete a node server from Web](#web-removal)
- [Reconnect after removal](#reconnect)
- [Automatic certificate maintenance](#certificate-maintenance)
- [Operations](#operations)
- [Troubleshooting](#troubleshooting)

<a id="dependencies"></a>
## System and software dependencies

Linux `amd64` and `arm64` are supported. Run commands in Bash; installation, updates, recreation, and removal require root. Node hosts must run systemd. At least 1 GiB of memory per server is recommended. Web and Node can run on separate servers connected through controlled network access.

The `web`, `node`, and `install` deployment commands only check dependencies. Run `deps install` separately to prepare the required software, or install it manually:

| Operation | Dependencies |
| --- | --- |
| Entrypoint, help, template download | Bash, curl, CA certificates, grep, coreutils |
| Deployment package extraction | Bash, tar, gzip, coreutils; an administrator-only working directory |
| Automatic dependency installation and removal | Entrypoint dependencies, util-linux (`flock`), root, supported Debian/Ubuntu, running systemd, apt-get, dpkg, dpkg-query, systemctl |
| One-command deployment and validation | Entrypoint dependencies and **mikefarah/yq v4** |
| Installation, updates, and recreation | Entrypoint dependencies, yq, a running Docker Engine, OpenSSL, tar, findutils, awk; Node also needs systemd |
| Removal | Bash, curl, CA certificates, grep, coreutils including realpath/rmdir, Docker Engine, yq; Node maintenance cleanup needs systemctl |

<a id="dependency-bootstrap"></a>
### Minimal bootstrap tools

Automatic dependency entrypoints support **Debian 12/13 and Ubuntu 22.04/24.04 on amd64/arm64**, with running systemd. Start a root Bash session, confirm the system provides `apt-get`, `dpkg`, `dpkg-query`, and `systemctl`, and ensure HTTPS access to GitHub, GHCR, and apt repositories.

Before downloading scripts or extracting a Node deployment package, prepare basic tools in root Bash:

```bash
apt-get update
apt-get install -y bash curl ca-certificates grep coreutils util-linux tar gzip
```

util-linux provides `flock`; tar and gzip extract the package. Once basic tools are ready, Web can run `deps install` below. Node can [extract the package securely and run its dependency script](#node-deployment-package). On other Linux distributions, use the distribution's package manager for these tools and [deployment dependencies](#dependency-manual), and skip the included automatic dependency script.

<a id="dependency-install"></a>
### Install dependencies

Supported systems are **Debian 12/13 and Ubuntu 22.04/24.04** on `amd64` or `arm64`, with root access and running systemd. Both Web and Node hosts can use this command. Follow the [manual instructions](#dependency-manual) for other Linux distributions.

Run in a root Bash session:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) deps install
```

The command prepares Bash, curl, CA certificates, grep, coreutils, OpenSSL, tar, findutils, awk (installing gawk if awk is missing), and the distribution's Docker Engine packages (`docker.io`, `containerd`, and `runc`; Debian 13 also needs `docker-cli`), then starts Docker. If compatible yq is missing, it downloads the architecture-specific **mikefarah/yq v4.53.6** binary and installs it after SHA256 verification. Existing compatible Docker and mikefarah/yq v4 are reused without replacing the original tools.

If entrypoint tools are missing, follow the [minimal bootstrap](#dependency-bootstrap). For a Node package downloaded from Web, use its [included dependency and installation steps](#node-deployment-package).

Dependency commands use the same release entrypoint as deployment commands. Select a release explicitly with:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) --version 1.0.2-rc.11 deps install
```

Installation records are stored in `/var/lib/trojanpanelnext-dependencies` (directory mode `0700`). They track Docker packages added by this command and the installed yq file's SHA256 for [dependency removal](#dependency-removal). Repeated installation checks and fills missing dependencies. If installation was interrupted, rerun `deps install` to repair it before removal.

<a id="dependency-manual"></a>
### Prepare dependencies manually

On other Linux distributions, prepare dependencies using the distribution's package manager. Debian/Ubuntu dependencies can also be prepared manually:

```bash
apt-get update
apt-get install -y bash curl ca-certificates grep coreutils util-linux openssl tar gzip findutils gawk docker.io
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

First [remove the project](#removal) on that host and remove containers belonging to other Docker services. After uninstalling a Node from Web, also wait for host maintenance cleanup. Deleting only Web records leaves the Node host running; remove it locally first. Then run:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) deps remove
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

This guide covers pre-release **v1.0.2-rc.11 (Pre-release)**. Version `1.0.2-rc.11` maps to Git tag `v1.0.2-rc.11`, configuration field `trojanpanelnext.release: "1.0.2-rc.11"`, and these product images:

| Component | Image |
| --- | --- |
| Web API | `ghcr.io/1linhao/trojanpanelnext-api:1.0.2-rc.11` |
| Web interface | `ghcr.io/1linhao/trojanpanelnext-web:1.0.2-rc.11` |
| Node Agent | `ghcr.io/1linhao/trojanpanelnext-node-agent:1.0.2-rc.11` |

All product images provide `linux/amd64` and `linux/arm64`. Caddy, MariaDB, and Redis use the separate upstream versions in the templates.

The shared entrypoint, `scripts/tp.sh`, fetches the script library and templates from the selected release tag, checks script versions, and executes the selected command. Temporary downloaded scripts are removed when the command ends. Installation and validation require configuration and product images matching the selected release. The [update command](#updates) uses existing supported configuration and binds its release and product images to the target version. Scripts do not infer migrations between different configuration schemas; only the current release is maintained.

Select a version explicitly when installing Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) --version 1.0.2-rc.11 web
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
| `--version <target> update --config <file>` | Update Web or Node product images using existing configuration |
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
| `--node-id <id>` | Integer Web server ID (≥ `1`), not an IP / domain or proxy ID |
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
| Proxy clients → Public entry | Proxy TCP/UDP ports in generated client configuration; external port when forwarding is enabled |
| Forwarding service → Node | Actual TCP/UDP proxy listening port |

API 8081, UI 8888, and Node API 8082 listen using host networking. Restrict access to these internal services instead of exposing every port publicly. Connect MariaDB and Redis through firewall rules or a trusted private network. Configuration does not create VPN connections or firewall rules.

When changing ports, update configuration, Web server registration, and firewall rules together. The maintenance port is not independently configurable; `grpc_port` must be at most `65534`. External certificate mode does not run Node Caddy or use its HTTP/HTTPS ports. Proxies still need available listening ports.

<a id="web"></a>
## One-command Web installation

Run on the Web host:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) web
```

Enter the Web domain and certificate contact email when prompted. Public parameters can also be supplied directly:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) web --hostname panel.example.com --email admin@example.com --output ./web.yaml
```

The default output is `./web.yaml`. First installation generates MariaDB and Redis passwords and writes them to the configuration, creates internal Web mTLS credentials, and deploys MariaDB, Redis, API, UI, and Web Caddy.

Visit `https://panel.example.com`. The initial username is `sysadmin`, with password `123456`; change the password after signing in. Panel credentials are separate from database and Redis passwords in YAML. Existing deployments use the password set for that account.

Back up `web.yaml` and the complete `pki_bundle_dir`. Configuration contains sensitive credentials and should be readable only by administrators; never commit it to a public repository.

<a id="node-registration"></a>
## Register a node server

### 1. Open Server management

Sign in with the `sysadmin` role, open **Server management** in the sidebar, and click **Add Node server**. You can also open `https://panel.example.com/#/server-manage/server-list`, replacing the example domain with your actual Web domain. Only `sysadmin` can add servers or generate deployment packages containing credentials.

This registers an entire Node host. Create individual proxy instances separately in Node management after installation.

### 2. Enter connection settings

| Field | Requirement |
| --- | --- |
| Server name | 2–20 characters identifying the host |
| Node address | IP or domain reachable from Web, such as `203.0.113.10` or `node.example.com` |
| gRPC port | Default `8100`; must match Node configuration and firewall rules |
| gRPC certificate domain | Domain covered by the Node certificate, such as `node.example.com`; must match `grpc_tls_server_name` |

### 3. Distinguish numeric ID from address

Saving assigns a Web database ID that is an **integer greater than or equal to `1`**. The list shows **ID** and **IP / domain** in separate columns. For ID `3` and address `node.example.com`, Node configuration uses `node_server_id: 3`.

The server ID is not its IP, domain, name, or the ID of a proxy instance on that host. Do not put an address in `node_server_id`, or retain the template example ID.

Saving automatically opens the **Deploy Node** guide. Click the **Deploy Node** icon on the corresponding server row to reopen it later. Deployment packages fill in this numeric server ID automatically. Offline status is expected before Node installation.

<a id="node"></a>
## Install Node

<a id="node-deployment-package"></a>
### Option 1: Download a deployment package from Web

[Register the server](#node-registration), then use **Deploy Node** to complete these steps.

#### 1. Confirm connectivity and certificate mode

Confirm the prefilled **Web address reachable from Node**, changing it if necessary to a reachable hostname or IP such as `panel.example.com`, without protocol, port, or path. Check the database and Redis address previews; the package uses Web’s actual connection ports and credentials.

Select **Caddy automatic issuance and renewal**, enter the certificate email, and prepare Node DNS and ACME validation ports. For **Existing host certificates**, enter absolute paths to the full chain and matching unencrypted key already present on Node. This mode does not require email, and host tools handle renewal. Certificates must cover the Node and gRPC certificate domains.

Web does not generate Node external TLS certificates or private keys. See [external certificate mode](#external-certificates) for path requirements. Before installation, check database, Redis, gRPC, and the `grpc_port + 1` maintenance port against the [network rules](#network).

#### 2. Download and securely transfer the package

From **Deployment parameters**, click **Next** to open **Download and install**, then click **Download deployment package**. The filename is `tpnext-node-<server ID>.tar.gz`. The package contains:

| File | Content |
| --- | --- |
| `tpnext/node.yaml` | Current-release Node configuration with actual numeric server ID, database / Redis settings, and credentials |
| `tpnext/client-ca.crt` | Web’s current public client CA, used by Node to verify Web management connections |
| `tpnext/install-dependencies.sh` | Invoke the package release's `deps install` to prepare deployment dependencies; mode `0700` |
| `tpnext/install-node.sh` | Prepare the public CA, then invoke the matching installation entrypoint with the included YAML; mode `0700` |
| `tpnext/README.md` | Package contents and installation instructions |

The package includes the server ID, database / Redis settings, and public CA. Separate manual configuration editing or CA transfer is unnecessary for this flow. Only `sysadmin` can generate this credential-bearing package. Keep the package and YAML in an administrator’s private directory with `0600` permissions, transfer them securely only to the target Node, and never commit them to a public repository. The package contains neither CA / Web client private keys nor a Node TLS private key.

#### 3. Prepare basic tools on Node

In a root Bash session on the target Node, follow the [minimal bootstrap](#dependency-bootstrap) to prepare Bash, curl, CA certificates, grep, coreutils, util-linux, tar, and gzip, and confirm systemd is running. Automatic dependency installation requires the system's apt-get, dpkg, dpkg-query, and systemctl, plus HTTPS access to GitHub, GHCR, and apt repositories.

#### 4. Transfer and extract securely

Securely transfer the package to a private working directory on Node. This example uses server ID `3`; substitute the actual downloaded filename:

```bash
umask 077
chmod 600 ./tpnext-node-3.tar.gz
tar -xzf ./tpnext-node-3.tar.gz
```

The five files extract into `tpnext/` (mode `0700`); both scripts have mode `0700`. Keep credential-bearing `node.yaml` at mode `0600`. Do not extract in a shared directory or a public Web directory.

#### 5. Run the included dependency entrypoint

Run from the extraction directory:

```bash
bash ./tpnext/install-dependencies.sh
```

This script invokes `deps install` through the package release's entrypoint to prepare Docker Engine, mikefarah/yq v4, OpenSSL, findutils, awk, and other deployment dependencies. It reuses compatible tools and `/var/lib/trojanpanelnext-dependencies` management records. It does not install Node, read credentials from `node.yaml`, or import the CA. Automatic installation supports Debian 12/13 and Ubuntu 22.04/24.04 on amd64/arm64. On other Linux distributions, [prepare dependencies manually](#dependency-manual) and skip this step.

#### 6. Run the installation entrypoint

Once dependencies are ready, run from the same extraction directory:

```bash
bash ./tpnext/install-node.sh
```

The script installs using the included `tpnext/node.yaml` and explicitly supplies the current Web public CA through `install --client-ca`. The installer validates the CA, backs up replaced trust files, and synchronizes bootstrap and runtime trust so a Node with retained data can reconnect to the current Web. This entrypoint does not force recreation of existing services; automatic CA rotation still follows [certificate maintenance](certificates.md).

Retain the actual deployment `tpnext/node.yaml` for [image updates](#updates), [recreation](#recreate), and [removal](#removal) on this host. Check the server is online in Web before creating proxy instances.

<a id="node-interactive"></a>
### Option 2: Interactive command-line installation

Deploy Web first, [register the node server](#node-registration), and record the integer greater than or equal to `1` in the **ID** column. This is not a Node address or proxy ID.

Securely transfer Web's current public `client-ca.crt` to the Node host, for example `/root/client-ca.crt`. Its default Web location is `/tpdata/trojanpanelnext-pki/client-ca.crt`. Transfer only the public CA. Keep `client-ca.key`, `client.key`, and `client.crt` on Web.

Run on the Node host:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) node
```

Provide the Node domain, Web address, registered server ID, local public CA path, and Web database/Redis credentials. Caddy mode also asks for a certificate email. Administrators can obtain connection passwords from Web YAML; sensitive input is not echoed to the terminal.

Public parameters can be supplied directly, with remaining fields requested interactively:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) node --hostname node.example.com --email admin@example.com --web-host panel.example.com --node-id 1 --client-ca /root/client-ca.crt
```

`--client-ca` points to a public PEM CA already securely transferred to this host. The file must contain only unexpired CA certificates; multiple old/new CAs are accepted during rotation. Leaf certificates, expired CAs, private keys, and other mixed content are rejected before new YAML or CA files are published. After validation, the script copies it to `/tpdata/trojanpanelnext-pki/client-ca.crt`. Omit the option if that default file already exists. The same CA can be reused; a different existing trust file is not overwritten. Follow [certificate maintenance](certificates.md) for trust changes.

The default output is `./node.yaml`. Node needs ongoing access to Web's MariaDB, Redis, and HTTPS. Web needs access to Node control and maintenance ports. Scripts do not register servers or retrieve the CA automatically.

Installation deploys the Agent, Node Caddy in the default certificate mode, and `trojanpanelnext-host.service`. The host maintenance service uses mTLS HTTPS and accepts authorized Web clients for remote removal. The Agent container has no `docker.sock` mount.

After installation, check that the server is online in Web, then create the required Xray, Hysteria2, or NaiveProxy proxies.

<a id="external-certificates"></a>
## Existing Node certificates

If Nginx and Certbot or another tool manage host certificates, issue a valid certificate first and select external mode:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) node --certificate-mode external
```

Enter absolute paths to the full chain and its unencrypted PEM private key. Paths can also be supplied directly:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) node --certificate-mode external --certificate /etc/letsencrypt/live/node.example.com/fullchain.pem --private-key /etc/letsencrypt/live/node.example.com/privkey.pem
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

Configure a proxy’s **External port** and **Actual port** in Web: clients use the public entry, while Node listens on the actual port. Host or gateway tools maintain forwarding; scripts do not configure Nginx / NAT. See [port forwarding](port-forwarding_EN.md#tls-sni) for one TPNext TLS proxy and an independent TLS service sharing TCP `443`, certificate matching, and Hysteria2 UDP requirements.

When changing an installed Node's certificate mode, paths, or symlink target directories, use `--force` to recreate mounts. Proxy references matching the previous certificate/key pair are updated, preserving proxy accounts and other settings. Both local removal modes and remote Web removal preserve external certificates, Nginx, Certbot, and their configuration.

<a id="configuration"></a>
## Configuration deployment and fields

For configuration deployment, follow: download a template → edit required fields → prepare Node's CA → validate → install. Dependencies, DNS, and networking still need to be prepared as described above.

<a id="configuration-download"></a>
### Download and edit configuration templates

Download the Web template on the Web host:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) config web --output ./web.yaml
```

Download the Node template on the Node host:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) config node --output ./node.yaml
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
| `node_server_id` | Integer in the ID column after [Web registration](#node-registration) (≥ `1`), not an IP / domain or proxy ID |
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
### Prepare Web public CA for manual configuration installation

From the Web host, securely copy the current public `client-ca.crt` from its configured `pki_bundle_dir` to Node. The default source is `/tpdata/trojanpanelnext-pki/client-ca.crt`. For example, run this on Web, substituting the Node SSH address, user, and port as needed:

```bash
scp /tpdata/trojanpanelnext-pki/client-ca.crt root@node.example.com:/root/client-ca.crt
```

Copy only the public `client-ca.crt`, never `client-ca.key`, `client.key`, or `client.crt`. Supply `--client-ca /root/client-ca.crt` in the Node installation command below. The installer validates the CA and writes it to the configured bootstrap directory and runtime `grpc_client_ca_path`. Different existing trust files are backed up before rebinding management trust to the current Web CA. No manual runtime-file overwrite or template-path change is needed.

Replaced trust files are backed up in `pki_bundle_dir/client-ca-backups/rebind.*`, with directory permissions `0700` and file permissions `0600`.

`install --client-ca` only supports Node installation. Installation without this option reuses live trust, and image updates preserve it; follow [certificate maintenance](certificates.md) for CA rotation.

<a id="configuration-validation"></a>
### Validate and install

After editing and preparation, validate on each corresponding host.

Validate Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) validate --config ./web.yaml
```

Install Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) install --config ./web.yaml
```

Validate Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) validate --config ./node.yaml
```

Install Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) install --config ./node.yaml --client-ca /root/client-ca.crt
```

Alternatively, use `web --config ./web.yaml` or `node --config ./node.yaml` to check the purpose and install. `validate` checks YAML, the release, and fields; it does not verify DNS, certificate contents, network connections, or service health. After installation, check Node's status in [Server management](#node-registration).

<a id="configuration-fields"></a>
### Configuration field reference

Complete editable templates: [Web](../scripts/deploy/templates/web.yaml) · [Node](../scripts/deploy/templates/node.yaml). All fields belong to the `trojanpanelnext` mapping:

| Shared field | Description |
| --- | --- |
| `release` | Required string; currently `"1.0.2-rc.11"`, matching the selected scripts |
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
| `node_server_id` | Integer Web database server ID (≥ `1`), not an IP / domain or proxy ID |
| `grpc_port`, `core_port` | Agent gRPC/API ports; defaults `8100`, `8082` |
| `grpc_tls_mode` | Must be `mtls` |
| `grpc_tls_server_name` | Node certificate domain; must match Web registration |
| `grpc_client_ca_path` | Absolute path to Node's runtime public CA trust file |
| `kernel_runtime_path` | Proxy runtime directory; defaults to `/tpdata/trojan-panel-core/runtime` |
| `node_certificate_mode`, `node_certificate_path`, `node_private_key_path` | [Certificate mode and paths](#external-certificates) |
| `node_caddy_http_port`, `node_caddy_https_port` | Caddy-mode ports; defaults `80`, `8863` |

Node template passwords and server ID are placeholders to replace, rather than actual deployment credentials. For configuration deployment, securely transfer the current Web public CA to Node and pass `--client-ca <file>` during installation; the installer validates it, backs up previous trust, and writes bootstrap and runtime paths.

<a id="recreate"></a>
## Recreate current-release services

After changing the current release's domain, application ports, or certificate mounts, recreate the corresponding host using its configuration:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) install --config ./tpnext/node.yaml --force
```

The Node examples use `./tpnext/node.yaml` from the extracted package; use the original YAML path for manual deployments, or `./web.yaml` for Web. `--force` pulls configured images and recreates API, UI, Agent, or Caddy containers. Existing MariaDB and Redis containers are retained. Changing an initialized database password requires updating the database itself and all clients; editing YAML alone is insufficient.

Use the [image update command](#updates) and target release for updates across versions. Installation, validation, and recreation use matching configuration and image versions. Back up data and PKI before maintenance.

<a id="updates"></a>
## Update Web and Node images

Run in a root Bash session on the corresponding host, using the **actual existing deployment YAML**. `--version` must explicitly select the target release. The entrypoint fetches that release’s update script and product images.

Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) --version 1.0.2-rc.11 update --config ./web.yaml
```

Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) --version 1.0.2-rc.11 update --config ./tpnext/node.yaml
```

Package-deployed Node uses `./tpnext/node.yaml`; substitute the original YAML path for manual deployments. YAML `purpose` selects the deployment to update. Do not change `release` or image fields beforehand, or replace existing configuration with an empty template. Updates accept versions `1.0` and newer with `schema_version: 1` and a supported container layout, and support upgrades or repairs of the same version. There is no automatic downgrade, and `latest` is not used. Prepare other configuration schemas according to the target release’s instructions.

| Item | Update behavior |
| --- | --- |
| Web product images | Update API and Web UI |
| Node product image | Update Node Agent and its associated host maintenance service |
| Deployment YAML | Change only `release` and official product image fields for the deployment; retain credentials, server ID, domains, ports, and paths |
| Data, PKI, certificates, and proxy runtime configuration | Retained; certificates are not reissued |
| API / Agent `config.ini` and UI Nginx configuration | Existing contents remain, including custom settings outside YAML; connection, port, and identity settings must match the original YAML |
| MariaDB, Redis, and Caddy | Existing containers and versions remain; product updates do not upgrade them |
| Previous product images | Retained for recovery; no Docker prune runs |

Back up service data and PKI first, and check that the existing YAML matches running container settings. Its `release` must match the tags of the running official GHCR product images. Restore the actual deployment configuration if they differ; do not substitute a new template. The update script checks container environment, mounts, networking, and ports, then pulls all target product images before switching. A failed pull leaves old containers running and the original configuration unchanged.

Before switching, a mode `0600` backup is created beside the original YAML as `<configuration>.backup-<previous-version>-<time>-<random>`. Switching product containers briefly interrupts Web or Node services. If switching or health checks fail, the script attempts to restore the original configuration, previous product containers, runtime configuration, and Node maintenance service. If restoration fails, recovery files remain and their directory is printed for manual repair. This restores deployment state only: database migrations or writes by the new version are not rolled back automatically. Restore the database from a backup made before upgrading when necessary.

Update Web and Node separately on their respective hosts. The command does not replace host Nginx or Certbot, or upgrade independent Xray / Hysteria2 kernel revisions; use Web kernel management for those.

<a id="removal"></a>
## Local removal

Run on the corresponding host. Node examples use `./tpnext/node.yaml` from the extracted package; substitute the original YAML path for manual deployments. Retain data:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) remove --config ./tpnext/node.yaml --keep-data
```

Delete project data:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.11/scripts/tp.sh) remove --config ./tpnext/node.yaml --purge-data
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

Local removal does not delete Web server registration. Choose Uninstall or Uninstall completely in Web to coordinate removal and registration cleanup. If the host was already cleaned locally or is offline, Delete can clear only its Web records.

<a id="web-removal"></a>
## Uninstall or delete a node server from Web

The server removal dialog offers Cancel (取消), Delete (删除), Uninstall (卸载), and Uninstall completely (彻底卸载):

| Action | Node host | Web |
| --- | --- | --- |
| Cancel | No action | No action |
| Delete | No Node connection; host services are neither stopped nor removed | Delete this server and associated proxies, protocol configuration, traffic, tasks, and connection records |
| Uninstall | Remove project containers and eligible images while retaining data | Delete server and associated proxy configuration; retain traffic/task history |
| Uninstall completely | Uninstall and delete project data | Delete this server and associated proxies, protocol configuration, traffic, tasks, and connection records |

**Delete** works without an online Node and can remove an unreachable or unmanaged server. It only clears Web data. Running Agent/proxy services, containers, images, and the host maintenance service on Node are not stopped or removed. To clean the host, run [local removal](#removal) on Node. If an uninstall or delete request for the same server is already running, a busy error asks you to wait and retry. An offline uninstallation on another server does not block this server’s Web-only deletion.

**Uninstall** and **Uninstall completely** first call the Node host maintenance service and clear Web registration only after success. Offline nodes, failed mTLS/network connections, or failed removal return an error and retain the server and associated proxy configuration. Fix the connection and retry, or choose to delete only Web records.

Web cleanup targets the server ID. Shared tasks retain task items for other servers; empty tasks belonging solely to this server are deleted, and canary targets pointing to it are cleared. Account-wide traffic totals, daily rankings, JWTs, system settings, and shared caches remain because they are account or system state spanning servers. Deleting an individual proxy removes only that proxy.

Remote removal temporarily retains TLS identity and a removal receipt. Maintenance files and the service are cleaned after Web commits and confirms the result. When the UI reports pending cleanup (`cleanupPending`), main services and Web records have been removed, while maintenance files await confirmation. Failed confirmation and cleanup retry in the background and recover after restart. Keep Web access to the Node maintenance port and Node access to Web HTTPS available until completion.

<a id="reconnect"></a>
## Reconnect after removal

Uninstall retains Node data, PKI, certificates, and YAML, while removing server/proxy registration. Reconnect as follows:

1. Wait for host maintenance cleanup, then register the server again in Web. Its name/address may be reused; obtain the new ID.
2. Download a new deployment package from **Deploy Node** for this server, checking the certificate mode and existing paths.
3. Follow the [package steps](#node-deployment-package) to extract securely, run `bash ./tpnext/install-dependencies.sh` to check and complete dependencies, then run `bash ./tpnext/install-node.sh`. Installation uses the new ID, current credentials, and Web CA. Existing data is retained, and replaced trust files are backed up. On other Linux distributions, prepare dependencies manually and run only the installation entrypoint.
4. Check that the server is online in Web and recreate the required proxies.

Retained history belongs to the original server ID and does not automatically migrate to the new ID. After Uninstall completely, prepare YAML and public CA following first-install instructions. External certificates remain managed and retained by the host.

If you only used Delete, Node may still run its previous configuration, while associated Web records and server-specific history have been cleared. Before reconnecting, uninstall locally while retaining data, register a new server ID, and redeploy as above. Do not keep using the deleted server ID.

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

First connection requires securely transferring the current public CA to Node. [Web deployment packages](#node-deployment-package) include it and prepare it through the bundled script; manual configuration follows [CA preparation](#configuration-node-ca). Back up the complete Web PKI directory, including `state.json`, `generations`, and symlinks. Nodes offline beyond their original trust validity or restored from expired backups need trust bootstrap again. See [certificate maintenance](certificates.md) for the complete mechanism.

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
| Web server uninstallation fails | Check Node maintenance, Web mTLS access to `grpc_port + 1`, and Node HTTPS access to Web; choose Delete if Node is unreachable and only Web cleanup is needed |
| Removal retains a shared image | Another container still references it; identify that consumer before manual cleanup |

When reporting issues, include the release, command with sensitive arguments removed, error, and sanitized logs. Do not upload actual deployment YAML, private keys, database passwords, or access tokens.
