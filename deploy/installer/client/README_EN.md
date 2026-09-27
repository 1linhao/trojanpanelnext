# Unified deployment configuration: local initialization

[简体中文](README.md)

On Linux, macOS, or Windows WSL, provide Bash, curl, tar, OpenSSH, `sha256sum` or `shasum`, and a user-installed [mikefarah yq v4](https://github.com/mikefarah/yq#install). `init` installs no dependencies and does not contact a VPS. Confirm the fixed tag and archive SHA256 from the matching GitHub Release. The SHA below belongs only to rc.3.

```bash
bash deploy/installer/client/tpnext.sh init \
  --config "$PWD/deployment.local.yaml" \
  --tag v0.1.0-rc.3 \
  --sha256 fe4e2b297756bf3a58db31f69636dd1ca8034196ef14e1184db14c4f8362d668
bash deploy/installer/client/tpnext.sh plan --config "$PWD/deployment.local.yaml"
```

The default template describes an external computer reaching separate Web and Node VPS hosts by SSH. If the operator runs on the Web VPS, pass `--web-transport local`; that example maps one Node to the same `web-host` and plans a `combined` installation. You may edit the YAML to move that Node to another SSH host or append independent Nodes. For a remote `combined` host, map the Node to the Web host and set `node_caddy_https_port` to 443; the shared Entry uses 80/443. At most one Node may share the Web host, and their domains must differ. Only the same explicit host id means the same machine; IPs and SSH aliases do not establish co-location.

`hosts` holds execution transport, while `web.domain`, `web.public_ip`, and each Node's `domain` and `public_ip` hold service addresses. SSH can use a root key, or an ordinary user's key with `sudo -n` in later deployment. `node_key` is the stable key for a future Node state path; appending or reordering the array preserves existing paths. Removing or renaming a Node does not authorize uninstall, revocation, or rotation. `plan` only validates YAML and topology and prints a secret-free per-host plan. This ticket provides no `check` or `deploy` command.

The config file is mode 0600 and the verified asset work directory is mode 0700. Use a `*.local.yaml` config name; inside a Git worktree, the CLI requires both config and work directory to be ignored by Git. Existing paths are never overwritten. Do not copy the Web-privileged unified YAML to a Node VPS.
