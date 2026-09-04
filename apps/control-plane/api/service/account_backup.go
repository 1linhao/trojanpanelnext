package service

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"mime/multipart"
	"time"
	"trojan-panel/dao"
	"trojan-panel/dao/redis"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/util"
)

func ExportAccount(accountId uint, accountUsername string) error {
	accounts, err := dao.SelectAccountAll()
	if err != nil {
		return err
	}
	if err = ExportTaskJson(accountId, accountUsername, constant.TaskTypeAccountExport, "accountExport", accounts); err != nil {
		return err
	}
	return nil
}

func ImportAccount(cover uint, file *multipart.FileHeader, accountId uint, accountUsername string) error {
	fileName := file.Filename

	var fileTaskType uint = constant.TaskTypeAccountImport
	var fileTaskStatus = constant.TaskDoing
	fileTask := model.FileTask{
		Name:            &fileName,
		Path:            nil,
		Type:            &fileTaskType,
		Status:          &fileTaskStatus,
		AccountId:       &accountId,
		AccountUsername: &accountUsername,
	}
	fileTaskId, err := dao.CreateFileTask(&fileTask)
	if err != nil {
		return err
	}

	go func(fileTaskId uint) {
		var fail = constant.TaskFail
		var success = constant.TaskSuccess
		fileTask := model.FileTask{
			Id:     &fileTaskId,
			Status: &fail,
		}

		src, err := file.Open()
		defer src.Close()
		if err != nil {
			logrus.Errorf("ImportAccount file Open err: %v", err)
			fileUploadError := constant.FileUploadError
			fileTask.ErrMsg = &fileUploadError
			if err = dao.UpdateFileTaskById(&fileTask); err != nil {
				logrus.Errorf("ImportAccount UpdateFileTaskById err: %v", err)
			}
			return
		}

		var accounts []model.Account
		decoder := json.NewDecoder(src)
		if err = decoder.Decode(&accounts); err != nil {
			logrus.Errorf("ImportAccount decoder Decode err: %v", err)
			fileUploadError := constant.FileUploadError
			fileTask.ErrMsg = &fileUploadError
			if err = dao.UpdateFileTaskById(&fileTask); err != nil {
				logrus.Errorf("ImportAccount UpdateFileTaskById err: %v", err)
			}
			return
		}
		if len(accounts) == 0 {
			logrus.Errorf("ImportAccount err: %s", constant.RowNotEnough)
			fileUploadError := constant.RowNotEnough
			fileTask.ErrMsg = &fileUploadError
			if err = dao.UpdateFileTaskById(&fileTask); err != nil {
				logrus.Errorf("ImportAccount UpdateFileTaskById err: %v", err)
			}
			return
		}
		// 在这里可以处理数据并将其存储到数据库中 todo 这里可能存在性能问题
		for _, item := range accounts {
			if item.RoleId != nil && *item.RoleId != constant.SYSADMIN {
				if err = dao.CreateOrUpdateAccount(item, cover); err != nil {
					continue
				}
			}
		}

		fileTask.Status = &success
		// 更新文件任务状态
		if err = dao.UpdateFileTaskById(&fileTask); err != nil {
			logrus.Errorf("ImportAccount UpdateFileTaskById err: %v", err)
		}
	}(fileTaskId)
	return nil
}

func ExportAccountUnused(accountId uint, accountUsername string) error {
	accountExportVo, err := dao.SelectAccountUnused()
	if err != nil {
		return err
	}
	if err = ExportTaskJson(accountId, accountUsername, constant.TaskTypeAccountExport, "accountUnusedExport", accountExportVo); err != nil {
		return err
	}
	return nil
}

// ExportTaskJson 导出json文件任务
func ExportTaskJson[T any](accountId uint, accountUsername string, fileTaskType uint, fileName string, data []T) error {
	fileNameRand := fmt.Sprintf("%s-%s.json", fileName, time.Now().Format("20060102150405"))
	filePath := fmt.Sprintf("%s/%s", constant.ExportPath, fileNameRand)

	var fileTaskStatus = constant.TaskDoing
	fileTask := model.FileTask{
		Name:            &fileNameRand,
		Path:            &filePath,
		Type:            &fileTaskType,
		Status:          &fileTaskStatus,
		AccountId:       &accountId,
		AccountUsername: &accountUsername,
	}
	fileTaskId, err := dao.CreateFileTask(&fileTask)
	if err != nil {
		return err
	}

	go func(data []T) {
		mutex, err := redis.RsLock(constant.ExportTaskJsonLock)
		if err != nil {
			return
		}
		var fail = constant.TaskFail
		var success = constant.TaskSuccess
		fileTask := model.FileTask{
			Id:     &fileTaskId,
			Status: &fail,
		}

		if err = util.ExportJson(filePath, data); err != nil {
			logrus.Errorf("ExportJson err: %v", err)
		} else {
			fileTask.Status = &success
		}

		// 更新文件任务状态
		if err = dao.UpdateFileTaskById(&fileTask); err != nil {
			logrus.Errorf("ExportJson UpdateFileTaskById err: %v", err)
		}
		redis.RsUnLock(mutex)
	}(data)

	return nil
}
