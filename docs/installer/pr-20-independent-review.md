# PR #20 独立代码审查（2026-09-23）

**结论：Request Changes。** Standards 2 项（最重 P1）；Spec 6 项（最重 P1）。固定比较点 `03b4842cd7a7f36163a467f54264335ce883e96e...a1b65c2d3dde07ea68a534a774a3291b286f048f`；merge-base 为前者，差异非空且只有提交 `a1b65c2`。规格来源为 Issue #7、父规格 #1、`CONTEXT.md` 与 ADR-0001～0004。P1 是合并阻断；P2 应在本 PR 修正或提供可验证的范围裁定。

## 规范符合度（Standards：2 项，最重 P1）

1. **P1 — combined 绕过 EntryController。** ADR-0001 明确安装器只学习 `EntryController`，入口 Adapter 内部处理证书、入口、持久所有权与切换事务。`deploy/installer/install.sh:2767-2769` 直接写 Caddyfile、启动 Caddy；`:2822-2833` 直接重写配置并重启。`validate_entry_spec_binding`（`:400-406`）只允许 external，combined 又强制 ACME（`:918`），因此此部署模式无法进入既定 EntryController seam。应让共享入口作为一个受控 Entry Deployment 收敛，并覆盖失败恢复。
2. **P1 — 卸载未核验所删除资源的所有权。** `CONTEXT.md` 定义没有所有权的资源只能观测；ADR-0001 要求入口资源所有权持久化。`deploy/installer/install.sh:2808` 仅核验 Caddy 的环境变量，`:2837-2846` 却按固定容器名删除 Web、Node、MariaDB、Redis，并在 purge 时删除对应目录。无法证明这些资源仍属本次部署；应逐项核验所有权及数据边界后删除。

未将风格启发式或工具已覆盖的问题列为规范违规。

## 需求符合度（Spec：6 项，最重 P1）

1. **P1 — 80/443 内部争抢未在变更前拒绝。** Issue #7 要求只有共享入口拥有 80/443，并校验端口前提。`deploy/installer/install.sh:919-925` 仅排除 Core 和 gRPC 端口；`UI_PORT`、`PANEL_PORT`、`MARIADB_PORT`、`REDIS_PORT` 均可配置为 80/443。以 `ui_port: 80` 为例，宿主端口预检可通过，但 `deploy_combined`（`:2754-2768`）先在 host network 启动 UI（`:2270-2274`），之后 Caddy 无法取得 80。既有测试 `combined_install_test.sh:240-247` 只模拟外部占用。
2. **P1 — ACME 失败没有共享入口切换/回滚边界。** Issue #7 要求单个共享入口服务两个域名，ADR-0001 要求失败恢复。`deploy/installer/install.sh:2749-2769` 已启动数据、Web、共享 Caddy；`wait_for_combined_certs`（`:2027-2031`）如第二域名签发失败，`wait_for_cert`（`:1997-2024`）仅超时退出，保留部分部署和入口配置。运行中的同名 Caddy 使宿主端口检查直接跳过（`:961-974`），也未证明实际监听归属。需明确失败状态与恢复行为。
3. **P1 — 破坏性域名变更未事先拒绝。** 父规格 #1 要求同版本重跑对域名等破坏性变化在修改前拒绝。当前 `validate_combined_entry_preconditions`（`deploy/installer/install.sh:909-937`）只验本次输入，无已部署域名对比。更换 Web 域名后，`:2749-2759` 先改写配置/启动容器，`:2767` 覆写 Caddyfile；运行中的 Caddy 在 `start_caddy`（`:1975-1977`）直接返回，不重新加载。安装随后等待新证书失败，运行态和文件态分叉。
4. **P1 — 先卸载 Web 再卸载 Node 会恢复已移除域名。** Issue #7 要求按角色卸载不误动另一角色资源。`deploy/installer/install.sh:2822` 在卸载 Web 时只保留 Node 域名；下一次卸载 Node 无条件执行 `write_web_caddyfile` 并重启 Caddy（`:2832-2833`），令已移除 Web 域名再次提供入口/尝试 ACME。`combined_install_test.sh:338` 甚至把该行为断言为成功。需记录剩余角色状态并据此调整或移除共享入口。
5. **P2 — 同配置重跑每次重建 Core。** 父规格 #1 要求同版本安全收敛。`deploy/installer/install.sh:2995-2996` 每次生成新的 challenge（`:625-630`），写入 Core 配置（`:1834`）；配置哈希检查（`:2540-2545`）因此每次变化并执行 `docker rm -f`（`:1160-1161`）。`combined_install_test.sh:292-298` 只确认没有第二个 Entry、没有 `docker restart core`，没有断言 Core 不被删除重建。重跑会中断 Node 服务，应使仅用于健康验证的 challenge 不触发持久配置重建。
6. **P1 — combined 验收未进入 CI，Docker 冒烟不覆盖真实安装路径。** Issue #7 要求 combined Docker 冒烟和双域名/监听断言；父规格 #1 要求 HTTPS、认证、数据层和 mTLS/gRPC 健康。`.github/workflows/ci.yml:125-135` 的安装器步骤未调用新增 `combined_install_test.sh` 或 `combined_docker_smoke_test.sh`。后者在 `combined_docker_smoke_test.sh:30-64` 自建三个 Caddy 容器与 `tls internal`，没有执行 `install.sh`、真实 Web/Node 镜像或公网 ACME。假宿主测试模拟 `docker`、`curl`、`openssl` 与 mTLS 返回值（`combined_install_test.sh:67-205`）；真实 mTLS/gRPC CLI 在安装器中调用，但新增测试未端到端执行该部署路径。需把 combined 测试接入 CI，并让冒烟覆盖实际编排；真实 VPS/公网 ACME 按人工门禁另行验收。

## 已验证与限制

- 固定 diff 的 `git diff --check`、相关 Bash 语法检查通过。
- 本地 `combined_install_test.sh`、`installer_cli_test.sh` 和 `combined_docker_smoke_test.sh` 通过。Docker 测试仅使用本机隔离容器，未接触真实 VPS。
- PR #20 当前 7 项 CI 检查均通过；其中 installer scripts 未执行新增 combined 测试。
- 凭据检查：新增 combined Core 使用独立 MariaDB/Redis 身份，运行时配置和凭据文件受限，当前未发现新秘密直接打印；这不能替代真实容器的身份和日志验收。
- 未修改实现、未提交、未合并、未触碰真实 VPS/DNS/云安全组。
