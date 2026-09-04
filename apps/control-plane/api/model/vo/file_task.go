package vo

import "time"

type FileTaskVo struct {
	Id              uint      `json:"id"`
	Name            string    `json:"name"`
	Type            uint      `json:"type"`
	Status          int       `json:"status"`
	ErrMsg          string    `json:"errMsg"`
	AccountUsername string    `json:"accountUsername"`
	CreateTime      time.Time `json:"createTime"`
}

type FileTaskPageVo struct {
	FileTaskVos []FileTaskVo `json:"fileTasks"`
	BaseVoPage
}
