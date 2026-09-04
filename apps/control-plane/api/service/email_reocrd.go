package service

import (
	"trojan-panel/dao"
	"trojan-panel/model"
	"trojan-panel/model/vo"
)

func SelectEmailRecordPage(queryToEmail *string, queryState *int, pageNum *uint, pageSize *uint) (*vo.EmailRecordPageVo, error) {
	return dao.SelectEmailRecordPage(queryToEmail, queryState, pageNum, pageSize)
}

func CreateEmailRecord(emailRecord model.EmailRecord) (uint, error) {
	return dao.CreateEmailRecord(emailRecord)
}
