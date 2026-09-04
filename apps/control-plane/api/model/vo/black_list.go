package vo

import "time"

type BlackListVo struct {
	Id         uint      `json:"id"`
	Ip         string    `json:"ip"`
	CreateTime time.Time `json:"createTime"`
}

type BlackListPageVo struct {
	BaseVoPage
	BlackLists []BlackListVo `json:"blackLists"`
}
