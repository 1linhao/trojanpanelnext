# 分诊标签

工程 Skills 使用五种固定分诊角色：

| Skill 角色 | GitHub 标签 | 含义 |
| --- | --- | --- |
| `needs-triage` | `needs-triage` | 维护者尚未评估该问题 |
| `needs-info` | `needs-info` | 等待报告者补充信息 |
| `ready-for-agent` | `ready-for-agent` | 信息完整，可交给 Agent 独立处理 |
| `ready-for-human` | `ready-for-human` | 需要人工判断或操作 |
| `wontfix` | `wontfix` | 已决定不处理 |

实现工单另用 `agent:flash` 或 `agent:gpt` 标记最低模型等级，并用 `risk:low`、`risk:medium` 或 `risk:high` 标记风险。`/to-tickets` 生成的工单直接标记 `ready-for-agent`，不重复分诊。
