package service

import (
	"os"
	"trojan-panel/dao"
	"trojan-panel/dao/redis"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
)

func SelectFileTaskPage(taskType *uint, accountUsername *string, pageNum *uint, pageSize *uint) (*vo.FileTaskPageVo, error) {
	return dao.SelectFileTaskPage(taskType, accountUsername, pageNum, pageSize)
}

func DeleteFileTaskById(id *uint) error {
	mutex, err := redis.RsLock(constant.DeleteFileTaskByIdLock)
	if err != nil {
		return err
	}
	fileTask, err := dao.SelectFileTaskById(id)
	if err != nil {
		return err
	}
	if err = os.Remove(*fileTask.Path); err != nil {
		return err
	}
	if err := dao.DeleteFileTaskById(id); err != nil {
		return err
	}
	redis.RsUnLock(mutex)
	return nil
}

func SelectFileTaskById(id *uint) (*model.FileTask, error) {
	return dao.SelectFileTaskById(id)
}
