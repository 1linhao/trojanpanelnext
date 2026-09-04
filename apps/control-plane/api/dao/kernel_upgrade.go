package dao

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"trojan-panel/model"
)

func SelectKernelReleaseCache(kernel, channel string) (*model.KernelReleaseCache, error) {
	var cache model.KernelReleaseCache
	err := db.QueryRow(`SELECT kernel_name,channel_name,payload,etag,fetched_at,error_message
		FROM kernel_release_cache WHERE kernel_name=? AND channel_name=?`, kernel, channel).
		Scan(&cache.Kernel, &cache.Channel, &cache.Payload, &cache.ETag, &cache.FetchedAt, &cache.Error)
	if err != nil {
		return nil, err
	}
	return &cache, nil
}

func UpsertKernelReleaseCache(cache model.KernelReleaseCache) error {
	_, err := db.Exec(`INSERT INTO kernel_release_cache
		(kernel_name,channel_name,payload,etag,fetched_at,error_message) VALUES (?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE payload=VALUES(payload),etag=VALUES(etag),
		fetched_at=VALUES(fetched_at),error_message=VALUES(error_message)`,
		cache.Kernel, cache.Channel, cache.Payload, cache.ETag, cache.FetchedAt, cache.Error)
	return err
}

func CreateKernelUpgradeTask(task *model.KernelUpgradeTask, items []model.KernelUpgradeTaskItem) error {
	transaction, err := db.Begin()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	result, err := transaction.Exec(`INSERT INTO kernel_upgrade_task
		(operator_id,operator_name,canary_node_id,status) VALUES (?,?,?,'queued')`,
		task.OperatorId, task.OperatorName, task.CanaryNodeId)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	task.Id = uint64(id)
	for index := range items {
		item := &items[index]
		item.TaskId = task.Id
		result, err = transaction.Exec(`INSERT INTO kernel_upgrade_task_item
			(task_id,node_server_id,node_server_name,kernel_name,target_version,channel_name,action_name,
			 stage,idempotency_key,attempt) VALUES (?,?,?,?,?,?,?,'queued',?,?)`,
			item.TaskId, item.NodeServerId, item.NodeServerName, item.Kernel,
			item.TargetVersion, item.Channel, item.Action, item.IdempotencyKey, item.Attempt)
		if err != nil {
			return err
		}
		itemId, lastErr := result.LastInsertId()
		if lastErr != nil {
			return lastErr
		}
		item.Id = uint64(itemId)
	}
	return transaction.Commit()
}

func SelectKernelUpgradeTask(id uint64) (*model.KernelUpgradeTask, error) {
	var task model.KernelUpgradeTask
	err := db.QueryRow(`SELECT id,operator_id,operator_name,canary_node_id,status,create_time,update_time
		FROM kernel_upgrade_task WHERE id=?`, id).
		Scan(&task.Id, &task.OperatorId, &task.OperatorName, &task.CanaryNodeId, &task.Status, &task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		return nil, err
	}
	items, err := selectKernelTaskItems("task_id=?", id)
	if err != nil {
		return nil, err
	}
	task.Items = items
	return &task, nil
}

