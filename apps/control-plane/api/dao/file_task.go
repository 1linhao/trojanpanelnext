package dao

import (
	"errors"
	"github.com/didi/gendry/builder"
	"github.com/didi/gendry/scanner"
	"github.com/sirupsen/logrus"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
)

func SelectFileTaskPage(taskType *uint, accountUsername *string, pageNum *uint, pageSize *uint) (*vo.FileTaskPageVo, error) {
	var (
		total     uint
		fileTasks []model.FileTask
	)

	// 查询总数
	var whereCount = map[string]interface{}{}
	if taskType != nil && *taskType != 0 {
		whereCount["`type`"] = *taskType
	}
	if accountUsername != nil && *accountUsername != "" {
		whereCount["`account_username`"] = *accountUsername
	}
	selectFieldsCount := []string{"count(1)"}
	buildSelect, values, err := builder.BuildSelect("file_task", whereCount, selectFieldsCount)
	if err = db.QueryRow(buildSelect, values...).Scan(&total); err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	// 分页查询
	where := map[string]interface{}{
		"_orderby": "create_time desc",
		"_limit":   []uint{(*pageNum - 1) * *pageSize, *pageSize}}
	if taskType != nil && *taskType != 0 {
		where["`type`"] = *taskType
	}
	if accountUsername != nil && *accountUsername != "" {
		where["`account_username`"] = *accountUsername
	}
	selectFields := []string{"id", "name", "`type`", "status", "err_msg", "account_username", "create_time"}
	selectSQL, values, err := builder.BuildSelect("file_task", where, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	rows, err := db.Query(selectSQL, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()

	if err = scanner.Scan(rows, &fileTasks); err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	var fileTaskVos = make([]vo.FileTaskVo, 0)
	for _, item := range fileTasks {
		fileTaskVos = append(fileTaskVos, vo.FileTaskVo{
			Id:              *item.Id,
			Name:            *item.Name,
			Type:            *item.Type,
			Status:          *item.Status,
			ErrMsg:          *item.ErrMsg,
			AccountUsername: *item.AccountUsername,
			CreateTime:      *item.CreateTime,
		})
	}

	fileTaskPageVo := vo.FileTaskPageVo{
		BaseVoPage: vo.BaseVoPage{
			PageNum:  *pageNum,
			PageSize: *pageSize,
			Total:    total,
		},
		FileTaskVos: fileTaskVos,
	}
	return &fileTaskPageVo, nil
}

func DeleteFileTaskById(id *uint) error {
	buildDelete, values, err := builder.BuildDelete("file_task", map[string]interface{}{"id": *id})
	if err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}

	if _, err = db.Exec(buildDelete, values...); err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}
	return nil
}

func SelectFileTaskById(id *uint) (*model.FileTask, error) {
	var fileTask model.FileTask

	where := map[string]interface{}{"id": *id}
	selectFields := []string{"id", "name", "path", "`type`", "status", "err_msg", "create_time"}
	buildSelect, values, err := builder.BuildSelect("file_task", where, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	rows, err := db.Query(buildSelect, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()

	if err = scanner.Scan(rows, &fileTask); err == scanner.ErrEmptyResult {
		return nil, errors.New(constant.FileTaskNotExist)
	} else if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	return &fileTask, nil
}

func CreateFileTask(fileTask *model.FileTask) (uint, error) {
	fileTaskCreate := map[string]interface{}{
		"name":   *fileTask.Name,
		"`type`": *fileTask.Type,
	}
	if fileTask.Path != nil && *fileTask.Path != "" {
		fileTaskCreate["path"] = *fileTask.Path
	}
	if fileTask.ErrMsg != nil && *fileTask.ErrMsg != "" {
		fileTaskCreate["err_msg"] = *fileTask.ErrMsg
	}
	if fileTask.Status != nil && *fileTask.Status != 0 {
		fileTaskCreate["status"] = *fileTask.Status
	}
	if fileTask.AccountUsername != nil && *fileTask.AccountUsername != "" {
		fileTaskCreate["account_username"] = *fileTask.AccountUsername
	}
	if fileTask.AccountId != nil && *fileTask.AccountId != 0 {
		fileTaskCreate["account_id"] = *fileTask.AccountId
	}
	var data []map[string]interface{}
	data = append(data, fileTaskCreate)

	buildInsert, values, err := builder.BuildInsert("file_task", data)
	if err != nil {
		logrus.Errorln(err.Error())
		return 0, errors.New(constant.SysError)
	}
	result, err := db.Exec(buildInsert, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return 0, errors.New(constant.SysError)
	}
	id, err := result.LastInsertId()
	if err != nil {
		logrus.Errorln(err.Error())
		return 0, errors.New(constant.SysError)
	}
	return uint(id), nil
}

func UpdateFileTaskById(fileTask *model.FileTask) error {
	where := map[string]interface{}{"id": *fileTask.Id}
	update := map[string]interface{}{}

	if fileTask.Status != nil {
		update["`status`"] = *fileTask.Status
	}

	if fileTask.ErrMsg != nil && *fileTask.ErrMsg != "" {
		update["err_msg"] = *fileTask.ErrMsg
	}

	if len(update) > 0 {
		buildUpdate, values, err := builder.BuildUpdate("file_task", where, update)
		if err != nil {
			logrus.Errorln(err.Error())
			return errors.New(constant.SysError)
		}

		if _, err = db.Exec(buildUpdate, values...); err != nil {
			logrus.Errorln(err.Error())
			return errors.New(constant.SysError)
		}
	}
	return nil
}
