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

func CountBlackListByIp(ip *string) (int, error) {
	var count int

	var where = map[string]interface{}{"ip": *ip}
	selectFields := []string{"count(1)"}
	buildSelect, values, err := builder.BuildSelect("black_list", where, selectFields)
	if err != nil {
		logrus.Errorln(err.Error())
		return 0, errors.New(constant.SysError)
	}

	if err := db.QueryRow(buildSelect, values...).Scan(&count); err != nil {
		logrus.Errorln(err.Error())
		return 0, errors.New(constant.SysError)
	}
	return count, nil
}

func DeleteBlackListByIp(ip *string) error {
	var where = map[string]interface{}{"ip": *ip}

	buildDelete, values, err := builder.BuildDelete("black_list", where)
	if err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}

	if _, err := db.Exec(buildDelete, values...); err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}
	return nil
}

// 批量添加至黑名单
func CreateBlackList(ips []string) error {
	var data []map[string]interface{}
	for _, ip := range ips {
		data = append(data, map[string]interface{}{
			"ip": ip,
		})
	}

	buildInsert, values, err := builder.BuildInsert("black_list", data)
	if err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}

	if _, err := db.Exec(buildInsert, values...); err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}
	return nil
}

func SelectBlackListPage(ip *string, pageNum *uint, pageSize *uint) (*vo.BlackListPageVo, error) {
	var (
		total      uint
		blackLists []model.BlackList
	)

	// 查询总数
	var whereCount = map[string]interface{}{}
	if ip != nil && *ip != "" {
		whereCount["ip like"] = fmt.Sprintf("%%%s%%", *ip)
	}
	selectFieldsCount := []string{"count(1)"}
	buildSelect, values, err := builder.BuildSelect("black_list", whereCount, selectFieldsCount)
	if err := db.QueryRow(buildSelect, values...).Scan(&total); err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	// 分页查询
	offset := (*pageNum - 1) * *pageSize
	where := map[string]interface{}{
		"_orderby": "create_time desc",
		"_limit":   []uint{offset, *pageSize}}
	if ip != nil && *ip != "" {
		where["ip like"] = fmt.Sprintf("%%%s%%", *ip)
	}
	selectFields := []string{"id", "ip", "create_time"}
	selectSQL, values, err := builder.BuildSelect("black_list", where, selectFields)
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

	if err := scanner.Scan(rows, &blackLists); err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	var blackListVos = make([]vo.BlackListVo, 0)
	for _, item := range blackLists {
		blackListVos = append(blackListVos, vo.BlackListVo{
			Id:         *item.Id,
			Ip:         *item.Ip,
			CreateTime: *item.CreateTime,
		})
	}

	blackListPageVo := vo.BlackListPageVo{
		BaseVoPage: vo.BaseVoPage{
			PageNum:  *pageNum,
			PageSize: *pageSize,
			Total:    total,
		},
		BlackLists: blackListVos,
	}
	return &blackListPageVo, nil
}
