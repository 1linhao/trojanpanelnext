# Unified deployment configuration: local initialization

[简体中文](README.md) | English

On Linux, macOS, or Windows WSL, provide Bash, curl, tar, OpenSSH, `sha256sum` or `shasum`, and a user-installed [mikefarah yq v4](https://github.com/mikefarah/yq#install). The official entrypoint is the archive from one fixed GitHub Release: check its tag, archive SHA256, and release attestations on the Release page; download it; verify its SHA256; then run its bundled CLI. `v0.1.0-rc.3` does not contain the unified CLI. Replace the placeholders below with values from the same newer Release.

```bash
TAG='v<version>'
SHA256='<release-archive-sha256>'
ARCHIVE="trojanpanelnext-installer-${TAG#v}.tar.gz"
curl -fL "https://github.com/1linhao/trojanpanelnext/releases/download/${TAG}/${ARCHIVE}" -o "$ARCHIVE"
printf '%s  %s\n' "$SHA256" "$ARCHIVE" | sha256sum -c -
mkdir -m 700 .tpnext-release
tar -xzf "$ARCHIVE" -C .tpnext-release
(cd .tpnext-release && sha256sum -c SHA256SUMS)
bash .tpnext-release/client/tpnext.sh init \
  --config "$PWD/deployment.local.yaml" \
  --tag "$TAG" --sha256 "$SHA256" --archive "$PWD/$ARCHIVE" \
  --work-dir "$PWD/deployment.local"
bash .tpnext-release/client/tpnext.sh plan --config "$PWD/deployment.local.yaml"
```

On macOS, replace both `sha256sum` commands with `shasum -a 256`. `init` verifies the fixed archive SHA, every internal SHA/manifest entry, version, and image digests again before running other bundled code. It creates local configuration and work directories without contacting a VPS. Operators invoke only `client/tpnext.sh`; its topology parser, verifier, and templates are internal assets from the same bundle. The archive also supplies `config-web.yaml`, `config-node.yaml`, and `config-combined.yaml` for #38 to compile one host configuration per unique host rather than passing the unified YAML to the remote installer. Later tickets provide `check` and `deploy`.

The default template describes an external computer reaching separate Web and Node VPS hosts by SSH. If the operator runs on the Web VPS, pass `--web-transport local`; that example maps one Node to the same `web-host` and plans a `combined` installation. You may edit the YAML to move that Node to another SSH host or append independent Nodes. For a remote `combined` host, map the Node to the Web host and set `node_caddy_https_port` to 443; the shared Entry uses 80/443. At most one Node may share the Web host, and their domains must differ. Only the same explicit host id means the same machine; IPs and SSH aliases do not establish co-location.

`hosts` holds execution transport, while `web.domain`, `web.public_ip`, and each Node's `domain` and `public_ip` hold service addresses. SSH can use a root key, or an ordinary user's key with `sudo -n` in later deployment. `node_key` is the stable key for a future Node state path; appending or reordering the array preserves existing paths. Removing or renaming a Node does not authorize uninstall, revocation, or rotation. `plan` only validates YAML and topology and prints a secret-free per-host plan. This ticket provides no `check` or `deploy` command.

The config file is mode 0600 and the verified asset work directory is mode 0700. Use a `*.local.yaml` config name; inside a Git worktree, the CLI requires both config and work directory to be ignored by Git. Existing paths are never overwritten. Do not copy the Web-privileged unified YAML to a Node VPS.
