# PR #18 最终 HEAD 独立复审

- PR：https://github.com/1linhao/trojanpanelnext/pull/18
- 本轮 PR 评论：https://github.com/1linhao/trojanpanelnext/pull/18#issuecomment-5795713418
- 固定 base：`64af248106e326e13097713c7af999eea4e97c2c`（`origin/codex/entry-provider-nginx-certbot`）
- 固定 head：`b10919b6d48be1afe871c0ca2fe76796244b44e2`
- 比较命令：`git diff origin/codex/entry-provider-nginx-certbot...b10919b6d48be1afe871c0ca2fe76796244b44e2`
- 范围：`6d67a43`、`818ef60`、`b10919b`；merge-base 等于固定 base，diff 非空。
- 结论：**Standards 0 / Spec 0**；首轮两项问题已修复，本轮未发现阻断项。

## 规范符合度

**0 项**（P0/P1/P2/P3 均为 0；smell 判断项 0）。

首轮 P1：`release_web_docker_smoke_test.sh:15-62,168-173` 将数据放在每次运行创建的临时工作区，先拒绝已占用目录和容器名，再按创建标记清理。`release_web_docker_smoke_cleanup_test.sh:34-56` 证明已存在的数据目录和容器不会被测试清理。

首轮 P2：`fixtures/release_web_smoke_helpers.sh:3-70` 从实际配置读取 `sysadmin`、MariaDB、Redis 密码，诊断先逐行脱敏再输出；`release_web_docker_smoke_test.sh:174-190,228-253` 对真实健康失败及成功输出均检查不泄漏。`release_web_docker_smoke_cleanup_test.sh:58-81` 覆盖三种密码的脱敏回归。

负向注入后的恢复：`release_web_docker_smoke_test.sh:192-245` 验证管理员凭据确实失效，将隔离的凭据文件恢复为配置值和恰好一个换行，再验证宿主、容器内与只读 verifier 状态；最后重新执行正式安装并通过 HTTPS 管理员登录。

本轴最严重问题：无。

## 需求符合度

**0 项**（缺失或部分实现 0、范围蔓延 0、表面实现但行为错误 0）。

Issue #9 的四条验收均有对应证据：发布资产、摘要、attestation 元数据、镜像 digest 和 schema 契约在 `release_assets_test.sh` 与 `jq_free_release_verification_test.sh` 中覆盖；Web 通过正式发布包的 `install.sh` 安装真实 API/UI/MariaDB/Redis，并验证 TLS 和管理员 API；Node 通过加密 Node 引导包、正式安装入口、独立数据层身份以及 Web→Node mTLS/gRPC 断言；`.github/workflows/ci.yml:154-198` 将 Docker smoke 纳入最终 `gate`，非成功结果会阻断。

真实 Debian 12 VPS、DNS、ACME 和 combined 拓扑验收属于 Issue #10，本 PR 未宣称完成该阶段。

本轴最严重问题：无。

## 验证证据与边界

- 本地通过：`release_web_docker_smoke_cleanup_test.sh`、`release_assets_test.sh`、`jq_free_release_verification_test.sh`、`web_bare_install_test.sh`、变更脚本的 `bash -n`、README 语言检查、`git diff --check`、两份 workflow 的 Prettier 检查。
- GitHub Actions run [35865492440](https://github.com/1linhao/trojanpanelnext/actions/runs/35865492440) 对精确 head `b10919b` 为 8/8 成功，包含 [Web/Node Docker smoke](https://github.com/1linhao/trojanpanelnext/actions/runs/35865492440/job/107195896926) 和最终 [gate](https://github.com/1linhao/trojanpanelnext/actions/runs/35865492440/job/107196978379)。
- 本审查工作区没有再次运行完整 Docker smoke；真实 Docker 执行以该精确提交的远端 CI 为证据。未修改实现、提交或合并 PR。

汇总：Standards **0**，最严重问题无；Spec **0**，最严重问题无。
