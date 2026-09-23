# 多模型协作流程

本流程适用于 GPT-6-sol（high）与 GPT-6-luna（xhigh）的多会话开发。新工单按风险和资格条件分派；旧模型标签只用于识别更新前的历史工单。

## Skill 主链

1. `/grill-with-docs` 与 `/domain-modeling` 对齐需求、词汇和 ADR。
2. `/research` 仅调查需要一手资料确认的外部技术事实。
3. `/to-spec` 由 GPT-6-sol（high）生成中文规格 issue，并先请用户确认测试 seam。
4. `/to-tickets` 由 GPT-6-sol（high）拆成单会话可完成的纵向切片，声明 blocker、模型等级和风险。
5. 每张工单在独立 branch 与 worktree 中由新会话运行 `/implement`；内部尽量使用 `/tdd`。
6. 每张实现工单创建一个 PR，并由独立 GPT-6-sol（high）会话运行 `/code-review`。
7. blockers-first 合并；下游审查前同步最新 `main` 并重跑测试。
8. 所有工单合并后，由独立 GPT-6-sol（high）集成会话运行完整 CI、Docker 冒烟和规格回归。
9. 真实 VPS、DNS 和云安全组操作必须等待人工批准；需要人工逐步操作时使用 `/wizard`。

`/handoff` 只在上下文将满或需要分支诊断时使用。常规交接以 Issue、PR、commit、`CONTEXT.md` 和 ADR 为准。难以复现的失败使用 `/diagnosing-bugs`。

## 模型职责

### GPT-6-sol（high）

负责所有规格、拆票、ADR、安全与凭据设计、跨模块接口、中高风险实现、调度、审查、集成和真实环境诊断。除下述合格的低风险实现外，其他工作均使用 `agent:sol`。所有代码审查都由 GPT-6-sol（high）承担；Luna 实现的 PR 必须由独立 Sol 会话完成 Standards 与 Spec 双轴审查，高风险 Sol PR 也必须由另一个 Sol 会话审查。Sol 调度会话维护 frontier，但不代替实现会话。

### GPT-6-luna（xhigh）

只领取同时满足以下条件的低风险实现工单：

- 标签为 `agent:luna` 和 `risk:low`。
- 所有 blocker 已关闭。
- 没有未决架构或安全决策。
- 验收标准、允许修改的模块边界和验证命令完整。

符合以上条件时，使用 GPT-6-luna（xhigh）；中风险、高风险及不符合任一条件的实现工单使用 GPT-6-sol（high）。规格、架构与安全决策、调度、集成和所有复审一律使用 GPT-6-sol（high）。

Luna 遇到 ADR 冲突、跨模块重构、安全选择、无法稳定复现的测试或需要扩大范围时，必须停止修改，在 issue 中记录命令、输出和已改文件，并升级给 GPT-6-sol（high）。

## 工单契约

每张实现工单必须包含：

- 父规格与 blocker。
- `推荐模型`：`agent:luna` 或 `agent:sol`。只有满足本文件 Luna 资格条件的低风险实现工单使用 `agent:luna`；其他工单使用 `agent:sol`。
- `风险`：`risk:low|medium|high`。
- 从用户视角描述的端到端交付内容。
- 可观察的成功和失败行为。
- 允许修改的模块边界。
- 明确非目标。
- 停止与升级条件。
- 局部测试和完整验证命令。
- 完成后应附带的验证证据。

不要给 Luna 预写容易过时的逐行实现步骤。

## Git 基线与 worktree

- 本功能的固定代码基线为 `50d039098efcc7a2feebee87248cad2ca2348f4e`，位于 `codex/entry-provider-nginx-certbot`。
- 该提交相对 `origin/main` 领先 8 个入口、external TLS、EntrySpec、Node route manifest 和 mTLS 相关提交，是本规格依赖的现有能力。
- 本轮 `AGENTS.md`、领域词汇、ADR、协作规则和规格形成独立的协作基线提交；远端集成分支为 `codex/entry-provider-nginx-certbot`。
- 本功能完成前，每张实现工单的 branch 与 worktree 必须从远端集成分支的最新已合并状态派生，PR 也以该集成分支为 base；禁止从脏工作区复制。
- blocker PR 合并后，下游分支先同步集成分支并重跑测试。整组功能验收后，再由集成分支向 `main` 提交最终 PR。
- `/code-review` 的固定点使用该 PR 创建时的集成分支 merge-base，并在审查前同步 blocker 的合并结果。

## 分支、审查与合并

- 一个会话只领取一张工单；多个会话不得共享同一个工作树。
- 每票独立 branch、worktree 和 PR。
- Luna PR 必须由独立 GPT-6-sol（high）做 Standards 与 Spec 双轴审查。
- `risk:high` 的 Sol PR 也必须由另一个 GPT-6-sol（high）会话审查。
- 默认按依赖图 blockers-first 合并，只并行不存在接口或文件所有权冲突的 frontier。

## 人工门禁

以下阶段必须等待用户明确批准：

- 规格与工单依赖图。
- 安全、凭据和身份生命周期 ADR。
- 真实 VPS、DNS、ACME、云安全组和清理操作。