func SelectKernelUpgradeTaskPage(pageNum, pageSize uint, status string) ([]model.KernelUpgradeTask, uint, error) {
	where := ""
	args := make([]any, 0, 3)
	if status != "" {
		where = " WHERE status=?"
		args = append(args, status)
	}
	var total uint
	if err := db.QueryRow("SELECT COUNT(1) FROM kernel_upgrade_task"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, (pageNum-1)*pageSize, pageSize)
	rows, err := db.Query(`SELECT id,operator_id,operator_name,canary_node_id,status,create_time,update_time
		FROM kernel_upgrade_task`+where+` ORDER BY create_time DESC LIMIT ?,?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var tasks []model.KernelUpgradeTask
	for rows.Next() {
		var task model.KernelUpgradeTask
		if err = rows.Scan(&task.Id, &task.OperatorId, &task.OperatorName, &task.CanaryNodeId, &task.Status, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, task)
	}
	return tasks, total, rows.Err()
}

func SelectKernelTaskItemsByStage(stages ...string) ([]model.KernelUpgradeTaskItem, error) {
	if len(stages) == 0 {
		return []model.KernelUpgradeTaskItem{}, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(stages)), ",")
	args := make([]any, len(stages))
	for index := range stages {
		args[index] = stages[index]
	}
	return selectKernelTaskItems("stage IN ("+placeholders+")", args...)
}

func selectKernelTaskItems(where string, args ...any) ([]model.KernelUpgradeTaskItem, error) {
	query := `SELECT id,task_id,node_server_id,node_server_name,kernel_name,from_version,
		target_version,channel_name,action_name,sha256,stage,result,error_message,rollback_result,
		core_operation_id,idempotency_key,attempt,create_time,update_time
		FROM kernel_upgrade_task_item WHERE ` + where + ` ORDER BY id`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.KernelUpgradeTaskItem, 0)
	for rows.Next() {
		var item model.KernelUpgradeTaskItem
		if err = rows.Scan(&item.Id, &item.TaskId, &item.NodeServerId, &item.NodeServerName,
			&item.Kernel, &item.FromVersion, &item.TargetVersion, &item.Channel, &item.Action, &item.SHA256,
			&item.Stage, &item.Result, &item.Error, &item.RollbackResult,
			&item.CoreOperationId, &item.IdempotencyKey, &item.Attempt,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func UpdateKernelTaskItem(item model.KernelUpgradeTaskItem) error {
	_, err := db.Exec(`UPDATE kernel_upgrade_task_item SET from_version=?,sha256=?,stage=?,
		result=?,error_message=?,rollback_result=?,core_operation_id=?,attempt=?,idempotency_key=? WHERE id=?`,
		item.FromVersion, item.SHA256, item.Stage, item.Result, item.Error,
		item.RollbackResult, item.CoreOperationId, item.Attempt, item.IdempotencyKey, item.Id)
	return err
}

func ResetKernelTaskItems(taskId uint64, itemIds []uint64) ([]model.KernelUpgradeTaskItem, error) {
	where := "task_id=? AND (stage='failed' OR stage='rolled_back')"
	args := []any{taskId}
	if len(itemIds) > 0 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(itemIds)), ",")
		where += " AND id IN (" + placeholders + ")"
		for _, id := range itemIds {
			args = append(args, id)
		}
	}
	items, err := selectKernelTaskItems(where, args...)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index].Attempt++
		items[index].IdempotencyKey = fmt.Sprintf("task-%d-item-%d-attempt-%d", taskId, items[index].Id, items[index].Attempt)
		items[index].Stage = "queued"
		items[index].Result = ""
		items[index].Error = ""
		items[index].RollbackResult = ""
		items[index].CoreOperationId = ""
		if _, err = db.Exec(`UPDATE kernel_upgrade_task_item SET stage='queued',result='',
			error_message='',rollback_result='',core_operation_id='',attempt=?,idempotency_key=? WHERE id=?`,
			items[index].Attempt, items[index].IdempotencyKey, items[index].Id); err != nil {
			return nil, err
		}
	}
	if len(items) > 0 {
		_, err = db.Exec("UPDATE kernel_upgrade_task SET status='queued' WHERE id=?", taskId)
	}
	return items, err
}

func RefreshKernelTaskStatus(taskId uint64) error {
	var total, success, failed, active uint
	err := db.QueryRow(`SELECT COUNT(1),SUM(stage='succeeded'),SUM(stage IN ('failed','rolled_back')),
		SUM(stage NOT IN ('succeeded','failed','rolled_back')) FROM kernel_upgrade_task_item WHERE task_id=?`, taskId).
		Scan(&total, &success, &failed, &active)
	if err != nil {
		return err
	}
	status := "running"
	switch {
	case active > 0:
		status = "running"
	case success == total:
		status = "succeeded"
	case failed == total:
		status = "failed"
	default:
		status = "partial"
	}
	_, err = db.Exec("UPDATE kernel_upgrade_task SET status=? WHERE id=?", status, taskId)
	return err
}

func CleanupKernelUpgradeAudit(before time.Time) (int64, error) {
	result, err := db.Exec("DELETE FROM kernel_upgrade_task WHERE create_time < ?", before)
	if err != nil {
		return 0, err
	}
	_, _ = db.Exec("DELETE i FROM kernel_upgrade_task_item i LEFT JOIN kernel_upgrade_task t ON t.id=i.task_id WHERE t.id IS NULL")
	return result.RowsAffected()
}

func MarkInterruptedKernelItemsFailed() error {
	_, err := db.Exec(`UPDATE kernel_upgrade_task_item SET stage='failed',result='failed',
		error_message='panel restarted while operation state was unavailable'
		WHERE stage NOT IN ('queued','succeeded','failed','rolled_back') AND core_operation_id=''`)
	return err
}

func isNoRows(err error) bool { return err == sql.ErrNoRows }
