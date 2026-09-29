# PR #18 独立代码审查

- PR：https://github.com/1linhao/trojanpanelnext/pull/18
- PR 审查评论：https://github.com/1linhao/trojanpanelnext/pull/18#issuecomment-5771142240
- 固定 base：`03b4842cd7a7f36163a467f54264335ce883e96e`
- 固定 head：`2f57c69885bd3e04ae8b81c0a2071b5f3217919b`
- 三点 diff：`git diff 03b4842cd7a7f36163a467f54264335ce883e96e...2f57c69885bd3e04ae8b81c0a2071b5f3217919b`
- 结论：**需修改**。Standards 2 项（P1 1、P2 1）；Spec 0 项。

## 规范符合度

### P1：前置条件失败仍会删除宿主既有 `/tpdata`

`deploy/installer/tests/release_web_docker_smoke_test.sh:17-23` 先注册 `EXIT` trap，随后才检查 `/tpdata` 是否已存在。若宿主已有真实部署，`fail` 仍会触发 `cleanup`，无条件执行 `sudo -n rm -rf -- /tpdata`；同时还会尝试删除固定名称的产品容器。这会把“拒绝在非空宿主运行”的保护反转为破坏性清理。

这违反 `docs/agents/workflow.md` 的人工清理门禁。应在注册清理前完成前置检查，并像 Node 集成测试一样只删除本测试已确认创建、且有 ownership 标记的目录与容器。

### P2：失败诊断可能先把凭据原样写入 CI 日志

`deploy/installer/tests/release_web_docker_smoke_test.sh:85-96` 在安装失败时先原样输出 `install.out`/`install.err`，成功时也只扫描固定的管理员密码；安装器写回配置的 MariaDB、Redis 随机密码没有被读取和扫描。若被测实现回归并泄漏这些凭据，失败分支会先将其发布到 CI 日志，无法验证 Issue #9 要求的“失败输出不泄露凭据”。

这与 `deploy/installer/README.md:67-75,185` 的无秘密日志契约冲突。应在输出诊断前从 `0600` 配置读取三类实际凭据、检查并脱敏日志；最好同时加入真实 Docker 健康失败注入，证明定位信息与不泄密可同时成立。

Standards 合计：**2**（P1 1、P2 1）；smell 判断项：**0**。最严重问题：失败前置检查会删除既有 `/tpdata`。

## 需求符合度

Spec 合计：**0**（P0/P1/P2/P3 均为 0），未发现缺失、错误实现或范围蔓延。

Issue #9 要求的正式 release bundle 与 `install.sh` 入口、真实 Web TLS/管理员 API/MariaDB/Redis、Node 独立身份及 Web→Node mTLS/gRPC 均有强断言；`docker-smoke` 已进入 `gate.needs`，无成功跳过路径。资产、manifest、helper、tag-only 镜像、替换 digest、混合版本和 schema 篡改均有失败断言。真实 Debian 12 VPS、DNS、ACME 与 combined 拓扑属于 Issue #10，本 PR 未越界宣称完成。

Spec 最严重问题：无。

## 验证证据

- ref、merge-base 与非空三点 diff：通过；merge-base 等于固定 base。
- `bash deploy/installer/tests/release_assets_test.sh`：通过；包含 mixed-version、config schema 与 manifest schema 明确拒绝 trace。
- `bash deploy/installer/tests/jq_free_release_verification_test.sh`：通过；资产、manifest、helper、tag-only 和替换 digest 均在宿主变更前拒绝。
- `bash deploy/installer/tests/web_bare_install_test.sh`：通过。
- 变更 shell `bash -n`、`git diff --check`、README 语言契约：通过。
- `npx --yes prettier@3.7.4 --check .github/workflows/ci.yml .github/workflows/publish-images.yml`：通过。
- GitHub Actions（精确 head `2f57c69`）：8/8 通过；`released Web and Node Docker smoke` 与最终 `gate` 均成功。
- 本地未运行 `docker_smoke_test.sh`：审查机已有 `/tpdata`，运行会触发上述 P1 的破坏性路径；真实 Docker 结果以远端精确 HEAD CI 为证据。

## 汇总

Standards：2 项，最严重为 P1 的既有数据删除风险；Spec：0 项，无最严重问题。PR 在修复 Standards 问题前不应合并。
