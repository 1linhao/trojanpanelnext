# 问题跟踪器：GitHub

本仓库的规格与工单统一存放在 GitHub Issues 中，所有操作使用 `gh` CLI。

## 基本约定

- 创建 issue：`gh issue create --title "..." --body "..."`；多行正文使用 heredoc。
- 读取 issue：`gh issue view <number> --comments`，同时读取标签。
- 评论 issue：`gh issue comment <number> --body "..."`。
- 添加或移除标签：`gh issue edit <number> --add-label "..."` 或 `--remove-label "..."`。
- 关闭 issue：`gh issue close <number> --comment "..."`。
- Skill 要求“发布到问题跟踪器”时，创建 GitHub issue；要求“读取相关工单”时，读取完整正文和评论。

## 请求与交付边界

Pull Request 默认不作为外部需求或缺陷的分诊入口。`/to-spec` 生成规格 issue，`/to-tickets` 生成可执行工单；这些工单已经 Agent-ready，不再经过 `/triage`。每张实现工单对应一个 PR，PR 必须引用其工单和父规格。

## 子工单与阻塞关系

- 父级工作使用一个规格 issue，子工作优先使用 GitHub sub-issue。
- 阻塞关系优先使用 GitHub 原生 issue dependencies；不可用时在正文顶部写 `Blocked by: #<n>, #<n>`。
- 只有所有 blocker 均关闭且无人领取的工单才属于 frontier。
- Agent 领取工单时先执行 `gh issue edit <n> --add-assignee @me`。
- 完成后在 issue 与 PR 中记录实现摘要、验证命令和结果。
