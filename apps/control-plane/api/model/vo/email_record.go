package vo

import "time"

type EmailRecordVo struct {
	Id         uint      `json:"id"`
	ToEmail    string    `json:"toEmail"`
	Subject    string    `json:"subject"`
	Content    string    `json:"content"`
	State      int       `json:"state"`
	CreateTime time.Time `json:"createTime"`
}

type EmailRecordPageVo struct {
	EmailRecordVos []EmailRecordVo `json:"emailRecords"`
	BaseVoPage
}
