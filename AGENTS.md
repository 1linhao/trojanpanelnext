# AGENTS.md

## Agent skills

### Issue tracker

规格与工单统一使用 GitHub Issues。详见 `docs/agents/issue-tracker.md`。

### Triage labels

使用默认五态分诊标签。详见 `docs/agents/triage-labels.md`。

### Domain docs

采用 single-context：开始工作前读取根目录 `CONTEXT.md`，再读取相关的 `docs/adr/`。详见 `docs/agents/domain.md`。

### Multi-agent workflow

多模型会话按 GitHub 依赖图、独立 branch/worktree 和每票一个 PR 协作。模型职责、质量门禁与越界升级规则见 `docs/agents/workflow.md`。
