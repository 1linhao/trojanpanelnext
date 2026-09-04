package model

import "time"

type KernelReleaseCache struct {
	Kernel    string
	Channel   string
	Payload   string
	ETag      string
	FetchedAt time.Time
	Error     string
}

type KernelUpgradeTask struct {
	Id           uint64                  `json:"id" ddb:"id"`
	OperatorId   uint                    `json:"operatorId" ddb:"operator_id"`
	OperatorName string                  `json:"operatorName" ddb:"operator_name"`
	CanaryNodeId uint                    `json:"canaryNodeId" ddb:"canary_node_id"`
	Status       string                  `json:"status" ddb:"status"`
	CreatedAt    time.Time               `json:"createdAt" ddb:"create_time"`
	UpdatedAt    time.Time               `json:"updatedAt" ddb:"update_time"`
	Items        []KernelUpgradeTaskItem `json:"items" ddb:"-"`
}

type KernelUpgradeTaskItem struct {
	Id              uint64    `json:"id" ddb:"id"`
	TaskId          uint64    `json:"taskId" ddb:"task_id"`
	NodeServerId    uint      `json:"nodeServerId" ddb:"node_server_id"`
	NodeServerName  string    `json:"nodeServerName" ddb:"node_server_name"`
	Kernel          string    `json:"kernel" ddb:"kernel_name"`
	FromVersion     string    `json:"fromVersion" ddb:"from_version"`
	TargetVersion   string    `json:"targetVersion" ddb:"target_version"`
	Channel         string    `json:"channel" ddb:"channel_name"`
	Action          string    `json:"action" ddb:"action_name"`
	SHA256          string    `json:"sha256" ddb:"sha256"`
	Stage           string    `json:"stage" ddb:"stage"`
	Result          string    `json:"result" ddb:"result"`
	Error           string    `json:"error" ddb:"error_message"`
	RollbackResult  string    `json:"rollbackResult" ddb:"rollback_result"`
	CoreOperationId string    `json:"coreOperationId" ddb:"core_operation_id"`
	IdempotencyKey  string    `json:"idempotencyKey" ddb:"idempotency_key"`
	Attempt         uint      `json:"attempt" ddb:"attempt"`
	CreatedAt       time.Time `json:"createdAt" ddb:"create_time"`
	UpdatedAt       time.Time `json:"updatedAt" ddb:"update_time"`
}
