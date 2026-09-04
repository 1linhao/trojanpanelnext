# Migration evidence

此目录记录从旧 workspace `444822da8c8c3aff7635607409b980e9aaf3bf8c` 创建 M0 根提交的来源、策略和工具：

- `source-baseline.json`：来源 commit/tree、导入策略、净化计数；
- `tooling/snapshot-map.json`：组件到 monorepo 的固定映射；
- `tooling/sanitize-snapshot.mjs`：可失败关闭的确定性净化规则；
- `tooling/create-snapshot`：预检、导入、提交和结构验证入口。

运行时证据（测试、泄露扫描、本地 source bundle）位于被忽略的 `.local/`，不会随
产品仓库发布；正式归档时应将其复制到受控的迁移档案位置。
