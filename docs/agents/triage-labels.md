# 分诊标签

工程 Skills 使用五种固定分诊角色：

| Skill 角色        | GitHub 标签       | 含义                            |
| ----------------- | ----------------- | ------------------------------- |
| `needs-triage`    | `needs-triage`    | 维护者尚未评估该问题            |
| `needs-info`      | `needs-info`      | 等待报告者补充信息              |
| `ready-for-agent` | `ready-for-agent` | 信息完整，可交给 Agent 独立处理 |
| `ready-for-human` | `ready-for-human` | 需要人工判断或操作              |
| `wontfix`         | `wontfix`         | 已决定不处理                    |

实现工单另用 `agent:luna` 或 `agent:sol` 标记推荐模型：仅当工单为低风险、所有 blocker 已关闭、没有未决架构或安全决策，且验收标准、模块边界和验证命令完整时，才使用 `agent:luna`（GPT-6-luna，xhigh）；其他实现工单使用 `agent:sol`（GPT-6-sol，high）。规格、架构与安全决策、调度、集成和所有复审也使用 `agent:sol`（GPT-6-sol，high）。风险仍使用 `risk:low`、`risk:medium` 或 `risk:high` 标记。

`agent:terra` 与 `agent:gpt` 是更新前工单的历史标签，保留用于识别已有工单，不用于新工单，也不应追溯改变历史工单的模型语义。`/to-tickets` 生成的工单直接标记 `ready-for-agent`，不重复分诊。
