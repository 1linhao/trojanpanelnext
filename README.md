# TrojanPanel

TrojanPanel 产品 monorepo。控制面、Web、节点 Agent、公共安装器和文档站从
workspace 基线 `444822da8c8c3aff7635607409b980e9aaf3bf8c` 迁移到一个无父提交的安全净化 root commit。

迁移前历史保留在原组件仓库；机器可读来源见
`migration/source-baseline.json`。其中逐项记录了原始 commit/tree、导入策略、
示例凭据脱敏和已跟踪文档编译产物排除情况。

本次迁移使用的映射、生成器和净化器已固化在 `migration/tooling/`；生成器可通过
`--source-workspace` 指向保留的旧 workspace 后重放。

当前目录是 M0 内容快照。统一工作区、契约、CI 和发布流程将在后续独立提交中建立。
