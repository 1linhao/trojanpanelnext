package dao

import (
	"errors"
	"fmt"
	"github.com/didi/gendry/builder"
	"github.com/didi/gendry/scanner"
	"github.com/sirupsen/logrus"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
)

func SelectEmailRecordPage(queryToEmail *string, queryState *int, pageNum *uint, pageSize *uint) (*vo.EmailRecordPageVo, error) {
	var (
		total        uint
		emailRecords []model.EmailRecord
	)

	// 查询总数
	var whereCount = map[string]interface{}{}
	if queryToEmail != nil && *queryToEmail != "" {
		whereCount["to_email like"] = fmt.Sprintf("%%%s%%", *queryToEmail)
	}
	if queryState != nil {
		whereCount["state"] = queryState
	}

	selectFieldsCount := []string{"count(1)"}
	buildSelect, values, err := builder.BuildSelect("email_record", whereCount, selectFieldsCount)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	if err := db.QueryRow(buildSelect, values...).Scan(&total); err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	// 分页查询
	offset := (*pageNum - 1) * *pageSize
	where := map[string]interface{}{
		"_orderby": "create_time desc",
		"_limit":   []uint{offset, *pageSize}}
	if queryToEmail != nil && *queryToEmail != "" {
		where["to_email like"] = fmt.Sprintf("%%%s%%", *queryToEmail)
	}
	if queryState != nil {
		where["state"] = queryState
	}
	selectFields := []string{"id", "`to_email`", "subject", "content",
		"state", "create_time"}
	selectSQL, values, err := builder.BuildSelect("email_record", where, selectFields)
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

	if err := scanner.Scan(rows, &emailRecords); err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	var emailRecordVos = make([]vo.EmailRecordVo, 0)
	for _, item := range emailRecords {
		emailRecordVos = append(emailRecordVos, vo.EmailRecordVo{
			Id:         *item.Id,
			ToEmail:    *item.ToEmail,
			Subject:    *item.Subject,
			Content:    *item.Content,
			State:      *item.State,
			CreateTime: *item.CreateTime,
		})
	}

	emailRecordPageVo := vo.EmailRecordPageVo{
		BaseVoPage: vo.BaseVoPage{
			PageNum:  *pageNum,
			PageSize: *pageSize,
			Total:    total,
		},
		EmailRecordVos: emailRecordVos,
	}
	return &emailRecordPageVo, nil
}

// 创建邮件记录并返回主键
func CreateEmailRecord(emailRecord model.EmailRecord) (uint, error) {
	var data []map[string]interface{}
	data = append(data, map[string]interface{}{
		"to_email": *emailRecord.ToEmail,
		"subject":  *emailRecord.Subject,
		"content":  *emailRecord.Content,
		"state":    *emailRecord.State,
	})

	buildInsert, values, err := builder.BuildInsert("email_record", data)
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

func UpdateEmailRecordSateById(id *uint, state *int) error {
	where := map[string]interface{}{"id": *id}
	update := map[string]interface{}{"`state`": *state}

	buildUpdate, values, err := builder.BuildUpdate("email_record", where, update)
	if err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}

	if _, err := db.Exec(buildUpdate, values...); err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}
	return nil
}
